package application

import (
	"errors"
	"strings"

	"github.com/Aether-v1/hcz/internal/constants"
	"github.com/Aether-v1/hcz/internal/shared/jsonmap"
)

// TemplateService 店面模板切换业务逻辑（仅 classic / vault）。
type TemplateService struct {
	settings SiteSettingsStore
}

// NewTemplateService 创建 TemplateService。
func NewTemplateService(settings SiteSettingsStore) *TemplateService {
	return &TemplateService{settings: settings}
}

// Get 读取当前模板。
func (s *TemplateService) Get() (string, error) {
	if s == nil || s.settings == nil {
		return constants.StorefrontTemplateDefault, nil
	}
	cfg, err := s.settings.GetByKey(constants.SettingKeySiteConfig)
	if err != nil {
		return "", err
	}
	if cfg == nil {
		return constants.StorefrontTemplateDefault, nil
	}
	if tpl, ok := cfg[constants.SettingFieldStorefrontTemplate].(string); ok && isValidTemplate(tpl) {
		return tpl, nil
	}
	return constants.StorefrontTemplateDefault, nil
}

// Switch 切换模板。
func (s *TemplateService) Switch(template string) (string, error) {
	template = strings.TrimSpace(strings.ToLower(template))
	if !isValidTemplate(template) {
		return "", errors.New("template must be classic or vault")
	}
	if s == nil || s.settings == nil {
		return template, nil
	}
	cfg, err := s.settings.GetByKey(constants.SettingKeySiteConfig)
	if err != nil {
		return "", err
	}
	if cfg == nil {
		cfg = jsonmap.JSON{}
	}
	cfg[constants.SettingFieldStorefrontTemplate] = template
	if _, err := s.settings.Update(constants.SettingKeySiteConfig, map[string]interface{}(cfg)); err != nil {
		return "", err
	}
	return template, nil
}

func isValidTemplate(tpl string) bool {
	return tpl == constants.StorefrontTemplateClassic || tpl == constants.StorefrontTemplateVault
}
