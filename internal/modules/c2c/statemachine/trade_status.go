// Package statemachine 实现 C2C 交易单的纯函数状态机。
// 不依赖数据库，调用方在更新 status 字段前必须先通过 Transition 校验流转合法性。
package statemachine

// 交易单状态。
const (
	StatusPendingPayment = "pending_payment" // 待付款
	StatusPaid           = "paid"            // 已付款，待卖家放行
	StatusCompleted      = "completed"       // 已完成（终态）
	StatusCanceled       = "canceled"        // 已取消（终态）
	StatusExpired        = "expired"         // 已超时（终态）
	StatusDisputed       = "disputed"        // 争议中
)

// 状态事件。
const (
	EventMarkPaid         = "mark_paid"         // 买家标记已付款
	EventCancel           = "cancel"            // 取消交易
	EventExpire           = "expire"            // 支付超时
	EventConfirm          = "confirm"           // 卖家确认放行
	EventDispute          = "dispute"           // 发起争议
	EventArbitrateRelease = "arbitrate_release" // 仲裁放行给买家
	EventArbitrateReturn  = "arbitrate_return"  // 仲裁退回给卖家
)
