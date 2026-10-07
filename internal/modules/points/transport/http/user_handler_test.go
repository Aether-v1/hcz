package pointsphttp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	pointscontract "github.com/Aether-v1/hcz/internal/modules/points/contract"
	pointsdomain "github.com/Aether-v1/hcz/internal/modules/points/domain"

	"github.com/gin-gonic/gin"
)

// fakePointsService 记录调用参数，用于验证 IDOR 与上下文取 ID 语义。
type fakePointsService struct {
	account    *pointsdomain.Account
	entries    []pointsdomain.LedgerEntry
	total      int64
	lastUserID uint
	lastFilter pointscontract.LedgerListFilter
	err        error
}

func (f *fakePointsService) GetAccount(userID uint) (*pointsdomain.Account, error) {
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

func (f *fakePointsService) ListLedgerEntries(filter pointscontract.LedgerListFilter) ([]pointsdomain.LedgerEntry, int64, error) {
	f.lastUserID = filter.UserID
	f.lastFilter = filter
	if f.err != nil {
		return nil, 0, f.err
	}
	return f.entries, f.total, nil
}

// userIDFromTestHeader 模拟 UserJWTAuthMiddleware：从 JWT 注入 user_id 到上下文。
// 真实生产链路中 user_id 只来自 JWT 且 handler 不接受客户端传入的 user_id。
func userIDFromTestHeader(c *gin.Context) {
	raw := c.GetHeader("X-Test-User-Id")
	if raw == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"status_code": 401, "msg": "unauthorized"})
		return
	}
	var uid uint
	if _, err := fmt.Sscanf(raw, "%d", &uid); err != nil || uid == 0 {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"status_code": 401, "msg": "unauthorized"})
		return
	}
	c.Set("user_id", uid)
	c.Next()
}

func newAuthedUserEngine(f *fakePointsService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewUserHandler(f)
	g := r.Group("", userIDFromTestHeader)
	RegisterUserRoutes(g, h)
	return r
}

// bodyStatusCode 解析 HCZ 响应体中的业务状态码（HTTP 状态恒为 200）。
func bodyStatusCode(t *testing.T, body []byte) int {
	t.Helper()
	var resp struct {
		StatusCode int `json:"status_code"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("decode status_code: %v body=%s", err, body)
	}
	return resp.StatusCode
}

func TestUserGetAccountRequiresAuthContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewUserHandler(&fakePointsService{})
	RegisterUserRoutes(r.Group(""), h) // 无中间件 → 无 user_id 上下文

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/points/account", nil)
	r.ServeHTTP(w, req)

	if code := bodyStatusCode(t, w.Body.Bytes()); code != 401 {
		t.Fatalf("expected business code 401 without user context, got %d body=%s", code, w.Body.String())
	}
}

func TestUserGetAccountReturnsZeroValueWhenNoAccount(t *testing.T) {
	f := &fakePointsService{}
	r := newAuthedUserEngine(f)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/points/account", nil)
	req.Header.Set("X-Test-User-Id", "42")
	r.ServeHTTP(w, req)

	if code := bodyStatusCode(t, w.Body.Bytes()); code != 0 {
		t.Fatalf("expected success (status_code 0), got %d body=%s", code, w.Body.String())
	}
	var resp struct {
		Data struct {
			Balance     int64 `json:"balance"`
			TotalEarned int64 `json:"total_earned"`
			TotalSpent  int64 `json:"total_spent"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v body=%s", err, w.Body.String())
	}
	if resp.Data.Balance != 0 || resp.Data.TotalEarned != 0 || resp.Data.TotalSpent != 0 {
		t.Fatalf("expected zero-value account view, got %+v", resp.Data)
	}
	if f.lastUserID != 42 {
		t.Fatalf("user id must come from auth context, got %d", f.lastUserID)
	}
}

func TestUserLedgerScopedToAuthContextUserID(t *testing.T) {
	f := &fakePointsService{total: 0, entries: []pointsdomain.LedgerEntry{}}
	r := newAuthedUserEngine(f)

	w := httptest.NewRecorder()
	// 客户端即使伪造 user_id 查询参数也必须被忽略（user_id 只来自 JWT）。
	req := httptest.NewRequest(http.MethodGet, "/points/ledger?user_id=999&page=2&page_size=15", nil)
	req.Header.Set("X-Test-User-Id", "7")
	r.ServeHTTP(w, req)

	if code := bodyStatusCode(t, w.Body.Bytes()); code != 0 {
		t.Fatalf("expected success, got %d", code)
	}
	if f.lastUserID != 7 {
		t.Fatalf("IDOR guard: ledger must be scoped to auth user, got userID=%d", f.lastUserID)
	}
	if f.lastFilter.Page != 2 || f.lastFilter.PageSize != 15 {
		t.Fatalf("pagination must pass through: %+v", f.lastFilter)
	}
}
