package settingspoints

import (
	"encoding/json"

	"github.com/Aether-v1/hcz/internal/shared/jsonmap"
)

// 签到业务配置（V1 最小集）。
//
// 规则：7 天循环奖励（Day1..Day7 = 1,2,3,4,5,6,10；第 8 天重新从 Day 1 循环，
// 但连续签到天数继续增长）。配置只影响未来签到：已签到的 points_awarded 是快照，
// 历史记录永不重算。
//
// 数值纪律：积分一律 int64（BIGINT），禁止 float；单个奖励上限 MaxCheckinReward，
// 防止管理员误输入超大数值造成资产风险。
const (
	// CheckinRewardsDays 是签到奖励周期长度（固定 7 天循环）。
	CheckinRewardsDays = 7
	// MaxCheckinReward 是单日签到奖励上限（Points BIGINT 约束 + 业务安全）。
	MaxCheckinReward = int64(1_000_000)
)

// CheckinConfig 是签到全局配置。
type CheckinConfig struct {
	// Enabled 签到功能总开关。
	Enabled bool `json:"enabled"`
	// Rewards 7 天循环奖励数组（下标 0 对应连续第 1 天）。
	Rewards []int64 `json:"rewards"`
}

// DefaultCheckinConfig 返回新安装默认值。
func DefaultCheckinConfig() CheckinConfig {
	return CheckinConfig{
		Enabled: true,
		Rewards: []int64{1, 2, 3, 4, 5, 6, 10},
	}
}

// NormalizeCheckinConfig 归一化签到配置：
//   - 缺失/非法项回退默认（读取路径安全兜底，不 panic）；
//   - 数组长度必须恰为 CheckinRewardsDays；
//   - 每项必须 >= 0 且 <= MaxCheckinReward；
//   - 长度/取值不合规整体回退默认（宁可关掉异常奖励也不放大奖励）。
func NormalizeCheckinConfig(cfg CheckinConfig) CheckinConfig {
	defaults := DefaultCheckinConfig()
	if len(cfg.Rewards) != CheckinRewardsDays {
		return defaults
	}
	normalized := make([]int64, CheckinRewardsDays)
	for i, reward := range cfg.Rewards {
		if reward < 0 || reward > MaxCheckinReward {
			return defaults
		}
		normalized[i] = reward
	}
	cfg.Rewards = normalized
	return cfg
}

// ValidateCheckinConfig 严格校验（Admin 保存路径）：非法直接返回错误，拒绝入库。
func ValidateCheckinConfig(cfg CheckinConfig) error {
	if len(cfg.Rewards) != CheckinRewardsDays {
		return ErrCheckinConfigInvalidRewards
	}
	for _, reward := range cfg.Rewards {
		if reward < 0 || reward > MaxCheckinReward {
			return ErrCheckinConfigInvalidRewards
		}
	}
	return nil
}

// DecodeCheckinConfig 解析原始 JSON 为归一化配置；raw 缺失/非法时回退默认。
func DecodeCheckinConfig(raw jsonmap.JSON, fallback CheckinConfig) CheckinConfig {
	if raw == nil {
		return NormalizeCheckinConfig(fallback)
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return NormalizeCheckinConfig(fallback)
	}
	result := fallback
	if err := json.Unmarshal(data, &result); err != nil {
		return NormalizeCheckinConfig(fallback)
	}
	return NormalizeCheckinConfig(result)
}

// EncodeCheckinConfig 把归一化配置序列化为 JSON map。
func EncodeCheckinConfig(cfg CheckinConfig) jsonmap.JSON {
	normalized := NormalizeCheckinConfig(cfg)
	data, err := json.Marshal(normalized)
	if err != nil {
		return jsonmap.JSON{}
	}
	var result jsonmap.JSON
	_ = json.Unmarshal(data, &result)
	return result
}

// NormalizeCheckinConfigJSON 是 Registry 使用的原始 JSON 写入策略（宽松净化）。
func NormalizeCheckinConfigJSON(value jsonmap.JSON) jsonmap.JSON {
	return EncodeCheckinConfig(DecodeCheckinConfig(value, DefaultCheckinConfig()))
}
