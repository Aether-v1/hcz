package settingsintegration

import (
	"errors"
	"math"

	settingsvalue "github.com/Aether-v1/hcz/internal/modules/settings/schema/value"
	"github.com/Aether-v1/hcz/internal/shared/jsonmap"
)

// ErrProfitGuardConfigInvalid 配置越界。
var ErrProfitGuardConfigInvalid = errors.New("profit guard config invalid")

const (
	profitGuardBufferPercentMin = 0
	profitGuardBufferPercentMax = 5
)

// ProfitGuardSetting 是 HCZ Profit Guard V1 的集中配置（Admin Pricing Settings）。
//
// 安全默认值（全部"不改变现有行为"）：
//   - Enabled=false               → 拒单门（成本门/最低利润门）关闭，不影响任何在途下单
//   - RequireCostPrice=false       → cost=0 商品不拒单（待 Admin 回填成本后再开启）
//   - RateSafetyBufferPercent=1.0  → 汇率安全缓冲默认 1%（0~5）
//   - MinimumProfitAmountCNY=0     → 固定最低利润 0
//   - MinimumProfitRatePercent=0   → 比例最低利润 0
type ProfitGuardSetting struct {
	Enabled                  bool    `json:"enabled"`
	RequireCostPrice         bool    `json:"require_cost_price"`
	RateSafetyBufferPercent  float64 `json:"rate_safety_buffer_percent"`
	MinimumProfitAmountCNY   float64 `json:"minimum_profit_amount_cny"`
	MinimumProfitRatePercent float64 `json:"minimum_profit_rate_percent"`
}

// DefaultProfitGuardSetting 返回稳定的安全默认配置。
func DefaultProfitGuardSetting() ProfitGuardSetting {
	return NormalizeProfitGuardSetting(ProfitGuardSetting{
		RateSafetyBufferPercent: 1.0,
	})
}

// NormalizeProfitGuardSetting 归一化数值范围。
func NormalizeProfitGuardSetting(setting ProfitGuardSetting) ProfitGuardSetting {
	setting.RateSafetyBufferPercent = roundTwoDecimals(setting.RateSafetyBufferPercent)
	if setting.RateSafetyBufferPercent < profitGuardBufferPercentMin {
		setting.RateSafetyBufferPercent = profitGuardBufferPercentMin
	}
	if setting.RateSafetyBufferPercent > profitGuardBufferPercentMax {
		setting.RateSafetyBufferPercent = profitGuardBufferPercentMax
	}
	if setting.MinimumProfitAmountCNY < 0 {
		setting.MinimumProfitAmountCNY = 0
	}
	if setting.MinimumProfitRatePercent < 0 {
		setting.MinimumProfitRatePercent = 0
	}
	return setting
}

// ValidateProfitGuardSetting 归一化后校验。
func ValidateProfitGuardSetting(setting ProfitGuardSetting) error {
	n := NormalizeProfitGuardSetting(setting)
	if n.RateSafetyBufferPercent < profitGuardBufferPercentMin || n.RateSafetyBufferPercent > profitGuardBufferPercentMax {
		return ErrProfitGuardConfigInvalid
	}
	if n.MinimumProfitAmountCNY < 0 || n.MinimumProfitRatePercent < 0 {
		return ErrProfitGuardConfigInvalid
	}
	return nil
}

// DecodeProfitGuardSetting 从持久化 JSON 解码，缺失字段用 fallback 补全。
func DecodeProfitGuardSetting(raw jsonmap.JSON, fallback ProfitGuardSetting) ProfitGuardSetting {
	result := fallback
	if value, exists := raw["enabled"]; exists {
		result.Enabled = settingsvalue.ParseBool(value)
	}
	if value, exists := raw["require_cost_price"]; exists {
		result.RequireCostPrice = settingsvalue.ParseBool(value)
	}
	if value, exists := raw["rate_safety_buffer_percent"]; exists {
		if parsed, err := settingsvalue.ParseFloat(value); err == nil {
			result.RateSafetyBufferPercent = parsed
		}
	}
	if value, exists := raw["minimum_profit_amount_cny"]; exists {
		if parsed, err := settingsvalue.ParseFloat(value); err == nil {
			result.MinimumProfitAmountCNY = parsed
		}
	}
	if value, exists := raw["minimum_profit_rate_percent"]; exists {
		if parsed, err := settingsvalue.ParseFloat(value); err == nil {
			result.MinimumProfitRatePercent = parsed
		}
	}
	return NormalizeProfitGuardSetting(result)
}

// EncodeProfitGuardSetting 编码为稳定 JSON。
func EncodeProfitGuardSetting(setting ProfitGuardSetting) jsonmap.JSON {
	n := NormalizeProfitGuardSetting(setting)
	return jsonmap.JSON{
		"enabled":                    n.Enabled,
		"require_cost_price":         n.RequireCostPrice,
		"rate_safety_buffer_percent": n.RateSafetyBufferPercent,
		"minimum_profit_amount_cny":   n.MinimumProfitAmountCNY,
		"minimum_profit_rate_percent": n.MinimumProfitRatePercent,
	}
}

// NormalizeProfitGuardSettingJSON 是 Registry 写入策略。
func NormalizeProfitGuardSettingJSON(raw jsonmap.JSON) jsonmap.JSON {
	return EncodeProfitGuardSetting(DecodeProfitGuardSetting(raw, DefaultProfitGuardSetting()))
}

func roundTwoDecimals(v float64) float64 {
	return math.Round(v*100) / 100
}
