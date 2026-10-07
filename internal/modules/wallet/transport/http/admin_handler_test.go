package wallethttp

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Aether-v1/hcz/internal/platform/http/stepup"
	"github.com/gin-gonic/gin"
)

// testStepUpVerifier 放行 admin_id=1 的挑战；scope 取自 header 本身，
// 与 handler 期望的 scope（wallet.adjust:user:<id>）对齐；consume 恒成功（无状态）。
type testStepUpVerifier struct{}

func (testStepUpVerifier) ParseChallengeToken(token string) (stepup.Claims, error) {
	return stepup.Claims{AdminID: 1, JTI: "stub-jti", Scope: token}, nil
}

func (testStepUpVerifier) ConsumeChallenge(string) bool { return true }

func TestAdjustUserWalletRequiresRemarkWithDedicatedMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("admin_id", uint(1))
	ctx.Params = gin.Params{{Key: "id", Value: "10"}}
	ctx.Request = httptest.NewRequest(
		http.MethodPost,
		"/api/v1/admin/users/10/wallet/adjust",
		strings.NewReader(`{"operation":"add","amount":"10.00","remark":"   "}`),
	)
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Request.Header.Set("Accept-Language", "en-US")
	ctx.Request.Header.Set("X-Auth-Challenge", stepup.Scope("wallet.adjust", "user", 10))
	ctx.Request.Header.Set("Idempotency-Key", "test-idem-key")

	(&AdminHandler{challenge: testStepUpVerifier{}}).AdjustUserWallet(ctx)

	if !strings.Contains(recorder.Body.String(), "A remark is required for wallet balance adjustments") {
		t.Fatalf("expected dedicated remark validation message, got %s", recorder.Body.String())
	}
}
