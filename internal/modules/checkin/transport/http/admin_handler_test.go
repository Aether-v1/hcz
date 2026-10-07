package checkinphttp

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	checkincontract "github.com/Aether-v1/hcz/internal/modules/checkin/contract"
	userdomain "github.com/Aether-v1/hcz/internal/modules/identity/user/domain"

	"github.com/gin-gonic/gin"
)

// adminFakeCheckinService 记录 Admin 侧调用的 userID/month。
type adminFakeCheckinService struct {
	history     *checkincontract.HistoryResult
	err         error
	lastUserID  uint
	lastMonth   string
	historyCall int
}

func (f *adminFakeCheckinService) History(userID uint, month string) (*checkincontract.HistoryResult, error) {
	f.lastUserID = userID
	f.lastMonth = month
	f.historyCall++
	if f.err != nil {
		return nil, f.err
	}
	if f.history == nil {
		return &checkincontract.HistoryResult{Year: 2026, Month: 10}, nil
	}
	return f.history, nil
}

type adminFakeUserReader struct {
	notFound bool
	err      error
}

func (f adminFakeUserReader) GetByID(id uint) (*userdomain.User, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.notFound || id == 0 {
		return nil, nil
	}
	return &userdomain.User{ID: id}, nil
}

// adminIDFromTestHeader 模拟 AdminJWTAuthMiddleware：admin_id 只来自上下文。
func adminIDFromTestHeader(c *gin.Context) {
	raw := c.GetHeader("X-Test-Admin-Id")
	if raw == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"status_code": 401, "msg": "unauthorized"})
		return
	}
	var aid uint
	if _, err := fmt.Sscanf(raw, "%d", &aid); err != nil || aid == 0 {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"status_code": 401, "msg": "unauthorized"})
		return
	}
	c.Set("admin_id", aid)
	c.Next()
}

func newAuthedAdminCheckinEngine(svc AdminCheckinService, users AdminUserReader) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterAdminRoutes(r.Group("/admin", adminIDFromTestHeader), NewAdminHandler(svc, users))
	return r
}

func doAdminRequest(r *gin.Engine, path, adminID string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if adminID != "" {
		req.Header.Set("X-Test-Admin-Id", adminID)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestCheckinAdmin_historyReturnsReadOnlyAggregate(t *testing.T) {
	svc := &adminFakeCheckinService{history: &checkincontract.HistoryResult{
		Year: 2026, Month: 10, Total: 2,
		CheckedDates: []string{"2026-10-01", "2026-10-02"},
		Entries: []checkincontract.HistoryEntry{
			{Date: "2026-10-01", PointsAwarded: 1, ConsecutiveDays: 1, CycleDay: 1},
			{Date: "2026-10-02", PointsAwarded: 2, ConsecutiveDays: 2, CycleDay: 2},
		},
	}}
	r := newAuthedAdminCheckinEngine(svc, adminFakeUserReader{})
	w := doAdminRequest(r, "/admin/users/42/checkins?month=2026-10", "9")
	if w.Code != http.StatusOK {
		t.Fatalf("http=%d body=%s", w.Code, w.Body.String())
	}
	if svc.lastUserID != 42 || svc.lastMonth != "2026-10" {
		t.Fatalf("unexpected call: user=%d month=%q", svc.lastUserID, svc.lastMonth)
	}
	body := w.Body.String()
	for _, want := range []string{`"user_id":42`, `"latest_consecutive_days":2`, `"total":2`} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %s: %s", want, body)
		}
	}
}

func TestCheckinAdmin_emptyMonthStreakIsZero(t *testing.T) {
	svc := &adminFakeCheckinService{}
	r := newAuthedAdminCheckinEngine(svc, adminFakeUserReader{})
	w := doAdminRequest(r, "/admin/users/7/checkins", "9")
	if !strings.Contains(w.Body.String(), `"latest_consecutive_days":0`) {
		t.Fatalf("empty history must report zero streak, body=%s", w.Body.String())
	}
	if svc.lastMonth != "" {
		t.Fatalf("empty month should fall back to service default, got %q", svc.lastMonth)
	}
}

func TestCheckinAdmin_invalidMonthIs400(t *testing.T) {
	svc := &adminFakeCheckinService{err: checkincontract.ErrInvalidMonth}
	r := newAuthedAdminCheckinEngine(svc, adminFakeUserReader{})
	w := doAdminRequest(r, "/admin/users/7/checkins?month=2026-13", "9")
	if !strings.Contains(w.Body.String(), `"status_code":400`) {
		t.Fatalf("want business 400, body=%s", w.Body.String())
	}
}

func TestCheckinAdmin_unknownUserIs404(t *testing.T) {
	svc := &adminFakeCheckinService{}
	r := newAuthedAdminCheckinEngine(svc, adminFakeUserReader{notFound: true})
	w := doAdminRequest(r, "/admin/users/404/checkins", "9")
	if !strings.Contains(w.Body.String(), `"status_code":404`) {
		t.Fatalf("want business 404, body=%s", w.Body.String())
	}
	if svc.historyCall != 0 {
		t.Fatalf("must not query history for nonexistent user")
	}
}

func TestCheckinAdmin_invalidIDParamIs400(t *testing.T) {
	svc := &adminFakeCheckinService{}
	r := newAuthedAdminCheckinEngine(svc, adminFakeUserReader{})
	for _, path := range []string{"/admin/users/abc/checkins", "/admin/users/-1/checkins", "/admin/users/0/checkins"} {
		w := doAdminRequest(r, path, "9")
		if !strings.Contains(w.Body.String(), `"status_code":400`) {
			t.Fatalf("%s: want business 400, body=%s", path, w.Body.String())
		}
	}
}

func TestCheckinAdmin_requiresAdminIdentity(t *testing.T) {
	svc := &adminFakeCheckinService{}
	r := newAuthedAdminCheckinEngine(svc, adminFakeUserReader{})
	w := doAdminRequest(r, "/admin/users/7/checkins", "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d body=%s", w.Code, w.Body.String())
	}
	if svc.historyCall != 0 {
		t.Fatalf("unauthenticated request must not reach service")
	}
}

// TestCheckinAdmin_noWriteRoutes V1 后台签到能力是只读的：
// 任何补签/删除/改期入口都必须不存在（防止伪造签到奖励）。
func TestCheckinAdmin_noWriteRoutes(t *testing.T) {
	svc := &adminFakeCheckinService{}
	r := newAuthedAdminCheckinEngine(svc, adminFakeUserReader{})
	for _, tc := range []struct{ method, path string }{
		{http.MethodPost, "/admin/users/7/checkins"},
		{http.MethodPut, "/admin/users/7/checkins"},
		{http.MethodPatch, "/admin/users/7/checkins"},
		{http.MethodDelete, "/admin/users/7/checkins"},
		{http.MethodPost, "/admin/users/7/checkins/manual"},
		{http.MethodDelete, "/admin/checkins/1"},
	} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		req.Header.Set("X-Test-Admin-Id", "9")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusNotFound {
			t.Fatalf("%s %s: admin check-in writes must not be routed, got %d", tc.method, tc.path, w.Code)
		}
	}
}
