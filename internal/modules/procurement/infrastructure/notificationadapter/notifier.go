package notificationadapter

import (
	"github.com/Aether-v1/hcz/internal/constants"
	notificationcontract "github.com/Aether-v1/hcz/internal/modules/notification/contract"
	procurementcontract "github.com/Aether-v1/hcz/internal/modules/procurement/contract"
	procurementdomain "github.com/Aether-v1/hcz/internal/modules/procurement/domain"
	"github.com/Aether-v1/hcz/internal/shared/jsonmap"
)

type Notifier struct {
	enqueuer notificationcontract.NotificationEnqueuer
}

var _ procurementcontract.FailureNotifier = (*Notifier)(nil)

func New(enqueuer notificationcontract.NotificationEnqueuer) procurementcontract.FailureNotifier {
	if enqueuer == nil {
		return nil
	}
	return &Notifier{enqueuer: enqueuer}
}

func (n *Notifier) NotifyFailure(order *procurementdomain.Order, message string) error {
	return n.enqueuer.Enqueue(notificationcontract.EnqueueInput{
		EventType: constants.NotificationEventExceptionAlert,
		BizType:   constants.NotificationBizTypeProcurement,
		BizID:     order.ID,
		Data: jsonmap.JSON{
			"procurement_order_id": order.ID,
			"local_order_no":       order.LocalOrderNo,
			"error":                message,
		},
	})
}
