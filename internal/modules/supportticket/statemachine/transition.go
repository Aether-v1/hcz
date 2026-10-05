package statemachine

import (
	"errors"
	"fmt"
	"time"
)

// ErrInvalidTransition 表示状态流转不合法。
var ErrInvalidTransition = errors.New("invalid support ticket status transition")

// ErrReopenExpired 表示超过 7 天重开窗口。
var ErrReopenExpired = errors.New("reopen window expired")

// terminalStates closed 为终态，拒绝任何后续流转。
var terminalStates = map[string]struct{}{
	StatusClosed: {},
}

// transitions 定义合法的状态流转表：currentStatus + event -> nextStatus。
var transitions = map[string]map[string]string{
	StatusOpen: {
		EventUserReply:    StatusWaitingSupport,
		EventAdminReply:   StatusWaitingUser,
		EventAdminResolve: StatusResolved,
		EventUserClose:    StatusClosed,
		EventAdminClose:   StatusClosed,
	},
	StatusWaitingUser: {
		EventUserReply:    StatusWaitingSupport,
		EventAdminResolve: StatusResolved,
		EventUserClose:    StatusClosed,
		EventAdminClose:   StatusClosed,
	},
	StatusWaitingSupport: {
		EventAdminReply:   StatusWaitingUser,
		EventAdminResolve: StatusResolved,
		EventUserClose:    StatusClosed,
		EventAdminClose:   StatusClosed,
	},
	StatusResolved: {
		EventUserReopen:  StatusWaitingSupport,
		EventAdminReopen: StatusWaitingSupport,
		EventUserClose:   StatusClosed,
		EventAdminClose:  StatusClosed,
	},
}

// Transition 校验并返回 currentStatus 在 event 下应进入的下一个状态。
// closed 为终态，拒绝任何流转；非法流转返回 ErrInvalidTransition。
func Transition(currentStatus, event string) (string, error) {
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

// IsTerminal 判断 status 是否为终态（closed）。
func IsTerminal(status string) bool {
	_, isTerminal := terminalStates[status]
	return isTerminal
}

// CanReopen 判断 resolved_at 是否仍在 7 天重开窗口内。
func CanReopen(resolvedAt time.Time, now time.Time) bool {
	if resolvedAt.IsZero() {
		return false
	}
	return now.Sub(resolvedAt) <= ReopenWindow
}

// ValidPriority 判断是否为合法优先级。
func ValidPriority(p string) bool {
	switch p {
	case PriorityLow, PriorityNormal, PriorityHigh, PriorityUrgent:
		return true
	default:
		return false
	}
}
