package settingsapp

import (
	"time"

	"github.com/Aether-v1/hcz/internal/constants"
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
