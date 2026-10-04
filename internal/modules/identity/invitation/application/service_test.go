package application

import (
	"testing"
	"time"

	userdomain "github.com/Aether-v1/hcz/internal/modules/identity/user/domain"
	userstore "github.com/Aether-v1/hcz/internal/modules/identity/user/infrastructure/gormstore"
	settingsapp "github.com/Aether-v1/hcz/internal/modules/settings/application"
	settingsstore "github.com/Aether-v1/hcz/internal/modules/settings/infrastructure/gormstore"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newInvitationServiceHarness(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()
	dsn := "file:invitation_me_" + time.Now().Format("150405.000000000") + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&userdomain.User{}, &settingsstore.SettingRecord{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	users := userstore.New(db)
	settings := settingsapp.NewService(settingsstore.New(db))
	return NewService(users, settings), db
}

func seedUser(t *testing.T, db *gorm.DB, email, displayName, inviteCode string, inviterID *uint, boundAt *time.Time) *userdomain.User {
	t.Helper()
	u := &userdomain.User{
		Email:         email,
		DisplayName:   displayName,
		InviteCode:    inviteCode,
		InviterID:     inviterID,
		InviteBoundAt: boundAt,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	if err := db.Create(u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return u
}

// 11. GET /invitation/me 返回正确字段（含上级与无上级两种情况）
func TestGetMyInvitationFields(t *testing.T) {
	svc, db := newInvitationServiceHarness(t)

	// 上级（无上级）
	inviter := seedUser(t, db, "boss@example.com", "BossName", "BOSSCODE", nil, nil)
	// 下级（绑定了 inviter）
	now := time.Now()
	child := seedUser(t, db, "child@example.com", "", "CHILDCODE", &inviter.ID, &now)

	// case A: 有上级
	got, err := svc.GetMyInvitation(child.ID)
	if err != nil {
		t.Fatalf("get my invitation: %v", err)
	}
	if got.InviteCode != "CHILDCODE" {
		t.Fatalf("invite_code mismatch: %q", got.InviteCode)
	}
	if got.InviterCode == nil || *got.InviterCode != "BOSSCODE" {
		t.Fatalf("inviter_code mismatch: %v", got.InviterCode)
	}
	if got.InviterDisplayName == nil || *got.InviterDisplayName != "BossName" {
		t.Fatalf("inviter_display_name mismatch: %v", got.InviterDisplayName)
	}
	if got.InviteBoundAt == nil {
		t.Fatalf("invite_bound_at should be set")
	}
	if got.InviteURL != "/auth/register?invite=CHILDCODE" {
		t.Fatalf("invite_url mismatch (no site url configured): %q", got.InviteURL)
	}

	// case B: 无上级 → inviter 字段为 null
	top, err := svc.GetMyInvitation(inviter.ID)
	if err != nil {
		t.Fatalf("get inviter: %v", err)
	}
	if top.InviterCode != nil || top.InviterDisplayName != nil || top.InviteBoundAt != nil {
		t.Fatalf("inviter should have no binding: %+v", top)
	}
}

// 10b. direct_invite_count 在 /me 返回正确
func TestGetMyInvitationDirectCount(t *testing.T) {
	svc, db := newInvitationServiceHarness(t)
	boss := seedUser(t, db, "boss@example.com", "Boss", "BOSSCODE", nil, nil)
	seedUser(t, db, "c1@example.com", "", "C1", &boss.ID, nil)
	seedUser(t, db, "c2@example.com", "", "C2", &boss.ID, nil)

	got, err := svc.GetMyInvitation(boss.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.DirectInviteCount != 2 {
		t.Fatalf("direct_invite_count expected 2, got %d", got.DirectInviteCount)
	}
}
