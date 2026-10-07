package checkindomain

import "time"

// UserCheckin 是用户每日签到记录。
//
// 事实来源：签到记录表本身是唯一权威（禁止把 users.last_checkin_at 之类的
// 缓存字段当作事实来源）；连续天数与奖励全部由记录推导/快照。
//
// 业务日语义：
//   - CheckinDate 统一为 Asia/Shanghai 业务日（00:00 ~ 23:59），以 UTC 零点
//     time.Time 表示（数据库列 type:date 存 "YYYY-MM-DD"）。服务器运行在哪个
//     时区都不改变签到业务日。
//
// 并发防线：
//   - UNIQUE(user_id, checkin_date)：同一天签到最多一条，是防重复签到的最终防线；
//   - 并发双击由应用层识别唯一索引冲突后幂等返回当天已签到结果（不 500）。
//
// 奖励快照：
//   - PointsAwarded 保存签到当时的奖励（即使管理员之后修改配置，历史不变）；
//   - PointsLedgerID 指向对应 CHECKIN_REWARD 流水；0 = 零积分日（不产生 Ledger）。
type UserCheckin struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	UserID          uint      `gorm:"column:user_id;not null;uniqueIndex:idx_user_checkin_date,priority:1" json:"user_id"`
	CheckinDate     time.Time `gorm:"column:checkin_date;type:date;not null;uniqueIndex:idx_user_checkin_date,priority:2" json:"checkin_date"`
	ConsecutiveDays int       `gorm:"column:consecutive_days;not null" json:"consecutive_days"`
	CycleDay        int       `gorm:"column:cycle_day;not null" json:"cycle_day"`
	PointsAwarded   int64     `gorm:"column:points_awarded;not null;default:0" json:"points_awarded"`
	PointsLedgerID  uint      `gorm:"column:points_ledger_id;not null;default:0" json:"points_ledger_id"`
	CreatedAt       time.Time `gorm:"column:created_at;not null" json:"created_at"`
}

func (UserCheckin) TableName() string { return "user_checkins" }
