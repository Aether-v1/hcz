// Package stepup 提供高风险管理员动作的 Step-Up（2FA 二次验证）强制校验。
//
// 复用现有 adminauth 挑战 token（JWT, Purpose="2fa_challenge"），不在此包内
// 重新签发或定义 purpose。各高风险 handler 在执行业务前调用 RequireFor。
//
// 安全模型（本轮收口）：
//   - 绑定：challenge 的 scope claim 必须等于本次动作的期望 scope
//     （action:resourceKind:resourceID，例如 wallet.adjust:user:123），
//     防止一个 wallet adjust challenge 被拿去调用 refund/withdraw。
//   - 单次：校验通过后立即原子消费 jti（Redis SETNX revoked 标记），
//     同一 challenge 第二次使用一律拒绝（STEP_UP_REPLAYED）。
//   - 登录 challenge（scope 为空）不被任何高风险动作接受。
package stepup

import (
	"errors"
	"strconv"
	"strings"

	"github.com/Aether-v1/hcz/internal/platform/http/ginutil"
	"github.com/Aether-v1/hcz/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

// Claims 是 Step-Up challenge 校验通过后的可信视图。
type Claims struct {
	AdminID uint
	JTI     string
	// Scope 绑定的动作域，例如 "wallet.adjust:user:123"。登录 challenge 为空。
	Scope string
}

// Verifier 解析并消费 Step-Up 挑战 token。
// 由 adminauth.Service 适配实现（见 bootstrap/stepup）。
type Verifier interface {
	// ParseChallengeToken 校验签名/用途/有效期，返回 claims。
	// 过期 token 返回 jwt.ErrTokenExpired（调用方映射为 STEP_UP_EXPIRED）。
	ParseChallengeToken(token string) (Claims, error)
	// ConsumeChallenge 原子地单次消费 jti。返回 true 表示本次调用抢先成功（可继续）；
	// false 表示该 jti 已被消费过（重放）或撤销存储不可用（fail-closed）。
	ConsumeChallenge(jti string) bool
}

// Step-Up 相关错误。handler 将其映射为统一业务响应码。
var (
	// ErrStepUpRequired 缺少或为空的挑战 token。
	ErrStepUpRequired = errors.New("step-up challenge required")
	// ErrStepUpInvalid 挑战 token 无效、与当前登录管理员不匹配或 scope 不匹配。
	ErrStepUpInvalid = errors.New("step-up challenge invalid")
	// ErrStepUpNotConfigured 服务端未装配挑战校验器（配置错误，应 fail-closed）。
	ErrStepUpNotConfigured = errors.New("step-up verifier not configured")
	// ErrStepUpExpired 挑战 token 已过有效期。
	ErrStepUpExpired = errors.New("step-up challenge expired")
	// ErrStepUpReplayed 挑战 token 已被消费过（单次使用，禁止重放）。
	ErrStepUpReplayed = errors.New("step-up challenge replayed")
)

// ErrTokenExpired 是 Verifier 实现应包装在底层 JWT 过期错误之上的哨兵，
// 供 RequireFor 区分"已过期"与"无效"。
var ErrTokenExpired = errors.New("step-up challenge token expired")

// Scope 构造动作绑定 scope："wallet.adjust:user:123"。
func Scope(action, resourceKind string, resourceID uint) string {
	return action + ":" + resourceKind + ":" + strconv.FormatUint(uint64(resourceID), 10)
}

// RequireFor 强制当前请求携带有效、未消费、且绑定到本次动作的 Step-Up 挑战：
//
//  1. 当前 JWT 管理员已登录；
//  2. verifier 已配置（否则 fail-closed）；
//  3. 请求携带 X-Auth-Challenge；
//  4. token 签名/用途/有效期合法；
//  5. token 内 admin_id 与当前登录管理员一致；
//  6. token 的 scope claim 与 expectedScope 完全一致（防止跨动作复用）；
//  7. 原子消费 jti 成功（防止重放）。
//
// 成功时返回当前 adminID；失败返回非 nil error，调用方应直接返回、不再执行业务。
// 本函数不写响应，响应由 RespondError 统一处理，避免重复写。
func RequireFor(c *gin.Context, verifier Verifier, expectedScope string) (uint, error) {
	adminID, ok := currentAdminID(c)
	if !ok {
		return 0, ErrStepUpRequired
	}
	if verifier == nil {
		return 0, ErrStepUpNotConfigured
	}

	token := strings.TrimSpace(c.GetHeader("X-Auth-Challenge"))
	if token == "" {
		// 兼容旧 header 命名（C2C 仲裁曾用此名）。
		token = strings.TrimSpace(c.GetHeader("X-Auth-Challenge-Token"))
	}
	if token == "" {
		return 0, ErrStepUpRequired
	}

	claims, err := verifier.ParseChallengeToken(token)
	if err != nil {
		if errors.Is(err, ErrTokenExpired) {
			return 0, ErrStepUpExpired
		}
		return 0, ErrStepUpInvalid
	}
	if claims.AdminID != adminID {
		return 0, ErrStepUpInvalid
	}
	// 动作绑定：期望 scope 非空，且必须与 token 内 scope 完全一致。
	// 登录 challenge scope 为空 → 天然被高风险动作拒绝。
	if expectedScope == "" || claims.Scope != expectedScope {
		return 0, ErrStepUpInvalid
	}
	// 单次使用：原子消费。失败 = 已被消费过（重放）或撤销存储不可用。
	if !verifier.ConsumeChallenge(claims.JTI) {
		return 0, ErrStepUpReplayed
	}
	return adminID, nil
}

// RespondError 将 RequireFor 返回的 step-up 错误映射为统一业务响应。
func RespondError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrStepUpNotConfigured):
		ginutil.RespondError(c, response.CodeInternal, "error.step_up_not_configured", nil)
	case errors.Is(err, ErrStepUpRequired):
		ginutil.RespondError(c, response.CodeUnauthorized, "error.step_up_required", nil)
	case errors.Is(err, ErrStepUpExpired):
		ginutil.RespondError(c, response.CodeUnauthorized, "error.step_up_expired", nil)
	case errors.Is(err, ErrStepUpReplayed):
		ginutil.RespondError(c, response.CodeUnauthorized, "error.step_up_replayed", nil)
	case errors.Is(err, ErrStepUpInvalid):
		ginutil.RespondError(c, response.CodeUnauthorized, "error.step_up_invalid", nil)
	default:
		ginutil.RespondError(c, response.CodeInternal, "error.internal", err)
	}
}

// currentAdminID 从 gin 上下文读取 JWT 中间件注入的 admin_id，不产生任何响应副作用。
func currentAdminID(c *gin.Context) (uint, bool) {
	value, exists := c.Get("admin_id")
	if !exists {
		return 0, false
	}
	switch v := value.(type) {
	case uint:
		return v, true
	case int:
		if v < 0 {
			return 0, false
		}
		return uint(v), true
	case float64:
		if v < 0 {
			return 0, false
		}
		return uint(v), true
	default:
		return 0, false
	}
}
