package pointsphttp

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	userdomain "github.com/Aether-v1/hcz/internal/modules/identity/user/domain"
	pointscontract "github.com/Aether-v1/hcz/internal/modules/points/contract"
	pointsdomain "github.com/Aether-v1/hcz/internal/modules/points/domain"

	"github.com/gin-gonic/gin"
)

// fakeAdminPointsService 是后台积分服务的测试替身。
type fakeAdminPointsService struct {
	account      *pointsdomain.Account
	entries      []pointsdomain.LedgerEntry
	total        int64
	lastInput    pointscontract.AdjustInput
	lastComp     pointscontract.CompensateInput
	lastFilter   pointscontract.LedgerListFilter
	lastAccounts pointscontract.AccountListFilter
	stats        *pointscontract.StatsResult
	statsErr     error
	err          error
	adjustErr    error
	lastUserID   uint
}

func (f *fakeAdminPointsService) GetAccount(userID uint) (*pointsdomain.Account, error) {
	f.lastUserID = userID
	if f.err != nil {
		return nil, f.err
	}
	if f.account == nil {
		return nil, nil
	}
	copy := *f.account
	return &copy, nil
}

func (f *fakeAdminPointsService) ListLedgerEntries(filter pointscontract.LedgerListFilter) ([]pointsdomain.LedgerEntry, int64, error) {
	f.lastUserID = filter.UserID
	f.lastFilter = filter
	if f.err != nil {
		return nil, 0, f.err
	}
	return f.entries, f.total, nil
}

func (f *fakeAdminPointsService) ListAccounts(filter pointscontract.AccountListFilter) ([]pointscontract.AccountWithUser, int64, error) {
	f.lastAccounts = filter
	if f.err != nil {
		return nil, 0, f.err
	}
	return []pointscontract.AccountWithUser{{UserID: 3, Balance: -50}}, 1, nil
}

func (f *fakeAdminPointsService) AdminAdjust(input pointscontract.AdjustInput) (*pointsdomain.Account, *pointsdomain.LedgerEntry, error) {
	f.lastInput = input
	if f.adjustErr != nil {
		return nil, nil, f.adjustErr
	}
	if f.account == nil {
		f.account = &pointsdomain.Account{UserID: input.UserID, Balance: 100}
	}
	return f.account, &pointsdomain.LedgerEntry{ID: 1, UserID: input.UserID}, nil
}

func (f *fakeAdminPointsService) AdminCompensate(input pointscontract.CompensateInput) (*pointsdomain.Account, *pointsdomain.LedgerEntry, error) {
	f.lastComp = input
	if f.adjustErr != nil {
		return nil, nil, f.adjustErr
	}
	if f.account == nil {
		f.account = &pointsdomain.Account{UserID: input.UserID, Balance: 100}
	}
	return f.account, &pointsdomain.LedgerEntry{ID: 2, UserID: input.UserID}, nil
}

func (f *fakeAdminPointsService) GetStats(_ time.Time) (*pointscontract.StatsResult, error) {
	if f.statsErr != nil {
		return nil, f.statsErr
	}
	if f.stats == nil {
		return &pointscontract.StatsResult{TotalBalance: 500, TodayGranted: 10}, nil
	}
	return f.stats, nil
}

type fakeAdminUserReader struct{}

func (fakeAdminUserReader) GetByID(id uint) (*userdomain.User, error) {
	if id == 999 {
		return nil, nil
	}
	return &userdomain.User{ID: id}, nil
}

func adminIDFromTestHeader(c *gin.Context) {
	raw := c.GetHeader("X-Test-Admin-Id")
	if raw == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"status_code": 401, "msg": "unauthorized"})
		return
	}
	var adminID uint
	if err := json.Unmarshal([]byte(raw), &adminID); err != nil || adminID == 0 {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"status_code": 401, "msg": "unauthorized"})
		return
	}
	c.Set("admin_id", adminID)
	c.Next()
}

// newAuthedAdminEngine 与生产装配同构：RegisterAdminRoutes 挂在 /admin 组下，
// 路由路径必须是相对形式（否则会以 /admin/admin/... 挂载且不被 RBAC 覆盖）。
func newAuthedAdminEngine(f *fakeAdminPointsService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewAdminHandler(f, fakeAdminUserReader{})
	RegisterAdminRoutes(r.Group("/admin", adminIDFromTestHeader), h)
	return r
}

func adminBodyStatusCode(t *testing.T, body []byte) int {
	t.Helper()
	var resp struct {
		StatusCode int `json:"status_code"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("decode status_code: %v body=%s", err, body)
	}
	return resp.StatusCode
}

func TestAdminAdjustRequiresAdminContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewAdminHandler(&fakeAdminPointsService{}, fakeAdminUserReader{})
	RegisterAdminRoutes(r.Group("/admin"), h)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/users/1/points/adjust", nil)
	r.ServeHTTP(w, req)

	if code := adminBodyStatusCode(t, w.Body.Bytes()); code != 401 {
		t.Fatalf("expected business code 401 without admin context, got %d body=%s", code, w.Body.String())
	}
}

func TestAdminAdjustValidations(t *testing.T) {
	cases := []struct {
		name       string
		body       string
		idemKey    *string
		wantStatus int
	}{
		{"missing idempotency key", `{"amount":100,"operation":"add","reason":"r"}`, nil, 400},
		{"missing reason", `{"amount":100,"operation":"add"}`, strPtr("k-2"), 400},
		{"zero amount rejected by required", `{"amount":0,"operation":"add","reason":"r"}`, strPtr("k-3"), 400},
		{"bad operation", `{"amount":100,"operation":"double","reason":"r"}`, strPtr("k-4"), 400},
		{"overlong idempotency key", `{"amount":100,"operation":"add","reason":"r"}`, strPtr(string(bytes.Repeat([]byte("k"), pointscontract.MaxIdempotencyKeyLength+1))), 400},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeAdminPointsService{}
			r := newAuthedAdminEngine(f)
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/admin/users/1/points/adjust", bytes.NewBufferString(tc.body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Test-Admin-Id", "9")
			if tc.idemKey != nil {
				req.Header.Set("Idempotency-Key", *tc.idemKey)
			}
			r.ServeHTTP(w, req)
			if code := adminBodyStatusCode(t, w.Body.Bytes()); code != tc.wantStatus {
				t.Fatalf("want status %d, got %d body=%s", tc.wantStatus, code, w.Body.String())
			}
		})
	}
}

func strPtr(s string) *string { return &s }

func TestAdminAdjustIdempotencyConflictReturns409(t *testing.T) {
	f := &fakeAdminPointsService{adjustErr: pointscontract.ErrIdempotencyConflict}
	r := newAuthedAdminEngine(f)
	w := httptest.NewRecorder()
	body := `{"amount":100,"operation":"add","reason":"dup"}`
	req := httptest.NewRequest(http.MethodPost, "/admin/users/1/points/adjust", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-Admin-Id", "9")
	req.Header.Set("Idempotency-Key", "dup-key")
	r.ServeHTTP(w, req)
	if code := adminBodyStatusCode(t, w.Body.Bytes()); code != 409 {
		t.Fatalf("expected 409, got %d body=%s", code, w.Body.String())
	}
}

func TestAdminAdjustSuccessPassesReferenceAndOperation(t *testing.T) {
	f := &fakeAdminPointsService{}
	r := newAuthedAdminEngine(f)
	w := httptest.NewRecorder()
	body := `{"amount":100,"operation":"subtract","reason":"penalty"}`
	req := httptest.NewRequest(http.MethodPost, "/admin/users/5/points/adjust", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-Admin-Id", "9")
	req.Header.Set("Idempotency-Key", "adj-1")
	r.ServeHTTP(w, req)

	if code := adminBodyStatusCode(t, w.Body.Bytes()); code != 0 {
		t.Fatalf("expected success, got %d body=%s", code, w.Body.String())
	}
	if f.lastInput.UserID != 5 || f.lastInput.OperatorAdminID != 9 {
		t.Fatalf("unexpected adjust input: %+v", f.lastInput)
	}
	if f.lastInput.Operation != "subtract" || f.lastInput.Amount != 100 {
		t.Fatalf("amount/operation must pass through: %+v", f.lastInput)
	}
	if f.lastInput.Reference != pointscontract.AdminAdjustReference("adj-1") {
		t.Fatalf("reference must derive from Idempotency-Key: %q", f.lastInput.Reference)
	}
}

func TestAdminAmountTooLargeMapsToBadRequest(t *testing.T) {
	f := &fakeAdminPointsService{adjustErr: pointscontract.ErrAmountTooLarge}
	r := newAuthedAdminEngine(f)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/users/5/points/adjust", bytes.NewBufferString(`{"amount":99999999999,"operation":"add","reason":"oops"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-Admin-Id", "9")
	req.Header.Set("Idempotency-Key", "big-1")
	r.ServeHTTP(w, req)
	if code := adminBodyStatusCode(t, w.Body.Bytes()); code != 400 {
		t.Fatalf("expected 400 for over-cap amount, got %d body=%s", code, w.Body.String())
	}
}

func TestAdminCompensateUsesOwnActionAndReference(t *testing.T) {
	f := &fakeAdminPointsService{}
	r := newAuthedAdminEngine(f)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/users/7/points/compensate", bytes.NewBufferString(`{"amount":300,"reason":"漏发补发","order_id":88}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-Admin-Id", "9")
	req.Header.Set("Idempotency-Key", "cmp-1")
	r.ServeHTTP(w, req)

	if code := adminBodyStatusCode(t, w.Body.Bytes()); code != 0 {
		t.Fatalf("expected success, got %d body=%s", code, w.Body.String())
	}
	if f.lastComp.UserID != 7 || f.lastComp.OperatorAdminID != 9 || f.lastComp.Amount != 300 {
		t.Fatalf("unexpected compensate input: %+v", f.lastComp)
	}
	if f.lastComp.Reference != pointscontract.AdminCompensationReference("cmp-1") {
		t.Fatalf("reference must use the compensation namespace: %q", f.lastComp.Reference)
	}
	// 补偿绝不复用订单奖励的 reference/action 语义。
	if f.lastComp.Reference == pointscontract.AdminAdjustReference("cmp-1") {
		t.Fatalf("compensation reference must not collide with admin adjust")
	}
}

func TestAdminCompensateRequiresIdempotencyKeyAndReason(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		idemKey string
	}{
		{"missing reason", `{"amount":10}`, "cmp-2"},
		{"missing idempotency key", `{"amount":10,"reason":"x"}`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeAdminPointsService{}
			r := newAuthedAdminEngine(f)
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/admin/users/7/points/compensate", bytes.NewBufferString(tc.body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Test-Admin-Id", "9")
			if tc.idemKey != "" {
				req.Header.Set("Idempotency-Key", tc.idemKey)
			}
			r.ServeHTTP(w, req)
			if code := adminBodyStatusCode(t, w.Body.Bytes()); code != 400 {
				t.Fatalf("want 400 for %s, got %d body=%s", tc.name, code, w.Body.String())
			}
		})
	}
}

func TestAdminGetUserPointsNotFound(t *testing.T) {
	f := &fakeAdminPointsService{}
	r := newAuthedAdminEngine(f)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/users/999/points", nil)
	req.Header.Set("X-Test-Admin-Id", "9")
	r.ServeHTTP(w, req)
	if code := adminBodyStatusCode(t, w.Body.Bytes()); code != 404 {
		t.Fatalf("expected 404 for missing user, got %d", code)
	}
}

func TestAdminGetUserPointsUsesPathParam(t *testing.T) {
	f := &fakeAdminPointsService{account: &pointsdomain.Account{UserID: 8, Balance: 25, TotalEarned: 100, TotalSpent: 75}}
	r := newAuthedAdminEngine(f)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/users/8/points", nil)
	req.Header.Set("X-Test-Admin-Id", "9")
	r.ServeHTTP(w, req)
	if code := adminBodyStatusCode(t, w.Body.Bytes()); code != 0 {
		t.Fatalf("expected success, got %d body=%s", code, w.Body.String())
	}
	if f.lastUserID != 8 {
		t.Fatalf("admin query must use path param user id, got %d", f.lastUserID)
	}
}

func TestAdminLedgerFilterPassesWhitelist(t *testing.T) {
	f := &fakeAdminPointsService{}
	r := newAuthedAdminEngine(f)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet,
		"/admin/users/8/points/ledger?action_type=ORDER_REWARD&source_type=order_reward&direction=income&page=2&page_size=50", nil)
	req.Header.Set("X-Test-Admin-Id", "9")
	r.ServeHTTP(w, req)
	if code := adminBodyStatusCode(t, w.Body.Bytes()); code != 0 {
		t.Fatalf("expected success, got %d body=%s", code, w.Body.String())
	}
	if f.lastFilter.UserID != 8 || f.lastFilter.ActionType != pointscontract.ActionOrderReward {
		t.Fatalf("unexpected filter: %+v", f.lastFilter)
	}
	if f.lastFilter.SourceType != pointscontract.SourceOrderReward || f.lastFilter.Direction != pointscontract.DirectionIncome {
		t.Fatalf("unexpected filter: %+v", f.lastFilter)
	}
	if f.lastFilter.Page != 2 || f.lastFilter.PageSize != 50 {
		t.Fatalf("pagination must pass through: %+v", f.lastFilter)
	}
}

func TestAdminLedgerFilterRejectsUnknownActionType(t *testing.T) {
	f := &fakeAdminPointsService{}
	r := newAuthedAdminEngine(f)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/users/8/points/ledger?action_type=DROP_TABLE", nil)
	req.Header.Set("X-Test-Admin-Id", "9")
	r.ServeHTTP(w, req)
	if code := adminBodyStatusCode(t, w.Body.Bytes()); code != 400 {
		t.Fatalf("unknown action_type must be rejected, got %d", code)
	}
}

func TestAdminLedgerFilterRejectsInvalidTimeRange(t *testing.T) {
	f := &fakeAdminPointsService{}
	r := newAuthedAdminEngine(f)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/users/8/points/ledger?created_from=2026-13-45", nil)
	req.Header.Set("X-Test-Admin-Id", "9")
	r.ServeHTTP(w, req)
	if code := adminBodyStatusCode(t, w.Body.Bytes()); code != 400 {
		t.Fatalf("invalid RFC3339 range must be rejected, got %d", code)
	}
}

func TestAdminLedgerFilterAcceptsValidTimeRange(t *testing.T) {
	f := &fakeAdminPointsService{}
	r := newAuthedAdminEngine(f)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet,
		"/admin/users/8/points/ledger?created_from=2026-10-01T00:00:00Z&created_to=2026-10-02T00:00:00Z", nil)
	req.Header.Set("X-Test-Admin-Id", "9")
	r.ServeHTTP(w, req)
	if code := adminBodyStatusCode(t, w.Body.Bytes()); code != 0 {
		t.Fatalf("expected success, got %d body=%s", code, w.Body.String())
	}
	if f.lastFilter.CreatedFrom.IsZero() || f.lastFilter.CreatedTo.IsZero() {
		t.Fatalf("time range must reach the filter: %+v", f.lastFilter)
	}
}

func TestAdminListAccountsNegativeOnly(t *testing.T) {
	f := &fakeAdminPointsService{}
	r := newAuthedAdminEngine(f)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/points/accounts?negative_only=true&page=1&page_size=20", nil)
	req.Header.Set("X-Test-Admin-Id", "9")
	r.ServeHTTP(w, req)
	if code := adminBodyStatusCode(t, w.Body.Bytes()); code != 0 {
		t.Fatalf("expected success, got %d body=%s", code, w.Body.String())
	}
	if !f.lastAccounts.NegativeOnly {
		t.Fatalf("negative_only must reach the filter: %+v", f.lastAccounts)
	}
}

func TestAdminGetStatsSuccess(t *testing.T) {
	f := &fakeAdminPointsService{}
	r := newAuthedAdminEngine(f)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/points/stats", nil)
	req.Header.Set("X-Test-Admin-Id", "9")
	r.ServeHTTP(w, req)
	if code := adminBodyStatusCode(t, w.Body.Bytes()); code != 0 {
		t.Fatalf("expected success, got %d body=%s", code, w.Body.String())
	}
}

func TestAdminGetStatsFailureIs500(t *testing.T) {
	f := &fakeAdminPointsService{statsErr: pointscontract.ErrStatsSourceRequired}
	r := newAuthedAdminEngine(f)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/points/stats", nil)
	req.Header.Set("X-Test-Admin-Id", "9")
	r.ServeHTTP(w, req)
	if code := adminBodyStatusCode(t, w.Body.Bytes()); code != 500 {
		t.Fatalf("stats wiring failure must surface as 500, got %d", code)
	}
}
