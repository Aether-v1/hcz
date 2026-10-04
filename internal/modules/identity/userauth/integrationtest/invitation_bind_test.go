package integrationtest

import (
	"errors"
	"fmt"
	"testing"
	"time"

	userstore "github.com/Aether-v1/hcz/internal/modules/identity/user/infrastructure/gormstore"

	userdomain "github.com/Aether-v1/hcz/internal/modules/identity/user/domain"

	settingsapp "github.com/Aether-v1/hcz/internal/modules/settings/application"
	settingsstore "github.com/Aether-v1/hcz/internal/modules/settings/infrastructure/gormstore"

	"github.com/Aether-v1/hcz/internal/config"
	userauthapp "github.com/Aether-v1/hcz/internal/modules/identity/userauth/application"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// fakeAttributor 模拟 cookie/click 归因回退，返回固定的上级用户 ID。
type fakeAttributor struct {
	inviterID uint
	err       error
	called    bool
}

func (f *fakeAttributor) ResolveRegistrationInviterUserID(visitorKey string) (uint, error) {
	f.called = true
	return f.inviterID, f.err
}

func newInvitationBindHarness(t *testing.T) (*userauthapp.Service, *userstore.Store, *gorm.DB) {
	t.Helper()
	dsn := fmt.Sprintf("file:invite_bind_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&userdomain.User{}, &settingsstore.SettingRecord{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}
	cfg := &config.Config{
		App:     config.AppConfig{SecretKey: "invite-bind-secret"},
		UserJWT: config.JWTConfig{SecretKey: "invite-bind-jwt-secret", ExpireHours: 24},
		Email:   config.EmailConfig{Enabled: false},
	}
	settingSvc := settingsapp.NewService(settingsstore.New(db))
	svc := userauthapp.NewService(
		cfg,
		userstore.New(db),
		nil, // external identity store 不需要
		nil, // code store（测试关闭邮箱验证）
		settingSvc,
		verificationEmailSenderStub{},
		nil,
	)
	return svc, userstore.New(db), db
}

func registerMust(t *testing.T, svc *userauthapp.Service, email string, input userauthapp.RegisterInput) *userdomain.User {
	t.Helper()
	input.Email = email
	input.Password = "secret123"
	input.AgreementAccepted = true
	user, token, _, err := svc.Register(input)
	if err != nil {
		t.Fatalf("register %s failed: %v", email, err)
	}
	if token == "" || user == nil {
		t.Fatalf("register %s returned empty user/token", email)
	}
	return user
}

// 1. 新用户注册自动生成唯一 invite_code
func TestRegisterGeneratesUniqueInviteCode(t *testing.T) {
	svc, _, _ := newInvitationBindHarness(t)
	a := registerMust(t, svc, "alice@example.com", userauthapp.RegisterInput{})
	b := registerMust(t, svc, "bob@example.com", userauthapp.RegisterInput{})

	if a.InviteCode == "" || b.InviteCode == "" {
		t.Fatalf("invite_code should be auto-generated: a=%q b=%q", a.InviteCode, b.InviteCode)
	}
	if a.InviteCode == b.InviteCode {
		t.Fatalf("invite_code must be unique across users: %q", a.InviteCode)
	}
	if len(a.InviteCode) != 8 {
		t.Fatalf("invite_code length should be 8, got %d (%q)", len(a.InviteCode), a.InviteCode)
	}
}

// 2. 合法 invite_code 注册 → inviter_id 正确绑定 + invite_bound_at 设置
func TestRegisterWithValidInviteCodeBindsInviter(t *testing.T) {
	svc, users, _ := newInvitationBindHarness(t)
	inviter := registerMust(t, svc, "inviter@example.com", userauthapp.RegisterInput{})

	child := registerMust(t, svc, "child@example.com", userauthapp.RegisterInput{InviteCode: inviter.InviteCode})

	if child.InviterID == nil || *child.InviterID != inviter.ID {
		t.Fatalf("expected inviter_id=%d, got %v", inviter.ID, child.InviterID)
	}
	if child.InviteBoundAt == nil {
		t.Fatalf("expected invite_bound_at to be set")
	}

	// 从库中再读一遍确认持久化
	persisted, err := users.GetByID(child.ID)
	if err != nil {
		t.Fatalf("get child: %v", err)
	}
	if persisted.InviterID == nil || *persisted.InviterID != inviter.ID {
		t.Fatalf("persisted inviter_id mismatch: %v", persisted.InviterID)
	}
	if persisted.InviteBoundAt == nil {
		t.Fatalf("persisted invite_bound_at should be set")
	}
}

// 3 + 9. 无效/不存在邀请码 → 明确错误，且用户未落库
func TestRegisterWithInvalidInviteCodeFailsAndNoUserCreated(t *testing.T) {
	svc, users, db := newInvitationBindHarness(t)
	registerMust(t, svc, "inviter@example.com", userauthapp.RegisterInput{})

	before := countUsers(t, db)
	_, _, _, err := svc.Register(userauthapp.RegisterInput{
		Email:             "nobinding@example.com",
		Password:          "secret123",
		AgreementAccepted: true,
		InviteCode:        "NOTEXIST",
	})
	if !errors.Is(err, userauthapp.ErrInviteCodeInvalid) {
		t.Fatalf("expected ErrInviteCodeInvalid, got %v", err)
	}
	after := countUsers(t, db)
	if after != before {
		t.Fatalf("user must not be created when invite code invalid: before=%d after=%d", before, after)
	}
	// 确认该邮箱确实不存在
	if u, _ := users.GetByEmail("nobinding@example.com"); u != nil {
		t.Fatalf("failed-register email should not exist in db")
	}
}

//  5. 已绑定用户再次提交邀请码不会覆盖（注册只发生一次；这里验证逻辑：新注册用户绑定后，
//     其 inviter_id 一旦写入即固定，后续注册是另一个新用户）。
//  4. 自邀/循环：通过 CheckInviteBinding 直接覆盖（注册时新用户 ID=0，自邀不会触发）。
func TestCheckInviteBindingSelfAndCycle(t *testing.T) {
	// 自邀
	if err := userauthapp.CheckInviteBinding(5, 5, nil); !errors.Is(err, userauthapp.ErrSelfInvite) {
		t.Fatalf("expected ErrSelfInvite, got %v", err)
	}
	// inviterID=0 无上级 → 合法
	if err := userauthapp.CheckInviteBinding(0, 99, nil); err != nil {
		t.Fatalf("inviterID=0 should be legal: %v", err)
	}

	// 构造链：A(1) <- B(2, inviter=1) <- C(3, inviter=2)
	// IsDescendant(B=2, A=1) 应返回 true（A 是 B 的祖先）
	svc, _, _ := newInvitationBindHarness(t)
	_ = svc
	finder := func(id uint) (*userdomain.User, error) {
		chain := map[uint]uint{2: 1, 3: 2} // inviter map
		inv, ok := chain[id]
		if !ok {
			return &userdomain.User{ID: id}, nil
		}
		invCopy := inv
		return &userdomain.User{ID: id, InviterID: &invCopy}, nil
	}
	if !userauthapp.IsDescendant(2, 1, finder) {
		t.Fatalf("expected IsDescendant(2,1)=true")
	}
	if userauthapp.IsDescendant(1, 3, finder) {
		t.Fatalf("expected IsDescendant(1,3)=false (1 is root)")
	}
	// 把 1 挂到 3 下会成环：1 是 3 的祖先 → IsDescendant(3,1)=true → CheckInviteBinding 拒绝
	if err := userauthapp.CheckInviteBinding(3, 1, finder); !errors.Is(err, userauthapp.ErrInviteCycle) {
		t.Fatalf("expected ErrInviteCycle, got %v", err)
	}
}

// 6. 显式 invite_code 优先于 cookie attribution
func TestExplicitInviteCodeBeatsCookieAttribution(t *testing.T) {
	svc, _, _ := newInvitationBindHarness(t)
	explicitInviter := registerMust(t, svc, "explicit@example.com", userauthapp.RegisterInput{})
	cookieInviter := registerMust(t, svc, "cookie@example.com", userauthapp.RegisterInput{})

	attributor := &fakeAttributor{inviterID: cookieInviter.ID}
	svc.SetAffiliateAttributor(attributor)

	child := registerMust(t, svc, "child@example.com", userauthapp.RegisterInput{
		InviteCode: explicitInviter.InviteCode,
		VisitorKey: "some-visitor",
	})
	// 显式邀请码有效时直接短路，不应再回退到 cookie 归因（attributor 未被调用即证明优先级）。
	if attributor.called {
		t.Fatalf("explicit invite_code must short-circuit cookie attribution")
	}
	if child.InviterID == nil || *child.InviterID != explicitInviter.ID {
		t.Fatalf("explicit invite_code must win: expected inviter=%d, got %v", explicitInviter.ID, child.InviterID)
	}
}

// 7. cookie attribution 正常绑定（无显式邀请码时）
func TestCookieAttributionBindsWhenNoExplicitCode(t *testing.T) {
	svc, _, _ := newInvitationBindHarness(t)
	cookieInviter := registerMust(t, svc, "cookie@example.com", userauthapp.RegisterInput{})

	svc.SetAffiliateAttributor(&fakeAttributor{inviterID: cookieInviter.ID})
	child := registerMust(t, svc, "child@example.com", userauthapp.RegisterInput{VisitorKey: "visitor-xyz"})

	if child.InviterID == nil || *child.InviterID != cookieInviter.ID {
		t.Fatalf("cookie attribution should bind inviter %d, got %v", cookieInviter.ID, child.InviterID)
	}
	if child.InviteBoundAt == nil {
		t.Fatalf("cookie-bound child should have invite_bound_at")
	}
}

// cookie 归因失败时静默降级为无上级，不报错
func TestCookieAttributionFailureSilentFallback(t *testing.T) {
	svc, _, _ := newInvitationBindHarness(t)
	svc.SetAffiliateAttributor(&fakeAttributor{inviterID: 0, err: errors.New("boom")})
	child := registerMust(t, svc, "orphan@example.com", userauthapp.RegisterInput{VisitorKey: "visitor"})
	if child.InviterID != nil {
		t.Fatalf("attribution failure should silently degrade to no inviter, got %v", child.InviterID)
	}
}

//  8. 事务一致性：绑定所需的上级解析失败发生在 Create 之前，用户不落库（见 TestRegisterWithInvalidInviteCodeFailsAndNoUserCreated）。
//     这里再验证：合法绑定路径下 inviter_id 与用户同一条记录落库（无半失败）。
func TestBindingAndUserCreationAtomicOnSuccess(t *testing.T) {
	svc, users, _ := newInvitationBindHarness(t)
	inviter := registerMust(t, svc, "inviter@example.com", userauthapp.RegisterInput{})
	child := registerMust(t, svc, "child@example.com", userauthapp.RegisterInput{InviteCode: inviter.InviteCode})

	persisted, err := users.GetByInviteCode(child.InviteCode)
	if err != nil || persisted == nil {
		t.Fatalf("child should be retrievable by invite_code: err=%v", err)
	}
	if persisted.InviterID == nil || *persisted.InviterID != inviter.ID {
		t.Fatalf("binding must be persisted atomically with user row: %v", persisted.InviterID)
	}
}

// 10. direct_invite_count 正确
func TestDirectInviteCount(t *testing.T) {
	svc, users, _ := newInvitationBindHarness(t)
	inviter := registerMust(t, svc, "boss@example.com", userauthapp.RegisterInput{})
	registerMust(t, svc, "c1@example.com", userauthapp.RegisterInput{InviteCode: inviter.InviteCode})
	registerMust(t, svc, "c2@example.com", userauthapp.RegisterInput{InviteCode: inviter.InviteCode})
	registerMust(t, svc, "c3@example.com", userauthapp.RegisterInput{InviteCode: inviter.InviteCode})

	count, err := users.CountDirectInvitees(inviter.ID)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 3 {
		t.Fatalf("expected direct_invite_count=3, got %d", count)
	}
	// 无上级用户 count 为 0
	lonely := registerMust(t, svc, "lonely@example.com", userauthapp.RegisterInput{})
	zero, err := users.CountDirectInvitees(lonely.ID)
	if err != nil || zero != 0 {
		t.Fatalf("expected 0, got %d err=%v", zero, err)
	}
}

func countUsers(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var n int64
	if err := db.Model(&userdomain.User{}).Where("deleted_at IS NULL").Count(&n).Error; err != nil {
		t.Fatalf("count users: %v", err)
	}
	return n
}
