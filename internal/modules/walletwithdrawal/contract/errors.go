package contract

import "errors"

// 提现领域错误。handler 层据此映射 HTTP 状态码。
var (
	ErrWithdrawalDisabled      = errors.New("withdrawal disabled")
	ErrWithdrawalNotFound      = errors.New("withdrawal not found")
	ErrWithdrawalStatusInvalid = errors.New("withdrawal status invalid")
	ErrWithdrawalAlreadyExists = errors.New("withdrawal already exists for idempotency key")
	ErrInvalidAmount           = errors.New("withdrawal invalid amount")
	ErrAmountTooSmall          = errors.New("withdrawal amount too small")
	ErrAmountTooLarge          = errors.New("withdrawal amount too large")
	ErrDailyLimitExceeded      = errors.New("withdrawal daily limit exceeded")
	ErrDailyCountExceeded      = errors.New("withdrawal daily count limit exceeded")
	ErrFirstWithdrawalLimit    = errors.New("withdrawal first withdrawal limit exceeded")
	ErrInsufficientBalance     = errors.New("withdrawal insufficient balance")
	ErrUnsupportedNetwork      = errors.New("withdrawal unsupported network")
	ErrInvalidAddress          = errors.New("withdrawal invalid address")
	ErrAddressBlacklisted      = errors.New("withdrawal address blacklisted")
	ErrTOTPRequired            = errors.New("withdrawal totp required")
	ErrTOTPInvalid             = errors.New("withdrawal totp invalid")
	ErrTOTPNotEnabled          = errors.New("withdrawal totp not enabled")
	ErrIdempotencyKeyRequired  = errors.New("withdrawal idempotency key required")
	ErrTxidRequired            = errors.New("withdrawal txid required")
	ErrRejectReasonRequired    = errors.New("withdrawal reject reason required")
	ErrAddressNotFound         = errors.New("withdrawal address not found")
	ErrAddressDuplicate        = errors.New("withdrawal address duplicate")
	ErrInvalidConfig           = errors.New("withdrawal config invalid")
	ErrNewUserCooldown         = errors.New("withdrawal new user cooldown not elapsed")
	ErrUserNotFound            = errors.New("withdrawal user not found")
)
