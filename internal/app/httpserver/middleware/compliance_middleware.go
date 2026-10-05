package middleware

import (
	complianceapp "github.com/Aether-v1/hcz/internal/modules/compliance/application"

	"github.com/gin-gonic/gin"
)

// PaymentComplianceRequired 历史上用于拦截支付/财务路由：未确认合规声明则阻断。
//
// 该阻断已按业务要求移除：合规声明不再作为支付/财务路由的访问闸门。
// 函数签名保留（避免破坏既有调用点与测试），内部改为直接放行（no-op）。
// complianceapp.Service 仍被注入以保持构造形态，但不再参与鉴权决策。
func PaymentComplianceRequired(cs *complianceapp.Service) gin.HandlerFunc {
	_ = cs
	return func(c *gin.Context) {
		c.Next()
	}
}
