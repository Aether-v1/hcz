package checkinbootstrap

import (
	"github.com/Aether-v1/hcz/internal/app/container"
	checkinapp "github.com/Aether-v1/hcz/internal/modules/checkin/application"
	checkinphttp "github.com/Aether-v1/hcz/internal/modules/checkin/transport/http"
)

// Handlers 持有签到模块的 HTTP handlers。
type Handlers struct {
	User  *checkinphttp.UserHandler
	Admin *checkinphttp.AdminHandler
}

// New 构造签到模块 handlers。
func New(c *container.Container) Handlers {
	return Handlers{
		User:  checkinphttp.NewUserHandler(c.CheckinService),
		Admin: checkinphttp.NewAdminHandler(c.CheckinService, c.UserStore),
	}
}

// compile-time guard：CheckinService 满足 transport 端口。
var (
	_ checkinphttp.UserCheckinService  = (*checkinapp.Service)(nil)
	_ checkinphttp.AdminCheckinService = (*checkinapp.Service)(nil)
)
