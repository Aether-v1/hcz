package container

// wireServiceDependencies 收口构造后才能建立的双向或延迟依赖。
func (c *Container) wireServiceDependencies() {
	c.UserAuthService.SetMemberLevelService(c.MemberLevelService)
	c.OrderRefundService.SetResellerAccounting(c.ResellerAccountingLedger)
	c.PaymentService.SetMemberLevelService(c.MemberLevelService)
	c.PaymentService.SetProcurementService(c.ProcurementOrderService)
	c.PaymentService.SetDownstreamCallbackService(c.DownstreamCallbackService)
	c.FulfillmentService.SetDownstreamCallbackService(c.DownstreamCallbackService)
	// Phase 1 用户站内通知：业务事件尽力而为写入（幂等）。
	c.PaymentService.SetUserNotifier(c.UserNotificationService)
	c.OrderService.SetUserNotifier(c.UserNotificationService)
	c.FulfillmentService.SetUserNotifier(c.UserNotificationService)
}
