package settingsapp

import (
	"github.com/Aether-v1/hcz/internal/config"
	"github.com/Aether-v1/hcz/internal/constants"
	"github.com/Aether-v1/hcz/internal/logger"
	settingscontract "github.com/Aether-v1/hcz/internal/modules/settings/contract"
	settingspoints "github.com/Aether-v1/hcz/internal/modules/settings/schema/points"
	"github.com/Aether-v1/hcz/internal/shared/jsonmap"
)

// Service 是站点设置的核心用例入口：读写、Registry 归一化与声明式副作用。
type Service struct {
	repo                  settingscontract.Store
	registry              Registry
	defaultOrderConfig    config.OrderConfig
	hasDefaultOrderConfig bool
}

// UpdateResult 包含持久化后的设置值及其声明式外部影响。
type UpdateResult struct {
	Value   jsonmap.JSON
	Effects []Effect
}

// HasEffect 判断设置更新是否声明了指定外部影响。
func (result UpdateResult) HasEffect(effect Effect) bool {
	for _, candidate := range result.Effects {
		if candidate == effect {
			return true
		}
	}
	return false
}

// NewService 创建设置服务。可选的订单配置仅在数据库尚未覆盖设置时使用。
func NewService(repo settingscontract.Store, defaultOrderConfig ...config.OrderConfig) *Service {
	service := &Service{
		repo:     repo,
		registry: defaultSettingRegistry,
	}
	if len(defaultOrderConfig) > 0 {
		service.defaultOrderConfig = defaultOrderConfig[0]
		service.hasDefaultOrderConfig = true
	}
	return service
}

// GetByKey 获取设置原始 JSON；不存在时返回 nil。
func (s *Service) GetByKey(key string) (jsonmap.JSON, error) {
	if s == nil || s.repo == nil {
		return nil, nil
	}
	value, found, err := s.repo.GetByKey(key)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}
	return value, nil
}

// Update 设置值。
func (s *Service) Update(key string, value map[string]interface{}) (jsonmap.JSON, error) {
	result, err := s.UpdateWithEffects(key, value)
	if err != nil {
		return nil, err
	}
	return result.Value, nil
}

// UpdateWithEffects 设置值并返回成功写入后需要处理的声明式外部影响。
func (s *Service) UpdateWithEffects(key string, value map[string]interface{}) (UpdateResult, error) {
	if s == nil || s.repo == nil {
		return UpdateResult{}, nil
	}
	normalized := s.registry.Normalize(key, jsonmap.JSON(value))

	stored, err := s.repo.Upsert(key, normalized)
	if err != nil {
		return UpdateResult{}, err
	}
	return UpdateResult{
		Value:   stored,
		Effects: s.registry.Effects(key),
	}, nil
}

// GetCheckinSetting 获取签到配置（优先 settings；缺失时回退默认）。
//
// 读取路径是宽松的：线上配置损坏（长度不为 7 / 含负数 / 超上限）时
// 回退安全默认并记录 warning，绝不 panic、绝不放大奖励。
func (s *Service) GetCheckinSetting() (settingspoints.CheckinConfig, error) {
	fallback := settingspoints.DefaultCheckinConfig()
	if s == nil {
		return fallback, nil
	}
	value, err := s.GetByKey(constants.SettingKeyCheckinConfig)
	if err != nil {
		return fallback, err
	}
	if value == nil {
		return fallback, nil
	}
	decoded := settingspoints.DecodeCheckinConfig(value, fallback)
	if validationErr := settingspoints.ValidateCheckinConfig(decoded); validationErr != nil {
		// 线上配置异常：不 panic、不拒绝服务，回退默认并留痕。
		logger.Warnw("checkin_config_invalid_fallback_to_default",
			"error", validationErr,
			"key", constants.SettingKeyCheckinConfig,
		)
		return fallback, nil
	}
	return decoded, nil
}

// UpdateCheckinSetting 更新签到配置（Admin 保存路径，严格校验，非法拒绝入库）。
//
// 签到奖励是资产规则，任何变更都必须留痕：记录操作管理员、变更前后的
// enabled 与 7 天奖励数组（不含任何用户敏感载荷）。
// 变更前配置通过读取路径获取，异常配置按既有宽松策略回退默认后如实记录。
func (s *Service) UpdateCheckinSetting(setting settingspoints.CheckinConfig, operatorAdminID uint) (settingspoints.CheckinConfig, error) {
	if operatorAdminID == 0 {
		return settingspoints.DefaultCheckinConfig(), settingspoints.ErrCheckinOperatorRequired
	}
	if validationErr := settingspoints.ValidateCheckinConfig(setting); validationErr != nil {
		return settingspoints.DefaultCheckinConfig(), validationErr
	}
	before, err := s.GetCheckinSetting()
	if err != nil {
		return settingspoints.DefaultCheckinConfig(), err
	}
	normalized := settingspoints.NormalizeCheckinConfig(setting)
	if _, err := s.Update(constants.SettingKeyCheckinConfig, map[string]interface{}(settingspoints.EncodeCheckinConfig(normalized))); err != nil {
		return settingspoints.DefaultCheckinConfig(), err
	}
	logger.Infow("checkin_config_updated",
		"admin_id", operatorAdminID,
		"before_enabled", before.Enabled,
		"before_rewards", before.Rewards,
		"after_enabled", normalized.Enabled,
		"after_rewards", normalized.Rewards,
	)
	return normalized, nil
}
