package pointsmallbootstrap

import (
	"github.com/Aether-v1/hcz/internal/app/container"
	pointsmallapp "github.com/Aether-v1/hcz/internal/modules/pointsmall/application"
	pointsmallphttp "github.com/Aether-v1/hcz/internal/modules/pointsmall/transport/http"
)

// Handlers 持有积分商城模块的 HTTP handlers。
type Handlers struct {
	User  *pointsmallphttp.UserHandler
	Admin *pointsmallphttp.AdminHandler
}

// New 构造积分商城模块 handlers。
func New(c *container.Container) Handlers {
	return Handlers{
		User:  pointsmallphttp.NewUserHandler(c.PointsmallService),
		Admin: pointsmallphttp.NewAdminHandler(c.PointsmallService),
	}
}

// compile-time guards：PointsmallService 满足 transport 端口。
var _ pointsmallphttp.UserService = (*pointsmallapp.Service)(nil)
var _ pointsmallphttp.AdminService = (*pointsmallapp.Service)(nil)
