package uploadhttp

import "github.com/gin-gonic/gin"

func RegisterAdminRoutes(admin gin.IRoutes, handler *AdminHandler) {
	admin.POST("/upload", handler.UploadFile)
}

// RegisterUserRoutes 注册用户侧文件上传路由。
// 复用通用 AdminHandler（其内部不区分 admin/user，仅做文件上传）。
func RegisterUserRoutes(user gin.IRoutes, handler *AdminHandler) {
	user.POST("/upload", handler.UploadFile)
}
