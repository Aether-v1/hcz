package settingspoints

import "errors"

// ErrCheckinConfigInvalidRewards 表示签到奖励数组非法（长度不为 7 / 含负数 / 超上限）。
// Admin 保存接口必须拒绝，禁止把异常配置入库。
var ErrCheckinConfigInvalidRewards = errors.New("checkin config: rewards must contain exactly 7 non-negative values <= 1_000_000")

// ErrCheckinOperatorRequired 表示签到配置变更缺少管理员身份。
// 签到奖励是资产规则，无操作者即无法审计，必须拒绝而不是匿名落库。
var ErrCheckinOperatorRequired = errors.New("checkin config: operator admin required")
