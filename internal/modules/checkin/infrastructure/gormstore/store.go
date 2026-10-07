package checkingormstore

import (
	"time"

	checkincontract "github.com/Aether-v1/hcz/internal/modules/checkin/contract"
	checkindomain "github.com/Aether-v1/hcz/internal/modules/checkin/domain"
	pointscontract "github.com/Aether-v1/hcz/internal/modules/points/contract"
	pointsgormstore "github.com/Aether-v1/hcz/internal/modules/points/infrastructure/gormstore"

	"gorm.io/gorm"
)

// Store 是签到记录的 GORM 持久化实现。
//
// 业务日序列化约定：checkin_date 列（type:date）存储/查询统一使用
// date.Format("2006-01-02 15:04:05")（UTC 零点 → "2026-10-01 00:00:00"）。
// 说明：SQLite 无真正 DATE 类型，GORM 将 time.Time 序列化为 datetime 文本；
// PostgreSQL date 列会把带时间的输入字符串隐式截断为日期，两种数据库行为一致。
// Store 是签到记录的 GORM 持久化实现。
//
// 业务日序列化约定：checkin_date 列（type:date）存储/查询统一直接传 time.Time
// （UTC 零点表示业务日），由各数据库驱动按自身规则序列化（SQLite 存 RFC3339
// "2026-10-01T00:00:00Z"；PostgreSQL date 列隐式截断为日期），两者行为一致。
// 禁止手工拼接 "YYYY-MM-DD" 字符串（会与驱动实际存储值不匹配）。
type Store struct {
	db *gorm.DB
}

// New 创建签到仓储。
func New(db *gorm.DB) *Store {
	return &Store{db: db}
}

// GetByUserAndDate 查询某用户某业务日签到记录。
func (s *Store) GetByUserAndDate(userID uint, date time.Time) (*checkindomain.UserCheckin, error) {
	var record checkindomain.UserCheckin
	err := s.db.Where("user_id = ? AND checkin_date = ?", userID, date).
		First(&record).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &record, nil
}

// LatestBefore 返回某用户最近一条 checkin_date < date 的记录（连签计算用）。
func (s *Store) LatestBefore(userID uint, date time.Time) (*checkindomain.UserCheckin, error) {
	var record checkindomain.UserCheckin
	err := s.db.Where("user_id = ? AND checkin_date < ?", userID, date).
		Order("checkin_date DESC").
		First(&record).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &record, nil
}

// Create 仅 INSERT；唯一索引 (user_id, checkin_date) 是防重复签到最终防线。
func (s *Store) Create(record *checkindomain.UserCheckin) error {
	return s.db.Create(record).Error
}

// UpdatePointsLedgerID 回填积分流水 ID（同事务内，奖励 > 0 时）。
func (s *Store) UpdatePointsLedgerID(userID uint, date time.Time, ledgerID uint) error {
	return s.db.Model(&checkindomain.UserCheckin{}).
		Where("user_id = ? AND checkin_date = ?", userID, date).
		UpdateColumn("points_ledger_id", ledgerID).Error
}

// ListMonth 返回 [start, end) 业务日区间内的签到记录（升序）。
func (s *Store) ListMonth(userID uint, start, end time.Time) ([]checkindomain.UserCheckin, error) {
	var records []checkindomain.UserCheckin
	err := s.db.Where("user_id = ? AND checkin_date >= ? AND checkin_date < ?",
		userID, start, end).
		Order("checkin_date ASC").
		Find(&records).Error
	if err != nil {
		return nil, err
	}
	return records, nil
}

// CountCheckinUsers 统计 [start, end) 业务日区间内的签到人数（DISTINCT user_id）。
// 只读聚合，供运营统计使用；业务日边界由调用方（签到域时钟）给出。
func (s *Store) CountCheckinUsers(start, end time.Time) (int64, error) {
	var count int64
	err := s.db.Model(&checkindomain.UserCheckin{}).
		Where("checkin_date >= ? AND checkin_date < ?", start, end).
		Distinct("user_id").
		Count(&count).Error
	if err != nil {
		return 0, err
	}
	return count, nil
}

// Checkins 返回绑定到当前事务的签到仓储视图。
func (s *Store) Checkins() checkincontract.Repository { return s }

// Points 返回绑定到同一事务的积分域视图（保证签到记录 + 积分入账同事务）。
func (s *Store) Points() pointscontract.Transaction { return pointsgormstore.UseTransaction(s.db) }

// WithinTransaction 开启事务并把事务视图传给回调。
func (s *Store) WithinTransaction(fn func(checkincontract.Transaction) error) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		return fn(&Store{db: tx})
	})
}
