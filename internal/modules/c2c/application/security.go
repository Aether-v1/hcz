package application

import (
	"strings"

	c2ccontract "github.com/Aether-v1/hcz/internal/modules/c2c/contract"

	"golang.org/x/crypto/bcrypt"
)

// VerifyStepUp 执行用户端二次验证（Step-Up）：
//   - 用户已开启 2FA（TOTPEnabledAt != nil）→ 校验 TOTP 口令；
//   - 用户未开启 2FA → 校验登录密码（bcrypt）。
//
// 返回 nil 表示通过。本函数及上层一律不得记录 TOTP code / 密码等敏感数据。
//
// Step-Up 子系统未挂载（s.totp == nil，典型为测试夹具）时跳过校验，保持向后兼容；
// 生产环境 s.totp 必然注入，此时强制校验。
func (s *Service) VerifyStepUp(userID uint, totpCode, password string) error {
	if userID == 0 {
		return c2ccontract.ErrStepUpFailed
	}
	if s.totp == nil {
		return nil
	}
	if s.users == nil {
		return c2ccontract.ErrStepUpFailed
	}
	user, err := s.users.GetByID(userID)
	if err != nil || user == nil {
		return c2ccontract.ErrStepUpFailed
	}

	if user.TOTPEnabledAt != nil {
		if strings.TrimSpace(totpCode) == "" {
			return c2ccontract.ErrStepUpFailed
		}
		if err := s.totp.VerifyChallengeCode(userID, strings.TrimSpace(totpCode)); err != nil {
			return c2ccontract.ErrStepUpFailed
		}
		return nil
	}

	// 2FA 未开启：校验登录密码。
	if strings.TrimSpace(password) == "" {
		return c2ccontract.ErrStepUpFailed
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return c2ccontract.ErrStepUpFailed
	}
	return nil
}
