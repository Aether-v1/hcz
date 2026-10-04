package settingssecurity

import (
	"encoding/json"
	"strings"

	"github.com/Aether-v1/hcz/internal/shared/jsonmap"
	"github.com/shopspring/decimal"
)

// WithdrawalConfig 提现全局配置。所有金额字段以字符串持久化，避免 float 精度丢失；
// 服务端在使用时统一用 decimal 解析并 2dp 舍入。
type WithdrawalConfig struct {
	Enabled              bool     `json:"enabled"`
	Network              string   `json:"network"`
	Currency             string   `json:"currency"`
	MinAmount            string   `json:"min_amount"`
	MaxAmount            string   `json:"max_amount"`
	DailyLimit           string   `json:"daily_limit"`
	DailyCountLimit      int      `json:"daily_count_limit"`
	FixedFee             string   `json:"fixed_fee"`
	PercentageFee        string   `json:"percentage_fee"`
	Require2FA           bool     `json:"require_2fa"`
	AddressBlacklist     []string `json:"address_blacklist"`
	NewUserCooldownHours int      `json:"new_user_cooldown_hours"`
	FirstWithdrawalMax   string   `json:"first_withdrawal_max"`
}

// DefaultWithdrawalConfig 返回新安装推荐值；总开关默认关闭，避免静默开放提现。
func DefaultWithdrawalConfig() WithdrawalConfig {
	return WithdrawalConfig{
		Enabled:              false,
		Network:              "TRC20",
		Currency:             "USDT",
		MinAmount:            "10.00",
		MaxAmount:            "10000.00",
		DailyLimit:           "50000.00",
		DailyCountLimit:      10,
		FixedFee:             "1.00",
		PercentageFee:        "0.00",
		Require2FA:           true,
		AddressBlacklist:     []string{},
		NewUserCooldownHours: 24,
		FirstWithdrawalMax:   "1000.00",
	}
}

// normalizeAmountString 把任意字符串归一化为合法的 2dp 十进制字符串；非法值回退 fallback。
func normalizeAmountString(raw, fallback string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return fallback
	}
	parsed, err := decimal.NewFromString(trimmed)
	if err != nil {
		return fallback
	}
	return parsed.Round(2).StringFixed(2)
}

// NormalizeWithdrawalConfig 归一化提现配置：空值回退默认、金额统一 2dp、地址黑名单去重去空。
func NormalizeWithdrawalConfig(cfg WithdrawalConfig) WithdrawalConfig {
	defaults := DefaultWithdrawalConfig()
	cfg.Network = strings.ToUpper(strings.TrimSpace(cfg.Network))
	if cfg.Network == "" {
		cfg.Network = defaults.Network
	}
	cfg.Currency = strings.ToUpper(strings.TrimSpace(cfg.Currency))
	if cfg.Currency == "" {
		cfg.Currency = defaults.Currency
	}
	cfg.MinAmount = normalizeAmountString(cfg.MinAmount, defaults.MinAmount)
	cfg.MaxAmount = normalizeAmountString(cfg.MaxAmount, defaults.MaxAmount)
	cfg.DailyLimit = normalizeAmountString(cfg.DailyLimit, defaults.DailyLimit)
	cfg.FixedFee = normalizeAmountString(cfg.FixedFee, defaults.FixedFee)
	cfg.PercentageFee = normalizeAmountString(cfg.PercentageFee, defaults.PercentageFee)
	cfg.FirstWithdrawalMax = normalizeAmountString(cfg.FirstWithdrawalMax, defaults.FirstWithdrawalMax)

	if cfg.DailyCountLimit < 0 {
		cfg.DailyCountLimit = defaults.DailyCountLimit
	}
	if cfg.NewUserCooldownHours < 0 {
		cfg.NewUserCooldownHours = defaults.NewUserCooldownHours
	}

	cleaned := make([]string, 0, len(cfg.AddressBlacklist))
	seen := make(map[string]struct{}, len(cfg.AddressBlacklist))
	for _, raw := range cfg.AddressBlacklist {
		entry := strings.TrimSpace(raw)
		if entry == "" {
			continue
		}
		if _, ok := seen[entry]; ok {
			continue
		}
		seen[entry] = struct{}{}
		cleaned = append(cleaned, entry)
	}
	cfg.AddressBlacklist = cleaned
	return cfg
}

// DecodeWithdrawalConfig 解析原始 JSON 为归一化配置。
func DecodeWithdrawalConfig(raw jsonmap.JSON, fallback WithdrawalConfig) WithdrawalConfig {
	if raw == nil {
		return NormalizeWithdrawalConfig(fallback)
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return NormalizeWithdrawalConfig(fallback)
	}
	result := fallback
	if err := json.Unmarshal(data, &result); err != nil {
		return NormalizeWithdrawalConfig(fallback)
	}
	return NormalizeWithdrawalConfig(result)
}

// EncodeWithdrawalConfig 把归一化配置序列化为 JSON map。
func EncodeWithdrawalConfig(cfg WithdrawalConfig) jsonmap.JSON {
	normalized := NormalizeWithdrawalConfig(cfg)
	data, err := json.Marshal(normalized)
	if err != nil {
		return jsonmap.JSON{}
	}
	var result jsonmap.JSON
	_ = json.Unmarshal(data, &result)
	return result
}

// NormalizeWithdrawalConfigJSON 是 Registry 使用的原始 JSON 写入策略。
func NormalizeWithdrawalConfigJSON(value jsonmap.JSON) jsonmap.JSON {
	return EncodeWithdrawalConfig(DecodeWithdrawalConfig(value, DefaultWithdrawalConfig()))
}
