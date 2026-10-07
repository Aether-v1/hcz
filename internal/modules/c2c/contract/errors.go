package contract

import "errors"

// C2C 领域错误。handler 层据此映射 HTTP 状态码。
var (
	ErrListingNotFound       = errors.New("c2c listing not found")
	ErrTradeNotFound         = errors.New("c2c trade not found")
	ErrPaymentMethodNotFound = errors.New("c2c payment method not found")
	ErrInsufficientBalance   = errors.New("c2c insufficient balance")
	ErrSelfTrade             = errors.New("c2c self trade is not allowed")
	ErrListingNotActive      = errors.New("c2c listing is not active")
	ErrAmountOutOfRange      = errors.New("c2c amount out of range")
	ErrTradeStatusInvalid    = errors.New("c2c trade status invalid")
	ErrIdempotencyConflict   = errors.New("c2c idempotency key conflict")
	ErrC2CDisabled           = errors.New("c2c trading is disabled")
	ErrTOTPRequired          = errors.New("c2c totp required")
	ErrNewUserCooldown       = errors.New("c2c new user cooldown not elapsed")
	ErrNoPaymentMethod       = errors.New("c2c no enabled payment method")
	ErrDailyLimitExceeded    = errors.New("c2c daily trade limit exceeded")
	ErrInvalidPaymentType    = errors.New("c2c invalid payment method type")
	ErrPermissionDenied      = errors.New("c2c permission denied")
	ErrInvalidAmount         = errors.New("c2c invalid amount")
	ErrUserBanned            = errors.New("c2c user is banned")
	ErrUserInactive          = errors.New("c2c user account is not active")

	// 争议 / 仲裁
	ErrDisputeNotFound           = errors.New("c2c dispute not found")
	ErrDisputeAlreadyOpen        = errors.New("c2c trade already has an open dispute")
	ErrDisputeAlreadyResolved    = errors.New("c2c dispute already resolved")
	ErrInvalidArbitrationResult  = errors.New("c2c invalid arbitration result")
	ErrArbitrationReasonRequired = errors.New("c2c arbitration reason is required")
	ErrChallengeRequired         = errors.New("c2c step-up challenge token is required")
	ErrChallengeInvalid          = errors.New("c2c step-up challenge token is invalid")
	ErrListingNotClosable        = errors.New("c2c listing cannot be closed")

	// 收款方式 / Step-Up
	ErrStepUpFailed            = errors.New("c2c step-up verification failed")
	ErrInvalidUSDTAddress      = errors.New("c2c invalid usdt trc20 address")
	ErrPaymentMethodLimit      = errors.New("c2c payment method per-type limit exceeded")
	ErrInvalidBankAccount      = errors.New("c2c invalid bank account number")
	ErrMissingRequiredField    = errors.New("c2c payment method missing required field")
	ErrInvalidPaymentMethodType = errors.New("c2c payment method type mismatch")
)
