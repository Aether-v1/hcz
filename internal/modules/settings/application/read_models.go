package settingsapp

import (
	"time"

	"github.com/Aether-v1/hcz/internal/constants"
	settingsintegration "github.com/Aether-v1/hcz/internal/modules/settings/schema/integration"
	settingssecurity "github.com/Aether-v1/hcz/internal/modules/settings/schema/security"
	settingsstorefront "github.com/Aether-v1/hcz/internal/modules/settings/schema/storefront"
	"github.com/Aether-v1/hcz/internal/shared/jsonmap"
)

// GetActiveHomeAnnouncement returns the currently displayable announcement.
func (s *Service) GetActiveHomeAnnouncement() (jsonmap.JSON, bool) {
	if s == nil {
		return nil, false
	}
	value, err := s.GetByKey(constants.SettingKeyHomeAnnouncement)
	if err != nil || value == nil {
		return nil, false
	}
	return settingsstorefront.ActiveHomeAnnouncement(value, time.Now())
}

// GetOrderRiskControlConfig returns the normalized order risk policy.
func (s *Service) GetOrderRiskControlConfig() (settingssecurity.OrderRiskControlConfig, error) {
	fallback := settingssecurity.DefaultOrderRiskControlConfig()
	if s == nil {
		return fallback, nil
	}
	value, err := s.GetByKey(constants.SettingKeyOrderRiskControlConfig)
	if err != nil {
		return fallback, err
	}
	return settingssecurity.DecodeOrderRiskControlConfig(value, fallback), nil
}

// GetWithdrawalConfig returns the normalized withdrawal config.
func (s *Service) GetWithdrawalConfig() (settingssecurity.WithdrawalConfig, error) {
	fallback := settingssecurity.DefaultWithdrawalConfig()
	if s == nil {
		return fallback, nil
	}
	value, err := s.GetByKey(constants.SettingKeyWithdrawalConfig)
	if err != nil {
		return fallback, err
	}
	return settingssecurity.DecodeWithdrawalConfig(value, fallback), nil
}

// GetC2CConfig returns the normalized C2C trading config.
func (s *Service) GetC2CConfig() (settingsintegration.C2CSetting, error) {
	fallback := settingsintegration.DefaultC2CSetting()
	if s == nil {
		return fallback, nil
	}
	value, err := s.GetByKey(constants.SettingKeyC2CConfig)
	if err != nil {
		return fallback, err
	}
	return settingsintegration.DecodeC2CSetting(value, fallback), nil
}

// UpdateC2CConfig 更新 C2C 交易设置（归一化 + 校验后落库）。
func (s *Service) UpdateC2CConfig(setting settingsintegration.C2CSetting) (settingsintegration.C2CSetting, error) {
	normalized := settingsintegration.NormalizeC2CSetting(setting)
	if err := settingsintegration.ValidateC2CSetting(normalized); err != nil {
		return settingsintegration.DefaultC2CSetting(), err
	}
	if _, err := s.Update(constants.SettingKeyC2CConfig, map[string]interface{}(settingsintegration.EncodeC2CSetting(normalized))); err != nil {
		return settingsintegration.DefaultC2CSetting(), err
	}
	return normalized, nil
}
