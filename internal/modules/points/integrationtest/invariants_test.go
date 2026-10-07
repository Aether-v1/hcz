// invariants_test.go — Points 系统跨模块全局不变量测试（P4 §38–§42）。
//
// 与 P0–P3 的单模块测试不同，本文件在同一张库里同时驱动
// Points Core / Check-in / Points Mall / Order Reward / Admin Compensation，
// 然后用**面向全集**的断言（遍历所有账户、所有签到行、所有兑换单）验证事实之间不漂移：
//
//	所有用户 account.balance == SUM(ledger.amount)
//	所有账户 total_earned / total_spent 与 actionMetas 语义一致
//	流水链式连续：balance_before + amount == balance_after，且累计与账户一致
//	reference 全局唯一（幂等最终防线）
//	同用户同业务日至多一条签到；有奖励签到恰一条 CHECKIN_REWARD；零奖励签到无流水
//	同订单至多一条 ORDER_REWARD；冲正累计不超过发放
//	每兑换单恰一条 REDEEM；仅 FAILED/CANCELLED 恰一条 REDEEM_REFUND；返还次数不超过扣减次数
package integrationtest

import (
	"fmt"
	"path/filepath"
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
	pointsmallapp "github.com/Aether-v1/hcz/internal/modules/pointsmall/application"
	pointsmallcontract "github.com/Aether-v1/hcz/internal/modules/pointsmall/contract"
	pointsmalldomain "github.com/Aether-v1/hcz/internal/modules/pointsmall/domain"
	pointsmallgormstore "github.com/Aether-v1/hcz/internal/modules/pointsmall/infrastructure/gormstore"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// fixture
// ---------------------------------------------------------------------------

type staticCheckinConfig struct {
	enabled bool
	rewards []int64
}

func (c staticCheckinConfig) GetCheckinConfig() (checkincontract.Config, error) {
	return checkincontract.Config{Enabled: c.enabled, Rewards: append([]int64(nil), c.rewards...)}, nil
}

type fixedClock struct{ at time.Time }

func (c fixedClock) Now() time.Time { return c.at }

type invariantFixture struct {
	db        *gorm.DB
	pointsSvc *pointsapp.Service
	checkin   *checkinapp.Service
	mall      *pointsmallapp.Service
	clock     *fixedClock
}

func dayAt(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 12, 0, 0, 0, time.UTC)
}

func dayOf(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func newInvariantFixture(t *testing.T) *invariantFixture {
	t.Helper()
	dir := t.TempDir()
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)",
		filepath.Join(dir, "invariants.db"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&pointsdomain.Account{}, &pointsdomain.LedgerEntry{},
		&checkindomain.UserCheckin{},
		&pointsmalldomain.PointsProduct{}, &pointsmalldomain.ExchangeOrder{},
	); err != nil {
		t.Fatalf("migrate schema: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("raw db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })

	pointsStore := pointsgormstore.New(db)
	checkinStore := checkingormstore.New(db)
	mallStore := pointsmallgormstore.New(db)
	pointsSvc := pointsapp.NewService(pointsapp.Options{Repository: pointsStore, Transactions: pointsStore})
	clock := &fixedClock{at: dayAt(2026, 10, 1)}
	checkinSvc := checkinapp.NewService(checkinapp.Options{
		Repository: checkinStore,
		UnitOfWork: checkinStore,
		Config:     staticCheckinConfig{enabled: true, rewards: []int64{1, 2, 3, 4, 5, 6, 10}},
		Clock:      clock,
		Points:     pointsSvc,
	})
	mallSvc := pointsmallapp.NewService(pointsmallapp.Options{
		Repository: mallStore,
		UnitOfWork: mallStore,
		Points:     pointsSvc,
	})
	return &invariantFixture{db: db, pointsSvc: pointsSvc, checkin: checkinSvc, mall: mallSvc, clock: clock}
}

func (f *invariantFixture) mustAdminAdd(t *testing.T, userID uint, amount int64, key string) {
	t.Helper()
	if _, _, err := f.pointsSvc.AdminAdjust(pointscontract.AdjustInput{
		UserID:          userID,
		OperatorAdminID: 9001,
		Operation:       "add",
		Amount:          amount,
		Reason:          "不变量测试充值",
		Reference:       pointscontract.AdminAdjustReference(key),
	}); err != nil {
		t.Fatalf("admin add user=%d amount=%d: %v", userID, amount, err)
	}
}

func (f *invariantFixture) mustAdminSubtract(t *testing.T, userID uint, amount int64, key string) {
	t.Helper()
	if _, _, err := f.pointsSvc.AdminAdjust(pointscontract.AdjustInput{
		UserID:          userID,
		OperatorAdminID: 9001,
		Operation:       "subtract",
		Amount:          amount,
		Reason:          "不变量测试扣减",
		Reference:       pointscontract.AdminAdjustReference(key),
	}); err != nil {
		t.Fatalf("admin subtract user=%d amount=%d: %v", userID, amount, err)
	}
}

// mustOrderReward 在独立事务内模拟订单完成发奖（与订单模块的调用方式一致：同事务）。
func (f *invariantFixture) mustOrderReward(t *testing.T, userID, orderID uint, amount int64) {
	t.Helper()
	if err := f.txWithin(func(tx pointscontract.Transaction) error {
		return f.pointsSvc.RewardOrderCompleted(tx, pointscontract.OrderRewardInput{
			UserID:    userID,
			OrderID:   orderID,
			Amount:    amount,
			Reason:    "订单完成奖励",
			Reference: pointscontract.OrderRewardReference(orderID),
		})
	}); err != nil {
		t.Fatalf("order reward order=%d: %v", orderID, err)
	}
}

func (f *invariantFixture) mustOrderReversal(t *testing.T, userID, orderID, refundRecordID uint, amount int64) {
	t.Helper()
	if err := f.txWithin(func(tx pointscontract.Transaction) error {
		return f.pointsSvc.ReverseOrderReward(tx, pointscontract.OrderReversalInput{
			UserID:         userID,
			OrderID:        orderID,
			RefundRecordID: refundRecordID,
			Amount:         amount,
			Reason:         "退款冲正",
			Reference:      pointscontract.OrderRefundReversalReference(refundRecordID),
		})
	}); err != nil {
		t.Fatalf("order reversal refund=%d: %v", refundRecordID, err)
	}
}

func (f *invariantFixture) txWithin(fn func(pointscontract.Transaction) error) error {
	store := pointsgormstore.New(f.db)
	return store.WithinTransaction(fn)
}

func (f *invariantFixture) mustCheckin(t *testing.T, userID uint, on time.Time) {
	t.Helper()
	f.clock.at = on
	if _, err := f.checkin.CheckIn(userID); err != nil {
		t.Fatalf("checkin user=%d date=%s: %v", userID, on.Format("2006-01-02"), err)
	}
}

func (f *invariantFixture) createProduct(t *testing.T, name string, price, stock int64, unlimited bool, limit int64) uint {
	t.Helper()
	product, err := f.mall.CreateProduct(pointsmallcontract.ProductInput{
		Name:            name,
		PointsPrice:     price,
		Stock:           stock,
		UnlimitedStock:  unlimited,
		Enabled:         true,
		PerUserLimit:    limit,
		FulfillmentType: pointsmalldomain.FulfillmentTypeManual,
		OperatorAdminID: 9001,
		Reason:          "不变量测试建品",
	})
	if err != nil {
		t.Fatalf("create product %s: %v", name, err)
	}
	return product.ID
}

func (f *invariantFixture) mustExchange(t *testing.T, userID, productID uint, key string) *pointsmallcontract.ExchangeResult {
	t.Helper()
	result, err := f.mall.CreateExchange(pointsmallcontract.CreateExchangeInput{
		UserID:         userID,
		ProductID:      productID,
		IdempotencyKey: key,
	})
	if err != nil {
		t.Fatalf("exchange user=%d product=%d: %v", userID, productID, err)
	}
	return result
}

// driveWorkload 产生一批覆盖全部 action 类型的真实业务流水。
func (f *invariantFixture) driveWorkload(t *testing.T) {
	t.Helper()
	productID := f.createProduct(t, "月度会员", 100, 10, false, 0)
	unlimitedID := f.createProduct(t, "虚拟徽章", 30, 0, true, 1)

	// 用户 101：充值 + 3 天签到 + 订单奖励 + 部分冲正 + 兑换后履约失败 + 人工补偿
	f.mustAdminAdd(t, 101, 1000, "seed-101")
	f.mustCheckin(t, 101, dayAt(2026, 10, 1))
	f.mustCheckin(t, 101, dayAt(2026, 10, 2))
	f.mustCheckin(t, 101, dayAt(2026, 10, 3))
	f.mustOrderReward(t, 101, 5001, 50)
	f.mustOrderReversal(t, 101, 5001, 7001, 20)
	failedOrderID := f.mustExchange(t, 101, productID, "ex-101-a").Order.ID
	if _, err := f.mall.AdminProcessOrder(pointsmallcontract.AdminExchangeActionInput{AdminID: 9001, OrderID: failedOrderID}); err != nil {
		t.Fatalf("process: %v", err)
	}
	if _, err := f.mall.AdminFailOrder(pointsmallcontract.AdminExchangeActionInput{
		AdminID: 9001, OrderID: failedOrderID, Note: "库存不足无法发放", Reason: "履约失败返还",
	}); err != nil {
		t.Fatalf("fail: %v", err)
	}
	if _, _, err := f.pointsSvc.AdminCompensate(pointscontract.CompensateInput{
		UserID: 101, OperatorAdminID: 9001, Amount: 7,
		Reason: "历史漏发补偿", Reference: pointscontract.AdminCompensationReference("cmp-101"), OrderID: 5001,
	}); err != nil {
		t.Fatalf("compensate: %v", err)
	}

	// 用户 102：充值 + 签到 + 兑换完成 + 无限库存兑换后用户取消
	f.mustAdminAdd(t, 102, 200, "seed-102")
	f.mustCheckin(t, 102, dayAt(2026, 10, 1))
	orderToComplete := f.mustExchange(t, 102, productID, "ex-102-a").Order.ID
	if _, err := f.mall.AdminProcessOrder(pointsmallcontract.AdminExchangeActionInput{AdminID: 9001, OrderID: orderToComplete}); err != nil {
		t.Fatalf("process 102: %v", err)
	}
	if _, err := f.mall.AdminCompleteOrder(pointsmallcontract.AdminExchangeActionInput{AdminID: 9001, OrderID: orderToComplete, Note: "已发放"}); err != nil {
		t.Fatalf("complete 102: %v", err)
	}
	orderToCancel := f.mustExchange(t, 102, unlimitedID, "ex-102-b").Order.ID
	if _, err := f.mall.CancelOrder(pointsmallcontract.CancelInput{UserID: 102, OrderID: orderToCancel}); err != nil {
		t.Fatalf("cancel 102: %v", err)
	}

	// 用户 103：Admin 扣减穿透到负余额（系统侧允许，用户消费侧禁止）
	f.mustAdminAdd(t, 103, 10, "seed-103")
	f.mustAdminSubtract(t, 103, 40, "deduct-103")
	if _, err := f.mall.CreateExchange(pointsmallcontract.CreateExchangeInput{
		UserID: 103, ProductID: unlimitedID, IdempotencyKey: "ex-103-should-fail",
	}); err == nil {
		t.Fatalf("negative-balance user must not redeem")
	}

	// 用户 104：零奖励日（签到成功但不产生流水，也不产生账户）
	f.clock.at = dayAt(2026, 10, 1)
	zeroRewardService := checkinapp.NewService(checkinapp.Options{
		Repository: checkingormstore.New(f.db),
		UnitOfWork: checkingormstore.New(f.db),
		Config:     staticCheckinConfig{enabled: true, rewards: []int64{0, 2, 3, 4, 5, 6, 10}},
		Clock:      f.clock,
		Points:     f.pointsSvc,
	})
	if _, err := zeroRewardService.CheckIn(104); err != nil {
		t.Fatalf("zero-reward checkin: %v", err)
	}
}

// ---------------------------------------------------------------------------
// 不变量断言
// ---------------------------------------------------------------------------

// earnedActions 与 actionMetas.trackEarned 一一对应（若语义表变动，本测试必须同步）。
var earnedActions = []string{
	pointscontract.ActionAdminAdd,
	pointscontract.ActionAdminCompensation,
	pointscontract.ActionOrderReward,
	pointscontract.ActionCheckinReward,
	pointscontract.ActionRedeemRefund,
}

func TestPointsGlobalInvariants(t *testing.T) {
	if testing.Short() {
		t.Skip("skip in short mode")
	}
	f := newInvariantFixture(t)
	f.driveWorkload(t)

	t.Run("every_account_balance_equals_ledger_sum", func(t *testing.T) {
		var accounts []pointsdomain.Account
		if err := f.db.Order("user_id").Find(&accounts).Error; err != nil {
			t.Fatalf("list accounts: %v", err)
		}
		if len(accounts) != 3 {
			t.Fatalf("want 3 accounts (101/102/103), got %d", len(accounts))
		}
		for _, account := range accounts {
			var sum int64
			if err := f.db.Model(&pointsdomain.LedgerEntry{}).
				Where("user_id = ?", account.UserID).
				Select("COALESCE(SUM(amount),0)").Scan(&sum).Error; err != nil {
				t.Fatalf("sum ledger user=%d: %v", account.UserID, err)
			}
			if account.Balance != sum {
				t.Fatalf("user=%d balance=%d ledger_sum=%d", account.UserID, account.Balance, sum)
			}
		}
		// 零奖励签到不得凭空创建账户（§39：签到事实与积分事实一致）。
		var zeroAccountCount int64
		f.db.Model(&pointsdomain.Account{}).Where("user_id = ?", 104).Count(&zeroAccountCount)
		if zeroAccountCount != 0 {
			t.Fatalf("user 104 has no points movement but an account exists")
		}
	})

	t.Run("total_earned_and_total_spent_match_action_semantics", func(t *testing.T) {
		type totals struct {
			earned int64
			spent  int64
		}
		for _, userID := range []uint{101, 102, 103} {
			var account pointsdomain.Account
			if err := f.db.Where("user_id = ?", userID).First(&account).Error; err != nil {
				t.Fatalf("load account user=%d: %v", userID, err)
			}
			var earned int64
			if err := f.db.Model(&pointsdomain.LedgerEntry{}).
				Where("user_id = ? AND action_type IN ?", userID, earnedActions).
				Select("COALESCE(SUM(amount),0)").Scan(&earned).Error; err != nil {
				t.Fatalf("sum earned user=%d: %v", userID, err)
			}
			var spent int64
			if err := f.db.Model(&pointsdomain.LedgerEntry{}).
				Where("user_id = ? AND action_type = ?", userID, pointscontract.ActionRedeem).
				Select("COALESCE(-SUM(amount),0)").Scan(&spent).Error; err != nil {
				t.Fatalf("sum spent user=%d: %v", userID, err)
			}
			if account.TotalEarned != earned {
				t.Fatalf("user=%d total_earned=%d but earned ledger=%d", userID, account.TotalEarned, earned)
			}
			if account.TotalSpent != spent {
				t.Fatalf("user=%d total_spent=%d but REDEEM ledger=%d", userID, account.TotalSpent, spent)
			}
			// total_spent 是历史累计消费口径：即使积分已返还，也不回滚（§21）。
			if account.TotalSpent+account.TotalEarned < account.Balance {
				t.Fatalf("user=%d totals inconsistent with balance", userID)
			}
		}
	})

	t.Run("ledger_chain_is_continuous", func(t *testing.T) {
		var entries []pointsdomain.LedgerEntry
		if err := f.db.Order("user_id, created_at, id").Find(&entries).Error; err != nil {
			t.Fatalf("list ledger: %v", err)
		}
		running := map[uint]int64{}
		seenRef := map[string]uint{}
		for _, e := range entries {
			before, ok := running[e.UserID]
			if !ok {
				before = 0
			}
			if e.BalanceBefore != before {
				t.Fatalf("ledger user=%d id=%d balance_before=%d but previous balance_after=%d",
					e.UserID, e.ID, e.BalanceBefore, before)
			}
			if e.BalanceBefore+e.Amount != e.BalanceAfter {
				t.Fatalf("ledger id=%d chain broken: %d + %d != %d", e.ID, e.BalanceBefore, e.Amount, e.BalanceAfter)
			}
			if e.Amount > 0 && e.ActionType != pointscontract.ActionAdminAdd &&
				e.ActionType != pointscontract.ActionAdminCompensation &&
				e.ActionType != pointscontract.ActionOrderReward &&
				e.ActionType != pointscontract.ActionCheckinReward &&
				e.ActionType != pointscontract.ActionRedeemRefund {
				t.Fatalf("ledger id=%d unexpected credit action %s", e.ID, e.ActionType)
			}
			if e.Amount < 0 && e.ActionType != pointscontract.ActionAdminDeduct &&
				e.ActionType != pointscontract.ActionOrderRewardReversal &&
				e.ActionType != pointscontract.ActionRedeem {
				t.Fatalf("ledger id=%d unexpected debit action %s", e.ID, e.ActionType)
			}
			if previous, dup := seenRef[e.Reference]; dup {
				t.Fatalf("reference %q reused by ledger id=%d and id=%d", e.Reference, previous, e.ID)
			}
			seenRef[e.Reference] = e.ID
			running[e.UserID] = e.BalanceAfter
		}
		for userID, balance := range running {
			var account pointsdomain.Account
			if err := f.db.Where("user_id = ?", userID).First(&account).Error; err != nil {
				t.Fatalf("load account user=%d: %v", userID, err)
			}
			if account.Balance != balance {
				t.Fatalf("user=%d account balance=%d but ledger chain ends at %d", userID, account.Balance, balance)
			}
		}
	})

	t.Run("checkin_records_are_unique_and_reward_linked", func(t *testing.T) {
		type dup struct {
			UserID uint
			Date   time.Time
			Count  int
		}
		var dups []dup
		if err := f.db.Model(&checkindomain.UserCheckin{}).
			Select("user_id, checkin_date, COUNT(*) AS count").
			Group("user_id, checkin_date").
			Having("COUNT(*) > 1").Scan(&dups).Error; err != nil {
			t.Fatalf("checkin uniqueness query: %v", err)
		}
		if len(dups) > 0 {
			t.Fatalf("duplicate checkin rows: %+v", dups)
		}

		var records []checkindomain.UserCheckin
		if err := f.db.Order("user_id, checkin_date").Find(&records).Error; err != nil {
			t.Fatalf("list checkins: %v", err)
		}
		if len(records) != 5 {
			t.Fatalf("want 5 checkin rows (101×3 + 102×1 + 104×1), got %d", len(records))
		}
		for _, record := range records {
			reference := pointscontract.CheckinReference(record.UserID, record.CheckinDate)
			var entries []pointsdomain.LedgerEntry
			if err := f.db.Where("reference = ?", reference).Find(&entries).Error; err != nil {
				t.Fatalf("load checkin ledger %s: %v", reference, err)
			}
			if record.PointsAwarded == 0 {
				if len(entries) != 0 {
					t.Fatalf("zero-reward checkin user=%d date=%v produced %d ledger rows", record.UserID, record.CheckinDate, len(entries))
				}
				continue
			}
			if len(entries) != 1 {
				t.Fatalf("checkin user=%d date=%v want exactly 1 ledger, got %d", record.UserID, record.CheckinDate, len(entries))
			}
			entry := entries[0]
			if entry.ActionType != pointscontract.ActionCheckinReward || entry.UserID != record.UserID {
				t.Fatalf("checkin ledger mismatch: %+v", entry)
			}
			if entry.Amount != record.PointsAwarded {
				t.Fatalf("checkin user=%d date=%v awarded=%d but ledger amount=%d",
					record.UserID, record.CheckinDate, record.PointsAwarded, entry.Amount)
			}
			if entry.OrderID != nil {
				t.Fatalf("checkin ledger must not carry order id")
			}
		}
	})

	t.Run("order_reward_and_reversal_are_bounded", func(t *testing.T) {
		type row struct {
			OrderID uint
			Kind    string
			Amount  int64
			Count   int
		}
		var rows []row
		if err := f.db.Model(&pointsdomain.LedgerEntry{}).
			Select("order_id AS order_id, action_type AS kind, COALESCE(SUM(amount),0) AS amount, COUNT(*) AS count").
			Where("action_type IN ? AND order_id IS NOT NULL",
				[]string{pointscontract.ActionOrderReward, pointscontract.ActionOrderRewardReversal}).
			Group("order_id, action_type").Scan(&rows).Error; err != nil {
			t.Fatalf("order reward query: %v", err)
		}
		rewardCount := map[uint]int{}
		rewardSum := map[uint]int64{}
		reversedSum := map[uint]int64{}
		for _, r := range rows {
			switch r.Kind {
			case pointscontract.ActionOrderReward:
				rewardCount[r.OrderID] += r.Count
				rewardSum[r.OrderID] += r.Amount
			case pointscontract.ActionOrderRewardReversal:
				reversedSum[r.OrderID] += -r.Amount
			}
		}
		for orderID, count := range rewardCount {
			if count > 1 {
				t.Fatalf("order=%d has %d ORDER_REWARD rows (max 1)", orderID, count)
			}
		}
		for orderID := range reversedSum {
			if reversedSum[orderID] > rewardSum[orderID] {
				t.Fatalf("order=%d reversed=%d exceeds reward=%d", orderID, reversedSum[orderID], rewardSum[orderID])
			}
		}
		if rewardSum[5001] != 50 || reversedSum[5001] != 20 {
			t.Fatalf("order 5001 expected reward=50 reversed=20, got reward=%d reversed=%d", rewardSum[5001], reversedSum[5001])
		}

		// 全额退款应完全冲正：奖励 50 后按同额冲正，累计仍不超过发放额。
		f.mustOrderReversal(t, 101, 5002, 7002, 10)
		f.mustOrderReward(t, 101, 5003, 30)
		f.mustOrderReversal(t, 101, 5003, 7003, 30)
		if err := f.txWithin(func(tx pointscontract.Transaction) error {
			reward, reversed, err := f.pointsSvc.OrderRewardSummary(tx, 5003)
			if err != nil {
				return err
			}
			if reward != 30 || reversed != 30 {
				return fmt.Errorf("order 5003 want reward=30 reversed=30, got %d/%d", reward, reversed)
			}
			// 重放同一订单奖励：必须幂等（不新增流水）。
			return f.pointsSvc.RewardOrderCompleted(tx, pointscontract.OrderRewardInput{
				UserID: 101, OrderID: 5003, Amount: 30, Reason: "重放", Reference: pointscontract.OrderRewardReference(5003),
			})
		}); err != nil {
			t.Fatalf("order reward summary/replay: %v", err)
		}
		var replayed int64
		f.db.Model(&pointsdomain.LedgerEntry{}).
			Where("action_type = ? AND order_id = ?", pointscontract.ActionOrderReward, 5003).Count(&replayed)
		if replayed != 1 {
			t.Fatalf("replayed ORDER_REWARD rows=%d, want 1", replayed)
		}
	})

	t.Run("exchange_orders_redeem_and_refund_exact_counts", func(t *testing.T) {
		var orders []pointsmalldomain.ExchangeOrder
		if err := f.db.Order("id").Find(&orders).Error; err != nil {
			t.Fatalf("list exchange orders: %v", err)
		}
		if len(orders) != 3 {
			t.Fatalf("want 3 exchange orders (1 FAILED / 1 COMPLETED / 1 CANCELLED), got %d", len(orders))
		}
		var refundedCount int
		for _, order := range orders {
			var redeems, refunds int64
			if err := f.db.Model(&pointsdomain.LedgerEntry{}).
				Where("action_type = ? AND exchange_order_id = ?", pointscontract.ActionRedeem, order.ID).
				Count(&redeems).Error; err != nil {
				t.Fatalf("count redeem order=%d: %v", order.ID, err)
			}
			if err := f.db.Model(&pointsdomain.LedgerEntry{}).
				Where("action_type = ? AND exchange_order_id = ?", pointscontract.ActionRedeemRefund, order.ID).
				Count(&refunds).Error; err != nil {
				t.Fatalf("count refund order=%d: %v", order.ID, err)
			}
			if redeems != 1 {
				t.Fatalf("order=%d want exactly 1 REDEEM, got %d", order.ID, redeems)
			}
			refundable := order.Status == pointsmalldomain.ExchangeStatusFailed ||
				order.Status == pointsmalldomain.ExchangeStatusCancelled
			if refundable {
				if refunds != 1 {
					t.Fatalf("order=%d status=%s want exactly 1 REDEEM_REFUND, got %d", order.ID, order.Status, refunds)
				}
				var refund pointsdomain.LedgerEntry
				if err := f.db.Where("action_type = ? AND exchange_order_id = ?",
					pointscontract.ActionRedeemRefund, order.ID).First(&refund).Error; err != nil {
					t.Fatalf("load refund order=%d: %v", order.ID, err)
				}
				if refund.Amount != order.TotalPoints {
					t.Fatalf("order=%d refunded=%d but original total_points=%d", order.ID, refund.Amount, order.TotalPoints)
				}
				refundedCount++
				continue
			}
			if refunds != 0 {
				t.Fatalf("order=%d status=%s must have no REDEEM_REFUND, got %d", order.ID, order.Status, refunds)
			}
		}
		// 返还次数 == 需要返还的订单数（禁止重复返还）。
		var totalRefunds int64
		f.db.Model(&pointsdomain.LedgerEntry{}).
			Where("action_type = ?", pointscontract.ActionRedeemRefund).Count(&totalRefunds)
		if int(totalRefunds) != refundedCount {
			t.Fatalf("REDEEM_REFUND rows=%d but refundable orders=%d", totalRefunds, refundedCount)
		}
		// 失败/取消必须恢复库存：月度会员初始 10 件，被 101、102 各兑 1 件（-2），
		// 101 履约失败返还 1 件（+1）→ 净 9 件；徽章为无限库存不参与扣减。
		var product pointsmalldomain.PointsProduct
		if err := f.db.Where("name = ?", "月度会员").First(&product).Error; err != nil {
			t.Fatalf("load product: %v", err)
		}
		if product.Stock != 9 {
			t.Fatalf("月度会员 stock=%d, want 9 (10 - 2 redeemed + 1 restored)", product.Stock)
		}
		var badge pointsmalldomain.PointsProduct
		if err := f.db.Where("name = ?", "虚拟徽章").First(&badge).Error; err != nil {
			t.Fatalf("load badge: %v", err)
		}
		if badge.Stock != 0 {
			t.Fatalf("unlimited-stock badge must never be decremented, got stock=%d", badge.Stock)
		}
	})

	t.Run("negative_balance_stays_visible", func(t *testing.T) {
		var account pointsdomain.Account
		if err := f.db.Where("user_id = ?", 103).First(&account).Error; err != nil {
			t.Fatalf("load account 103: %v", err)
		}
		if account.Balance != -30 {
			t.Fatalf("user 103 balance=%d, want -30 (must not be clamped to 0)", account.Balance)
		}
		var negativeUserIDs []uint
		if err := f.db.Model(&pointsdomain.Account{}).
			Where("balance < 0").Order("user_id").Pluck("user_id", &negativeUserIDs).Error; err != nil {
			t.Fatalf("negative balance query: %v", err)
		}
		if len(negativeUserIDs) != 1 || negativeUserIDs[0] != 103 {
			t.Fatalf("negative-only filter got %v, want [103]", negativeUserIDs)
		}
	})

	t.Run("admin_idempotency_replays_without_new_ledger", func(t *testing.T) {
		var before, after int64
		f.db.Model(&pointsdomain.LedgerEntry{}).Count(&before)
		f.mustAdminAdd(t, 101, 1000, "seed-101") // 同 Idempotency-Key 重放
		f.db.Model(&pointsdomain.LedgerEntry{}).Count(&after)
		if after != before {
			t.Fatalf("replayed admin add created %d new ledger rows", after-before)
		}
	})
}
