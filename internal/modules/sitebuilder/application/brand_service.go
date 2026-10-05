package application

import (
	"strings"

	"github.com/Aether-v1/hcz/internal/constants"
	"github.com/Aether-v1/hcz/internal/shared/jsonmap"
)

// DefaultPrimaryColor 默认品牌主色。
const DefaultPrimaryColor = "#4F46E5"

// BrandService 品牌配置业务逻辑（primary_color / copyright）。
type BrandService struct {
	settings SiteSettingsStore
}

// NewBrandService 创建 BrandService。
func NewBrandService(settings SiteSettingsStore) *BrandService {
	return &BrandService{settings: settings}
}

// BrandResult 品牌配置返回值。
type BrandResult struct {
	PrimaryColor string `json:"primary_color"`
	Copyright    string `json:"copyright"`
}

// Get 读取品牌配置（从 site_config.brand 提取）。
func (s *BrandService) Get() (*BrandResult, error) {
	result := &BrandResult{PrimaryColor: DefaultPrimaryColor}
	if s == nil || s.settings == nil {
		return result, nil
	}
	cfg, err := s.settings.GetByKey(constants.SettingKeySiteConfig)
	if err != nil {
		return nil, err
	}
	brand, _ := cfg["brand"].(map[string]interface{})
	if brand == nil {
		return result, nil
	}
	if color, ok := brand["primary_color"].(string); ok && color != "" {
		result.PrimaryColor = color
	}
	if copyright, ok := brand["copyright"].(string); ok {
		result.Copyright = copyright
	}
	return result, nil
}

// Update 更新品牌配置：读-改-写整份 site_config，保留其它字段。
func (s *BrandService) Update(primaryColor, copyright string) (*BrandResult, error) {
	primaryColor = strings.TrimSpace(primaryColor)
	if primaryColor == "" {
		primaryColor = DefaultPrimaryColor
	}
	if err := ValidatePrimaryColor(primaryColor); err != nil {
		return nil, err
	}
	copyright = strings.TrimSpace(copyright)

	if s == nil || s.settings == nil {
		return &BrandResult{PrimaryColor: primaryColor, Copyright: copyright}, nil
	}
	cfg, err := s.settings.GetByKey(constants.SettingKeySiteConfig)
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		cfg = jsonmap.JSON{}
	}
	brand, _ := cfg["brand"].(map[string]interface{})
	if brand == nil {
		brand = map[string]interface{}{}
	}
	brand["primary_color"] = primaryColor
	brand["copyright"] = copyright
	cfg["brand"] = brand

	if _, err := s.settings.Update(constants.SettingKeySiteConfig, map[string]interface{}(cfg)); err != nil {
		return nil, err
	}
	return &BrandResult{PrimaryColor: primaryColor, Copyright: copyright}, nil
}
