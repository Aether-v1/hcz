package statemachine

import (
	"errors"
	"fmt"
)

// terminalStates 是不允许任何后续流转的终态集合。
var terminalStates = map[string]struct{}{
	StatusCompleted: {},
	StatusCanceled:  {},
	StatusExpired:   {},
}

// transitions 定义合法的状态流转表：currentStatus + event -> nextStatus。
var transitions = map[string]map[string]string{
	StatusPendingPayment: {
		EventMarkPaid: StatusPaid,
		EventCancel:   StatusCanceled,
		EventExpire:   StatusExpired,
	},
	StatusPaid: {
		EventConfirm: StatusCompleted,
		EventDispute: StatusDisputed,
	},
	StatusDisputed: {
		EventArbitrateRelease: StatusCompleted,
		EventArbitrateReturn:  StatusCanceled,
	},
}

// ErrInvalidTransition 表示状态流转不合法。
var ErrInvalidTransition = errors.New("invalid c2c trade status transition")

// Transition 校验并返回 currentStatus 在 event 下应进入的下一个状态。
// 终态（completed/canceled/expired）拒绝任何流转；非法流转返回明确错误。
func Transition(currentStatus string, event string) (string, error) {
	if _, isTerminal := terminalStates[currentStatus]; isTerminal {
		return "", fmt.Errorf("%w: status %q is terminal, event %q rejected", ErrInvalidTransition, currentStatus, event)
	}
	events, exists := transitions[currentStatus]
	if !exists {
		return "", fmt.Errorf("%w: unknown status %q, event %q rejected", ErrInvalidTransition, currentStatus, event)
	}
	next, exists := events[event]
	if !exists {
		return "", fmt.Errorf("%w: cannot apply event %q to status %q", ErrInvalidTransition, event, currentStatus)
	}
	return next, nil
}

// IsTerminal 判断 status 是否为终态（completed/canceled/expired）。
func IsTerminal(status string) bool {
	_, isTerminal := terminalStates[status]
	return isTerminal
}
