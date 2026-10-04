package usernotificationhttp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	usernotificationapp "github.com/Aether-v1/hcz/internal/modules/usernotification/application"
	"github.com/Aether-v1/hcz/internal/modules/usernotification/contract"
	"github.com/Aether-v1/hcz/internal/modules/usernotification/domain"
	usernotificationgormstore "github.com/Aether-v1/hcz/internal/modules/usernotification/infrastructure/gormstore"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupHandlerHarness(t *testing.T) (*gin.Engine, *usernotificationapp.Service) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	dsn := fmt.Sprintf("file:unotif_http_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&domain.UserNotification{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	svc := usernotificationapp.NewService(usernotificationgormstore.New(db))
	h := NewUserHandler(svc)

	r := gin.New()
	// 模拟登录态中间件：从 header 解析 user_id 注入（测试用）。
	r.Use(func(c *gin.Context) {
		if uid := c.GetHeader("X-Test-User-ID"); uid != "" {
			var id uint
			fmt.Sscanf(uid, "%d", &id)
			c.Set("user_id", id)
		}
		c.Next()
	})
	g := r.Group("/api/v1")
	RegisterUserRoutes(g, h)
	return r, svc
}

func doJSON(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("response not json: %v (body=%s)", err, w.Body.String())
	}
	return body
}

// 10. 未登录访问 -> 业务层 401（HTTP 200 + body status_code，真实 401 由 auth 中间件 ErrorWithHTTPStatus 负责）。
func TestHandlerUnauthenticated(t *testing.T) {
	r, _ := setupHandlerHarness(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/notifications/unread-count", nil)
	r.ServeHTTP(w, req)
	body := doJSON(t, w)
	if body["status_code"].(float64) != 401 {
		t.Fatalf("unauthenticated status_code = %v, want 401 (body=%s)", body["status_code"], w.Body.String())
	}
}

// 5. unread-count 返回 {count:N}。
func TestHandlerUnreadCount(t *testing.T) {
	r, svc := setupHandlerHarness(t)
	for i := 0; i < 2; i++ {
		_ = svc.CreateNotification(nil, contract.CreateInput{
			UserID: 7, Type: domain.TypeWalletRecharge, Title: "t",
			BizType: domain.BizTypeWalletRecharge, BizID: uint(i + 1),
		})
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/notifications/unread-count", nil)
	req.Header.Set("X-Test-User-ID", "7")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	body := doJSON(t, w)
	data, _ := body["data"].(map[string]interface{})
	if data["count"].(float64) != 2 {
		t.Fatalf("count = %v, want 2 (body=%s)", data["count"], w.Body.String())
	}
}

// 9. User A 无法 mark User B 的通知 -> 业务层 404（不暴露存在性）。
func TestHandlerMarkReadIDOR(t *testing.T) {
	r, svc := setupHandlerHarness(t)
	_ = svc.CreateNotification(nil, contract.CreateInput{
		UserID: 7, Type: domain.TypeOrderCompleted, Title: "t",
		BizType: domain.BizTypeOrder, BizID: 1,
	})
	// User 8 试图 mark User 7 的通知（id=1）。
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notifications/1/read", nil)
	req.Header.Set("X-Test-User-ID", "8")
	r.ServeHTTP(w, req)
	body := doJSON(t, w)
	if body["status_code"].(float64) != 404 {
		t.Fatalf("IDOR mark status_code = %v, want 404 (body=%s)", body["status_code"], w.Body.String())
	}
}

// 6. mark read 成功 -> {ok:true}。
func TestHandlerMarkReadSuccess(t *testing.T) {
	r, svc := setupHandlerHarness(t)
	_ = svc.CreateNotification(nil, contract.CreateInput{
		UserID: 7, Type: domain.TypeOrderCompleted, Title: "t",
		BizType: domain.BizTypeOrder, BizID: 1,
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notifications/1/read", nil)
	req.Header.Set("X-Test-User-ID", "7")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("mark read status = %d, body=%s", w.Code, w.Body.String())
	}
	body := doJSON(t, w)
	data, _ := body["data"].(map[string]interface{})
	if data["ok"] != true {
		t.Fatalf("ok = %v, want true (body=%s)", data["ok"], w.Body.String())
	}
}

// 列表 DTO 形状 + 分页包装。
func TestHandlerListShape(t *testing.T) {
	r, svc := setupHandlerHarness(t)
	_ = svc.CreateNotification(nil, contract.CreateInput{
		UserID: 7, Type: domain.TypeWalletRecharge, Title: "钱包充值到账",
		Body:    "USDT 充值已到账",
		Data:    map[string]interface{}{"recharge_no": "RC1", "amount": "100.00", "currency": "USDT"},
		BizType: domain.BizTypeWalletRecharge, BizID: 1,
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/notifications?page=1&page_size=20", nil)
	req.Header.Set("X-Test-User-ID", "7")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("list status = %d, body=%s", w.Code, w.Body.String())
	}
	body := doJSON(t, w)
	data, _ := body["data"].(map[string]interface{})
	if data["total"].(float64) != 1 || data["page"].(float64) != 1 || data["page_size"].(float64) != 20 {
		t.Fatalf("pagination wrong: %s", w.Body.String())
	}
	items, _ := data["items"].([]interface{})
	if len(items) != 1 {
		t.Fatalf("items len = %d, want 1", len(items))
	}
	item := items[0].(map[string]interface{})
	for _, k := range []string{"id", "type", "title", "body", "data", "biz_type", "biz_id", "is_read", "read_at", "created_at"} {
		if _, ok := item[k]; !ok {
			t.Fatalf("dto missing field %q: %s", k, w.Body.String())
		}
	}
	if item["type"] != "wallet_recharge" || item["biz_type"] != "wallet_recharge" {
		t.Fatalf("dto type/biz_type wrong: %s", w.Body.String())
	}
}
