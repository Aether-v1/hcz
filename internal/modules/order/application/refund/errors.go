package refund

import orderapp "github.com/Aether-v1/hcz/internal/modules/order/application"

var (
	ErrOrderFetchFailed         = orderapp.ErrOrderFetchFailed
	ErrOrderNotFound            = orderapp.ErrOrderNotFound
	ErrOrderRefundExpired       = orderapp.ErrOrderRefundExpired
	ErrOrderStatusInvalid       = orderapp.ErrOrderStatusInvalid
	ErrOrderUpdateFailed        = orderapp.ErrOrderUpdateFailed
	ErrRefundRecordCreateFailed = orderapp.ErrRefundRecordCreateFailed
)

