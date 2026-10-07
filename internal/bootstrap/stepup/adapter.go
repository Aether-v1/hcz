// Package stepup 提供 adminauth.Service 到 stepup.Verifier 的共享适配，
// 供钱包/提现/订单退款等高风险 handler 装配复用。
package stepup

import (
	"context"
	"errors"
	"fmt"

	"github.com/Aether-v1/hcz/internal/cache"
	adminauthapp "github.com/Aether-v1/hcz/internal/modules/identity/adminauth/application"
	adminchallenge "github.com/Aether-v1/hcz/internal/modules/identity/adminauth/challenge"
	"github.com/Aether-v1/hcz/internal/platform/http/stepup"

	"github.com/golang-jwt/jwt/v5"
)

// Verifier 适配 adminauthapp.Service 到 stepup.Verifier 端口。
type Verifier struct {
	auth *adminauthapp.Service
}

// ParseChallengeToken 校验签名/用途/有效期，返回绑定的 admin_id/jti/scope。
func (v Verifier) ParseChallengeToken(token string) (stepup.Claims, error) {
	if v.auth == nil {
		return stepup.Claims{}, fmt.Errorf("challenge verifier unavailable")
	}
	claims, err := v.auth.ParseChallengeToken(token)
	if err != nil {
		// 区分"已过期"与"无效"，供 handler 返回 STEP_UP_EXPIRED。
		if errors.Is(err, jwt.ErrTokenExpired) {
			return stepup.Claims{}, fmt.Errorf("%w: %v", stepup.ErrTokenExpired, err)
		}
		return stepup.Claims{}, err
	}
	return stepup.Claims{AdminID: claims.AdminID, JTI: claims.JTI, Scope: claims.Scope}, nil
}

// ConsumeChallenge 原子地单次消费 jti：SETNX revoked key。
// 第一次调用者获胜（true）；已消费过返回 false；Redis 不可用时 fail-closed 返回 false。
// key TTL 与 challenge TTL 一致，过期后自动清理。
func (v Verifier) ConsumeChallenge(jti string) bool {
	rdb := cache.Client()
	if rdb == nil {
		return false
	}
	ok, err := rdb.SetNX(
		context.Background(),
		adminchallenge.RevocationKey(jti),
		"1",
		adminchallenge.TTL,
	).Result()
	if err != nil {
		return false
	}
	return ok
}

// NewVerifier 用容器内的 adminauth.Service 构造 stepup.Verifier。
// authService 为 nil 时返回 nil（handler 侧会 fail-closed）。
func NewVerifier(authService *adminauthapp.Service) stepup.Verifier {
	if authService == nil {
		return nil
	}
	return Verifier{auth: authService}
}
