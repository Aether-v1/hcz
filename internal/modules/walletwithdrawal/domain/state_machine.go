package domain

// allowedTransitions 是提现状态机的唯一真源。
// handler / application 禁止直接写 status，必须经 CanTransition 校验。
var allowedTransitions = map[string]map[string]struct{}{
	StatusPending: {
		StatusApproved: {},
		StatusRejected: {},
		StatusCanceled: {},
	},
	StatusApproved: {
		StatusProcessing: {},
		StatusRejected:   {},
	},
	StatusProcessing: {
		StatusCompleted: {},
		StatusRejected:  {},
	},
	// 终态无出边。
}

// CanTransition 判断 from -> to 是否为合法状态迁移。
func CanTransition(from, to string) bool {
	if from == to {
		return false
	}
	nexts, ok := allowedTransitions[from]
	if !ok {
		return false
	}
	_, ok = nexts[to]
	return ok
}

// CanCancel 判断该状态是否允许用户取消（仅 pending）。
func CanCancel(status string) bool {
	return status == StatusPending
}

// CanRefund 判断该状态是否允许退款（reject/cancel 触发；仅 pending/approved/processing）。
// 注意：cancel 只允许 pending；reject 允许 pending/approved/processing。
// 退款本身由状态机 + 唯一 reference 兜底幂等。
func CanRefund(status string) bool {
	switch status {
	case StatusPending, StatusApproved, StatusProcessing:
		return true
	default:
		return false
	}
}
