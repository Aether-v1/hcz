package httpserver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Aether-v1/hcz/internal/app/httpserver/middleware"
	"github.com/Aether-v1/hcz/internal/constants"
	complianceapp "github.com/Aether-v1/hcz/internal/modules/compliance/application"
	orderdomain "github.com/Aether-v1/hcz/internal/modules/order/domain"
	ordertransport "github.com/Aether-v1/hcz/internal/modules/order/transport/http"
	"github.com/Aether-v1/hcz/internal/platform/http/stepup"
	settingsstore "github.com/Aether-v1/hcz/internal/modules/settings/infrastructure/gormstore"
	"github.com/Aether-v1/hcz/internal/shared/money"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// newRefundComplianceEngine 把真实的退款写路由挂在 PaymentComplianceRequired 之后，
// 复现 routes_admin.go 中 refund write 路由的挂载形态，用于验证合规闸门确实拦截在 handler 之前。
func newRefundComplianceEngine(t *testing.T, handler *ordertransport.AdminRefundHandler) (*gin.Engine, *complianceapp.Service) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	dsn := fmt.Sprintf("file:refund_compliance_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite for compliance: %v", err)
	}
	if err := db.AutoMigrate(&settingsstore.SettingRecord{}); err != nil {
		t.Fatalf("migrate setting record: %v", err)
	}
	cs := complianceapp.NewService(settingsstore.New(db))

	r := gin.New()
	// 模拟 JWT+RBAC 之后的 admin 组；X-Test-Super=1 等价于超管上下文。
	admin := r.Group("/admin", func(c *gin.Context) {
		if c.GetHeader("X-Test-Super") == "1" {
			c.Set("admin_is_super", true)
		}
		c.Set("admin_id", uint(1))
		c.Next()
	})
	paymentProtected := admin.Group("", middleware.PaymentComplianceRequired(cs))
	ordertransport.RegisterAdminRefundWriteRoutes(paymentProtected, handler)
	return r, cs
}

// TestRefundWriteRoutesNotGatedByCompliance 验证：合规声明阻断已移除。
// 退款写路由（refund-to-wallet / manual-refund / payment-fee）无论是否确认合规声明，
// 都不再返回 compliance_required，请求会穿过 PaymentComplianceRequired（现为 no-op）到达下游。
// handler 是否成功取决于测试桩依赖，这里只断言不再被合规闸门拦截。
func TestRefundWriteRoutesNotGatedByCompliance(t *testing.T) {
	handler, _ := setupAdminOrderRefundHandlerTest(t)
	r, _ := newRefundComplianceEngine(t, handler)

	endpoints := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/admin/orders/1/refund-to-wallet"},
		{http.MethodPost, "/admin/orders/1/manual-refund"},
		{http.MethodPatch, "/admin/order-refunds/1/payment-fee"},
	}
	for _, ep := range endpoints {
		t.Run(ep.method+" "+ep.path, func(t *testing.T) {
			body := bytes.NewBufferString(`{"amount":"1","payment_fee_refunded":true}`)
			req := httptest.NewRequest(ep.method, ep.path, body)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Test-Super", "1")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if bytes.Contains(w.Body.Bytes(), []byte("compliance_required")) {
				t.Fatalf("refund write must NOT be gated by compliance anymore, but got blocked: %s", w.Body.String())
			}
		})
	}
}

// TestDuplicateManualRefundDoesNotDoubleCredit 验证服务层 refundable 上限校验：
// 同一订单第二次超额手动退款不会再产生退款记录 / 增加 refunded_amount（防重复 credit 兜底）。
func TestDuplicateManualRefundDoesNotDoubleCredit(t *testing.T) {
	handler, db := setupAdminOrderRefundHandlerTest(t)
	fixture := seedAdminOrderRefundData(t, db)

	// 播种一个全新的、已支付、可退 100 的会员订单（不带历史退款）。
	now := time.Now().UTC().Truncate(time.Second)
	paidAt := now.Add(-30 * time.Minute)
	order := &orderdomain.Order{
		OrderNo:          "DJ-REFUND-DUP-1",
		UserID:           fixture.MemberUserID,
		Status:           constants.OrderStatusPartiallyRefunded,
		Currency:         "CNY",
		OriginalAmount:   money.FromDecimal(decimal.NewFromInt(100)),
		TotalAmount:      money.FromDecimal(decimal.NewFromInt(100)),
		OnlinePaidAmount: money.FromDecimal(decimal.NewFromInt(100)),
		WalletPaidAmount: money.FromDecimal(decimal.Zero),
		RefundedAmount:   money.FromDecimal(decimal.Zero),
		PaidAt:           &paidAt,
		CreatedAt:        now.Add(-1 * time.Hour),
		UpdatedAt:        now.Add(-1 * time.Hour),
	}
	if err := db.Create(order).Error; err != nil {
		t.Fatalf("seed paid order: %v", err)
	}

	doRefund := func(amount string) int {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("admin_id", uint(1))
		c.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", order.ID)}}
		c.Request = httptest.NewRequest(http.MethodPost,
			fmt.Sprintf("/admin/orders/%d/manual-refund", order.ID),
			bytes.NewBufferString(fmt.Sprintf(`{"amount":%q,"remark":"dup"}`, amount)),
		)
		c.Request.Header.Set("Content-Type", "application/json")
		c.Request.Header.Set("X-Auth-Challenge", stepup.Scope("refund.manual", "order", uint(order.ID)))
		handler.AdminManualRefundOrder(c)
		var resp struct {
			StatusCode int `json:"status_code"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		return resp.StatusCode
	}

	// 第一次：全额 100 退款应成功。
	if sc := doRefund("100"); sc != 0 {
		t.Fatalf("first full manual refund should succeed (status_code=0), got %d", sc)
	}

	var refundCount int64
	db.Model(&orderdomain.OrderRefundRecord{}).Where("order_id = ?", order.ID).Count(&refundCount)
	if refundCount != 1 {
		t.Fatalf("expected exactly 1 refund record after first refund, got %d", refundCount)
	}

	// 第二次：再退 1，已无可退余额，必须被服务层拒绝，且不新增退款记录。
	secondSC := doRefund("1")
	if secondSC == 0 {
		t.Fatalf("second over-limit manual refund must be rejected, but returned success")
	}
	db.Model(&orderdomain.OrderRefundRecord{}).Where("order_id = ?", order.ID).Count(&refundCount)
	if refundCount != 1 {
		t.Fatalf("duplicate refund must not create a second record, got %d", refundCount)
	}

	// 订单累计退款额仍为 100，未被重复增加。
	var reloaded orderdomain.Order
	if err := db.First(&reloaded, order.ID).Error; err != nil {
		t.Fatalf("reload order: %v", err)
	}
	if !reloaded.RefundedAmount.Decimal.Equal(decimal.NewFromInt(100)) {
		t.Fatalf("refunded_amount must stay 100 after rejected duplicate, got %s", reloaded.RefundedAmount.Decimal.String())
	}
}
