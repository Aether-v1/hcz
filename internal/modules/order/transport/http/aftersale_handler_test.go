package orderhttp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Aether-v1/hcz/internal/constants"
	affiliateapp "github.com/Aether-v1/hcz/internal/modules/affiliate/application"
	affiliategormstore "github.com/Aether-v1/hcz/internal/modules/affiliate/infrastructure/gormstore"
	affiliatedomain "github.com/Aether-v1/hcz/internal/modules/affiliate/domain"
	fulfillmentdomain "github.com/Aether-v1/hcz/internal/modules/fulfillment/domain"
	userdomain "github.com/Aether-v1/hcz/internal/modules/identity/user/domain"
	userstore "github.com/Aether-v1/hcz/internal/modules/identity/user/infrastructure/gormstore"
	"github.com/Aether-v1/hcz/internal/modules/order/application/aftersale"
	"github.com/Aether-v1/hcz/internal/modules/order/application/refund"
	orderdomain "github.com/Aether-v1/hcz/internal/modules/order/domain"
	ordergormstore "github.com/Aether-v1/hcz/internal/modules/order/infrastructure/gormstore"
	paymentgormstore "github.com/Aether-v1/hcz/internal/modules/payment/infrastructure/gormstore"
	settingsapp "github.com/Aether-v1/hcz/internal/modules/settings/application"
	settingsstore "github.com/Aether-v1/hcz/internal/modules/settings/infrastructure/gormstore"
	walletapp "github.com/Aether-v1/hcz/internal/modules/wallet/application"
	walletdomain "github.com/Aether-v1/hcz/internal/modules/wallet/domain"
	walletgormstore "github.com/Aether-v1/hcz/internal/modules/wallet/infrastructure/gormstore"
	"github.com/Aether-v1/hcz/internal/shared/money"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func setupAfterSaleHandlerTest(t *testing.T) (*AfterSaleHandler, *gorm.DB, uint, uint) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	dsn := fmt.Sprintf("file:ash_handler_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&userdomain.User{},
		&fulfillmentdomain.Fulfillment{},
		&affiliatedomain.Profile{},
		&affiliatedomain.Commission{},
		&affiliatedomain.WithdrawRequest{},
		&orderdomain.Order{},
		&orderdomain.OrderItem{},
		&orderdomain.OrderRefundRecord{},
		&orderdomain.AfterSaleTicket{},
		&walletdomain.Account{},
		&walletdomain.Transaction{},
		&settingsstore.SettingRecord{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	orderStore := ordergormstore.New(db, "test-guest-credential-secret-with-32-bytes")
	walletStore := walletgormstore.New(db)
	walletSvc := walletapp.NewService(walletapp.Options{Repository: walletStore, Transactions: walletStore})
	affiliateSvc := affiliateapp.NewService(affiliategormstore.New(db), nil, nil, nil, nil)
	settingSvc := settingsapp.NewService(settingsstore.New(db))
	paymentStore := paymentgormstore.New(db, "test-guest-credential-secret-with-32-bytes")
	refundSvc := refund.New(orderStore, userstore.New(db), affiliateSvc, settingSvc, walletSvc, paymentStore)
	svc := aftersale.NewService(orderStore, aftersale.NewWalletRefunderAdapter(refundSvc))
	handler := NewAfterSaleHandler(svc, orderStore)

	user := &userdomain.User{Email: "h@test.com", Status: "active"}
	if err := db.Create(user).Error; err != nil {
		t.Fatal(err)
	}
	acct := &walletdomain.Account{UserID: user.ID, Balance: money.FromDecimal(decimal.RequireFromString("100.00"))}
	if err := db.Create(acct).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	order := &orderdomain.Order{
		UserID: user.ID, OrderNo: fmt.Sprintf("H%d", now.UnixNano()),
		Status: constants.OrderStatusCompleted, Currency: "CNY",
		TotalAmount:      money.FromDecimal(decimal.RequireFromString("71.80")),
		UsdtTotalAmount:  money.FromDecimal(decimal.RequireFromString("10.00")),
		WalletPaidAmount: money.FromDecimal(decimal.RequireFromString("10.00")),
		RefundStatus:     "none",
		PaidAt:           &now,
		CreatedAt:        now, UpdatedAt: now,
	}
	if err := db.Create(order).Error; err != nil {
		t.Fatal(err)
	}
	return handler, db, user.ID, order.ID
}

func newTestContext(method, path string, body interface{}, userID uint) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	c.Request = httptest.NewRequest(method, path, &buf)
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", 0)}}
	if userID > 0 {
		c.Set("user_id", userID)
	}
	return c, w
}

func setOrderIDParam(c *gin.Context, orderID uint) {
	c.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", orderID)}}
}

// respStatusCode 解析统一响应体的业务状态码（HTTP 始终 200，业务码在 body.status_code）。
func respStatusCode(t *testing.T, w *httptest.ResponseRecorder) int {
	t.Helper()
	var resp struct {
		StatusCode int `json:"status_code"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse response: %v, body=%s", err, w.Body.String())
	}
	return resp.StatusCode
}

// User 发起售后成功
func TestAfterSaleUserCreateSuccess(t *testing.T) {
	handler, db, userID, orderID := setupAfterSaleHandlerTest(t)
	c, w := newTestContext(http.MethodPost, "/api/v1/orders/1/after-sale", map[string]string{
		"type": "not_received", "reason": "未收到充值",
	}, userID)
	setOrderIDParam(c, orderID)
	handler.UserCreateAfterSale(c)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Data AfterSaleDTO `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Data.Status != "pending" {
		t.Fatalf("expected pending, got %s", resp.Data.Status)
	}
	if resp.Data.RefundCurrency != "USDT" {
		t.Fatalf("expected USDT, got %s", resp.Data.RefundCurrency)
	}
	if resp.Data.OrderStatus != constants.OrderStatusCompleted {
		t.Fatalf("order status must remain completed, got %s", resp.Data.OrderStatus)
	}
	var order orderdomain.Order
	db.First(&order, orderID)
	if order.AfterSaleStatus != "pending" {
		t.Fatalf("order.after_sale_status must be pending, got %s", order.AfterSaleStatus)
	}
}

// invalid type 拒绝
func TestAfterSaleUserCreateInvalidTypeRejected(t *testing.T) {
	handler, _, userID, orderID := setupAfterSaleHandlerTest(t)
	c, w := newTestContext(http.MethodPost, "/api/v1/orders/1/after-sale", map[string]string{
		"type": "wrong_type", "reason": "test",
	}, userID)
	setOrderIDParam(c, orderID)
	handler.UserCreateAfterSale(c)
	if code := respStatusCode(t, w); code != http.StatusBadRequest {
		t.Fatalf("expected business code 400 for invalid type, got %d", code)
	}
}

// 重复 pending 拒绝
func TestAfterSaleUserCreateDuplicateRejected(t *testing.T) {
	handler, _, userID, orderID := setupAfterSaleHandlerTest(t)
	// 第一次
	c1, _ := newTestContext(http.MethodPost, "/api/v1/orders/1/after-sale", map[string]string{
		"type": "not_received", "reason": "first",
	}, userID)
	setOrderIDParam(c1, orderID)
	handler.UserCreateAfterSale(c1)
	// 第二次
	c2, w2 := newTestContext(http.MethodPost, "/api/v1/orders/1/after-sale", map[string]string{
		"type": "not_received", "reason": "second",
	}, userID)
	setOrderIDParam(c2, orderID)
	handler.UserCreateAfterSale(c2)
	if code := respStatusCode(t, w2); code != http.StatusBadRequest {
		t.Fatalf("expected business code 400 for duplicate pending, got %d", code)
	}
}

// User GET 自己的售后
func TestAfterSaleUserGetSuccess(t *testing.T) {
	handler, _, userID, orderID := setupAfterSaleHandlerTest(t)
	// 先创建
	c1, _ := newTestContext(http.MethodPost, "/api/v1/orders/1/after-sale", map[string]string{
		"type": "not_received", "reason": "test",
	}, userID)
	setOrderIDParam(c1, orderID)
	handler.UserCreateAfterSale(c1)
	// GET
	c2, w2 := newTestContext(http.MethodGet, "/api/v1/orders/1/after-sale", nil, userID)
	setOrderIDParam(c2, orderID)
	handler.UserGetAfterSale(c2)
	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w2.Code)
	}
}

// 未登录 401
func TestAfterSaleUserUnauthorized(t *testing.T) {
	handler, _, _, orderID := setupAfterSaleHandlerTest(t)
	c, w := newTestContext(http.MethodPost, "/api/v1/orders/1/after-sale", map[string]string{
		"type": "not_received", "reason": "test",
	}, 0)
	setOrderIDParam(c, orderID)
	handler.UserCreateAfterSale(c)
	// 测试上下文下 RespondError 可能重复写 body，用 Contains 校验业务码
	if !strings.Contains(w.Body.String(), `"status_code":401`) {
		t.Fatalf("expected body to contain status_code 401, got %s", w.Body.String())
	}
}

// Admin reject
func TestAfterSaleAdminReject(t *testing.T) {
	handler, db, userID, orderID := setupAfterSaleHandlerTest(t)
	// 创建 pending
	c1, _ := newTestContext(http.MethodPost, "/api/v1/orders/1/after-sale", map[string]string{
		"type": "not_received", "reason": "test",
	}, userID)
	setOrderIDParam(c1, orderID)
	handler.UserCreateAfterSale(c1)
	// Admin reject
	c2, w2 := newTestContext(http.MethodPost, "/api/admin/v1/orders/1/after-sale/action", map[string]string{
		"action": "reject", "admin_note": "已核实",
	}, 0)
	setOrderIDParam(c2, orderID)
	handler.AdminAfterSaleAction(c2)
	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", w2.Code, w2.Body.String())
	}
	var order orderdomain.Order
	db.First(&order, orderID)
	if order.AfterSaleStatus != "rejected" {
		t.Fatalf("expected rejected, got %s", order.AfterSaleStatus)
	}
	if order.Status != constants.OrderStatusCompleted {
		t.Fatalf("order status must remain completed, got %s", order.Status)
	}
}

// Admin partial_refund — 真实退款链
func TestAfterSaleAdminPartialRefund(t *testing.T) {
	handler, db, userID, orderID := setupAfterSaleHandlerTest(t)
	// 创建 pending
	c1, _ := newTestContext(http.MethodPost, "/api/v1/orders/1/after-sale", map[string]string{
		"type": "not_received", "reason": "test",
	}, userID)
	setOrderIDParam(c1, orderID)
	handler.UserCreateAfterSale(c1)
	// Admin partial_refund
	c2, w2 := newTestContext(http.MethodPost, "/api/admin/v1/orders/1/after-sale/action", map[string]string{
		"action": "partial_refund", "refund_amount": "3.00",
	}, 0)
	setOrderIDParam(c2, orderID)
	handler.AdminAfterSaleAction(c2)
	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", w2.Code, w2.Body.String())
	}
	var order orderdomain.Order
	db.First(&order, orderID)
	if order.RefundStatus != "partial" {
		t.Fatalf("expected refund_status=partial, got %s", order.RefundStatus)
	}
	if order.Status != constants.OrderStatusCompleted {
		t.Fatalf("order status must remain completed, got %s", order.Status)
	}
	// wallet balance should increase by 3.00
	var acct walletdomain.Account
	db.Where("user_id = ?", userID).First(&acct)
	if !acct.Balance.Decimal.Equal(decimal.RequireFromString("103.00")) {
		t.Fatalf("wallet should be 103.00, got %s", acct.Balance.Decimal)
	}
}

// Admin full_refund
func TestAfterSaleAdminFullRefund(t *testing.T) {
	handler, db, userID, orderID := setupAfterSaleHandlerTest(t)
	c1, _ := newTestContext(http.MethodPost, "/api/v1/orders/1/after-sale", map[string]string{
		"type": "not_received", "reason": "test",
	}, userID)
	setOrderIDParam(c1, orderID)
	handler.UserCreateAfterSale(c1)
	c2, w2 := newTestContext(http.MethodPost, "/api/admin/v1/orders/1/after-sale/action", map[string]string{
		"action": "full_refund",
	}, 0)
	setOrderIDParam(c2, orderID)
	handler.AdminAfterSaleAction(c2)
	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", w2.Code, w2.Body.String())
	}
	var order orderdomain.Order
	db.First(&order, orderID)
	if order.RefundStatus != "full" {
		t.Fatalf("expected refund_status=full, got %s", order.RefundStatus)
	}
	var acct walletdomain.Account
	db.Where("user_id = ?", userID).First(&acct)
	if !acct.Balance.Decimal.Equal(decimal.RequireFromString("110.00")) {
		t.Fatalf("wallet should be 110.00, got %s", acct.Balance.Decimal)
	}
}

// Admin invalid action
func TestAfterSaleAdminInvalidAction(t *testing.T) {
	handler, _, userID, orderID := setupAfterSaleHandlerTest(t)
	c1, _ := newTestContext(http.MethodPost, "/api/v1/orders/1/after-sale", map[string]string{
		"type": "not_received", "reason": "test",
	}, userID)
	setOrderIDParam(c1, orderID)
	handler.UserCreateAfterSale(c1)
	c2, w2 := newTestContext(http.MethodPost, "/api/admin/v1/orders/1/after-sale/action", map[string]string{
		"action": "invalid_action",
	}, 0)
	setOrderIDParam(c2, orderID)
	handler.AdminAfterSaleAction(c2)
	if code := respStatusCode(t, w2); code != http.StatusBadRequest {
		t.Fatalf("expected business code 400, got %d, body=%s", code, w2.Body.String())
	}
}

// Admin GET
func TestAfterSaleAdminGet(t *testing.T) {
	handler, _, userID, orderID := setupAfterSaleHandlerTest(t)
	c1, _ := newTestContext(http.MethodPost, "/api/v1/orders/1/after-sale", map[string]string{
		"type": "not_received", "reason": "test",
	}, userID)
	setOrderIDParam(c1, orderID)
	handler.UserCreateAfterSale(c1)
	c2, w2 := newTestContext(http.MethodGet, "/api/admin/v1/orders/1/after-sale", nil, 0)
	setOrderIDParam(c2, orderID)
	handler.AdminGetAfterSale(c2)
	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w2.Code)
	}
}
