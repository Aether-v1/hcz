package stepup

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// fakeStepUpVerifier 内存 fake：claims 由 token 决定，consumed 集合模拟 Redis SETNX。
type fakeStepUpVerifier struct {
	adminID  uint
	jti      string
	scope    string
	expired  bool
	consumed map[string]struct{}
}

func (f *fakeStepUpVerifier) ParseChallengeToken(token string) (Claims, error) {
	if token == "bad-token" {
		return Claims{}, errors.New("signature invalid")
	}
	if f.expired {
		return Claims{}, ErrTokenExpired
	}
	return Claims{AdminID: f.adminID, JTI: f.jti, Scope: f.scope}, nil
}

func (f *fakeStepUpVerifier) ConsumeChallenge(jti string) bool {
	if f.consumed == nil {
		f.consumed = map[string]struct{}{}
	}
	if _, ok := f.consumed[jti]; ok {
		return false
	}
	f.consumed[jti] = struct{}{}
	return true
}

func newStepUpCtx(adminID uint, challengeHeader string) *gin.Context {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("admin_id", adminID)
	c.Request = httptest.NewRequest(http.MethodPost, "/admin/x", nil)
	if challengeHeader != "" {
		c.Request.Header.Set("X-Auth-Challenge", challengeHeader)
	}
	return c
}

func TestRequireForMissingChallenge(t *testing.T) {
	v := &fakeStepUpVerifier{adminID: 1, jti: "j1", scope: "wallet.adjust:user:10"}
	_, err := RequireFor(newStepUpCtx(1, ""), v, "wallet.adjust:user:10")
	if !errors.Is(err, ErrStepUpRequired) {
		t.Fatalf("want ErrStepUpRequired, got %v", err)
	}
}

func TestRequireForBadToken(t *testing.T) {
	v := &fakeStepUpVerifier{adminID: 1, jti: "j1", scope: "wallet.adjust:user:10"}
	_, err := RequireFor(newStepUpCtx(1, "bad-token"), v, "wallet.adjust:user:10")
	if !errors.Is(err, ErrStepUpInvalid) {
		t.Fatalf("want ErrStepUpInvalid, got %v", err)
	}
}

func TestRequireForExpired(t *testing.T) {
	v := &fakeStepUpVerifier{adminID: 1, jti: "j1", scope: "wallet.adjust:user:10", expired: true}
	_, err := RequireFor(newStepUpCtx(1, "tok"), v, "wallet.adjust:user:10")
	if !errors.Is(err, ErrStepUpExpired) {
		t.Fatalf("want ErrStepUpExpired, got %v", err)
	}
}

// 其他 admin 拿同一张 challenge 调用 → 拒绝。
func TestRequireForOtherAdminRejected(t *testing.T) {
	v := &fakeStepUpVerifier{adminID: 1, jti: "j1", scope: "wallet.adjust:user:10"}
	// 当前登录 admin=2，challenge 签发给 admin=1。
	_, err := RequireFor(newStepUpCtx(2, "tok"), v, "wallet.adjust:user:10")
	if !errors.Is(err, ErrStepUpInvalid) {
		t.Fatalf("want ErrStepUpInvalid (other admin), got %v", err)
	}
}

// 跨动作复用：challenge 绑定 wallet.adjust:user:10，拿去调 refund:order:99 → 拒绝。
func TestRequireForCrossOperationRejected(t *testing.T) {
	v := &fakeStepUpVerifier{adminID: 1, jti: "j1", scope: "wallet.adjust:user:10"}
	_, err := RequireFor(newStepUpCtx(1, "tok"), v, "refund.wallet:order:99")
	if !errors.Is(err, ErrStepUpInvalid) {
		t.Fatalf("want ErrStepUpInvalid (cross-op), got %v", err)
	}
}

// 登录 challenge（scope 为空）不得被高风险动作接受。
func TestRequireForLoginChallengeScopeEmptyRejected(t *testing.T) {
	v := &fakeStepUpVerifier{adminID: 1, jti: "j1", scope: ""}
	_, err := RequireFor(newStepUpCtx(1, "tok"), v, "wallet.adjust:user:10")
	if !errors.Is(err, ErrStepUpInvalid) {
		t.Fatalf("want ErrStepUpInvalid (empty scope), got %v", err)
	}
}

// 首次成功；同一张 challenge 第二次（同操作）必须被拒（single-use）。
func TestRequireForSingleUseReplayRejected(t *testing.T) {
	scope := "wallet.adjust:user:10"
	v := &fakeStepUpVerifier{adminID: 1, jti: "j-replay", scope: scope}

	if _, err := RequireFor(newStepUpCtx(1, "tok"), v, scope); err != nil {
		t.Fatalf("first use should succeed, got %v", err)
	}
	// 第二次：同 admin 同操作同 challenge。
	_, err := RequireFor(newStepUpCtx(1, "tok"), v, scope)
	if !errors.Is(err, ErrStepUpReplayed) {
		t.Fatalf("want ErrStepUpReplayed on second use, got %v", err)
	}
}

// verifier 未配置 → fail-closed。
func TestRequireForNotConfiguredFailClosed(t *testing.T) {
	_, err := RequireFor(newStepUpCtx(1, "tok"), nil, "wallet.adjust:user:10")
	if !errors.Is(err, ErrStepUpNotConfigured) {
		t.Fatalf("want ErrStepUpNotConfigured, got %v", err)
	}
}

func TestScopeFormat(t *testing.T) {
	if got := Scope("wallet.adjust", "user", 123); got != "wallet.adjust:user:123" {
		t.Fatalf("unexpected scope: %s", got)
	}
}
