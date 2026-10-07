package pointsbootstrap

import (
	"github.com/Aether-v1/hcz/internal/app/container"
	pointsapp "github.com/Aether-v1/hcz/internal/modules/points/application"
	pointsphttp "github.com/Aether-v1/hcz/internal/modules/points/transport/http"
)

// Handlers 持有积分模块的 HTTP handlers。
type Handlers struct {
	User  *pointsphttp.UserHandler
	Admin *pointsphttp.AdminHandler
}

func New(c *container.Container) Handlers {
	return Handlers{
		User: pointsphttp.NewUserHandler(c.PointsService),
		Admin: pointsphttp.NewAdminHandler(
			c.PointsService,
			c.UserStore,
		),
	}
}

// compile-time guard：应用服务满足 transport 端口（装配期即失败，不留到运行期 panic）。
var (
	_ pointsphttp.UserPointsService  = (*pointsapp.Service)(nil)
	_ pointsphttp.AdminPointsService = (*pointsapp.Service)(nil)
)
