package settingsintegration

import (
	"errors"
	"fmt"
	"math"

	settingsvalue "github.com/Aether-v1/hcz/internal/modules/settings/schema/value"
	"github.com/Aether-v1/hcz/internal/shared/jsonmap"
)

const (
	c2cTradeTimeoutMinutesMin  = 1
	c2cTradeTimeoutMinutesMax  = 1440
	c2cNewUserCooldownHoursMin = 0
	c2cNewUserCooldownHoursMax = 720
	c2cMaxCancelCountMin       = 0
	c2cFeeRateMin              = 0
	c2cFeeRateMax              = 100
)

var ErrC2CConfigInvalid = errors.New("c2c config invalid")

// C2CSetting 是 C2C 交易设置的 typed representation。
type C2CSetting struct {
	Enabled              bool    `json:"enabled"`
	TradeTimeoutMinutes  int     `json:"trade_timeout_minutes"`
	NewUserCooldownHours int     `json:"new_user_cooldown_hours"`
	MinTradeUSDT         float64 `json:"min_trade_usdt"`
	MaxTradeUSDT         float64 `json:"max_trade_usdt"`
	DailyTradeLimitUSDT  float64 `json:"daily_trade_limit_usdt"`
	MaxCancelCount       int     `json:"max_cancel_count"`
	FeeRate              float64 `json:"fee_rate"` // 第一版固定 0
}

// DefaultC2CSetting 返回稳定的 C2C 默认设置。
func DefaultC2CSetting() C2CSetting {
	return NormalizeC2CSetting(C2CSetting{
		Enabled:              true,
		TradeTimeoutMinutes:  30,
		NewUserCooldownHours: 24,
		MinTradeUSDT:         1,
		MaxTradeUSDT:         10000,
		DailyTradeLimitUSDT:  50000,
		MaxCancelCount:       5,
		FeeRate:              0,
	})
}

// NormalizeC2CSetting 归一化数值范围，保证持久化数据落在合法区间内。
func NormalizeC2CSetting(setting C2CSetting) C2CSetting {
	if setting.TradeTimeoutMinutes < c2cTradeTimeoutMinutesMin {
		setting.TradeTimeoutMinutes = c2cTradeTimeoutMinutesMin
	}
	if setting.TradeTimeoutMinutes > c2cTradeTimeoutMinutesMax {
		setting.TradeTimeoutMinutes = c2cTradeTimeoutMinutesMax
	}
	if setting.NewUserCooldownHours < c2cNewUserCooldownHoursMin {
		setting.NewUserCooldownHours = c2cNewUserCooldownHoursMin
	}
	if setting.NewUserCooldownHours > c2cNewUserCooldownHoursMax {
		setting.NewUserCooldownHours = c2cNewUserCooldownHoursMax
	}
	setting.MinTradeUSDT = roundC2CDecimal(setting.MinTradeUSDT)
	if setting.MinTradeUSDT < 0 {
		setting.MinTradeUSDT = 0
	}
	setting.MaxTradeUSDT = roundC2CDecimal(setting.MaxTradeUSDT)
	if setting.MaxTradeUSDT < setting.MinTradeUSDT {
		setting.MaxTradeUSDT = setting.MinTradeUSDT
	}
	setting.DailyTradeLimitUSDT = roundC2CDecimal(setting.DailyTradeLimitUSDT)
	if setting.DailyTradeLimitUSDT < 0 {
		setting.DailyTradeLimitUSDT = 0
	}
	if setting.MaxCancelCount < c2cMaxCancelCountMin {
		setting.MaxCancelCount = c2cMaxCancelCountMin
	}
	setting.FeeRate = roundC2CDecimal(setting.FeeRate)
	if setting.FeeRate < c2cFeeRateMin {
		setting.FeeRate = c2cFeeRateMin
	}
	if setting.FeeRate > c2cFeeRateMax {
		setting.FeeRate = c2cFeeRateMax
	}
	return setting
}

// ValidateC2CSetting 对归一化后的设置做合同校验。
func ValidateC2CSetting(setting C2CSetting) error {
	normalized := NormalizeC2CSetting(setting)
	if normalized.TradeTimeoutMinutes < c2cTradeTimeoutMinutesMin || normalized.TradeTimeoutMinutes > c2cTradeTimeoutMinutesMax {
		return fmt.Errorf("%w: 交易超时分钟数必须在 %d-%d 之间", ErrC2CConfigInvalid, c2cTradeTimeoutMinutesMin, c2cTradeTimeoutMinutesMax)
	}
	if normalized.NewUserCooldownHours < c2cNewUserCooldownHoursMin || normalized.NewUserCooldownHours > c2cNewUserCooldownHoursMax {
		return fmt.Errorf("%w: 新用户冷却小时数必须在 %d-%d 之间", ErrC2CConfigInvalid, c2cNewUserCooldownHoursMin, c2cNewUserCooldownHoursMax)
	}
	if normalized.MinTradeUSDT < 0 {
		return fmt.Errorf("%w: 最小交易 USDT 不能小于 0", ErrC2CConfigInvalid)
	}
	if normalized.MaxTradeUSDT < normalized.MinTradeUSDT {
		return fmt.Errorf("%w: 最大交易 USDT 不能小于最小交易 USDT", ErrC2CConfigInvalid)
	}
	if normalized.DailyTradeLimitUSDT < 0 {
		return fmt.Errorf("%w: 每日交易限额 USDT 不能小于 0", ErrC2CConfigInvalid)
	}
	if normalized.MaxCancelCount < c2cMaxCancelCountMin {
		return fmt.Errorf("%w: 最大取消次数不能小于 0", ErrC2CConfigInvalid)
	}
	if normalized.FeeRate < c2cFeeRateMin || normalized.FeeRate > c2cFeeRateMax {
		return fmt.Errorf("%w: 手续费率必须在 %d-%d 之间", ErrC2CConfigInvalid, c2cFeeRateMin, c2cFeeRateMax)
	}
	return nil
}

// DecodeC2CSetting 从持久化 JSON 解码，并对缺失字段使用 fallback。
func DecodeC2CSetting(raw jsonmap.JSON, fallback C2CSetting) C2CSetting {
	result := fallback
	if value, exists := raw["enabled"]; exists {
		result.Enabled = settingsvalue.ParseBool(value)
	}
	if value, exists := raw["trade_timeout_minutes"]; exists {
		if parsed, err := settingsvalue.ParseInt(value); err == nil {
			result.TradeTimeoutMinutes = parsed
		}
	}
	if value, exists := raw["new_user_cooldown_hours"]; exists {
		if parsed, err := settingsvalue.ParseInt(value); err == nil {
			result.NewUserCooldownHours = parsed
		}
	}
	if value, exists := raw["min_trade_usdt"]; exists {
		if parsed, err := settingsvalue.ParseFloat(value); err == nil {
			result.MinTradeUSDT = parsed
		}
	}
	if value, exists := raw["max_trade_usdt"]; exists {
		if parsed, err := settingsvalue.ParseFloat(value); err == nil {
			result.MaxTradeUSDT = parsed
		}
	}
	if value, exists := raw["daily_trade_limit_usdt"]; exists {
		if parsed, err := settingsvalue.ParseFloat(value); err == nil {
			result.DailyTradeLimitUSDT = parsed
		}
	}
	if value, exists := raw["max_cancel_count"]; exists {
		if parsed, err := settingsvalue.ParseInt(value); err == nil {
			result.MaxCancelCount = parsed
		}
	}
	if value, exists := raw["fee_rate"]; exists {
		if parsed, err := settingsvalue.ParseFloat(value); err == nil {
			result.FeeRate = parsed
		}
	}
	return NormalizeC2CSetting(result)
}

// EncodeC2CSetting 把 typed setting 编码为稳定的持久化 JSON。
func EncodeC2CSetting(setting C2CSetting) jsonmap.JSON {
	normalized := NormalizeC2CSetting(setting)
	return jsonmap.JSON{
		"enabled":                 normalized.Enabled,
		"trade_timeout_minutes":   normalized.TradeTimeoutMinutes,
		"new_user_cooldown_hours": normalized.NewUserCooldownHours,
		"min_trade_usdt":          normalized.MinTradeUSDT,
		"max_trade_usdt":          normalized.MaxTradeUSDT,
		"daily_trade_limit_usdt":  normalized.DailyTradeLimitUSDT,
		"max_cancel_count":        normalized.MaxCancelCount,
		"fee_rate":                normalized.FeeRate,
	}
}

// NormalizeC2CSettingJSON 是 Registry 使用的原始 JSON 写入策略。
func NormalizeC2CSettingJSON(raw jsonmap.JSON) jsonmap.JSON {
	return EncodeC2CSetting(DecodeC2CSetting(raw, DefaultC2CSetting()))
}

func roundC2CDecimal(value float64) float64 {
	return math.Round(value*100) / 100
}
