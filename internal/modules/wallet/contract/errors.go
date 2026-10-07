package contract

import "errors"

var (
	ErrInvalidAmount           = errors.New("wallet invalid amount")
	ErrInsufficientBalance     = errors.New("wallet insufficient balance")
	ErrAccountNotFound         = errors.New("wallet account not found")
	ErrAccountCreateFailed     = errors.New("wallet account create failed")
	ErrAccountUpdateFailed     = errors.New("wallet account update failed")
	ErrTransactionCreateFailed = errors.New("wallet transaction create failed")
	ErrRefundExceeded          = errors.New("wallet refund exceeded")
	ErrNotSupportedForGuest    = errors.New("wallet not supported for guest")
	ErrRechargeNotFound        = errors.New("wallet recharge not found")
	ErrRechargeStatusInvalid   = errors.New("wallet recharge status invalid")
	ErrOnlyPaymentRequired     = errors.New("wallet only payment required")
	ErrTransactionRequired     = errors.New("wallet transaction required")
	ErrInsufficientFrozen      = errors.New("wallet insufficient frozen balance")
	ErrSameAccount             = errors.New("wallet source and target must be different accounts")
	// ErrIdempotencyConflict 同一 Idempotency-Key 已存在但参数（用户/金额/方向）不一致。
	ErrIdempotencyConflict = errors.New("idempotency key conflict")
)
