package settingshttp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	settingspoints "github.com/Aether-v1/hcz/internal/modules/settings/schema/points"

	"github.com/gin-gonic/gin"
)

type fakeCheckinSettingService struct {
	current    settingspoints.CheckinConfig
	updated    settingspoints.CheckinConfig
	operatorID uint
	updateErr  error
}

func (f *fakeCheckinSettingService) GetCheckinSetting() (settingspoints.CheckinConfig, error) {
	return f.current, nil
}

func (f *fakeCheckinSettingService) UpdateCheckinSetting(s settingspoints.CheckinConfig, operatorAdminID uint) (settingspoints.CheckinConfig, error) {
	if operatorAdminID == 0 {
		return settingspoints.DefaultCheckinConfig(), settingspoints.ErrCheckinOperatorRequired
	}
	if err := settingspoints.ValidateCheckinConfig(s); err != nil {
		f.updateErr = err
		return settingspoints.DefaultCheckinConfig(), err
	}
	normalized := settingspoints.NormalizeCheckinConfig(s)
	f.updated = normalized
	f.operatorID = operatorAdminID
	return normalized, nil
}

func TestCheckinHandlerGetReturnsConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &fakeCheckinSettingService{current: settingspoints.DefaultCheckinConfig()}
	h := NewCheckinHandler(svc)

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/admin/settings/checkin", nil)
	h.GetCheckin(ctx)
	if w.Code != http.StatusOK {
		t.Fatalf("GET http=%d body=%s", w.Code, w.Body.String())
	}
	var body struct {
		Data settingspoints.CheckinConfig `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !body.Data.Enabled || len(body.Data.Rewards) != 7 {
		t.Fatalf("unexpected default config: %+v", body.Data)
	}
}

func TestCheckinHandlerUpdateValid(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &fakeCheckinSettingService{current: settingspoints.DefaultCheckinConfig()}
	h := NewCheckinHandler(svc)

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Set("admin_id", uint(7))
	ctx.Request = httptest.NewRequest(http.MethodPut, "/admin/settings/checkin",
		strings.NewReader(`{"enabled":true,"rewards":[1,2,3,0,5,6,10]}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	h.UpdateCheckin(ctx)
	if w.Code != http.StatusOK {
		t.Fatalf("PUT http=%d body=%s", w.Code, w.Body.String())
	}
	if svc.updated.Rewards[3] != 0 {
		t.Fatalf("zero reward day not persisted: %+v", svc.updated)
	}
	if svc.operatorID != 7 {
		t.Fatalf("operator admin id not carried into audit path: got %d", svc.operatorID)
	}
}

func TestCheckinHandlerUpdateWithoutAdminIsRejected(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &fakeCheckinSettingService{current: settingspoints.DefaultCheckinConfig()}
	h := NewCheckinHandler(svc)

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest(http.MethodPut, "/admin/settings/checkin",
		strings.NewReader(`{"enabled":true,"rewards":[1,2,3,4,5,6,10]}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	h.UpdateCheckin(ctx)
	if !strings.Contains(w.Body.String(), `"status_code":401`) && !strings.Contains(w.Body.String(), `"status_code":403`) {
		t.Fatalf("missing admin identity must be rejected, got %s", w.Body.String())
	}
	if svc.operatorID != 0 {
		t.Fatalf("config changed without operator: %+v", svc)
	}
}

func TestCheckinHandlerUpdateOperatorRequiredErrorMapsTo403(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewCheckinHandler(erroringCheckinService{err: settingspoints.ErrCheckinOperatorRequired})

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Set("admin_id", uint(0))
	ctx.Request = httptest.NewRequest(http.MethodPut, "/admin/settings/checkin",
		strings.NewReader(`{"enabled":true,"rewards":[1,2,3,4,5,6,10]}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	h.UpdateCheckin(ctx)
	if !strings.Contains(w.Body.String(), `"status_code":403`) {
		t.Fatalf("operator required must map to 403, got %s", w.Body.String())
	}
}

type erroringCheckinService struct {
	err error
}

func (e erroringCheckinService) GetCheckinSetting() (settingspoints.CheckinConfig, error) {
	return settingspoints.DefaultCheckinConfig(), nil
}

func (e erroringCheckinService) UpdateCheckinSetting(settingspoints.CheckinConfig, uint) (settingspoints.CheckinConfig, error) {
	return settingspoints.DefaultCheckinConfig(), e.err
}

func TestCheckinHandlerRejectsInvalidRewards(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &fakeCheckinSettingService{current: settingspoints.DefaultCheckinConfig()}
	h := NewCheckinHandler(svc)

	for _, tc := range []struct {
		name string
		body string
	}{
		{"wrong length", `{"enabled":true,"rewards":[1,2,3]}`},
		{"negative", `{"enabled":true,"rewards":[1,-2,3,4,5,6,10]}`},
		{"overflow", `{"enabled":true,"rewards":[1,2,3,4,5,6,999999999999999999]}`},
	} {
		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		ctx.Set("admin_id", uint(9))
		ctx.Request = httptest.NewRequest(http.MethodPut, "/admin/settings/checkin", strings.NewReader(tc.body))
		ctx.Request.Header.Set("Content-Type", "application/json")
		h.UpdateCheckin(ctx)
		if w.Code != http.StatusOK {
			t.Fatalf("%s: HCZ business code on HTTP 200 expected, got %d body=%s", tc.name, w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), `"status_code":400`) {
			t.Fatalf("%s: expected business status 400, got %s", tc.name, w.Body.String())
		}
	}
}
