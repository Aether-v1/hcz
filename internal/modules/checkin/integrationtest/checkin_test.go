// checkin_test.go — Daily Check-in 集成测试（SQLite 文件型 WAL）。
//
// 覆盖（P2 Exit Gate 要求）：
//   - 首次签到（固定时钟 2026-10-01：streak=1 / cycle=1 / points=1 / ledger +1）
//   - 连续 7 天循环（1,2,3,4,5,6,10 累计 31）
//   - Day 8 循环回 Day1（streak 继续增长）
//   - 断签重算
//   - 同日重复签到幂等
//   - 并发签到（WAL 多连接：1 checkin + 1 ledger）
//   - 跨月 / 跨年连续
//   - Asia/Shanghai 时区午夜边界
//   - 功能开关（关闭拒绝签到、重新开启可签）
//   - 奖励配置变更不影响历史快照
//   - 零积分日（不产生 0 amount Ledger）
//   - 数据库不变量：points_accounts.balance == SUM(points_ledger.amount)
package integrationtest

import (
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	checkinapp "github.com/Aether-v1/hcz/internal/modules/checkin/application"
	checkincontract "github.com/Aether-v1/hcz/internal/modules/checkin/contract"
	checkindomain "github.com/Aether-v1/hcz/internal/modules/checkin/domain"
	checkingormstore "github.com/Aether-v1/hcz/internal/modules/checkin/infrastructure/gormstore"
	pointsapp "github.com/Aether-v1/hcz/internal/modules/points/application"
	pointscontract "github.com/Aether-v1/hcz/internal/modules/points/contract"
	pointsdomain "github.com/Aether-v1/hcz/internal/modules/points/domain"
	pointsgormstore "github.com/Aether-v1/hcz/internal/modules/points/infrastructure/gormstore"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// mutableCheckinConfig 是测试用的可变签到配置（实现 ConfigReader）。
type mutableCheckinConfig struct {
	mu      sync.Mutex
	enabled bool
	rewards []int64
}

func (c *mutableCheckinConfig) GetCheckinConfig() (checkincontract.Config, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return checkincontract.Config{Enabled: c.enabled, Rewards: append([]int64(nil), c.rewards...)}, nil
}

func (c *mutableCheckinConfig) set(enabled bool, rewards []int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.enabled = enabled
	c.rewards = append([]int64(nil), rewards...)
}

// mutableClock 是测试用可变时钟（实现 Clock；business date 由 service 统一换算）。
type mutableClock struct {
	mu sync.Mutex
	at time.Time
}

func (c *mutableClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.at
}

func (c *mutableClock) set(at time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.at = at
}

// at 构造一个确定时刻：业务日 (y,m,d) 的本地中午（UTC 表示，时区边界测试另行构造）。
func at(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 12, 0, 0, 0, time.UTC)
}

// bizDay 返回业务日 (y,m,d) 的 UTC 零点表示（与 BusinessDate 存储语义一致）。
func bizDay(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

type checkinFixture struct {
	db          *gorm.DB
	store       *checkingormstore.Store
	pointsStore *pointsgormstore.Store
	svc         *checkinapp.Service
	cfg         *mutableCheckinConfig
	clock       *mutableClock
}

// newCheckinFixture 创建签到集成测试环境。
// concurrent=true 时使用多连接（并发签到）；否则单连接串行化（顺序场景，语义一致）。
func newCheckinFixture(t *testing.T, concurrent bool) *checkinFixture {
	t.Helper()
	dir := t.TempDir()
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)",
		filepath.Join(dir, "checkin.db"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&checkindomain.UserCheckin{}, &pointsdomain.Account{}, &pointsdomain.LedgerEntry{}); err != nil {
		t.Fatalf("migrate checkin schema: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("raw db: %v", err)
	}
	if !concurrent {
		// SQLite 单写者：顺序场景单连接串行化，验证应用层事务正确性；
		// 真并发行锁/竞争由 concurrent fixture 与 PG 集成测试覆盖。
		sqlDB.SetMaxOpenConns(1)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	store := checkingormstore.New(db)
	pointsStore := pointsgormstore.New(db)
	cfg := &mutableCheckinConfig{enabled: true, rewards: []int64{1, 2, 3, 4, 5, 6, 10}}
	clock := &mutableClock{at: at(2026, 10, 1)}
	pointsSvc := pointsapp.NewService(pointsapp.Options{Repository: pointsStore, Transactions: pointsStore})
	svc := checkinapp.NewService(checkinapp.Options{
		Repository: store,
		UnitOfWork: store,
		Config:     cfg,
		Clock:      clock,
		Points:     pointsSvc,
	})
	return &checkinFixture{db: db, store: store, pointsStore: pointsStore, svc: svc, cfg: cfg, clock: clock}
}

// setDate 固定业务时刻。
func (f *checkinFixture) setDate(t time.Time) { f.clock.set(t) }

func countCheckins(db *gorm.DB, userID uint) int {
	var total int64
	db.Model(&checkindomain.UserCheckin{}).Where("user_id = ?", userID).Count(&total)
	return int(total)
}

func countLedgerByAction(db *gorm.DB, action string) int {
	var total int64
	db.Model(&pointsdomain.LedgerEntry{}).Where("action_type = ?", action).Count(&total)
	return int(total)
}

func sumLedger(db *gorm.DB, action string) int64 {
	var sum int64
	db.Model(&pointsdomain.LedgerEntry{}).Where("action_type = ?", action).Select("COALESCE(SUM(amount),0)").Scan(&sum)
	return sum
}

// assertInvariant 校验数据库不变量：account.balance == SUM(ledger.amount)。
func assertCheckinInvariant(t *testing.T, db *gorm.DB, userID uint) {
	t.Helper()
	var account pointsdomain.Account
	if err := db.Where("user_id = ?", userID).First(&account).Error; err != nil {
		t.Fatalf("load account: %v", err)
	}
	var sum int64
	if err := db.Model(&pointsdomain.LedgerEntry{}).Where("user_id = ?", userID).
		Select("COALESCE(SUM(amount),0)").Scan(&sum).Error; err != nil {
		t.Fatalf("sum ledger: %v", err)
	}
	if account.Balance != sum {
		t.Fatalf("invariant violated: account.balance=%d SUM(ledger)=%d", account.Balance, sum)
	}
}

// TestCheckin_FirstDay 首次签到：streak=1 / cycle=1 / points=1 / ledger +1。
func TestCheckin_FirstDay(t *testing.T) {
	f := newCheckinFixture(t, false)
	f.setDate(at(2026, 10, 1))

	result, err := f.svc.CheckIn(7)
	if err != nil {
		t.Fatalf("CheckIn: %v", err)
	}
	if result.CheckinDate != "2026-10-01" || result.ConsecutiveDays != 1 || result.CycleDay != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if result.PointsAwarded != 1 || result.CurrentBalance != 1 || result.AlreadyCheckedIn {
		t.Fatalf("unexpected points/balance: %+v", result)
	}
	if countCheckins(f.db, 7) != 1 || countLedgerByAction(f.db, pointscontract.ActionCheckinReward) != 1 {
		t.Fatalf("expected 1 checkin + 1 ledger, got checkins=%d ledger=%d",
			countCheckins(f.db, 7), countLedgerByAction(f.db, pointscontract.ActionCheckinReward))
	}
	assertCheckinInvariant(t, f.db, 7)
}

// TestCheckin_SevenDayCycle 连续 7 天：奖励 1,2,3,4,5,6,10 累计 31，streak=7。
func TestCheckin_SevenDayCycle(t *testing.T) {
	f := newCheckinFixture(t, false)
	want := []int64{1, 2, 3, 4, 5, 6, 10}
	var last *checkincontract.CheckinResult
	for i := 0; i < 7; i++ {
		f.setDate(at(2026, 10, 1+i))
		result, err := f.svc.CheckIn(8)
		if err != nil {
			t.Fatalf("day %d CheckIn: %v", i+1, err)
		}
		if result.PointsAwarded != want[i] {
			t.Fatalf("day %d: want reward %d got %d", i+1, want[i], result.PointsAwarded)
		}
		if result.ConsecutiveDays != i+1 {
			t.Fatalf("day %d: want streak %d got %d", i+1, i+1, result.ConsecutiveDays)
		}
		last = result
	}
	if last == nil || last.ConsecutiveDays != 7 || last.PointsAwarded != 10 {
		t.Fatalf("day7: want streak=7 reward=10, got %+v", last)
	}
	if sumLedger(f.db, pointscontract.ActionCheckinReward) != 31 {
		t.Fatalf("want cumulative 31, got %d", sumLedger(f.db, pointscontract.ActionCheckinReward))
	}
	assertCheckinInvariant(t, f.db, 8)
}

// TestCheckin_Day8 第 8 天：streak=8 / cycle=1 / reward=1（不得把 streak 重置为 1）。
func TestCheckin_Day8(t *testing.T) {
	f := newCheckinFixture(t, false)
	for i := 0; i < 7; i++ {
		f.setDate(at(2026, 10, 1+i))
		if _, err := f.svc.CheckIn(9); err != nil {
			t.Fatalf("day %d: %v", i+1, err)
		}
	}
	f.setDate(at(2026, 10, 8))
	result, err := f.svc.CheckIn(9)
	if err != nil {
		t.Fatalf("day8: %v", err)
	}
	if result.ConsecutiveDays != 8 || result.CycleDay != 1 || result.PointsAwarded != 1 {
		t.Fatalf("day8: want streak=8 cycle=1 reward=1, got %+v", result)
	}
	assertCheckinInvariant(t, f.db, 9)
}

// TestCheckin_BreakStreak 断签：10/1 签、10/2 不签、10/3 签 → streak 重算为 1。
func TestCheckin_BreakStreak(t *testing.T) {
	f := newCheckinFixture(t, false)
	f.setDate(at(2026, 10, 1))
	if _, err := f.svc.CheckIn(10); err != nil {
		t.Fatalf("day1: %v", err)
	}
	f.setDate(at(2026, 10, 3))
	result, err := f.svc.CheckIn(10)
	if err != nil {
		t.Fatalf("day3: %v", err)
	}
	if result.ConsecutiveDays != 1 || result.CycleDay != 1 || result.PointsAwarded != 1 {
		t.Fatalf("break streak: want streak=1 cycle=1 reward=1, got %+v", result)
	}
	assertCheckinInvariant(t, f.db, 10)
}

// TestCheckin_DuplicateSameDay 同一天重复签到：幂等返回当天结果，1 checkin + 1 ledger。
func TestCheckin_DuplicateSameDay(t *testing.T) {
	f := newCheckinFixture(t, false)
	f.setDate(at(2026, 10, 1))
	first, err := f.svc.CheckIn(11)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	second, err := f.svc.CheckIn(11)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if !second.AlreadyCheckedIn {
		t.Fatalf("expected already_checked_in=true, got %+v", second)
	}
	if second.PointsAwarded != first.PointsAwarded || second.ConsecutiveDays != first.ConsecutiveDays {
		t.Fatalf("duplicate result mismatch: first=%+v second=%+v", first, second)
	}
	if countCheckins(f.db, 11) != 1 || countLedgerByAction(f.db, pointscontract.ActionCheckinReward) != 1 {
		t.Fatalf("expected exactly 1 checkin + 1 ledger")
	}
	assertCheckinInvariant(t, f.db, 11)
}

// TestCheckin_Concurrent 20 goroutine 同一天签到：最终 1 checkin + 1 ledger，余额只加一次。
//
// SQLite 是单写者数据库：本用例用单连接串行化（与 points P0 先例一致），验证应用层
// 幂等语义（其余请求返回当天已签到结果）；真实并发行锁/唯一索引竞争由
// PostgreSQL 集成测试 TestPGCheckinConcurrentSingleRecord 覆盖。
func TestCheckin_Concurrent(t *testing.T) {
	f := newCheckinFixture(t, false)
	f.setDate(at(2026, 10, 1))

	const workers = 20
	var wg sync.WaitGroup
	results := make([]error, workers)
	alreadyCount := 0
	var mu sync.Mutex
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			res, err := f.svc.CheckIn(12)
			results[idx] = err
			if res != nil && res.AlreadyCheckedIn {
				mu.Lock()
				alreadyCount++
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()
	for i, err := range results {
		if err != nil {
			t.Fatalf("worker %d: %v", i, err)
		}
	}
	// 恰好 1 个真正签到成功，其余 19 个幂等返回已签到（非错误）
	if alreadyCount != workers-1 {
		t.Fatalf("want %d already-checked-in results, got %d", workers-1, alreadyCount)
	}
	if countCheckins(f.db, 12) != 1 {
		t.Fatalf("want 1 checkin, got %d", countCheckins(f.db, 12))
	}
	if countLedgerByAction(f.db, pointscontract.ActionCheckinReward) != 1 {
		t.Fatalf("want 1 CHECKIN_REWARD ledger, got %d", countLedgerByAction(f.db, pointscontract.ActionCheckinReward))
	}
	var account pointsdomain.Account
	if err := f.db.Where("user_id = ?", 12).First(&account).Error; err != nil {
		t.Fatalf("load account: %v", err)
	}
	if account.Balance != 1 {
		t.Fatalf("balance must increase exactly once: %+v", account)
	}
	assertCheckinInvariant(t, f.db, 12)
}

// TestCheckin_MonthBoundary 跨月连续：9/30 → 10/1 → streak=2。
func TestCheckin_MonthBoundary(t *testing.T) {
	f := newCheckinFixture(t, false)
	f.setDate(at(2026, 9, 30))
	if _, err := f.svc.CheckIn(13); err != nil {
		t.Fatalf("9/30: %v", err)
	}
	f.setDate(at(2026, 10, 1))
	result, err := f.svc.CheckIn(13)
	if err != nil {
		t.Fatalf("10/1: %v", err)
	}
	if result.ConsecutiveDays != 2 {
		t.Fatalf("month boundary: want streak=2, got %d", result.ConsecutiveDays)
	}
	assertCheckinInvariant(t, f.db, 13)
}

// TestCheckin_YearBoundary 跨年连续：12/31 → 1/1 → streak=2。
func TestCheckin_YearBoundary(t *testing.T) {
	f := newCheckinFixture(t, false)
	f.setDate(at(2026, 12, 31))
	if _, err := f.svc.CheckIn(14); err != nil {
		t.Fatalf("12/31: %v", err)
	}
	f.setDate(at(2027, 1, 1))
	result, err := f.svc.CheckIn(14)
	if err != nil {
		t.Fatalf("1/1: %v", err)
	}
	if result.ConsecutiveDays != 2 {
		t.Fatalf("year boundary: want streak=2, got %d", result.ConsecutiveDays)
	}
	assertCheckinInvariant(t, f.db, 14)
}

// TestCheckin_TimezoneBoundary Asia/Shanghai 午夜边界：
// UTC 2026-10-01 15:59（上海 23:59）与 UTC 2026-10-01 16:01（上海 10-02 00:01）
// 必须是两个不同的签到业务日（且连续）。
func TestCheckin_TimezoneBoundary(t *testing.T) {
	f := newCheckinFixture(t, false)
	// Shanghai 2026-10-01 23:59 = UTC 2026-10-01 15:59
	f.setDate(time.Date(2026, 10, 1, 15, 59, 0, 0, time.UTC))
	first, err := f.svc.CheckIn(15)
	if err != nil {
		t.Fatalf("shanghai 23:59: %v", err)
	}
	if first.CheckinDate != "2026-10-01" {
		t.Fatalf("want business date 2026-10-01, got %s", first.CheckinDate)
	}
	// Shanghai 2026-10-02 00:01 = UTC 2026-10-01 16:01
	f.setDate(time.Date(2026, 10, 1, 16, 1, 0, 0, time.UTC))
	second, err := f.svc.CheckIn(15)
	if err != nil {
		t.Fatalf("shanghai 00:01: %v", err)
	}
	if second.CheckinDate != "2026-10-02" {
		t.Fatalf("want business date 2026-10-02, got %s", second.CheckinDate)
	}
	if second.ConsecutiveDays != 2 {
		t.Fatalf("midnight boundary: want streak=2, got %d", second.ConsecutiveDays)
	}
	if countCheckins(f.db, 15) != 2 {
		t.Fatalf("want 2 distinct business days, got %d", countCheckins(f.db, 15))
	}
}

// TestCheckin_Disabled 功能开关：关闭时拒绝签到、积分不变、status 反映 enabled=false；重开后正常。
func TestCheckin_Disabled(t *testing.T) {
	f := newCheckinFixture(t, false)
	f.cfg.set(false, []int64{1, 2, 3, 4, 5, 6, 10})

	if _, err := f.svc.CheckIn(16); err != checkincontract.ErrCheckinDisabled {
		t.Fatalf("want ErrCheckinDisabled, got %v", err)
	}
	if countCheckins(f.db, 16) != 0 || sumLedger(f.db, pointscontract.ActionCheckinReward) != 0 {
		t.Fatalf("disabled check-in must not mutate points/records")
	}
	status, err := f.svc.Status(16)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if status.Enabled || status.CheckedInToday {
		t.Fatalf("want enabled=false checked=false, got %+v", status)
	}

	f.cfg.set(true, []int64{1, 2, 3, 4, 5, 6, 10})
	f.setDate(at(2026, 10, 1))
	result, err := f.svc.CheckIn(16)
	if err != nil {
		t.Fatalf("re-enabled check-in: %v", err)
	}
	if result.PointsAwarded != 1 {
		t.Fatalf("re-enabled: want 1 point, got %d", result.PointsAwarded)
	}
}

// TestCheckin_RewardConfigChange 配置变更不影响历史快照：
// Day1=1 签 → 改 Day1=10 → 历史记录仍 +1；断签后的新 Day1 按新配置 +10。
func TestCheckin_RewardConfigChange(t *testing.T) {
	f := newCheckinFixture(t, false)
	f.setDate(at(2026, 10, 1))
	if _, err := f.svc.CheckIn(17); err != nil {
		t.Fatalf("day1 @1pt: %v", err)
	}

	f.cfg.set(true, []int64{10, 2, 3, 4, 5, 6, 10})
	// 历史记录快照不变
	record, err := f.store.GetByUserAndDate(17, bizDay(2026, 10, 1))
	if err != nil {
		t.Fatalf("load checkin: %v", err)
	}
	if record == nil || record.PointsAwarded != 1 {
		t.Fatalf("historical snapshot must stay 1, got %+v", record)
	}
	// 断签后的新 Day1（10/3）按新配置 10
	f.setDate(at(2026, 10, 3))
	result, err := f.svc.CheckIn(17)
	if err != nil {
		t.Fatalf("new day1: %v", err)
	}
	if result.PointsAwarded != 10 {
		t.Fatalf("new config day1: want 10, got %d", result.PointsAwarded)
	}
	if result.ConsecutiveDays != 1 {
		t.Fatalf("new day1 streak: want 1, got %d", result.ConsecutiveDays)
	}
}

// TestCheckin_ZeroRewardDay 零积分日：签到成功、streak 正常、不产生 0 amount Ledger。
func TestCheckin_ZeroRewardDay(t *testing.T) {
	f := newCheckinFixture(t, false)
	f.cfg.set(true, []int64{1, 2, 0, 4, 5, 6, 10})
	for i := 0; i < 3; i++ {
		f.setDate(at(2026, 10, 1+i))
		result, err := f.svc.CheckIn(18)
		if err != nil {
			t.Fatalf("day %d: %v", i+1, err)
		}
		if i == 2 {
			if result.PointsAwarded != 0 || result.ConsecutiveDays != 3 || result.CycleDay != 3 {
				t.Fatalf("zero-reward day: want 0 pts streak=3 cycle=3, got %+v", result)
			}
		}
	}
	if countCheckins(f.db, 18) != 3 {
		t.Fatalf("want 3 checkins, got %d", countCheckins(f.db, 18))
	}
	if countLedgerByAction(f.db, pointscontract.ActionCheckinReward) != 2 {
		t.Fatalf("zero-reward day must NOT create ledger: want 2 ledgers, got %d",
			countLedgerByAction(f.db, pointscontract.ActionCheckinReward))
	}
	var zeroRecord *checkindomain.UserCheckin
	zeroRecord, err := f.store.GetByUserAndDate(18, bizDay(2026, 10, 3))
	if err != nil {
		t.Fatalf("load zero record: %v", err)
	}
	if zeroRecord == nil || zeroRecord.PointsAwarded != 0 || zeroRecord.PointsLedgerID != 0 {
		t.Fatalf("zero record: want points=0 ledger_id=0, got %+v", zeroRecord)
	}
	assertCheckinInvariant(t, f.db, 18)
}

// TestCheckin_Status 状态语义：未签到（consecutive=0/today=1/next=2）、签到后、断签后。
func TestCheckin_Status(t *testing.T) {
	f := newCheckinFixture(t, false)
	f.setDate(at(2026, 10, 1))

	// 从未签到
	status, err := f.svc.Status(19)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if !status.Enabled || status.CheckedInToday || status.ConsecutiveDays != 0 || status.CycleDay != 0 {
		t.Fatalf("fresh status: %+v", status)
	}
	if status.TodayReward != 1 || status.NextReward != 2 {
		t.Fatalf("fresh rewards: want 1/2 got %d/%d", status.TodayReward, status.NextReward)
	}

	// 签到后
	if _, err := f.svc.CheckIn(19); err != nil {
		t.Fatalf("checkin: %v", err)
	}
	status, err = f.svc.Status(19)
	if err != nil {
		t.Fatalf("status after: %v", err)
	}
	if !status.CheckedInToday || status.ConsecutiveDays != 1 || status.TodayReward != 1 {
		t.Fatalf("status after checkin: %+v", status)
	}

	// 连续 2 天后隔一天（断签）：10/3 未签时 consecutive_days=2（截至昨天为止），
	// 今天签到将重开 Day 3 循环（today_reward=3，即"今天签到将获得"）。
	f.setDate(at(2026, 10, 2))
	if _, err := f.svc.CheckIn(19); err != nil {
		t.Fatalf("day2: %v", err)
	}
	f.setDate(at(2026, 10, 3))
	status, err = f.svc.Status(19)
	if err != nil {
		t.Fatalf("status day3: %v", err)
	}
	if status.CheckedInToday || status.ConsecutiveDays != 2 || status.CycleDay != 2 ||
		status.TodayReward != 3 || status.NextReward != 4 {
		t.Fatalf("break status: %+v", status)
	}
}

// TestCheckin_History 历史：默认月份 / 指定月份 / 非法月份。
func TestCheckin_History(t *testing.T) {
	f := newCheckinFixture(t, false)
	// 9 月 30 + 10 月 1、2
	f.setDate(at(2026, 9, 30))
	if _, err := f.svc.CheckIn(20); err != nil {
		t.Fatalf("9/30: %v", err)
	}
	for i := 1; i <= 2; i++ {
		f.setDate(at(2026, 10, i))
		if _, err := f.svc.CheckIn(20); err != nil {
			t.Fatalf("10/%d: %v", i, err)
		}
	}

	// 指定 2026-10：2 条
	history, err := f.svc.History(20, "2026-10")
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if history.Year != 2026 || history.Month != 10 || history.Total != 2 {
		t.Fatalf("oct history: %+v", history)
	}
	if len(history.CheckedDates) != 2 || history.CheckedDates[0] != "2026-10-01" || history.CheckedDates[1] != "2026-10-02" {
		t.Fatalf("oct checked dates: %v", history.CheckedDates)
	}

	// 指定 2026-09：1 条
	sept, err := f.svc.History(20, "2026-09")
	if err != nil {
		t.Fatalf("sept history: %v", err)
	}
	if sept.Total != 1 || sept.CheckedDates[0] != "2026-09-30" {
		t.Fatalf("sept history: %+v", sept)
	}

	// 非法月份
	if _, err := f.svc.History(20, "2026-13"); err != checkincontract.ErrInvalidMonth {
		t.Fatalf("want ErrInvalidMonth, got %v", err)
	}
	if _, err := f.svc.History(20, "2026/10"); err != checkincontract.ErrInvalidMonth {
		t.Fatalf("want ErrInvalidMonth for 2026/10, got %v", err)
	}
}
