// Package c2c 装配 C2C 后台管理 Handler：注入 Step-Up challenge 校验器。
package c2c

import (
	"fmt"

	"github.com/Aether-v1/hcz/internal/app/container"
	c2chttp "github.com/Aether-v1/hcz/internal/modules/c2c/transport/http"
	adminauthapp "github.com/Aether-v1/hcz/internal/modules/identity/adminauth/application"
)

// adminChallengeVerifier 适配 adminauthapp.Service 到 C2C 后台 Step-Up 校验端口。
type adminChallengeVerifier struct {
	auth *adminauthapp.Service
}

func (v adminChallengeVerifier) ParseChallengeToken(token string) (uint, string, error) {
	if v.auth == nil {
		return 0, "", fmt.Errorf("challenge verifier unavailable")
	}
	claims, err := v.auth.ParseChallengeToken(token)
	if err != nil {
		return 0, "", err
	}
	return claims.AdminID, claims.JTI, nil
}

// NewAdminHandler 构造 C2C 后台管理 Handler。
func NewAdminHandler(c *container.Container) *c2chttp.AdminHandler {
	var verifier c2chttp.ChallengeVerifier
	if c.AuthService != nil {
		verifier = adminChallengeVerifier{auth: c.AuthService}
	}
	return c2chttp.NewAdminHandler(c.C2CService, verifier, c.SettingService)
}
