package contract

import notificationcontract "github.com/Aether-v1/hcz/internal/modules/notification/contract"

var (
	ErrNotifyConfigInvalid = notificationcontract.ErrConfigInvalid
	ErrNotifySendFailed    = notificationcontract.ErrSendFailed
)
