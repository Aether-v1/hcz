package settingsapp

import (
	"regexp"
	"strings"

	"github.com/Aether-v1/hcz/internal/constants"
	"github.com/Aether-v1/hcz/internal/shared/jsonmap"
)

var settingSupportedLanguages = append([]string(nil), constants.SupportedLocales...)
var settingCurrencyCodePattern = regexp.MustCompile(`^[A-Z]{3}$`)
var sitePrimaryColorPattern = regexp.MustCompile(`^#([A-Fa-f0-9]{6}|[A-Fa-f0-9]{3})$`)

const (
	settingSiteScriptsMaxCount       = 20
	settingSiteScriptNameMaxRuneSize = 120
	settingSiteScriptCodeMaxRuneSize = 20000

	settingSiteFooterLinksMaxCount       = 20
	settingSiteFooterLinkNameMaxRuneSize = 120
	settingSiteFooterLinkURLMaxRuneSize  = 2000

	settingNavCustomItemsMaxCount        = 10
	settingNavCustomItemTitleMaxRuneSize = 120
	settingNavCustomItemURLMaxRuneSize   = 2000

	settingRegistrationEmailDomainMaxCount  = 100
	settingRegistrationEmailDomainMaxLength = 253
)

// normalizeSiteSetting 归一化站点配置结构。
func normalizeSiteSetting(value map[string]interface{}) jsonmap.JSON {
	normalized := make(jsonmap.JSON, len(value)+8)
	for key, raw := range value {
		normalized[key] = raw
	}

	normalized["brand"] = normalizeSiteBrand(value["brand"])
	normalized["contact"] = normalizeSiteContact(value["contact"])
	normalized["seo"] = normalizeSiteLocalizedBlock(value["seo"], []string{"title", "keywords", "description"})
	normalized["legal"] = normalizeSiteLocalizedBlock(value["legal"], []string{"terms", "privacy"})
	normalized["about"] = normalizeSiteAbout(value["about"])
	// LEGACY_DISABLED: scripts 功能已禁用，存储型 XSS 高危，不再归一化任何输入。
	// 始终返回空数组，即使 Admin 直接调 API 传入 scripts 也不会被存储/输出。
	normalized["scripts"] = []interface{}{}
	normalized["footer_links"] = normalizeSiteFooterLinks(value["footer_links"])
	normalized[constants.SettingFieldSiteCurrency] = normalizeSiteCurrency(value[constants.SettingFieldSiteCurrency])
	normalized["template_mode"] = normalizeSiteTemplateMode(value["template_mode"])

	if raw, ok := value["languages"]; ok {
		normalized["languages"] = normalizeSiteLanguages(raw)
	}

	// P1-5: nav_config 是独立 settings key（key='nav_config'），不允许嵌套在 site_config 中。
	// 历史上 Site Builder → NavFooter 会把导航配置嵌套写入 site_config（Storage C），
	// 但 public/handler.go 只读独立 nav_config 行（Storage A），嵌套字段是死存储。
	// 这里显式剔除，即使前端误传嵌套 nav_config 也不会落库。
	delete(normalized, "nav_config")

	return normalized
}

// normalizeSiteScripts 已禁用：scripts 功能存在存储型 XSS 高危风险。
// LEGACY_DISABLED: 保留函数签名以兼容现有调用点，但直接丢弃所有输入并返回空数组。
func normalizeSiteScripts(raw interface{}) []interface{} {
	return make([]interface{}, 0)
}

func normalizeSiteFooterLinks(raw interface{}) []interface{} {
	listRaw, ok := raw.([]interface{})
	if !ok {
		return make([]interface{}, 0)
	}

	result := make([]interface{}, 0, len(listRaw))
	for _, itemRaw := range listRaw {
		itemMap, ok := itemRaw.(map[string]interface{})
		if !ok {
			continue
		}

		name := normalizeSettingTextWithRuneLimit(itemMap["name"], settingSiteFooterLinkNameMaxRuneSize)
		if name == "" {
			continue
		}

		url := normalizeSettingTextWithRuneLimit(itemMap["url"], settingSiteFooterLinkURLMaxRuneSize)

		result = append(result, map[string]interface{}{
			"name": name,
			"url":  url,
		})

		if len(result) >= settingSiteFooterLinksMaxCount {
			break
		}
	}

	return result
}

func normalizeSiteContact(raw interface{}) map[string]interface{} {
	result := map[string]interface{}{
		"telegram":     "",
		"whatsapp":     "",
		"social_links": normalizeSiteSocialLinks(nil),
	}
	contactMap, ok := raw.(map[string]interface{})
	if !ok {
		return result
	}
	result["telegram"] = normalizeSettingText(contactMap["telegram"])
	result["whatsapp"] = normalizeSettingText(contactMap["whatsapp"])
	result["social_links"] = normalizeSiteSocialLinks(contactMap["social_links"])
	return result
}

// normalizeSiteSocialLinks 归一化社媒链接数组。
// 保持向后兼容：contact 顶层 telegram/whatsapp 字符串字段继续保留。
func normalizeSiteSocialLinks(raw interface{}) []interface{} {
	channels := []string{"telegram", "whatsapp", "x", "discord", "email", "custom"}
	// 默认返回全部渠道占位项（enabled=false），保证前端结构稳定。
	result := make([]interface{}, 0, len(channels))
	for _, channel := range channels {
		result = append(result, map[string]interface{}{
			"channel":      channel,
			"label":        "",
			"url_or_value": "",
			"enabled":      false,
		})
	}
	listRaw, ok := raw.([]interface{})
	if !ok {
		return result
	}
	index := make(map[string]int, len(result))
	for i, item := range result {
		m := item.(map[string]interface{})
		index[m["channel"].(string)] = i
	}
	for _, itemRaw := range listRaw {
		itemMap, ok := itemRaw.(map[string]interface{})
		if !ok {
			continue
		}
		channel := normalizeSettingText(itemMap["channel"])
		pos, exists := index[channel]
		if !exists {
			continue
		}
		result[pos] = map[string]interface{}{
			"channel":      channel,
			"label":        normalizeSettingText(itemMap["label"]),
			"url_or_value": normalizeSettingText(itemMap["url_or_value"]),
			"enabled":      parseSettingBool(itemMap["enabled"]),
		}
	}
	return result
}

func normalizeSiteBrand(raw interface{}) map[string]interface{} {
	// site_logo 用于前台导航栏、页脚等页面品牌展示，与浏览器 favicon 的 site_icon 保持独立。
	result := map[string]interface{}{
		"site_name":        "",
		"site_url":         "",
		"site_icon":        "",
		"site_logo":        "",
		"site_description": normalizeSiteLocalizedField(nil),
		"primary_color":    "#4F46E5",
		"copyright":        "",
	}
	brandMap, ok := raw.(map[string]interface{})
	if !ok {
		return result
	}
	result["site_name"] = normalizeSettingText(brandMap["site_name"])
	result["site_url"] = strings.TrimRight(normalizeSettingText(brandMap["site_url"]), "/")
	result["site_icon"] = normalizeSettingText(brandMap["site_icon"])
	// 未配置 Logo 时保留空字符串，让前台继续使用各主题原有的 fallback 行为。
	result["site_logo"] = normalizeSettingText(brandMap["site_logo"])
	result["site_description"] = normalizeSiteLocalizedField(brandMap["site_description"])
	// primary_color：仅接受合法 HEX，否则回退默认值。
	primaryColor := normalizeSettingText(brandMap["primary_color"])
	if sitePrimaryColorPattern.MatchString(primaryColor) {
		result["primary_color"] = primaryColor
	}
	result["copyright"] = normalizeSettingText(brandMap["copyright"])
	return result
}

func normalizeSiteLocalizedBlock(raw interface{}, fields []string) map[string]interface{} {
	result := make(map[string]interface{}, len(fields))
	blockMap, _ := raw.(map[string]interface{})

	for _, field := range fields {
		if blockMap == nil {
			result[field] = normalizeSiteLocalizedField(nil)
			continue
		}
		result[field] = normalizeSiteLocalizedField(blockMap[field])
	}

	return result
}

func normalizeSiteLocalizedField(raw interface{}) map[string]interface{} {
	fieldResult := make(map[string]interface{}, len(settingSupportedLanguages))
	for _, language := range settingSupportedLanguages {
		fieldResult[language] = ""
	}

	fieldRaw, ok := raw.(map[string]interface{})
	if !ok {
		return fieldResult
	}

	for _, language := range settingSupportedLanguages {
		fieldResult[language] = normalizeSettingText(fieldRaw[language])
	}

	return fieldResult
}

func normalizeSiteLocalizedList(raw interface{}, maxItems int) []interface{} {
	listRaw, ok := raw.([]interface{})
	if !ok {
		return make([]interface{}, 0)
	}

	result := make([]interface{}, 0, len(listRaw))
	for _, item := range listRaw {
		normalized := normalizeSiteLocalizedField(item)
		hasText := false
		for _, language := range settingSupportedLanguages {
			text, _ := normalized[language].(string)
			if text != "" {
				hasText = true
				break
			}
		}
		if !hasText {
			continue
		}

		result = append(result, normalized)
		if maxItems > 0 && len(result) >= maxItems {
			break
		}
	}

	return result
}

func normalizeSiteAbout(raw interface{}) map[string]interface{} {
	result := map[string]interface{}{
		"hero":         normalizeSiteLocalizedBlock(nil, []string{"title", "subtitle"}),
		"introduction": normalizeSiteLocalizedField(nil),
		"services": map[string]interface{}{
			"title": normalizeSiteLocalizedField(nil),
			"items": make([]interface{}, 0),
		},
		"contact": map[string]interface{}{
			"title": normalizeSiteLocalizedField(nil),
			"text":  normalizeSiteLocalizedField(nil),
		},
	}

	aboutMap, ok := raw.(map[string]interface{})
	if !ok {
		return result
	}

	result["hero"] = normalizeSiteLocalizedBlock(aboutMap["hero"], []string{"title", "subtitle"})
	result["introduction"] = normalizeSiteLocalizedField(aboutMap["introduction"])

	services := map[string]interface{}{
		"title": normalizeSiteLocalizedField(nil),
		"items": make([]interface{}, 0),
	}
	if servicesRaw, ok := aboutMap["services"].(map[string]interface{}); ok {
		services["title"] = normalizeSiteLocalizedField(servicesRaw["title"])
		services["items"] = normalizeSiteLocalizedList(servicesRaw["items"], 12)
	}
	result["services"] = services

	contact := map[string]interface{}{
		"title": normalizeSiteLocalizedField(nil),
		"text":  normalizeSiteLocalizedField(nil),
	}
	if contactRaw, ok := aboutMap["contact"].(map[string]interface{}); ok {
		contact["title"] = normalizeSiteLocalizedField(contactRaw["title"])
		contact["text"] = normalizeSiteLocalizedField(contactRaw["text"])
	}
	result["contact"] = contact

	return result
}

func normalizeSiteLanguages(raw interface{}) []string {
	list := make([]string, 0)
	switch value := raw.(type) {
	case []string:
		list = append(list, value...)
	case []interface{}:
		for _, item := range value {
			list = append(list, normalizeSettingText(item))
		}
	default:
		return append([]string(nil), settingSupportedLanguages...)
	}

	result := make([]string, 0, len(list))
	seen := make(map[string]struct{}, len(list))
	for _, item := range list {
		lang := strings.TrimSpace(item)
		if lang == "" {
			continue
		}
		if _, exists := seen[lang]; exists {
			continue
		}
		seen[lang] = struct{}{}
		result = append(result, lang)
	}
	if len(result) == 0 {
		return append([]string(nil), settingSupportedLanguages...)
	}
	return result
}

// normalizeSiteTemplateMode 归一化站点模板模式，允许 "card" 或 "list"，默认 "card"。
func normalizeSiteTemplateMode(raw interface{}) string {
	mode := normalizeSettingText(raw)
	if mode == "list" {
		return "list"
	}
	return "card"
}

func normalizeNavConfig(value map[string]interface{}) jsonmap.JSON {
	// builtin: blog / notice / about 开关，默认 true
	builtin := map[string]interface{}{
		"blog":   true,
		"notice": true,
		"about":  true,
	}
	if builtinRaw, ok := value["builtin"].(map[string]interface{}); ok {
		for _, key := range []string{"blog", "notice", "about"} {
			if raw, exists := builtinRaw[key]; exists {
				builtin[key] = parseSettingBool(raw)
			}
		}
	}

	// custom_items: 自定义导航项
	customItems := make([]interface{}, 0)
	if itemsRaw, ok := value["custom_items"].([]interface{}); ok {
		for _, itemRaw := range itemsRaw {
			itemMap, ok := itemRaw.(map[string]interface{})
			if !ok {
				continue
			}

			title := normalizeSiteLocalizedField(itemMap["title"])
			// 跳过全空标题项
			allEmpty := true
			for _, lang := range settingSupportedLanguages {
				if s, _ := title[lang].(string); s != "" {
					allEmpty = false
					break
				}
			}
			if allEmpty {
				continue
			}
			// 截断标题长度
			for _, lang := range settingSupportedLanguages {
				if s, _ := title[lang].(string); s != "" {
					title[lang] = normalizeSettingTextWithRuneLimit(s, settingNavCustomItemTitleMaxRuneSize)
				}
			}

			linkType := normalizeSettingText(itemMap["link_type"])
			if linkType != "internal" && linkType != "external" {
				linkType = "internal"
			}

			target := normalizeSettingText(itemMap["target"])
			if target != "_self" && target != "_blank" {
				target = "_self"
			}

			url := normalizeSettingTextWithRuneLimit(itemMap["url"], settingNavCustomItemURLMaxRuneSize)

			sortOrder := 0
			if v, err := parseSettingInt(itemMap["sort_order"]); err == nil {
				sortOrder = v
			}

			icon := normalizeSettingText(itemMap["icon"])
			if icon == "" {
				icon = "link"
			}

			// 保留前端生成的 id
			id := float64(0)
			if v, ok := itemMap["id"].(float64); ok {
				id = v
			}

			customItems = append(customItems, map[string]interface{}{
				"id":         id,
				"title":      title,
				"link_type":  linkType,
				"url":        url,
				"target":     target,
				"sort_order": sortOrder,
				"enabled":    parseSettingBool(itemMap["enabled"]),
				"icon":       icon,
			})

			if len(customItems) >= settingNavCustomItemsMaxCount {
				break
			}
		}
	}

	return jsonmap.JSON{
		"builtin":      builtin,
		"custom_items": customItems,
	}
}

func normalizeRegistrationEmailDomains(raw interface{}) []string {
	var candidates []string
	switch value := raw.(type) {
	case string:
		candidates = strings.FieldsFunc(value, func(r rune) bool {
			return r == ',' || r == '\n' || r == '\r' || r == '\t' || r == ' '
		})
	case []string:
		candidates = append(candidates, value...)
	case []interface{}:
		for _, item := range value {
			candidates = append(candidates, normalizeSettingText(item))
		}
	default:
		return []string{}
	}

	seen := make(map[string]struct{}, len(candidates))
	result := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		domain := strings.ToLower(strings.TrimSpace(candidate))
		domain = strings.TrimPrefix(domain, "@")
		if len(domain) == 0 || len(domain) > settingRegistrationEmailDomainMaxLength {
			continue
		}
		if !isValidRegistrationEmailDomain(domain) {
			continue
		}
		if _, exists := seen[domain]; exists {
			continue
		}
		seen[domain] = struct{}{}
		result = append(result, domain)
		if len(result) >= settingRegistrationEmailDomainMaxCount {
			break
		}
	}
	return result
}

func isValidRegistrationEmailDomain(domain string) bool {
	if strings.Contains(domain, "..") || !strings.Contains(domain, ".") {
		return false
	}
	labels := strings.Split(domain, ".")
	for _, label := range labels {
		if label == "" || len(label) > 63 || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return false
		}
		for _, r := range label {
			if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
				continue
			}
			return false
		}
	}
	return true
}

// normalizeRegistrationSetting 归一化注册配置。
func normalizeRegistrationSetting(value map[string]interface{}) jsonmap.JSON {
	normalized := make(jsonmap.JSON, 4)
	registrationEnabled := true
	if raw, ok := value[constants.SettingFieldRegistrationEnabled]; ok {
		registrationEnabled = parseSettingBool(raw)
	}
	normalized[constants.SettingFieldRegistrationEnabled] = registrationEnabled

	emailVerificationEnabled := true
	if raw, ok := value[constants.SettingFieldEmailVerificationEnabled]; ok {
		emailVerificationEnabled = parseSettingBool(raw)
	}
	normalized[constants.SettingFieldEmailVerificationEnabled] = emailVerificationEnabled

	emailDomainAllowlistEnabled := false
	if raw, ok := value[constants.SettingFieldEmailDomainAllowlistEnabled]; ok {
		emailDomainAllowlistEnabled = parseSettingBool(raw)
	}
	normalized[constants.SettingFieldEmailDomainAllowlistEnabled] = emailDomainAllowlistEnabled
	normalized[constants.SettingFieldAllowedEmailDomains] = normalizeRegistrationEmailDomains(value[constants.SettingFieldAllowedEmailDomains])

	return normalized
}
