package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Aether-v1/hcz/internal/app/httpserver/middleware"
	userdomain "github.com/Aether-v1/hcz/internal/modules/identity/user/domain"
	userauthapp "github.com/Aether-v1/hcz/internal/modules/identity/userauth/application"
	userauthtransport "github.com/Aether-v1/hcz/internal/modules/identity/userauth/transport/http"
	"github.com/Aether-v1/hcz/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

// ---- fakes for the userauth transport ports ----

type p1LoginSettings struct {
	registrationEnabled bool
	emailVerified       bool
}

func (f p1LoginSettings) GetRegistrationEnabled(bool) (bool, error) {
	return f.registrationEnabled, nil
}
func (f p1LoginSettings) GetEmailVerificationEnabled(bool) (bool, error) { return f.emailVerified, nil }

type p1LoginAuth struct{ registerErr error }

func (f p1LoginAuth) Register(input userauthapp.RegisterInput) (*userdomain.User, string, time.Time, error) {
	return nil, "", time.Time{}, f.registerErr
}

func (f p1LoginAuth) LoginStep1(email, password string, rememberMe bool) (*userauthtransport.AuthLoginResult, error) {
	return nil, nil
}

type p1VerifySettings struct {
	emailVerified       bool
	registrationEnabled bool
}

func (f p1VerifySettings) GetEmailVerificationEnabled(bool) (bool, error) {
	return f.emailVerified, nil
}
func (f p1VerifySettings) GetRegistrationEnabled(bool) (bool, error) {
	return f.registrationEnabled, nil
}

type p1VerifyAuth struct{ sendErrByEmail map[string]error }

func (f p1VerifyAuth) SendVerifyCode(_ context.Context, email, purpose, locale string) error {
	if err, ok := f.sendErrByEmail[email]; ok {
		return err
	}
	return nil
}

type p1PasswordService struct{ resetErrByEmail map[string]error }

func (f p1PasswordService) GetEmailVerificationEnabled(bool) (bool, error) { return true, nil }
func (f p1PasswordService) ResetPassword(email, code, newPassword string) error {
	if err, ok := f.resetErrByEmail[email]; ok {
		return err
	}
	return nil
}
func (f p1PasswordService) ChangePassword(_ uint, _, _ string) error { return nil }

// ---- helpers ----

func p1Post(t *testing.T, r *gin.Engine, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "203.0.113.7:4567"
	r.ServeHTTP(w, req)
	return w
}

func p1StatusCode(t *testing.T, w *httptest.ResponseRecorder) int {
	t.Helper()
	var env struct {
		StatusCode int `json:"status_code"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal body %q: %v", w.Body.String(), err)
	}
	return env.StatusCode
}

// p1StrictLimit 返回一个 MaxRequests=1 的本地限流中间件（无 Redis，进程内计数）。
func p1StrictLimit(prefix string) gin.HandlerFunc {
	return middleware.RateLimitMiddleware(nil, middleware.RateLimitRule{
		Prefix:        prefix,
		WindowSeconds: 60,
		MaxRequests:   1,
		BlockSeconds:  60,
		MessageKey:    "error.rate_limited",
	}, middleware.KeyByIPAndJSONField("email"))
}

// ---- route mounting integration tests: 超频必须 429 ----

// TestP1_2RegisterRouteRateLimitedConfirmsMiddlewareMounted 验证注册路由确实在 handler 前挂了限流。
func TestP1_2RegisterRouteRateLimitedConfirmsMiddlewareMounted(t *testing.T) {
	gin.SetMode(gin.TestMode)
	loginHandler := userauthtransport.NewUserLoginHandler(
		p1LoginSettings{registrationEnabled: true, emailVerified: true},
		p1LoginAuth{registerErr: userauthtransport.ErrInvalidEmail},
		nil, nil,
	)
	r := gin.New()
	auth := r.Group("/auth")
	userauthtransport.RegisterUserRegisterAuthRoutes(auth, loginHandler, p1StrictLimit("test:rate:register"))

	body := `{"email":"ratelimit@example.com","password":"Str0ng#Pass","agreement_accepted":true}`
	first := p1Post(t, r, "/auth/register", body)
	if first.Code == http.StatusTooManyRequests {
		t.Fatalf("first register request must pass rate limit (normal frequency), got 429. body=%s", first.Body.String())
	}
	second := p1Post(t, r, "/auth/register", body)
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("second register request must be 429 (registerRule mounted), got %d. body=%s", second.Code, second.Body.String())
	}
}

// TestP1_2VerifyRouteRateLimitedConfirmsMiddlewareMounted 验证发送验证码路由挂了限流。
func TestP1_2VerifyRouteRateLimitedConfirmsMiddlewareMounted(t *testing.T) {
	gin.SetMode(gin.TestMode)
	verifyHandler := userauthtransport.NewUserVerifyHandler(
		p1VerifySettings{emailVerified: true, registrationEnabled: true},
		nil,
		p1VerifyAuth{},
	)
	r := gin.New()
	auth := r.Group("/auth")
	userauthtransport.RegisterUserVerifyAuthRoutes(auth, verifyHandler, p1StrictLimit("test:rate:verify"))

	body := `{"email":"ratelimit@example.com","purpose":"register"}`
	first := p1Post(t, r, "/auth/send-verify-code", body)
	if first.Code == http.StatusTooManyRequests {
		t.Fatalf("first verify request must pass rate limit, got 429. body=%s", first.Body.String())
	}
	second := p1Post(t, r, "/auth/send-verify-code", body)
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("second verify request must be 429 (verifyRule mounted), got %d. body=%s", second.Code, second.Body.String())
	}
}

// TestP1_2ForgotRouteRateLimitedConfirmsMiddlewareMounted 验证忘记密码路由挂了限流。
func TestP1_2ForgotRouteRateLimitedConfirmsMiddlewareMounted(t *testing.T) {
	gin.SetMode(gin.TestMode)
	passwordHandler := userauthtransport.NewUserPasswordHandler(p1PasswordService{})
	r := gin.New()
	auth := r.Group("/auth")
	userauthtransport.RegisterUserPasswordAuthRoutes(auth, passwordHandler, p1StrictLimit("test:rate:forgot"))

	body := `{"email":"ratelimit@example.com","code":"123456","new_password":"Str0ng#Pass"}`
	first := p1Post(t, r, "/auth/forgot-password", body)
	if first.Code == http.StatusTooManyRequests {
		t.Fatalf("first forgot-password request must pass rate limit, got 429. body=%s", first.Body.String())
	}
	second := p1Post(t, r, "/auth/forgot-password", body)
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("second forgot-password request must be 429 (forgotRule mounted), got %d. body=%s", second.Code, second.Body.String())
	}
}

// TestP1_2NormalFrequencyNotBlocked 验证正常频率（窗口内 1 次）不会被误伤。
func TestP1_2NormalFrequencyNotBlocked(t *testing.T) {
	gin.SetMode(gin.TestMode)
	loginHandler := userauthtransport.NewUserLoginHandler(
		p1LoginSettings{registrationEnabled: true, emailVerified: true},
		p1LoginAuth{registerErr: userauthtransport.ErrInvalidEmail},
		nil, nil,
	)
	r := gin.New()
	auth := r.Group("/auth")
	// 宽松规则：窗口内 5 次
	auth.POST("/register", middleware.RateLimitMiddleware(nil, middleware.RateLimitRule{
		Prefix: "test:rate:register:loose", WindowSeconds: 60, MaxRequests: 5, BlockSeconds: 60,
	}, middleware.KeyByIPAndJSONField("email")), loginHandler.UserRegister)

	body := `{"email":"normal@example.com","password":"Str0ng#Pass","agreement_accepted":true}`
	w := p1Post(t, r, "/auth/register", body)
	if w.Code == http.StatusTooManyRequests {
		t.Fatalf("single request within limit must not be 429, got 429. body=%s", w.Body.String())
	}
}

// ---- account-existence enumeration consistency ----

// TestP1_2ForgotPasswordDoesNotLeakAccountExistence 验证忘记密码提交端点对
// 「邮箱未注册」与「验证码错误」返回完全相同的业务码，攻击者无法区分账号是否存在。
func TestP1_2ForgotPasswordDoesNotLeakAccountExistence(t *testing.T) {
	gin.SetMode(gin.TestMode)
	passwordHandler := userauthtransport.NewUserPasswordHandler(p1PasswordService{
		resetErrByEmail: map[string]error{
			"unknown@nowhere.com": userauthtransport.ErrUserNotFound,
			"known@nowhere.com":   userauthtransport.ErrVerifyCodeInvalid,
		},
	})
	r := gin.New()
	// 不限流，隔离 handler 响应映射
	r.POST("/auth/forgot-password", passwordHandler.UserForgotPassword)

	unknown := p1Post(t, r, "/auth/forgot-password",
		`{"email":"unknown@nowhere.com","code":"000000","new_password":"Str0ng#Pass"}`)
	known := p1Post(t, r, "/auth/forgot-password",
		`{"email":"known@nowhere.com","code":"000000","new_password":"Str0ng#Pass"}`)

	unknownCode := p1StatusCode(t, unknown)
	knownCode := p1StatusCode(t, known)
	if unknownCode != knownCode {
		t.Fatalf("forgot-password must not distinguish unknown vs known email: unknown=%d known=%d (leaks account existence)", unknownCode, knownCode)
	}
	if unknownCode == response.CodeNotFound {
		t.Fatalf("unknown email must no longer return 404 user_not_found (was %d)", unknownCode)
	}
}

// TestP1_2SendVerifyResetDoesNotLeakAccountExistence 验证发送重置验证码时，
// 对未注册邮箱统一返回「已发送」成功响应，与成功路径一致，不泄露账号是否存在。
func TestP1_2SendVerifyResetDoesNotLeakAccountExistence(t *testing.T) {
	gin.SetMode(gin.TestMode)
	verifyHandler := userauthtransport.NewUserVerifyHandler(
		p1VerifySettings{emailVerified: true, registrationEnabled: true},
		nil,
		p1VerifyAuth{
			sendErrByEmail: map[string]error{
				"unknown@nowhere.com": userauthtransport.ErrUserNotFound,
			},
		},
	)
	r := gin.New()
	r.POST("/auth/send-verify-code", verifyHandler.SendUserVerifyCode)

	unknown := p1Post(t, r, "/auth/send-verify-code",
		`{"email":"unknown@nowhere.com","purpose":"reset"}`)
	known := p1Post(t, r, "/auth/send-verify-code",
		`{"email":"known@nowhere.com","purpose":"reset"}`)

	unknownCode := p1StatusCode(t, unknown)
	knownCode := p1StatusCode(t, known)
	if unknownCode != response.CodeOK {
		t.Fatalf("reset verify-code for unregistered email must return success (enumeration-safe), got status_code=%d body=%s", unknownCode, unknown.Body.String())
	}
	if knownCode != response.CodeOK {
		t.Fatalf("reset verify-code for registered email must return success, got status_code=%d body=%s", knownCode, known.Body.String())
	}
	if unknownCode != knownCode {
		t.Fatalf("reset verify-code must return identical response for unknown vs known email: unknown=%d known=%d", unknownCode, knownCode)
	}
}
