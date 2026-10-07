package checkincontract

import (
	"errors"
	"time"

	checkindomain "github.com/Aether-v1/hcz/internal/modules/checkin/domain"
	pointscontract "github.com/Aether-v1/hcz/internal/modules/points/contract"
)

// 签到业务错误（HTTP 映射见 transport handler）。
var (
	// ErrCheckinDisabled 签到功能未开放（checkin_enabled=false）。
	ErrCheckinDisabled = errors.New("checkin: disabled by config")
	// ErrInvalidMonth 月份参数非法（必须 YYYY-MM）。
	ErrInvalidMonth = errors.New("checkin: invalid month parameter")
	// ErrUserRequired 缺少用户上下文。
	ErrUserRequired = errors.New("checkin: user required")
)

// Config 是签到业务配置（由 ConfigReader 提供，settings 系统存储）。
type Config struct {
	Enabled bool
	Rewards []int64 // 恰 7 项；下标 0 对应连续第 1 天
}

// ConfigReader 是签到配置读取端口（bootstrap 用 SettingService 适配）。
type ConfigReader interface {
	GetCheckinConfig() (Config, error)
}

// Clock 提供当前时刻（可注入测试时钟；业务日期由 service 统一换算 Asia/Shanghai）。
type Clock interface {
	Now() time.Time
}

// Repository 拥有签到记录的持久化。
// 事务调用方通过 Transaction 拿到绑定到同一事务的同一端口。
type Repository interface {
	// GetByUserAndDate 查询某用户某业务日签到记录（date 为 UTC 零点表示）。
	GetByUserAndDate(userID uint, date time.Time) (*checkindomain.UserCheckin, error)
	// LatestBefore 返回某用户最近一条 checkin_date < date 的记录（用于连签计算）。
	LatestBefore(userID uint, date time.Time) (*checkindomain.UserCheckin, error)
	// Create 仅 INSERT；唯一索引 (user_id, checkin_date) 是防重复最终防线。
	Create(record *checkindomain.UserCheckin) error
	// UpdatePointsLedgerID 回填积分流水 ID（同事务内，签到成功且奖励 > 0 时）。
	UpdatePointsLedgerID(userID uint, date time.Time, ledgerID uint) error
	// ListMonth 返回 [start, end) 业务日区间内的签到记录（升序）。
	ListMonth(userID uint, start, end time.Time) ([]checkindomain.UserCheckin, error)
}

// Transaction 是已打开数据库事务的签到域视图。
// Points() 暴露同一事务的积分域视图，保证"签到记录 + 积分入账"同事务。
type Transaction interface {
	Checkins() Repository
	Points() pointscontract.Transaction
}

// UnitOfWork 暴露开启事务的能力。
type UnitOfWork interface {
	WithinTransaction(fn func(Transaction) error) error
}

// CheckinResult 是签到成功（或当天已签到幂等返回）的结果。
type CheckinResult struct {
	CheckinDate      string // "2006-01-02"（Asia/Shanghai 业务日）
	PointsAwarded    int64
	ConsecutiveDays  int
	CycleDay         int
	CurrentBalance   int64 // 事务后的权威 Points Account 余额
	AlreadyCheckedIn bool
}

// StatusResult 是签到状态。
//
// 语义（未签到）：consecutive_days / cycle_day 表达"截至今天之前"的连续状态；
// today_reward = 今天签到将获得的奖励；next_reward = 明天签到将获得的奖励。
// 语义（已签到）：consecutive_days / cycle_day 为当天记录值；
// today_reward = 当天实际获得的奖励（配置变更不影响历史）。
type StatusResult struct {
	Enabled         bool
	CheckedInToday  bool
	ConsecutiveDays int
	CycleDay        int
	TodayReward     int64
	NextReward      int64
}

// HistoryEntry 是单日签到记录。
type HistoryEntry struct {
	Date            string // "2006-01-02"
	PointsAwarded   int64
	ConsecutiveDays int
	CycleDay        int
}

// HistoryResult 是本月签到日历数据（后端只提供业务数据，不返回 UI 文案/CSS）。
type HistoryResult struct {
	Year         int
	Month        int
	CheckedDates []string
	Entries      []HistoryEntry
	Total        int
}
