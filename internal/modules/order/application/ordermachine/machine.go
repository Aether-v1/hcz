// Package ordermachine 是 HCZ Business Order 五主状态的唯一权威迁移入口。
// 纯逻辑：不碰 DB、不碰资金，只负责状态归一、迁移合法性、权限与副作用分类。
// 所有 Admin/User/系统状态变更必须经过这里。
package ordermachine

import "github.com/Aether-v1/hcz/internal/constants"

// View 是归一后对外输出的订单主状态视图。
type View struct {
	Status      string // 五主状态之一
	RefundStatus string
}

// Normalize 把历史 9 态（或新 5 态）归一为对外五主状态 + 独立退款子状态。
// 不做破坏性假设：读 oldStatus + 已有 refundStatus 提示。
func Normalize(oldStatus string) View {
	switch oldStatus {
	case constants.OrderStatusPendingRecharge:
		return View{Status: constants.OrderStatusPendingRecharge, RefundStatus: constants.OrderRefundStatusNone}
	case constants.OrderStatusProcessing:
		return View{Status: constants.OrderStatusProcessing, RefundStatus: constants.OrderRefundStatusNone}
	case constants.OrderStatusFailed:
		return View{Status: constants.OrderStatusFailed, RefundStatus: constants.OrderRefundStatusFull}
	case constants.OrderStatusCanceled:
		return View{Status: constants.OrderStatusCanceled, RefundStatus: constants.OrderRefundStatusFull}

	// 历史映射
	case constants.OrderStatusPendingPayment, constants.OrderStatusPaid:
		return View{Status: constants.OrderStatusPendingRecharge, RefundStatus: constants.OrderRefundStatusNone}
	case constants.OrderStatusFulfilling, constants.OrderStatusPartiallyDelivered:
		return View{Status: constants.OrderStatusProcessing, RefundStatus: constants.OrderRefundStatusNone}
	case constants.OrderStatusDelivered, constants.OrderStatusCompleted:
		return View{Status: constants.OrderStatusCompleted, RefundStatus: constants.OrderRefundStatusNone}
	case constants.OrderStatusPartiallyRefunded:
		// 旧「部分退款」主状态 → 归一为已完成 + 独立 partial
		return View{Status: constants.OrderStatusCompleted, RefundStatus: constants.OrderRefundStatusPartial}
	case constants.OrderStatusRefunded:
		return View{Status: constants.OrderStatusCompleted, RefundStatus: constants.OrderRefundStatusFull}
	default:
		return View{Status: oldStatus, RefundStatus: constants.OrderRefundStatusNone}
	}
}

var allowed = map[string]map[string]bool{
	constants.OrderStatusPendingRecharge: {
		constants.OrderStatusProcessing: true,
		constants.OrderStatusFailed:     true,
		constants.OrderStatusCanceled:   true,
	},
	constants.OrderStatusProcessing: {
		constants.OrderStatusCompleted: true,
		constants.OrderStatusFailed:    true,
	},
	// completed / failed / canceled 为终态，无出边
}

// CanTransition 判断主状态迁移是否合法。
func CanTransition(from, to string) bool {
	return allowed[from][to]
}

// RequiresAutoRefund 标记该迁移是否触发 USDT 自动退款（复用 P0-2 退款链）。
func RequiresAutoRefund(from, to string) bool {
	return to == constants.OrderStatusFailed || to == constants.OrderStatusCanceled
}

// AllowedByUser 标记该迁移是否允许 User 发起（仅 pending_recharge→canceled）。
func AllowedByUser(from, to string) bool {
	return from == constants.OrderStatusPendingRecharge && to == constants.OrderStatusCanceled
}

// IsTerminal 主状态是否终态。
func IsTerminal(status string) bool {
	return status == constants.OrderStatusCompleted ||
		status == constants.OrderStatusFailed ||
		status == constants.OrderStatusCanceled
}
