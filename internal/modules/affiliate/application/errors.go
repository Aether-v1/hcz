package application

import "errors"

var (
	ErrNotFound               = errors.New("affiliate resource not found")
	ErrDisabled               = errors.New("affiliate disabled")
	ErrNotOpened              = errors.New("affiliate not opened")
	ErrCodeInvalid            = errors.New("affiliate code invalid")
	ErrProfileStatusInvalid   = errors.New("affiliate profile status invalid")
	ErrWithdrawAmountInvalid  = errors.New("affiliate withdraw amount invalid")
	ErrWithdrawChannelInvalid = errors.New("affiliate withdraw channel invalid")
	ErrWithdrawInsufficient   = errors.New("affiliate withdraw insufficient")
	ErrWithdrawStatusInvalid  = errors.New("affiliate withdraw status invalid")
	// ErrWithdrawInsufficientAfterRefund 提现审核后发生退款，导致可用余额不足，阻止出金。
	ErrWithdrawInsufficientAfterRefund = errors.New("affiliate withdraw insufficient after refund, blocked payout")
	// ErrUserDisabled 开通推广时目标用户已禁用（与用户域共用同一文案哨兵）。
	ErrUserDisabled = errors.New("user disabled")

	// ---- 佣金划转至主钱包（替代旧独立提现）----
	ErrTransferAmountInvalid = errors.New("affiliate transfer amount invalid")
	ErrTransferInsufficient  = errors.New("affiliate transfer insufficient balance")
	ErrTransferInDebt        = errors.New("affiliate account in debt, cannot transfer")
	// ErrWithdrawRetired 旧独立提现入口已退休，请使用 transfer-to-wallet。
	ErrWithdrawRetired = errors.New("affiliate withdrawal has been retired, use transfer-to-wallet instead")

	// ---- 推广申请审核 ----
	// ErrAlreadyActive 用户已有 active profile，无需申请。
	ErrAlreadyActive = errors.New("affiliate already active")
	// ErrApplicationPending 用户已有 pending 申请，等待审核。
	ErrApplicationPending = errors.New("affiliate application pending")
	// ErrApplicationRejected 申请被拒绝。
	ErrApplicationRejected = errors.New("affiliate application rejected")
	// ErrNotActive 用户没有 active profile。
	ErrNotActive = errors.New("affiliate not active")
	// ErrApplicationNotFound 申请不存在。
	ErrApplicationNotFound = errors.New("affiliate application not found")
	// ErrApplicationAlreadyReviewed 申请已被审核（不能重复审核）。
	ErrApplicationAlreadyReviewed = errors.New("affiliate application already reviewed")
	// ErrOpenRetired 旧直接开通入口已退休，请使用 apply。
	ErrOpenRetired = errors.New("affiliate open retired, use apply instead")
	// ErrTransferNotApproved 推广申请未通过，禁止划转至钱包。
	ErrTransferNotApproved = errors.New("affiliate transfer not approved")
	// ErrProfileDisabledCannotApprove 管理员不得通过申请来静默恢复已禁用的 profile。
	ErrProfileDisabledCannotApprove = errors.New("affiliate profile disabled, cannot approve")
)
