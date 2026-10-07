package i18n

import "testing"

// TestAllLocalesExposeSameKeys 是三语错误契约守卫（P4 §26）。
//
// 后台/用户端点只返回稳定的 message key，文案由本表提供。
// 若新增 key 只补了简体中文，繁体与英文用户会拿到裸 key。
// 该测试让"漏翻译"在提交时就失败，而不是在线上暴露。
func TestAllLocalesExposeSameKeys(t *testing.T) {
	required := []string{LocaleZH, LocaleTW, LocaleEN}
	for _, locale := range required {
		if _, ok := messages[locale]; !ok {
			t.Fatalf("locale %q has no message table", locale)
		}
	}
	baseline := messages[LocaleZH]
	for _, locale := range required[1:] {
		current := messages[locale]
		for key := range baseline {
			if _, ok := current[key]; !ok {
				t.Errorf("locale %q misses key %q", locale, key)
			}
		}
		for key := range current {
			if _, ok := baseline[key]; !ok {
				t.Errorf("locale %q has extra key %q missing in %q", locale, key, LocaleZH)
			}
		}
	}
	for _, locale := range required {
		for key, value := range messages[locale] {
			if value == "" {
				t.Errorf("locale %q key %q has empty message", locale, key)
			}
		}
	}
}
