// Package statemachine 实现工单状态机：状态常量、事件、流转表与重开守卫。
package statemachine

import "time"

// 工单状态。
const (
	StatusOpen           = "open"
	StatusWaitingUser    = "waiting_user"
	StatusWaitingSupport = "waiting_support"
	StatusResolved       = "resolved"
	StatusClosed         = "closed"
)

// 状态事件。
const (
	EventUserReply    = "user_reply"
	EventAdminReply   = "admin_reply"
	EventAdminResolve = "admin_resolve"
	EventUserClose    = "user_close"
	EventAdminClose   = "admin_close"
	EventUserReopen   = "user_reopen"
	EventAdminReopen  = "admin_reopen"
)

// 发送方 / 上传方类型。
const (
	SenderUser   = "user"
	SenderAdmin  = "admin"
	SenderSystem = "system"
)

// 消息类型。
const (
	MessageTypeText         = "text"
	MessageTypeSystemNotice = "system_notice"
)

// 优先级。
const (
	PriorityLow    = "low"
	PriorityNormal = "normal"
	PriorityHigh   = "high"
	PriorityUrgent = "urgent"
)

// 审计动作。
const (
	AuditActionAssign         = "assign"
	AuditActionStatusChange   = "status_change"
	AuditActionPriorityChange = "priority_change"
	AuditActionResolve        = "resolve"
	AuditActionClose          = "close"
	AuditActionReopen         = "reopen"
)

// ReopenWindow resolved 状态允许重开的时间窗口（7 天）。
const ReopenWindow = 7 * 24 * time.Hour
