package application

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// 首页入口 internal 路由白名单。
var homeEntryRouteWhitelist = map[string]struct{}{
	"recharge":   {},
	"c2c":        {},
	"wallet":     {},
	"withdrawal": {},
	"invitation": {},
	"support":    {},
	"orders":     {},
}

// 首页入口 icon 白名单。
var homeEntryIconWhitelist = map[string]struct{}{
	"recharge":   {},
	"c2c":        {},
	"wallet":     {},
	"withdrawal": {},
	"invitation": {},
	"support":    {},
	"orders":     {},
	"gift":       {},
	"ticket":     {},
	"discovery":  {},
}

// primary_color HEX 校验：#RGB 或 #RRGGBB。
var primaryColorPattern = regexp.MustCompile(`^#([A-Fa-f0-9]{6}|[A-Fa-f0-9]{3})$`)

// IsAllowedHomeEntryRoute 判断 internal 路由是否在白名单内。
func IsAllowedHomeEntryRoute(route string) bool {
	_, ok := homeEntryRouteWhitelist[strings.TrimSpace(route)]
	return ok
}

// IsAllowedHomeEntryIcon 判断 icon 是否在白名单内。
func IsAllowedHomeEntryIcon(icon string) bool {
	_, ok := homeEntryIconWhitelist[strings.TrimSpace(icon)]
	return ok
}

// ValidateExternalURL 校验外链安全性。
// 仅允许 http/https；拒绝 javascript:/data:/file:/vbscript: 等协议与不可解析地址。
func ValidateExternalURL(raw string) error {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return fmt.Errorf("url is required")
	}
	lower := strings.ToLower(trimmed)
	if !strings.HasPrefix(lower, "http://") && !strings.HasPrefix(lower, "https://") {
		return fmt.Errorf("only http/https urls are allowed")
	}
	u, err := url.Parse(trimmed)
	if err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}
	if u.Host == "" {
		return fmt.Errorf("url host is required")
	}
	return nil
}

// ValidatePrimaryColor 校验 HEX 颜色值。
func ValidatePrimaryColor(raw string) error {
	if !primaryColorPattern.MatchString(strings.TrimSpace(raw)) {
		return fmt.Errorf("invalid primary_color, expected #RGB or #RRGGBB")
	}
	return nil
}
