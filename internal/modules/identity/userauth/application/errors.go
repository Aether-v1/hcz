package application

import "errors"

var (
	ErrNotFound                     = errors.New("user not found")
	ErrInvalidCredentials           = errors.New("invalid credentials")
	ErrInvalidPassword              = errors.New("invalid password")
	ErrEmailExists                  = errors.New("email exists")
	ErrEmailNotVerified             = errors.New("email not verified")
	ErrUserDisabled                 = errors.New("user disabled")
	ErrInvalidEmail                 = errors.New("invalid email")
	ErrInvalidVerifyPurpose         = errors.New("invalid verify purpose")
	ErrAgreementRequired            = errors.New("agreement required")
	ErrVerifyCodeInvalid            = errors.New("verify code invalid")
	ErrVerifyCodeExpired            = errors.New("verify code expired")
	ErrVerifyCodeTooFrequent        = errors.New("verify code too frequent")
	ErrVerifyCodeAttemptsExceeded   = errors.New("verify code attempts exceeded")
	ErrEmailServiceNotConfigured    = errors.New("email service not configured")
	ErrUserOAuthIdentityExists      = errors.New("user oauth identity exists")
	ErrUserOAuthAlreadyBound        = errors.New("user oauth already bound")
	ErrUserOAuthNotBound            = errors.New("user oauth not bound")
	ErrTelegramUnbindRequiresEmail  = errors.New("telegram unbind requires real email")
	ErrGoogleAutoLinkForbidden      = errors.New("google email auto link forbidden")
	ErrGoogleUnbindLocked           = errors.New("google unbind would lock account")
	ErrGoogleRedirectUnavailable    = errors.New("google redirect state store unavailable")
	ErrGoogleRedirectSessionExpired = errors.New("google redirect session expired")
	ErrGoogleRedirectTenantMismatch = errors.New("google redirect tenant mismatch")
	ErrGoogleRedirectUserMismatch   = errors.New("google redirect user mismatch")
	ErrGoogleRedirectFlowInvalid    = errors.New("google redirect flow invalid")
	ErrProfileEmpty                 = errors.New("profile empty")
	ErrEmailChangeInvalid           = errors.New("email change invalid")
	ErrEmailChangeExists            = errors.New("email change exists")
	ErrRegistrationDisabled         = errors.New("registration disabled")

	// 邀请绑定（Phase 2）
	ErrInviteCodeInvalid = errors.New("invite code invalid") // 邀请码不存在/无效
	ErrSelfInvite        = errors.New("self invite")         // 不能绑定自己为上级
	ErrInviteCycle       = errors.New("invite cycle")        // 会形成上下级环
)

var errExternalIdentityUnbindLocked = errors.New("external identity unbind would lock account")
var errGoogleLoginMappingChanged = errors.New("google identity mapping changed during login")
