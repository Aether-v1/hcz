package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	settingsstore "github.com/Aether-v1/hcz/internal/modules/settings/infrastructure/gormstore"

	complianceapp "github.com/Aether-v1/hcz/internal/modules/compliance/application"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupComplianceMW 构造一个挂了 PaymentComplianceRequired 的最小路由。
// 该中间件已改为 no-op（合规声明不再阻断），这里只验证它始终放行到下游 handler。
func setupComplianceMW(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&settingsstore.SettingRecord{}))
	cs := complianceapp.NewService(settingsstore.New(db))

	r := gin.New()
	r.GET("/proto",
		func(c *gin.Context) {
			if c.GetHeader("X-Test-Super") == "1" {
				c.Set("admin_is_super", true)
			}
			c.Next()
		},
		PaymentComplianceRequired(cs),
		func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) },
	)
	return r
}

// TestPaymentComplianceRequired_AlwaysPass 验证合规声明阻断已移除：
// 无论是否确认声明、是否超管、服务是否为 nil，中间件都直接放行到下游 handler。
func TestPaymentComplianceRequired_AlwaysPass(t *testing.T) {
	r := setupComplianceMW(t)

	cases := []struct {
		name       string
		superAdmin bool
	}{
		{name: "super admin", superAdmin: true},
		{name: "non-super admin", superAdmin: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/proto", nil)
			if tc.superAdmin {
				req.Header.Set("X-Test-Super", "1")
			}
			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Contains(t, w.Body.String(), "\"ok\":true")
			assert.NotContains(t, w.Body.String(), "compliance_required")
		})
	}
}

// TestPaymentComplianceRequired_NilService 防御性：服务为 nil 时也放行（no-op）。
func TestPaymentComplianceRequired_NilService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/proto",
		PaymentComplianceRequired(nil),
		func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) },
	)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/proto", nil))
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "\"ok\":true")
	assert.NotContains(t, w.Body.String(), "compliance_required")
}
