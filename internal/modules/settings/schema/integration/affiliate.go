package settingsintegration

import (
	"errors"
	"fmt"
	"math"
	"strings"

	settingsvalue "github.com/Aether-v1/hcz/internal/modules/settings/schema/value"
	"github.com/Aether-v1/hcz/internal/shared/jsonmap"
)

const (
	affiliateCommissionRateMin       = 0
	affiliateCommissionRateMax       = 100
	affiliateConfirmDaysMin          = 0
	affiliateConfirmDaysMax          = 3650
	affiliateMinWithdrawAmountMin    = 0
	affiliateWithdrawChannelsMaxSize = 20
	affiliateWithdrawChannelMaxRune  = 50
	affiliateMaxLevelMin             = 1
	affiliateMaxLevelMax             = 10
	affiliateLevelCount              = 10
)

var ErrAffiliateConfigInvalid = errors.New("affiliate config invalid")

// LevelRate 是多级返利中某一层级的费率配置。
type LevelRate struct {
	Level   int     `json:"level"`
	Enabled bool    `json:"enabled"`
	Rate    float64 `json:"rate"`
}

// AffiliateSetting 是推广返利设置的 typed representation。
type AffiliateSetting struct {
	Enabled           bool     `json:"enabled"`
	CommissionRate    float64  `json:"commission_rate"`
	ConfirmDays       int      `json:"confirm_days"`
	MinWithdrawAmount float64  `json:"min_withdraw_amount"`
	WithdrawChannels  []string `json:"withdraw_channels"`
	// MaxLevel 是实际向上返利的最大层级（1-10），默认 1 即仅一级。
	MaxLevel int `json:"max_level"`
	// LevelRates 固定长度 10，下标 i 对应第 i+1 层。Normalize 会补全并归一化。
	LevelRates []LevelRate `json:"level_rates"`
}

// DefaultAffiliateSetting 返回稳定的推广返利默认设置。
func DefaultAffiliateSetting() AffiliateSetting {
	return NormalizeAffiliateSetting(AffiliateSetting{
		WithdrawChannels: []string{},
	})
}

// normalizeLevelRates 返回长度固定为 10 的层级费率列表，并补全/归一化每一项。
func normalizeLevelRates(rates []LevelRate, fallbackL1 float64) []LevelRate {
	result := make([]LevelRate, affiliateLevelCount)
	byLevel := make(map[int]LevelRate, len(rates))
	for _, item := range rates {
		level := item.Level
		if level < affiliateMaxLevelMin || level > affiliateMaxLevelMax {
			continue
		}
		rate := roundAffiliateDecimal(item.Rate)
		if rate < affiliateCommissionRateMin {
			rate = affiliateCommissionRateMin
		}
		if rate > affiliateCommissionRateMax {
			rate = affiliateCommissionRateMax
		}
		byLevel[level] = LevelRate{
			Level:   level,
			Enabled: item.Enabled,
			Rate:    rate,
		}
	}
	for i := 1; i <= affiliateLevelCount; i++ {
		if existing, ok := byLevel[i]; ok {
			result[i-1] = existing
			continue
		}
		item := LevelRate{Level: i, Enabled: false, Rate: 0}
		// 兼容旧版：当 LevelRates 未显式提供时，把 legacy CommissionRate 作为 L1 默认费率并启用。
		if i == 1 && fallbackL1 > 0 {
			item.Enabled = true
			item.Rate = roundAffiliateDecimal(fallbackL1)
		}
		result[i-1] = item
	}
	return result
}

// NormalizeAffiliateSetting 归一化数值范围和提现渠道。
func NormalizeAffiliateSetting(setting AffiliateSetting) AffiliateSetting {
	setting.CommissionRate = roundAffiliateDecimal(setting.CommissionRate)
	if setting.CommissionRate < affiliateCommissionRateMin {
		setting.CommissionRate = affiliateCommissionRateMin
	}
	if setting.CommissionRate > affiliateCommissionRateMax {
		setting.CommissionRate = affiliateCommissionRateMax
	}
	if setting.ConfirmDays < affiliateConfirmDaysMin {
		setting.ConfirmDays = affiliateConfirmDaysMin
	}
	if setting.ConfirmDays > affiliateConfirmDaysMax {
		setting.ConfirmDays = affiliateConfirmDaysMax
	}
	setting.MinWithdrawAmount = roundAffiliateDecimal(setting.MinWithdrawAmount)
	if setting.MinWithdrawAmount < affiliateMinWithdrawAmountMin {
		setting.MinWithdrawAmount = affiliateMinWithdrawAmountMin
	}
	setting.WithdrawChannels = normalizeAffiliateWithdrawChannels(setting.WithdrawChannels)

	if setting.MaxLevel < affiliateMaxLevelMin {
		setting.MaxLevel = affiliateMaxLevelMin
	}
	if setting.MaxLevel > affiliateMaxLevelMax {
		setting.MaxLevel = affiliateMaxLevelMax
	}
	// 当 LevelRates 为空时（首次升级/旧数据），用 legacy CommissionRate 作为 L1 兼容费率。
	setting.LevelRates = normalizeLevelRates(setting.LevelRates, setting.CommissionRate)
	return setting
}

// ValidateAffiliateSetting 保留现有归一化后校验合同。
func ValidateAffiliateSetting(setting AffiliateSetting) error {
	normalized := NormalizeAffiliateSetting(setting)
	if normalized.CommissionRate < affiliateCommissionRateMin || normalized.CommissionRate > affiliateCommissionRateMax {
		return fmt.Errorf("%w: 返利比例必须在 0-100 之间", ErrAffiliateConfigInvalid)
	}
	if normalized.ConfirmDays < affiliateConfirmDaysMin || normalized.ConfirmDays > affiliateConfirmDaysMax {
		return fmt.Errorf("%w: 佣金确认天数必须在 0-3650 之间", ErrAffiliateConfigInvalid)
	}
	if normalized.MinWithdrawAmount < affiliateMinWithdrawAmountMin {
		return fmt.Errorf("%w: 最低提现金额不能小于 0", ErrAffiliateConfigInvalid)
	}
	if normalized.MaxLevel < affiliateMaxLevelMin || normalized.MaxLevel > affiliateMaxLevelMax {
		return fmt.Errorf("%w: 最大返利层级必须在 1-10 之间", ErrAffiliateConfigInvalid)
	}
	// 所有 enabled 层级费率之和不超过 100，避免基数被过度瓜分。
	sum := 0.0
	for _, item := range normalized.LevelRates {
		if item.Level > normalized.MaxLevel {
			continue
		}
		if !item.Enabled {
			continue
		}
		sum += item.Rate
	}
	if roundAffiliateDecimal(sum) > affiliateCommissionRateMax {
		return fmt.Errorf("%w: 启用层级费率之和不能超过 100", ErrAffiliateConfigInvalid)
	}
	return nil
}

// DecodeAffiliateSetting 从持久化 JSON 解码，并对缺失字段使用 fallback。
func DecodeAffiliateSetting(raw jsonmap.JSON, fallback AffiliateSetting) AffiliateSetting {
	result := fallback
	if value, exists := raw["enabled"]; exists {
		result.Enabled = settingsvalue.ParseBool(value)
	}
	if value, exists := raw["commission_rate"]; exists {
		if parsed, err := settingsvalue.ParseFloat(value); err == nil {
			result.CommissionRate = parsed
		}
	}
	if value, exists := raw["confirm_days"]; exists {
		if parsed, err := settingsvalue.ParseInt(value); err == nil {
			result.ConfirmDays = parsed
		}
	}
	if value, exists := raw["min_withdraw_amount"]; exists {
		if parsed, err := settingsvalue.ParseFloat(value); err == nil {
			result.MinWithdrawAmount = parsed
		}
	}
	if value, exists := raw["withdraw_channels"]; exists {
		result.WithdrawChannels = settingsvalue.NormalizeStringList(value)
	}
	if value, exists := raw["max_level"]; exists {
		if parsed, err := settingsvalue.ParseInt(value); err == nil {
			result.MaxLevel = parsed
		}
	}
	if value, exists := raw["level_rates"]; exists {
		result.LevelRates = decodeLevelRates(value)
	}
	return NormalizeAffiliateSetting(result)
}

func decodeLevelRates(value interface{}) []LevelRate {
	// 输入可能来自两条路径：
	//  1. JSON 反序列化（DB / HTTP body）-> []interface{}，元素为 map[string]interface{}
	//  2. 进程内 EncodeAffiliateSetting 的输出     -> []map[string]interface{}
	// 两种都要支持，否则 registry.Normalize(Decode(Encode(...))) 往返时会丢失层级费率。
	var items []map[string]interface{}
	switch typed := value.(type) {
	case []interface{}:
		for _, rawItem := range typed {
			obj, ok := rawItem.(map[string]interface{})
			if !ok {
				continue
			}
			items = append(items, obj)
		}
	case []map[string]interface{}:
		items = typed
	default:
		return nil
	}
	result := make([]LevelRate, 0, len(items))
	for _, obj := range items {
		level := 0
		if parsed, err := settingsvalue.ParseInt(obj["level"]); err == nil {
			level = parsed
		}
		rate := 0.0
		if parsed, err := settingsvalue.ParseFloat(obj["rate"]); err == nil {
			rate = parsed
		}
		result = append(result, LevelRate{
			Level:   level,
			Enabled: settingsvalue.ParseBool(obj["enabled"]),
			Rate:    rate,
		})
	}
	return result
}

// EncodeAffiliateSetting 把 typed setting 编码为稳定的持久化 JSON。
func EncodeAffiliateSetting(setting AffiliateSetting) jsonmap.JSON {
	normalized := NormalizeAffiliateSetting(setting)
	levelRatesOut := make([]map[string]interface{}, 0, len(normalized.LevelRates))
	for _, item := range normalized.LevelRates {
		levelRatesOut = append(levelRatesOut, map[string]interface{}{
			"level":   item.Level,
			"enabled": item.Enabled,
			"rate":    item.Rate,
		})
	}
	return jsonmap.JSON{
		"enabled":             normalized.Enabled,
		"commission_rate":     normalized.CommissionRate,
		"confirm_days":        normalized.ConfirmDays,
		"min_withdraw_amount": normalized.MinWithdrawAmount,
		"withdraw_channels":   settingsvalue.CloneStringSlice(normalized.WithdrawChannels),
		"max_level":           normalized.MaxLevel,
		"level_rates":         levelRatesOut,
	}
}

// NormalizeAffiliateSettingJSON 是 Registry 使用的原始 JSON 写入策略。
func NormalizeAffiliateSettingJSON(raw jsonmap.JSON) jsonmap.JSON {
	return EncodeAffiliateSetting(DecodeAffiliateSetting(raw, DefaultAffiliateSetting()))
}

func roundAffiliateDecimal(value float64) float64 {
	return math.Round(value*100) / 100
}

func normalizeAffiliateWithdrawChannels(channels []string) []string {
	if len(channels) == 0 {
		return []string{}
	}
	result := make([]string, 0, len(channels))
	seen := make(map[string]struct{}, len(channels))
	for _, raw := range channels {
		value := settingsvalue.NormalizeTextWithRuneLimit(raw, affiliateWithdrawChannelMaxRune)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, value)
		if len(result) >= affiliateWithdrawChannelsMaxSize {
			break
		}
	}
	return result
}
