package contract

import "errors"

var (
	ErrInvalidAmount       = errors.New("points invalid amount")
	ErrInvalidOperation    = errors.New("points invalid operation")
	ErrAccountNotFound     = errors.New("points account not found")
	ErrAccountCreateFailed = errors.New("points account create failed")
	ErrAccountUpdateFailed = errors.New("points account update failed")
	ErrLedgerCreateFailed  = errors.New("points ledger create failed")
	ErrReasonRequired      = errors.New("points reason required")
	ErrReferenceRequired   = errors.New("points reference required")
	// ErrIdempotencyConflict 同一 Idempotency-Key 已存在但参数（用户/方向/金额）不一致。
	ErrIdempotencyConflict = errors.New("points idempotency conflict")
	ErrTransactionRequired = errors.New("points transaction required")
	// ErrNegativeNotAllowed 用户消费类 mutation 不允许产生负余额（本轮仅系统/Admin 可负）。
	ErrNegativeNotAllowed = errors.New("points negative balance not allowed")
	// ErrReasonTooLong reason 超过列宽（255）：返回 400，而不是让 DB 截断/报错变 500。
	ErrReasonTooLong = errors.New("points reason too long")
	// ErrReferenceTooLong reference / Idempotency-Key 派生后超过列宽（120）。
	ErrReferenceTooLong = errors.New("points reference too long")
	// ErrAmountTooLarge 单笔 mutation 超出业务硬上限（MaxPointsAmount）。
	ErrAmountTooLarge = errors.New("points amount exceeds limit")
	// ErrStatsSourceRequired 统计聚合端口未注入（装配错误，不应发生在生产）。
	ErrStatsSourceRequired = errors.New("points stats source required")
)
