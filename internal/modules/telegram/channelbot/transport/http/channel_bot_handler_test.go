package telegramhttp

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	settingsmessaging "github.com/Aether-v1/hcz/internal/modules/settings/schema/messaging"

	"github.com/gin-gonic/gin"
)

type fakeBotSettings struct {
	config        settingsmessaging.TelegramBotConfigSetting
	runtime       settingsmessaging.TelegramBotRuntimeStatusSetting
	getErr        error
	updateErr     error
	capturedStore settingsmessaging.TelegramBotRuntimeStatusSetting
	updateCalled  bool
}

func (f *fakeBotSettings) GetTelegramBotConfig() (settingsmessaging.TelegramBotConfigSetting, error) {
	return f.config, nil
}

func (f *fakeBotSettings) GetTelegramBotRuntimeStatus() (settingsmessaging.TelegramBotRuntimeStatusSetting, error) {
	return f.runtime, f.getErr
}

func (f *fakeBotSettings) UpdateTelegramBotRuntimeStatus(status settingsmessaging.TelegramBotRuntimeStatusSetting) error {
	f.capturedStore = status
	f.updateCalled = true
	return f.updateErr
}

type fakeTokenProvider struct{ token string }

func (f *fakeTokenProvider) DecryptBotTokenByClientID(uint) (string, error) { return f.token, nil }

func newHeartbeatRouter(h *ChannelBotHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.POST("/api/v1/channel/telegram/heartbeat", func(c *gin.Context) {
		c.Set("channel_client_id", uint(1))
		h.ReportHeartbeat(c)
	})
	return engine
}

func decodeEnvelope(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("bad envelope: %v", err)
	}
	return out
}

func TestHeartbeatPersistsRealRuntimeState(t *testing.T) {
	settings := &fakeBotSettings{}
	h := NewChannelBotHandler(settings, &fakeTokenProvider{})
	engine := newHeartbeatRouter(h)

	payload := `{"bot_version":"1.2.3","webhook_status":"disabled","warnings":["disk_low"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/channel/telegram/heartbeat", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	if !settings.updateCalled {
		t.Fatal("heartbeat should update runtime status")
	}
	stored := settings.capturedStore
	if !stored.Connected {
		t.Fatal("heartbeat should mark connected=true")
	}
	if stored.BotVersion != "1.2.3" || stored.WebhookStatus != "disabled" {
		t.Fatalf("real fields not propagated: %+v", stored)
	}
	if len(stored.Warnings) != 1 || stored.Warnings[0] != "disk_low" {
		t.Fatalf("warnings not propagated: %+v", stored.Warnings)
	}
	if stored.LastSeenAt == "" {
		t.Fatal("last_seen_at should be stamped by heartbeat")
	}
	// Retired license fields must never be written into persisted storage.
	encoded, _ := json.Marshal(settingsmessaging.EncodeTelegramBotRuntimeStatus(stored))
	var asMap map[string]any
	_ = json.Unmarshal(encoded, &asMap)
	for _, retired := range []string{"machine_code", "license_status", "license_expires_at"} {
		if _, ok := asMap[retired]; ok {
			t.Fatalf("retired field %q must not be persisted", retired)
		}
	}
}

// 旧客户端仍可能发送带 License 字段的 payload：请求应成功，但字段被忽略、不存储。
func TestHeartbeatAcceptsButIgnoresLegacyLicensePayload(t *testing.T) {
	settings := &fakeBotSettings{}
	h := NewChannelBotHandler(settings, &fakeTokenProvider{})
	engine := newHeartbeatRouter(h)

	payload := `{"bot_version":"old","webhook_status":"enabled","machine_code":"ABC","license_status":"valid","license_expires_at":"2099-01-01"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/channel/telegram/heartbeat", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("legacy payload should still succeed (accept-but-ignore), got %d body=%s", w.Code, w.Body.String())
	}
	if !settings.updateCalled {
		t.Fatal("legacy payload should still persist real runtime state")
	}
	if settings.capturedStore.BotVersion != "old" {
		t.Fatalf("bot_version should bind, got %q", settings.capturedStore.BotVersion)
	}
}

func TestHeartbeatStoreErrorReturns500(t *testing.T) {
	settings := &fakeBotSettings{updateErr: errors.New("boom")}
	h := NewChannelBotHandler(settings, &fakeTokenProvider{})
	engine := newHeartbeatRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/channel/telegram/heartbeat", bytes.NewBufferString(`{"bot_version":"1"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on store error, got %d", w.Code)
	}
	env := decodeEnvelope(t, w.Body.Bytes())
	if code, ok := env["status_code"].(float64); !ok || int(code) == 0 {
		t.Fatalf("expected non-zero error code, got %+v", env)
	}
}
