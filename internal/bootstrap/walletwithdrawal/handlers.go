package walletwithdrawalbootstrap

import (
	"github.com/Aether-v1/hcz/internal/app/container"
	withdrawalhttp "github.com/Aether-v1/hcz/internal/modules/walletwithdrawal/transport/http"
)

// Handlers 聚合提现用户/后台 Handler。
type Handlers struct {
	User  *withdrawalhttp.UserHandler
	Admin *withdrawalhttp.AdminHandler
}

// New 构造提现 Handlers。
func New(c *container.Container) Handlers {
	return Handlers{
		User:  withdrawalhttp.NewUserHandler(c.WithdrawalService),
		Admin: withdrawalhttp.NewAdminHandler(c.WithdrawalService),
	}
}
