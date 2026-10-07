package checkinphttp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	checkincontract "github.com/Aether-v1/hcz/internal/modules/checkin/contract"

	"github.com/gin-gonic/gin"
)

// fakeCheckinService 记录调用参数，用于验证路由、幂等语义与 IDOR 防御。
type fakeCheckinService struct {
	statusResult  *checkincontract.StatusResult
	checkinResult *checkincontract.CheckinResult
	historyResult *checkincontract.HistoryResult
	lastUserID    uint
	lastMonth     string
	checkinErr    error
	historyErr    error
}

func (f *fakeCheckinService) CheckIn(userID uint) (*checkincontract.CheckinResult, error) {
	f.lastUserID = userID
	if f.checkinErr != nil {
		return nil, f.checkinErr
	}
	return f.checkinResult, nil
}

func (f *fakeCheckinService) Status(userID uint) (*checkincontract.StatusResult, error) {
	f.lastUserID = userID
	if f.statusResult == nil {
		f.statusResult = &checkincontract.StatusResult{}
	}
	return f.statusResult, nil
}

func (f *fakeCheckinService) History(userID uint, month string) (*checkincontract.HistoryResult, error) {
	f.lastUserID = userID
	f.lastMonth = month
	if f.historyErr != nil {
		return nil, f.historyErr
	}
	return f.historyResult, nil
}

// userIDFromTestHeader 模拟 UserJWTAuthMiddleware：user_id 只来自 JWT 上下文。
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

func newAuthedCheckinEngine(f *fakeCheckinService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewUserHandler(f)
	g := r.Group("", userIDFromTestHeader)
	RegisterUserRoutes(g, h)
	return r
}

func doRequest(r *gin.Engine, method, path, userID string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	if userID != "" {
		req.Header.Set("X-Test-User-Id", userID)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// TestCheckinHTTP_Status 正常状态返回字段完整。
func TestCheckinHTTP_Status(t *testing.T) {
	f := &fakeCheckinService{statusResult: &checkincontract.StatusResult{
		Enabled: true, CheckedInToday: false,
		ConsecutiveDays: 4, CycleDay: 4, TodayReward: 5, NextReward: 6,
	}}
	r := newAuthedCheckinEngine(f)
	w := doRequest(r, http.MethodGet, "/checkin/status", "42")
	if w.Code != http.StatusOK {
		t.Fatalf("status code: %d body=%s", w.Code, w.Body.String())
	}
	var body struct {
		Data struct {
			Enabled         bool  `json:"enabled"`
			CheckedInToday  bool  `json:"checked_in_today"`
			ConsecutiveDays int   `json:"consecutive_days"`
			CycleDay        int   `json:"cycle_day"`
			TodayReward     int64 `json:"today_reward"`
			NextReward      int64 `json:"next_reward"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, w.Body.String())
	}
	if body.Data.Enabled != true || body.Data.CheckedInToday || body.Data.ConsecutiveDays != 4 ||
		body.Data.CycleDay != 4 || body.Data.TodayReward != 5 || body.Data.NextReward != 6 {
		t.Fatalf("unexpected status body: %+v", body.Data)
	}
	if f.lastUserID != 42 {
		t.Fatalf("IDOR: handler must use JWT user id (42), got %d", f.lastUserID)
	}
}

// TestCheckinHTTP_CheckIn 签到成功返回完整字段。
func TestCheckinHTTP_CheckIn(t *testing.T) {
	f := &fakeCheckinService{checkinResult: &checkincontract.CheckinResult{
		CheckinDate: "2026-10-01", PointsAwarded: 1, ConsecutiveDays: 1, CycleDay: 1,
		CurrentBalance: 1, AlreadyCheckedIn: false,
	}}
	r := newAuthedCheckinEngine(f)
	w := doRequest(r, http.MethodPost, "/checkin", "7")
	if w.Code != http.StatusOK {
		t.Fatalf("status code: %d body=%s", w.Code, w.Body.String())
	}
	var body struct {
		Data struct {
			CheckinDate      string `json:"checkin_date"`
			PointsAwarded    int64  `json:"points_awarded"`
			ConsecutiveDays  int    `json:"consecutive_days"`
			CycleDay         int    `json:"cycle_day"`
			CurrentBalance   int64  `json:"current_balance"`
			AlreadyCheckedIn bool   `json:"already_checked_in"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, w.Body.String())
	}
	if body.Data.CheckinDate != "2026-10-01" || body.Data.PointsAwarded != 1 ||
		body.Data.ConsecutiveDays != 1 || body.Data.CycleDay != 1 || body.Data.CurrentBalance != 1 {
		t.Fatalf("unexpected checkin body: %+v", body.Data)
	}
	if f.lastUserID != 7 {
		t.Fatalf("IDOR: handler must use JWT user id (7), got %d", f.lastUserID)
	}
}

// TestCheckinHTTP_DuplicateIsNotError 重复签到（already_checked_in=true）返回 200 而非 5xx。
func TestCheckinHTTP_DuplicateIsNotError(t *testing.T) {
	f := &fakeCheckinService{checkinResult: &checkincontract.CheckinResult{
		CheckinDate: "2026-10-01", PointsAwarded: 1, ConsecutiveDays: 1, CycleDay: 1,
		CurrentBalance: 1, AlreadyCheckedIn: true,
	}}
	r := newAuthedCheckinEngine(f)
	w := doRequest(r, http.MethodPost, "/checkin", "7")
	if w.Code != http.StatusOK {
		t.Fatalf("duplicate check-in must be HTTP 200, got %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"already_checked_in":true`) {
		t.Fatalf("expected already_checked_in=true, body=%s", w.Body.String())
	}
}

// TestCheckinHTTP_Disabled 功能关闭时签到返回业务错误（非 500）。
func TestCheckinHTTP_Disabled(t *testing.T) {
	f := &fakeCheckinService{checkinErr: checkincontract.ErrCheckinDisabled}
	r := newAuthedCheckinEngine(f)
	w := doRequest(r, http.MethodPost, "/checkin", "7")
	if w.Code != http.StatusOK {
		t.Fatalf("HCZ returns HTTP 200 with business code, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"status_code":400`) {
		t.Fatalf("expected business status 400, body=%s", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "签到") {
		t.Fatalf("expected localized disabled message, body=%s", w.Body.String())
	}
}

// TestCheckinHTTP_History 历史返回日历结构 + 非法月份业务错误。
func TestCheckinHTTP_History(t *testing.T) {
	f := &fakeCheckinService{historyResult: &checkincontract.HistoryResult{
		Year: 2026, Month: 10, Total: 1,
		CheckedDates: []string{"2026-10-01"},
		Entries:      []checkincontract.HistoryEntry{{Date: "2026-10-01", PointsAwarded: 1, ConsecutiveDays: 1, CycleDay: 1}},
	}}
	r := newAuthedCheckinEngine(f)
	w := doRequest(r, http.MethodGet, "/checkin/history?month=2026-10", "42")
	if w.Code != http.StatusOK {
		t.Fatalf("status code: %d", w.Code)
	}
	if f.lastMonth != "2026-10" {
		t.Fatalf("month param not forwarded: %q", f.lastMonth)
	}
	if !strings.Contains(w.Body.String(), `"checked_dates"`) {
		t.Fatalf("expected checked_dates in body, got %s", w.Body.String())
	}

	f.historyErr = checkincontract.ErrInvalidMonth
	w = doRequest(r, http.MethodGet, "/checkin/history?month=2026-13", "42")
	if !strings.Contains(w.Body.String(), `"status_code":400`) {
		t.Fatalf("expected business status 400 for invalid month, body=%s", w.Body.String())
	}
}

// TestCheckinHTTP_RequiresAuth 无 JWT 上下文 → 401。
func TestCheckinHTTP_RequiresAuth(t *testing.T) {
	f := &fakeCheckinService{}
	r := newAuthedCheckinEngine(f)
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/checkin/status"},
		{http.MethodPost, "/checkin"},
		{http.MethodGet, "/checkin/history"},
	} {
		w := doRequest(r, tc.method, tc.path, "")
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s: want 401, got %d", tc.method, tc.path, w.Code)
		}
	}
}

// TestCheckinHTTP_NoUserIDInRequest 用户 API 不接受 user_id 参数（IDOR 防御：
// 伪造 X-Test-User-Id 之外的 query/path user_id 被忽略，仅 JWT 上下文生效）。
func TestCheckinHTTP_NoUserIDInRequest(t *testing.T) {
	f := &fakeCheckinService{statusResult: &checkincontract.StatusResult{Enabled: true}}
	r := newAuthedCheckinEngine(f)
	// 携带 ?user_id=999 请求，handler 必须仍使用 JWT 用户 42
	w := doRequest(r, http.MethodGet, "/checkin/status?user_id=999", "42")
	if w.Code != http.StatusOK {
		t.Fatalf("status code: %d", w.Code)
	}
	if f.lastUserID != 42 {
		t.Fatalf("IDOR: handler must ignore client user_id and use JWT (42), got %d", f.lastUserID)
	}
}
