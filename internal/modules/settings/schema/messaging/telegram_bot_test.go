package settingsmessaging

import (
	"testing"

	"github.com/Aether-v1/hcz/internal/shared/jsonmap"
)

func TestNormalizeMenuItemsBackfillsBuiltinKeys(t *testing.T) {
	t.Parallel()

	// 模拟老库数据：只有 3 项内置菜单
	items := []TelegramBotMenuItem{
		{Key: "shop_home", Enabled: true, Order: 1, Action: TelegramBotMenuAction{Type: "builtin"}},
		{Key: "my_orders", Enabled: false, Order: 2, Action: TelegramBotMenuAction{Type: "builtin"}},
		{Key: "contact_support", Enabled: true, Order: 3, Action: TelegramBotMenuAction{Type: "builtin"}},
	}
	normalized := NormalizeTelegramBotMenuItems(items)

	keys := map[string]TelegramBotMenuItem{}
	for _, it := range normalized {
		keys[it.Key] = it
	}
	for _, want := range BuiltinTelegramBotMenuKeysOrder {
		if _, ok := keys[want]; !ok {
			t.Fatalf("expected builtin key %q to be backfilled, got=%v", want, keys)
		}
	}
	// 已有项的 enabled 状态应保留（my_orders 仍为 false）
	if keys["my_orders"].Enabled {
		t.Fatalf("expected existing my_orders enabled=false to be preserved")
	}
	// 新补齐的内置项默认 enabled=true
	if !keys["affiliate"].Enabled {
		t.Fatalf("expected backfilled affiliate to default enabled=true")
	}
	if !keys["gift_card"].Enabled {
		t.Fatalf("expected backfilled gift_card to default enabled=true")
	}
}

func TestTelegramBotConfigDefaultIncludesAllBuiltinMenu(t *testing.T) {
	t.Parallel()

	cfg := DefaultTelegramBotConfig()
	if len(cfg.Menu.Items) != len(BuiltinTelegramBotMenuKeysOrder) {
		t.Fatalf("expected %d builtin menu items in default seed, got=%d", len(BuiltinTelegramBotMenuKeysOrder), len(cfg.Menu.Items))
	}
	for i, want := range BuiltinTelegramBotMenuKeysOrder {
		if cfg.Menu.Items[i].Key != want {
			t.Fatalf("expected default menu[%d].Key=%q, got=%q", i, want, cfg.Menu.Items[i].Key)
		}
	}
}

func TestTelegramBotConfigHelpRoundTrip(t *testing.T) {
	t.Parallel()

	original := DefaultTelegramBotConfig()
	original.Help.Title["en-US"] = "Support Center"
	original.Help.CenterHint["en-US"] = "Configured center hint"
	original.Help.Items = append(original.Help.Items, TelegramBotHelpItem{
		Key:     "custom",
		Enabled: true,
		Order:   9,
		Summary: LocalizedText{"zh-CN": "🧪 自定义", "zh-TW": "🧪 自訂", "en-US": "🧪 Custom"},
		Title:   LocalizedText{"zh-CN": "🧪 自定义", "zh-TW": "🧪 自訂", "en-US": "🧪 Custom"},
		Content: LocalizedText{"zh-CN": "内容", "zh-TW": "內容", "en-US": "Content"},
	})

	serialized := EncodeTelegramBotConfig(original)
	parsed := DecodeTelegramBotConfig(serialized, DefaultTelegramBotConfig())
	if parsed.Help.Title["en-US"] != "Support Center" {
		t.Fatalf("expected help title to survive round trip, got=%q", parsed.Help.Title["en-US"])
	}
	if parsed.Help.CenterHint["en-US"] != "Configured center hint" {
		t.Fatalf("expected help center hint to survive round trip, got=%q", parsed.Help.CenterHint["en-US"])
	}
	if len(parsed.Help.Items) != len(original.Help.Items) {
		t.Fatalf("expected help items to survive round trip, got=%d", len(parsed.Help.Items))
	}
	if parsed.Help.Items[len(parsed.Help.Items)-1].Key != "custom" {
		t.Fatalf("expected custom help item key, got=%q", parsed.Help.Items[len(parsed.Help.Items)-1].Key)
	}
}

func TestNormalizeTelegramBotConfigNormalizesHelpTexts(t *testing.T) {
	t.Parallel()

	normalized := NormalizeTelegramBotConfigJSON(map[string]interface{}{
		"help": map[string]interface{}{
			"enabled": true,
			"title": map[string]interface{}{
				"zh-CN": "  帮助中心  ",
			},
			"center_hint": map[string]interface{}{
				"zh-CN": "  中间提示  ",
			},
			"items": []interface{}{
				map[string]interface{}{
					"key":     "  faq  ",
					"enabled": true,
					"order":   1,
					"summary": map[string]interface{}{"zh-CN": "  简介  "},
					"title":   map[string]interface{}{"zh-CN": "  标题  "},
					"content": map[string]interface{}{"zh-CN": "  内容  "},
				},
			},
		},
	})

	helpRaw := normalized["help"].(map[string]interface{})
	title := helpRaw["title"].(map[string]interface{})
	if title["zh-CN"] != "帮助中心" {
		t.Fatalf("expected trimmed help title, got=%q", title["zh-CN"])
	}
	centerHint := helpRaw["center_hint"].(map[string]interface{})
	if centerHint["zh-CN"] != "中间提示" {
		t.Fatalf("expected trimmed help center hint, got=%q", centerHint["zh-CN"])
	}

	items := helpRaw["items"].([]interface{})
	first := items[0].(map[string]interface{})
	if first["key"] != "faq" {
		t.Fatalf("expected trimmed help key, got=%q", first["key"])
	}
	summary := first["summary"].(map[string]interface{})
	if summary["zh-CN"] != "简介" {
		t.Fatalf("expected trimmed help summary, got=%q", summary["zh-CN"])
	}
}

// 旧商业 License 遥测已退休：encode 只输出真实运行状态字段。
func TestEncodeRuntimeStatusOmitsRetiredLicenseFields(t *testing.T) {
	t.Parallel()

	encoded := EncodeTelegramBotRuntimeStatus(TelegramBotRuntimeStatusSetting{
		Connected:     true,
		BotVersion:    "1.0.0",
		WebhookStatus: "disabled",
		Warnings:      []string{"disk_low"},
	})

	for _, retired := range []string{"machine_code", "license_status", "license_expires_at"} {
		if _, ok := encoded[retired]; ok {
			t.Fatalf("retired license field %q must not be persisted", retired)
		}
	}
	for _, kept := range []string{"connected", "bot_version", "webhook_status", "warnings"} {
		if _, ok := encoded[kept]; !ok {
			t.Fatalf("runtime field %q must still be persisted", kept)
		}
	}
}

// 旧库 JSON 里残留的 License 字段必须被忽略、不报错，其余运行状态正常读取，
// 且再次 encode 时不再写回旧字段（accept old JSON / ignore retired fields / do not re-persist）。
func TestDecodeRuntimeStatusIgnoresLegacyLicenseJSON(t *testing.T) {
	t.Parallel()

	legacy := jsonmap.JSON{
		"connected":          true,
		"machine_code":       "ABC",
		"license_status":     "valid",
		"license_expires_at": "2099-01-01",
		"bot_version":        "1.2.3",
		"webhook_status":     "enabled",
		"config_version":     9,
	}

	decoded := DecodeTelegramBotRuntimeStatus(legacy, DefaultTelegramBotRuntimeStatus())

	if !decoded.Connected {
		t.Fatal("connected should survive legacy decode")
	}
	if decoded.BotVersion != "1.2.3" || decoded.WebhookStatus != "enabled" {
		t.Fatalf("real runtime fields not read: %+v", decoded)
	}
	if decoded.ConfigVersion != 9 {
		t.Fatalf("config_version should be read, got=%d", decoded.ConfigVersion)
	}

	// Re-encode: the retired fields must not reappear.
	for _, retired := range []string{"machine_code", "license_status", "license_expires_at"} {
		if _, ok := EncodeTelegramBotRuntimeStatus(decoded)[retired]; ok {
			t.Fatalf("legacy field %q must not be re-persisted after decode", retired)
		}
	}
}
