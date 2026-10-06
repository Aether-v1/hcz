package integrationtest

// application_http_test.go — Application 相关 HTTP 层测试：
//   25. POST /affiliate/open 已退役，返回 HTTP 410。
//   39. Admin application API 未认证 → 401。
//   40. 普通用户 token（仅 user_id）调用 admin approve → 拒绝（401）。
//   41. 普通用户不能访问 admin application 列表 → 401。
//
// 说明：生产环境 admin 路由挂载在 `authorized` 组（admin JWT 中间件注入 admin_id）。
// 本测试用等价的 requireAdmin 中间件复刻该挂载：无 admin_id 即 401。
// approve/reject handler 内部另有 GetAdminID 防御层。

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	affiliateapp "github.com/Aether-v1/hcz/internal/modules/affiliate/application"
	affiliatetransport "github.com/Aether-v1/hcz/internal/modules/affiliate/transport/http"
	"github.com/Aether-v1/hcz/internal/platform/http/response"
)

func init() { gin.SetMode(gin.TestMode) }

// apiResponse 统一响应结构（与 response.Response 对齐）。
type apiResponse struct {
	StatusCode int             `json:"status_code"`
	Msg        string          `json:"msg"`
	Data       json.RawMessage `json:"data"`
}

// requireAdmin 复刻生产 admin JWT 中间件：无 admin_id 即 401。
func requireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, ok := c.Get("admin_id"); !ok {
			response.Error(c, response.CodeUnauthorized, "unauthorized")
			c.Abort()
			return
		}
		c.Next()
	}
}

// buildAdminRouter 搭建带 admin 守卫的管理端路由。
func buildAdminRouter(svc *affiliateapp.Service) *gin.Engine {
	r := gin.New()
	adminH := affiliatetransport.NewAdminHandler(svc)
	admin := r.Group("/admin/affiliates")
	admin.Use(requireAdmin())
	admin.GET("/applications", adminH.ListAffiliateApplications)
	admin.GET("/applications/:id", adminH.GetAffiliateApplication)
	admin.POST("/applications/:id/approve", adminH.ApproveAffiliateApplication)
	admin.POST("/applications/:id/reject", adminH.RejectAffiliateApplication)
	return r
}

func doReq(t *testing.T, r *gin.Engine, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// B25. TestOpenAffiliate_Returns410
func TestOpenAffiliate_Returns410(t *testing.T) {
	svc, _, _ := setupAppServiceTest(t, true)
	r := gin.New()
	userH := affiliatetransport.NewHandler(svc)
	r.POST("/affiliate/open", userH.OpenAffiliate)

	w := doReq(t, r, http.MethodPost, "/affiliate/open")
	if w.Code != http.StatusGone {
		t.Fatalf("POST /affiliate/open want HTTP 410, got %d (body=%s)", w.Code, w.Body.String())
	}
}

// F39. TestAdminApplicationAPI_RequiresAuth —— 未登录调用 admin approve → 401。
func TestAdminApplicationAPI_RequiresAuth(t *testing.T) {
	svc, db, _ := setupAppServiceTest(t, true)
	u := createAffiliateTestUser(t, db, "rbac-reqauth@hcz.test")
	app := mustApply(t, svc, u.ID, "x")
	r := buildAdminRouter(svc)

	w := doReq(t, r, http.MethodPost, "/admin/affiliates/applications/"+itoa(int(app.ID))+"/approve")

	var resp apiResponse
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.StatusCode != response.CodeUnauthorized {
		t.Fatalf("admin approve without auth want status_code=401, got http=%d body=%s", w.Code, w.Body.String())
	}
}

// F40. TestUserCannotApproveOwnApplication —— 普通用户 token（仅 user_id，无 admin_id）→ 拒绝。
func TestUserCannotApproveOwnApplication(t *testing.T) {
	svc, db, _ := setupAppServiceTest(t, true)
	u := createAffiliateTestUser(t, db, "rbac-user@hcz.test")
	app := mustApply(t, svc, u.ID, "x")

	// 复刻"普通用户已登录、但无 admin 权限"：注入 user_id，不注入 admin_id。
	r := gin.New()
	adminH := affiliatetransport.NewAdminHandler(svc)
	admin := r.Group("/admin/affiliates")
	admin.Use(func(c *gin.Context) {
		c.Set("user_id", u.ID) // 普通用户身份
		if _, ok := c.Get("admin_id"); !ok {
			response.Error(c, response.CodeUnauthorized, "forbidden: admin only")
			c.Abort()
			return
		}
		c.Next()
	})
	admin.POST("/applications/:id/approve", adminH.ApproveAffiliateApplication)

	w := doReq(t, r, http.MethodPost, "/admin/affiliates/applications/"+itoa(int(app.ID))+"/approve")

	var resp apiResponse
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.StatusCode != response.CodeUnauthorized {
		t.Fatalf("normal user approving want 401, got http=%d body=%s", w.Code, w.Body.String())
	}
}

// F41. TestUserCannotAccessAdminApplicationsList —— 未认证访问 admin 列表 → 401。
func TestUserCannotAccessAdminApplicationsList(t *testing.T) {
	svc, _, _ := setupAppServiceTest(t, true)
	r := buildAdminRouter(svc)

	w := doReq(t, r, http.MethodGet, "/admin/affiliates/applications")

	var resp apiResponse
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.StatusCode != response.CodeUnauthorized {
		t.Fatalf("admin list without auth want 401, got http=%d body=%s", w.Code, w.Body.String())
	}
}
