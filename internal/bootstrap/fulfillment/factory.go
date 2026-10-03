package fulfillmentwiring

import (
	"github.com/Aether-v1/hcz/internal/app/container"
	fulfillmenttransport "github.com/Aether-v1/hcz/internal/modules/fulfillment/transport/http"
)

func NewAdminHandler(c *container.Container) *fulfillmenttransport.AdminHandler {
	return fulfillmenttransport.NewAdminHandler(
		fulfillmentManualCreatorAdapter{svc: c.FulfillmentService},
		fulfillmentAdminOrderAdapter{orders: c.OrderService},
	)
}

