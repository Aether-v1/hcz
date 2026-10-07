package checkinapp

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/Aether-v1/hcz/internal/logger"
	checkincontract "github.com/Aether-v1/hcz/internal/modules/checkin/contract"
	checkindomain "github.com/Aether-v1/hcz/internal/modules/checkin/domain"
	pointsapp "github.com/Aether-v1/hcz/internal/modules/points/application"
	pointscontract "github.com/Aether-v1/hcz/internal/modules/points/contract"
)

// Options 是签到服务的依赖。
type Options struct {
	Repository checkincontract.Repository
	UnitOfWork checkincontract.UnitOfWork
	Config     checkincontract.ConfigReader
	Clock      checkincontract.Clock
	// Points 是积分模块权威服务（CheckinReward 同事务发放；GetAccount 供幂等重读余额）。
	Points *pointsapp.Service
}

// Service 是每日签到的应用用例。
//
// 事务契约：签到记录 + 积分 Ledger 必须在同一事务（任一失败整体回滚）。
// 唯一事实来源：user_checkins 记录表；不维护任何"最后签到时间"缓存字段。
type Service struct {
	repository checkincontract.Repository
	unitOfWork checkincontract.UnitOfWork
	config     checkincontract.ConfigReader
	clock      checkincontract.Clock
	points     *pointsapp.Service
}

// NewService 创建签到服务。
func NewService(options Options) *Service {
	return &Service{
		repository: options.Repository,
		unitOfWork: options.UnitOfWork,
		config:     options.Config,
		clock:      options.Clock,
		points:     options.Points,
	}
}

// RewardForCycleDay 是 7 天循环奖励唯一 resolver（禁止把奖励数组散落在 Handler/Repository/DTO）。
// cycleDay 取值 1..7；非法输入防御性返回 0。
func RewardForCycleDay(rewards []int64, cycleDay int) int64 {
	if len(rewards) != 7 {
		return 0
	}
	if cycleDay < 1 || cycleDay > 7 {
		return 0
	}
	return rewards[cycleDay-1]
}

// cycleDayFor 连续第 N 天 → 循环周期内第几天的奖励（第 8 天回到 Day 1，但连续天数继续增长）。
func cycleDayFor(consecutiveDays int) int {
	if consecutiveDays < 1 {
		return 0
	}
	return ((consecutiveDays - 1) % 7) + 1
}

// CheckIn 每日签到。
//
// 事务内流程：确定业务日 → 查当日记录（幂等）→ 查最近签到计算连签 →
// 创建记录 → 奖励 > 0 时经 Points Core 发放 → 回填 ledger id → 提交。
// 并发双击：唯一索引 (user_id, checkin_date) 是最终防线；撞索引的请求事务回滚后，
// 在事务外重读权威记录并幂等返回当天已签到结果（不 500、不把并发当错误）。
func (s *Service) CheckIn(userID uint) (*checkincontract.CheckinResult, error) {
	if userID == 0 {
		return nil, checkincontract.ErrUserRequired
	}
	cfg, err := s.config.GetCheckinConfig()
	if err != nil {
		return nil, err
	}
	if !cfg.Enabled {
		return nil, checkincontract.ErrCheckinDisabled
	}
	today := BusinessDate(s.clock.Now())
	now := time.Now()

	var result *checkincontract.CheckinResult
	var duplicate bool
	err = s.unitOfWork.WithinTransaction(func(tx checkincontract.Transaction) error {
		existing, err := tx.Checkins().GetByUserAndDate(userID, today)
		if err != nil {
			return err
		}
		if existing != nil {
			result = s.buildResult(tx, existing, true, now)
			return nil
		}

		consecutive, err := s.computeConsecutive(tx, userID, today)
		if err != nil {
			return err
		}
		cycleDay := cycleDayFor(consecutive)
		reward := RewardForCycleDay(cfg.Rewards, cycleDay)

		record := &checkindomain.UserCheckin{
			UserID:          userID,
			CheckinDate:     today,
			ConsecutiveDays: consecutive,
			CycleDay:        cycleDay,
			PointsAwarded:   reward,
			CreatedAt:       now,
		}
		if err := tx.Checkins().Create(record); err != nil {
			if isDuplicateKeyError(err) {
				// 并发双击：另一事务先插入。当前事务内快照读不到未提交数据，
				// 标记 duplicate 并正常提交空事务，之后在事务外重读权威记录。
				duplicate = true
				return nil
			}
			return err
		}

		if reward > 0 {
			reference := pointscontract.CheckinReference(userID, today)
			if err := s.points.CheckinReward(tx.Points(), pointscontract.CheckinRewardInput{
				UserID:    userID,
				CheckinID: record.ID,
				Amount:    reward,
				Date:      today,
				Reason:    "daily check-in",
				Reference: reference,
			}); err != nil {
				return err
			}
			entry, queryErr := tx.Points().Points().GetLedgerEntryByReference(reference)
			if queryErr != nil {
				return queryErr
			}
			if entry != nil && entry.ID > 0 {
				if err := tx.Checkins().UpdatePointsLedgerID(userID, today, entry.ID); err != nil {
					return err
				}
			}
		}
		result = s.buildResult(tx, record, false, now)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if duplicate {
		// 事务外重读：此时并发写入已提交，返回权威的"当天已签到"结果。
		existing, reloadErr := s.repository.GetByUserAndDate(userID, today)
		if reloadErr != nil {
			return nil, reloadErr
		}
		if existing == nil {
			// 防御：append-only 表不应出现"记录消失"；返回错误避免静默丢数据。
			return nil, fmt.Errorf("checkin: duplicate detected but record not found for user=%d date=%s", userID, FormatBusinessDate(today))
		}
		return s.buildResultPlain(existing, true), nil
	}
	return result, nil
}

// computeConsecutive 计算连签天数（最近一条业务日 < today）。
//
// 异常数据防御：历史出现未来日期（checkin_date >= today）时不参与连签推导，
// 视为断签并记录 warning，绝不返回错误 streak。
func (s *Service) computeConsecutive(tx checkincontract.Transaction, userID uint, today time.Time) (int, error) {
	prev, err := tx.Checkins().LatestBefore(userID, today)
	if err != nil {
		return 0, err
	}
	if prev == nil {
		return 1, nil
	}
	if prev.CheckinDate.AddDate(0, 0, 1).Equal(today) {
		return prev.ConsecutiveDays + 1, nil
	}
	if !prev.CheckinDate.Before(today) {
		logger.Warnw("checkin_future_date_anomaly",
			"user_id", userID,
			"anomaly_date", FormatBusinessDate(prev.CheckinDate),
			"business_date", FormatBusinessDate(today),
		)
	}
	return 1, nil
}

// buildResult 组装签到结果（余额来自事务后的权威 Points Account；无账户按 0）。
func (s *Service) buildResult(tx checkincontract.Transaction, record *checkindomain.UserCheckin, already bool, now time.Time) *checkincontract.CheckinResult {
	balance := int64(0)
	if account, err := tx.Points().Points().GetAccountByUserID(record.UserID); err == nil && account != nil {
		balance = account.Balance
	}
	return &checkincontract.CheckinResult{
		CheckinDate:      FormatBusinessDate(record.CheckinDate),
		PointsAwarded:    record.PointsAwarded,
		ConsecutiveDays:  record.ConsecutiveDays,
		CycleDay:         record.CycleDay,
		CurrentBalance:   balance,
		AlreadyCheckedIn: already,
	}
}

// buildResultPlain 组装"并发幂等重读"的已签到结果（事务外直查权威账户）。
func (s *Service) buildResultPlain(record *checkindomain.UserCheckin, already bool) *checkincontract.CheckinResult {
	balance := int64(0)
	if account, err := s.points.GetAccount(record.UserID); err == nil && account != nil {
		balance = account.Balance
	}
	return &checkincontract.CheckinResult{
		CheckinDate:      FormatBusinessDate(record.CheckinDate),
		PointsAwarded:    record.PointsAwarded,
		ConsecutiveDays:  record.ConsecutiveDays,
		CycleDay:         record.CycleDay,
		CurrentBalance:   balance,
		AlreadyCheckedIn: already,
	}
}

// Status 返回签到状态（只读，不创建任何记录）。
func (s *Service) Status(userID uint) (*checkincontract.StatusResult, error) {
	if userID == 0 {
		return nil, checkincontract.ErrUserRequired
	}
	cfg, err := s.config.GetCheckinConfig()
	if err != nil {
		return nil, err
	}
	today := BusinessDate(s.clock.Now())

	existing, err := s.repository.GetByUserAndDate(userID, today)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return &checkincontract.StatusResult{
			Enabled:         cfg.Enabled,
			CheckedInToday:  true,
			ConsecutiveDays: existing.ConsecutiveDays,
			CycleDay:        existing.CycleDay,
			TodayReward:     existing.PointsAwarded,
			NextReward:      RewardForCycleDay(cfg.Rewards, cycleDayFor(existing.ConsecutiveDays+1)),
		}, nil
	}

	prev, err := s.repository.LatestBefore(userID, today)
	if err != nil {
		return nil, err
	}
	consecutiveNow := 0
	if prev != nil && prev.CheckinDate.AddDate(0, 0, 1).Equal(today) {
		consecutiveNow = prev.ConsecutiveDays
	}
	return &checkincontract.StatusResult{
		Enabled:         cfg.Enabled,
		CheckedInToday:  false,
		ConsecutiveDays: consecutiveNow,
		CycleDay:        cycleDayFor(consecutiveNow),
		TodayReward:     RewardForCycleDay(cfg.Rewards, cycleDayFor(consecutiveNow+1)),
		NextReward:      RewardForCycleDay(cfg.Rewards, cycleDayFor(consecutiveNow+2)),
	}, nil
}

// monthPattern 严格校验 YYYY-MM（防极端年份/非法月份导致 SQL 异常）。
var monthPattern = regexp.MustCompile(`^\d{4}-(0[1-9]|1[0-2])$`)

// History 返回指定月份（默认当前 Asia/Shanghai 月份）的签到日历数据。
func (s *Service) History(userID uint, month string) (*checkincontract.HistoryResult, error) {
	if userID == 0 {
		return nil, checkincontract.ErrUserRequired
	}
	year, monthNumber, err := s.resolveMonth(month)
	if err != nil {
		return nil, err
	}
	start := time.Date(year, time.Month(monthNumber), 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)

	records, err := s.repository.ListMonth(userID, start, end)
	if err != nil {
		return nil, err
	}
	result := &checkincontract.HistoryResult{
		Year:  year,
		Month: monthNumber,
		Total: len(records),
	}
	for _, record := range records {
		date := FormatBusinessDate(record.CheckinDate)
		result.CheckedDates = append(result.CheckedDates, date)
		result.Entries = append(result.Entries, checkincontract.HistoryEntry{
			Date:            date,
			PointsAwarded:   record.PointsAwarded,
			ConsecutiveDays: record.ConsecutiveDays,
			CycleDay:        record.CycleDay,
		})
	}
	return result, nil
}

// resolveMonth 解析月份参数；空值回退当前 Asia/Shanghai 月份。
func (s *Service) resolveMonth(month string) (int, int, error) {
	trimmed := strings.TrimSpace(month)
	if trimmed == "" {
		now := s.clock.Now().In(shanghaiLocation())
		return now.Year(), int(now.Month()), nil
	}
	if !monthPattern.MatchString(trimmed) {
		return 0, 0, checkincontract.ErrInvalidMonth
	}
	year := 0
	monthNumber := 0
	if _, scanErr := fmt.Sscanf(trimmed, "%d-%d", &year, &monthNumber); scanErr != nil {
		return 0, 0, checkincontract.ErrInvalidMonth
	}
	if year < 1970 || year > 9999 {
		return 0, 0, checkincontract.ErrInvalidMonth
	}
	return year, monthNumber, nil
}

// isDuplicateKeyError 识别唯一索引冲突（与 Points 模块口径一致）。
func isDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "unique constraint") ||
		strings.Contains(lower, "duplicate key") ||
		strings.Contains(lower, "duplicate entry") ||
		strings.Contains(lower, "unique index")
}
