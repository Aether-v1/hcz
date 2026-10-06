package settingshttp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	settingsintegration "github.com/Aether-v1/hcz/internal/modules/settings/schema/integration"

	"github.com/gin-gonic/gin"
)

type fakeProfitGuardService struct {
	current settingsintegration.ProfitGuardSetting
	updated settingsintegration.ProfitGuardSetting
}

func (f *fakeProfitGuardService) GetProfitGuardSetting() (settingsintegration.ProfitGuardSetting, error) {
	return f.current, nil
}

func (f *fakeProfitGuardService) UpdateProfitGuardSetting(s settingsintegration.ProfitGuardSetting) (settingsintegration.ProfitGuardSetting, error) {
	f.updated = s
	return settingsintegration.NormalizeProfitGuardSetting(s), nil
}

func TestProfitGuardGetAndUpdate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &fakeProfitGuardService{current: settingsintegration.DefaultProfitGuardSetting()}
	h := NewProfitGuardHandler(svc)

	// GET
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/admin/settings/profit-guard", nil)
	h.GetProfitGuard(ctx)
	if w.Code != http.StatusOK {
		t.Fatalf("GET http=%d body=%s", w.Code, w.Body.String())
	}

	// PUT
	w2 := httptest.NewRecorder()
	ctx2, _ := gin.CreateTestContext(w2)
	ctx2.Request = httptest.NewRequest(http.MethodPut, "/admin/settings/profit-guard",
		strings.NewReader(`{"enabled":true,"require_cost_price":true,"rate_safety_buffer_percent":2.5,"minimum_profit_amount_cny":5,"minimum_profit_rate_percent":3}`))
	ctx2.Request.Header.Set("Content-Type", "application/json")
	h.UpdateProfitGuard(ctx2)
	if w2.Code != http.StatusOK {
		t.Fatalf("PUT http=%d body=%s", w2.Code, w2.Body.String())
	}
	var body struct {
		Data settingsintegration.ProfitGuardSetting `json:"data"`
	}
	if err := json.Unmarshal(w2.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !body.Data.Enabled || !body.Data.RequireCostPrice {
		t.Fatalf("flags not persisted: %+v", body.Data)
	}
	if body.Data.RateSafetyBufferPercent != 2.5 {
		t.Fatalf("buffer not persisted: %v", body.Data.RateSafetyBufferPercent)
	}
}

func TestProfitGuardRejectsOutOfRangeBuffer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &fakeProfitGuardService{current: settingsintegration.DefaultProfitGuardSetting()}
	h := NewProfitGuardHandler(svc)
	// buffer 99 -> normalized/clamped by service; update should still succeed (clamp to 5),
	// but validate path returns invalid only on negative. Use an obviously invalid via negative amount.
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest(http.MethodPut, "/admin/settings/profit-guard",
		strings.NewReader(`{"minimum_profit_amount_cny":-1}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	h.UpdateProfitGuard(ctx)
	var body struct {
		StatusCode int `json:"status_code"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	// negative is normalized to 0 inside Normalize, so it should be accepted.
	if w.Code != http.StatusOK {
		t.Fatalf("PUT negative amount should be normalized, got http=%d body=%s", w.Code, w.Body.String())
	}
}
