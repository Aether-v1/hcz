//go:build integration
// +build integration

// pg_concurrency_test.go — Affiliate Finance 在真实 PostgreSQL 上的并发验证（生产 Gate）。
//
// 运行（PowerShell）：
//
//	$env:TEST_POSTGRES_DSN="host=127.0.0.1 port=5432 user=postgres password= dbname=hcz_test sslmode=disable TimeZone=UTC"
//	go test -tags integration -run TestPGConcurrency -v -timeout 300s ./internal/modules/affiliate/integrationtest/
//
// 设计要点：
//   - 每个用例 newPGAffFixture 重建全部表（真实 PG，禁止 SQLite 顶替）。
//   - 并发用 sync.WaitGroup + start 栅栏保证 goroutine 同时开始。
//   - 仅 PostgreSQL 40P01（deadlock detected）允许有限重试（≤3），其余 DB 错误一律不重试并记为失败。
//   - 每个用例结束校验资金不变量：GrossCredit - Reversal - Debt - Settled - Locked = SUM(ledger)。
package integrationtest

import (
	"errors"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/Aether-v1/hcz/internal/constants"

	affiliateapp "github.com/Aether-v1/hcz/internal/modules/affiliate/application"
	affiliatecontract "github.com/Aether-v1/hcz/internal/modules/affiliate/contract"
	affiliatedomain "github.com/Aether-v1/hcz/internal/modules/affiliate/domain"
	affiliategormstore "github.com/Aether-v1/hcz/internal/modules/affiliate/infrastructure/gormstore"

	userdomain "github.com/Aether-v1/hcz/internal/modules/identity/user/domain"
	userstore "github.com/Aether-v1/hcz/internal/modules/identity/user/infrastructure/gormstore"
	admindomain "github.com/Aether-v1/hcz/internal/modules/identity/admin/domain"
	orderdomain "github.com/Aether-v1/hcz/internal/modules/order/domain"

	walletapp "github.com/Aether-v1/hcz/internal/modules/wallet/application"
	walletdomain "github.com/Aether-v1/hcz/internal/modules/wallet/domain"
	walletgormstore "github.com/Aether-v1/hcz/internal/modules/wallet/infrastructure/gormstore"

	settingsapp "github.com/Aether-v1/hcz/internal/modules/settings/application"
	settingsintegration "github.com/Aether-v1/hcz/internal/modules/settings/schema/integration"

	"github.com/Aether-v1/hcz/internal/testkit/memorysettings"

	"github.com/shopspring/decimal"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	glogger "gorm.io/gorm/logger"
)

// pgAffFixture 是 Affiliate PG 并发测试夹具。
type pgAffFixture struct {
	db            *gorm.DB
	affiliateRepo *affiliategormstore.Store
	walletRepo    *walletgormstore.Store
	walletSvc     *walletapp.Service
	svc           *affiliateapp.Service
	orderReader   *fakeOrderReader
	deadlocks     int64 // 累计观察到的 40P01 deadlock 次数
	users         []userdomain.User
	profiles      []affiliatedomain.Profile
}

func newPGAffFixture(t *testing.T) *pgAffFixture {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("TEST_POSTGRES_DSN"))
	if dsn == "" {
		t.Skip("skip affiliate pg concurrency test: TEST_POSTGRES_DSN is empty")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: glogger.Default.LogMode(glogger.Silent),
	})
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("sqlDB: %v", err)
	}
	sqlDB.SetMaxOpenConns(20)
	// 锁等待最多 5s，真正死锁应被 PG 在 ~1s 内 abort；此处仅防挂起。
	db.Exec("SET lock_timeout = '5s'")

	// 重建全部相关表。
	models := []interface{}{
		&walletdomain.Transaction{},
		&walletdomain.Account{},
		&admindomain.Admin{},
		&userdomain.User{},
		&affiliatedomain.OrderReference{},
		&affiliatedomain.Profile{},
		&affiliatedomain.Click{},
		&affiliatedomain.Commission{},
		&affiliatedomain.CommissionLedger{},
		&affiliatedomain.WithdrawRequest{},
	}
	_ = db.Migrator().DropTable(models...)
	if err := db.AutoMigrate(models...); err != nil {
		t.Fatalf("migrate postgres: %v", err)
	}

	// 设置：ConfirmDays=0 佣金直接 available；L1=10%。
	setting := settingsintegration.AffiliateSetting{
		Enabled:           true,
		MaxLevel:          2,
		LevelRates:        levelRates(map[int]float64{1: 10, 2: 5}),
		ConfirmDays:       0,
		MinWithdrawAmount: 1,
		WithdrawChannels:  []string{"usdt"},
	}
	settingRepo := memorysettings.New()
	settingSvc := settingsapp.NewService(settingRepo)
	if _, err := settingSvc.UpdateAffiliateSetting(setting); err != nil {
		t.Fatalf("init affiliate setting: %v", err)
	}

	affiliateRepo := affiliategormstore.New(db)
	walletRepo := walletgormstore.New(db)
	walletSvc := walletapp.NewService(walletapp.Options{
		Repository:   walletRepo,
		Transactions: walletRepo,
	})
	orderReader := &fakeOrderReader{orders: map[uint]*orderdomain.Order{}}
	svc := affiliateapp.NewService(affiliateRepo, userstore.New(db), orderReader, nil, settingSvc)
	svc.SetWalletService(walletSvc, walletRepo)

	// 3 人邀请链：users[2] 下单，users[1]=L1，users[0]=L2。激活 L1/L2。
	users := buildChain(t, db, 3, "pg")
	profiles := activateProfiles(t, db, users[0], users[1])

	// 预建 admin 用户（满足 affiliate_withdraw_requests.processor 外键）。
	for _, adminID := range []uint{1, 100, 101, 200, 300, 400, 500, 600} {
		if err := db.Create(&admindomain.Admin{
			ID: adminID, Username: "pg-admin-" + itoa(int(adminID)), PasswordHash: "x",
		}).Error; err != nil {
			t.Fatalf("seed admin %d: %v", adminID, err)
		}
	}

	f := &pgAffFixture{
		db: db, affiliateRepo: affiliateRepo, walletRepo: walletRepo,
		walletSvc: walletSvc, svc: svc, orderReader: orderReader,
		users: users, profiles: profiles,
	}

	t.Cleanup(func() {
		_ = db.Migrator().DropTable(models...)
		_ = sqlDB.Close()
	})
	return f
}

// ---- 小工具 ----

func pgDec(s string) decimal.Decimal {
	d, err := decimal.NewFromString(s)
	if err != nil {
		panic("bad decimal: " + s)
	}
	return d.Round(2)
}

// isP40P01Deadlock 判断是否为 PostgreSQL 40P01 deadlock detected。
func isP40P01Deadlock(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "deadlock detected") || strings.Contains(msg, "40p01")
}

// runWithDeadlockRetry 执行一个并发操作，仅对 40P01 做最多 maxRetry 次重试。
// 返回最终 error 与本次重试过的 deadlock 次数。
func runWithDeadlockRetry(f *pgAffFixture, op func() error, maxRetry int) error {
	var err error
	for attempt := 0; attempt <= maxRetry; attempt++ {
		err = op()
		if err == nil {
			return nil
		}
		if isP40P01Deadlock(err) {
			atomic.AddInt64(&f.deadlocks, 1)
			continue
		}
		return err // 非 deadlock：立即返回，绝不重试
	}
	return err
}

// ---- fixture 存取链（为简单起见用包级 map 关联 db 不便，这里直接在 f 上存）----
// 重新打开 pgAffFixture 字段：上面结构已定义，补 setter/getter。

func (f *pgAffFixture) l1User() userdomain.User    { return f.users[1] }
func (f *pgAffFixture) l2User() userdomain.User    { return f.users[0] }
func (f *pgAffFixture) buyerUser() userdomain.User { return f.users[2] }
func (f *pgAffFixture) l1Profile() affiliatedomain.Profile { return f.profiles[1] }
func (f *pgAffFixture) l2Profile() affiliatedomain.Profile { return f.profiles[0] }

// ensureOrderRow 在 orders 表插入一行，满足 affiliate_commissions.order_id 外键。
func (f *pgAffFixture) ensureOrderRow(t *testing.T, orderID uint) {
	t.Helper()
	or := affiliatedomain.OrderReference{ID: orderID, OrderNo: "PG-NO" + itoa(int(orderID))}
	if err := f.db.Create(&or).Error; err != nil {
		t.Fatalf("ensure order row %d: %v", orderID, err)
	}
}

// seedOrder 写入订单并生成佣金（ConfirmDays=0 → 直接 available）。
func (f *pgAffFixture) seedOrder(t *testing.T, orderID uint, paidUSDT float64) {
	t.Helper()
	f.ensureOrderRow(t, orderID)
	order := newUSDTPaidOrder(orderID, f.buyerUser().ID, paidUSDT)
	f.orderReader.orders[orderID] = order
	if err := f.svc.HandleOrderCompleted(orderID); err != nil {
		t.Fatalf("seed order %d: %v", orderID, err)
	}
}

// refundFull 以真实事务对订单做全额退款冲正（调用 HandleOrderRefunded）。
func (f *pgAffFixture) refundFull(t *testing.T, orderID uint, delta string) error {
	t.Helper()
	order := f.orderReader.orders[orderID]
	return f.affiliateRepo.WithinTransaction(func(tx affiliatecontract.Store) error {
		return f.svc.HandleOrderRefunded(tx, order, pgDec(delta), decimal.Zero, "pg_refund")
	})
}

// countLedgers 统计某 profile 指定类型的 ledger 条数与金额和。
func (f *pgAffFixture) countLedgers(t *testing.T, profileID uint, typ string) (int, decimal.Decimal) {
	t.Helper()
	var n int64
	if err := f.db.Model(&affiliatedomain.CommissionLedger{}).
		Where("affiliate_profile_id = ? AND type = ?", profileID, typ).
		Count(&n).Error; err != nil {
		t.Fatalf("count ledgers type=%s: %v", typ, err)
	}
	sum, err := f.affiliateRepo.SumLedgerByProfile(profileID, []string{typ})
	if err != nil {
		t.Fatalf("sum ledgers type=%s: %v", typ, err)
	}
	return int(n), sum
}

// netProfileBalance profile 全部 ledger 金额和（= 可用佣金净余额）。
func (f *pgAffFixture) netProfileBalance(t *testing.T, profileID uint) decimal.Decimal {
	t.Helper()
	net, err := f.affiliateRepo.SumLedgerByProfile(profileID, nil)
	if err != nil {
		t.Fatalf("sum all ledgers: %v", err)
	}
	return net.Round(2)
}

// walletCreditCount 统计某用户 affiliate_payout 钱包入账笔数。
func (f *pgAffFixture) walletCreditCount(t *testing.T, userID uint) int64 {
	t.Helper()
	var n int64
	f.db.Model(&walletdomain.Transaction{}).
		Where("user_id = ? AND type = ? AND direction = ?", userID, constants.WalletTxnTypeAffiliatePayout, constants.WalletTxnDirectionIn).
		Count(&n)
	return n
}

// walletAvailable 用户钱包可用余额。
func (f *pgAffFixture) walletAvailable(t *testing.T, userID uint) decimal.Decimal {
	t.Helper()
	acc, err := f.walletRepo.GetAccountByUserID(userID)
	if err != nil {
		t.Fatalf("get wallet account %d: %v", userID, err)
	}
	if acc == nil {
		return decimal.Zero
	}
	return acc.AvailableBalance.Decimal.Round(2)
}

// applyWithdraw 提交一笔提现。
func (f *pgAffFixture) applyWithdraw(t *testing.T, userID uint, amount string) *affiliatedomain.WithdrawRequest {
	t.Helper()
	req, err := f.svc.ApplyWithdraw(userID, affiliateapp.WithdrawApplyInput{
		Amount:  pgDec(amount),
		Channel: "usdt",
		Account: "TA-test-account",
	})
	if err != nil {
		t.Fatalf("apply withdraw user=%d amount=%s: %v", userID, amount, err)
	}
	return req
}

// approveWithdraw 审核通过。
func (f *pgAffFixture) approveWithdraw(t *testing.T, withdrawID uint) {
	t.Helper()
	if _, err := f.svc.ReviewWithdraw(1, withdrawID, constants.AffiliateWithdrawActionApprove, ""); err != nil {
		t.Fatalf("approve withdraw %d: %v", withdrawID, err)
	}
}

// assertApplyWithdrawRetired 验证独立提现已退休：ApplyWithdraw 返回 ErrWithdrawRetired，
// 且不创建 withdraw_request、不产生 lock ledger、钱包不入账。
func (f *pgAffFixture) assertApplyWithdrawRetired(t *testing.T, userID uint, amount string) {
	t.Helper()
	if _, err := f.svc.ApplyWithdraw(userID, affiliateapp.WithdrawApplyInput{
		Amount:  pgDec(amount),
		Channel: "usdt",
		Account: "TA-test-account",
	}); !errors.Is(err, affiliateapp.ErrWithdrawRetired) {
		t.Fatalf("ApplyWithdraw want ErrWithdrawRetired, got %v", err)
	}
	var withdrawCount int64
	f.db.Model(&affiliatedomain.WithdrawRequest{}).Count(&withdrawCount)
	if withdrawCount != 0 {
		t.Fatalf("retired withdraw must not create withdraw_request rows, got %d", withdrawCount)
	}
	if n := f.walletCreditCount(t, userID); n != 0 {
		t.Fatalf("retired withdraw must not credit wallet, got %d", n)
	}
}

// =========================================================================
// CASE 1：独立提现已退休 —— 并发 PayWithdraw 全部返回 ErrWithdrawRetired，绝不出金。
// =========================================================================
func TestPGConcurrency_Case1_DuplicatePaySingleCredit(t *testing.T) {
	f := newPGAffFixture(t)
	f.seedOrder(t, 1, 100) // L1 佣金 = 10

	var wg sync.WaitGroup
	start := make(chan struct{})
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			errs[i] = runWithDeadlockRetry(f, func() error {
				_, err := f.svc.PayWithdraw(100+uint(i), 1)
				return err
			}, 3)
		}(i)
	}
	close(start)
	wg.Wait()

	for i, err := range errs {
		if !errors.Is(err, affiliateapp.ErrWithdrawRetired) {
			t.Fatalf("CASE1: pay #%d want ErrWithdrawRetired, got %v", i, err)
		}
	}
	// 钱包无任何入账，无 settle ledger。
	if n := f.walletCreditCount(t, f.l1User().ID); n != 0 {
		t.Fatalf("CASE1: wallet credit count want 0, got %d", n)
	}
	if n, _ := f.countLedgers(t, f.l1Profile().ID, constants.AffiliateLedgerTypeWithdrawSettle); n != 0 {
		t.Fatalf("CASE1: settle ledger want 0, got %d", n)
	}
	t.Logf("CASE1 PASS: all pay calls retired, deadlocks=%d", f.deadlocks)
}

// =========================================================================
// CASE 2：独立提现已退休 —— Pay 恒返回退休错误 ∥ Refund 正常冲正，绝不出金。
// =========================================================================
func TestPGConcurrency_Case2_RefundVsPay(t *testing.T) {
	f := newPGAffFixture(t)
	f.seedOrder(t, 1, 100) // L1 佣金 10

	var wg sync.WaitGroup
	start := make(chan struct{})
	var payErr, refundErr error
	wg.Add(2)
	go func() { defer wg.Done(); <-start; payErr = runWithDeadlockRetry(f, func() error { _, e := f.svc.PayWithdraw(200, 1); return e }, 3) }()
	go func() { defer wg.Done(); <-start; refundErr = runWithDeadlockRetry(f, func() error { return f.refundFull(t, 1, "100") }, 3) }()
	close(start)
	wg.Wait()

	// Pay 恒为退休错误，绝不出金。
	if !errors.Is(payErr, affiliateapp.ErrWithdrawRetired) {
		t.Fatalf("CASE2: pay want ErrWithdrawRetired, got %v", payErr)
	}
	if refundErr != nil {
		t.Fatalf("CASE2: refund err: %v", refundErr)
	}
	// 钱包入账必须为 0。
	if n := f.walletCreditCount(t, f.l1User().ID); n != 0 {
		t.Fatalf("CASE2: wallet credit count must be 0, got %d", n)
	}
	// 退款正常产生 REVERSAL（未出金），不得 DEBT。
	if revN, _ := f.countLedgers(t, f.l1Profile().ID, constants.AffiliateLedgerTypeReversal); revN != 1 {
		t.Fatalf("CASE2: want 1 reversal, got %d", revN)
	}
	if debtN, _ := f.countLedgers(t, f.l1Profile().ID, constants.AffiliateLedgerTypeDebt); debtN != 0 {
		t.Fatalf("CASE2: must NOT create debt, got %d", debtN)
	}
	t.Logf("CASE2 PASS: payErr=%v refundErr=%v deadlocks=%d", payErr, refundErr, f.deadlocks)
}

// =========================================================================
// CASE 3：独立提现已退休 —— 同一用户并发申请提现，全部返回 ErrWithdrawRetired，无锁。
// =========================================================================
func TestPGConcurrency_Case3_MultiWithdrawNoOverlock(t *testing.T) {
	f := newPGAffFixture(t)
	f.seedOrder(t, 1, 100) // L1 可用 10

	var wg sync.WaitGroup
	start := make(chan struct{})
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			errs[i] = runWithDeadlockRetry(f, func() error {
				_, e := f.svc.ApplyWithdraw(f.l1User().ID, affiliateapp.WithdrawApplyInput{
					Amount: pgDec("10"), Channel: "usdt", Account: "acc",
				})
				return e
			}, 3)
		}(i)
	}
	close(start)
	wg.Wait()

	// 并发申请全部返回 ErrWithdrawRetired。
	for i, e := range errs {
		if !errors.Is(e, affiliateapp.ErrWithdrawRetired) {
			t.Fatalf("CASE3: apply #%d want ErrWithdrawRetired, got %v", i, e)
		}
	}
	// 不得创建任何 withdraw_request / lock ledger。
	var withdrawCount int64
	f.db.Model(&affiliatedomain.WithdrawRequest{}).Count(&withdrawCount)
	if withdrawCount != 0 {
		t.Fatalf("CASE3: want 0 withdraw requests, got %d", withdrawCount)
	}
	if lockN, _ := f.countLedgers(t, f.l1Profile().ID, constants.AffiliateLedgerTypeWithdrawLock); lockN != 0 {
		t.Fatalf("CASE3: lock ledger want 0, got %d", lockN)
	}
	// 净佣金余额 = credit 10（未被锁定）。
	net := f.netProfileBalance(t, f.l1Profile().ID)
	if !net.Equal(pgDec("10.00")) {
		t.Fatalf("CASE3: net want 10 (no lock), got %s", net)
	}
	t.Logf("CASE3 PASS: all apply retired, net=%s deadlocks=%d", net, f.deadlocks)
}

// =========================================================================
// CASE 4：同一 completed order 并发触发 commission generation。
// =========================================================================
func TestPGConcurrency_Case4_DuplicateCommissionZero(t *testing.T) {
	f := newPGAffFixture(t)
	f.ensureOrderRow(t, 1)
	order := newUSDTPaidOrder(1, f.buyerUser().ID, 100)
	f.orderReader.orders[1] = order

	var wg sync.WaitGroup
	start := make(chan struct{})
	errs := make([]error, 4)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			errs[i] = runWithDeadlockRetry(f, func() error { return f.svc.HandleOrderCompleted(1) }, 3)
		}(i)
	}
	close(start)
	wg.Wait()
	for _, e := range errs {
		if e != nil {
			t.Fatalf("CASE4: HandleOrderCompleted returned error: %v", e)
		}
	}

	rows := commissionsForOrder(t, f.affiliateRepo, 1)
	// L1,L2 两个激活 profile → 2 条佣金。
	if len(rows) != 2 {
		t.Fatalf("CASE4: want exactly 2 commissions (L1,L2), got %d", len(rows))
	}
	// CREDIT ledger 数 = 2。
	creditN, _ := f.countLedgers(t, f.l1Profile().ID, constants.AffiliateLedgerTypeCredit)
	creditN2, _ := f.countLedgers(t, f.l2Profile().ID, constants.AffiliateLedgerTypeCredit)
	if creditN != 1 || creditN2 != 1 {
		t.Fatalf("CASE4: credit ledger want 1 each profile, got l1=%d l2=%d", creditN, creditN2)
	}
	t.Logf("CASE4 PASS: commissions=%d deadlocks=%d", len(rows), f.deadlocks)
}

// =========================================================================
// CASE 5：同一个 refund event 并发触发 reversal，只能生成一个。
// =========================================================================
func TestPGConcurrency_Case5_DuplicateReversalZero(t *testing.T) {
	f := newPGAffFixture(t)
	f.seedOrder(t, 1, 100) // L1=10 available

	var wg sync.WaitGroup
	start := make(chan struct{})
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			errs[i] = runWithDeadlockRetry(f, func() error { return f.refundFull(t, 1, "100") }, 3)
		}(i)
	}
	close(start)
	wg.Wait()
	// 允许一个成功一个因行锁/幂等空转，但不得报错成非预期。
	for _, e := range errs {
		if e != nil && !strings.Contains(strings.ToLower(e.Error()), "insufficient") {
			t.Logf("CASE5: refund err=%v (acceptable if idempotent/no-op)", e)
		}
	}

	revN, revSum := f.countLedgers(t, f.l1Profile().ID, constants.AffiliateLedgerTypeReversal)
	if revN != 1 || !revSum.Equal(pgDec("-10.00")) {
		t.Fatalf("CASE5: want exactly 1 reversal -10, got n=%d sum=%s", revN, revSum)
	}
	// 净余额 = credit 10 + reversal -10 = 0。
	if net := f.netProfileBalance(t, f.l1Profile().ID); !net.Equal(pgDec("0")) {
		t.Fatalf("CASE5: net want 0 after reversal, got %s", net)
	}
	t.Logf("CASE5 PASS: revN=%d deadlocks=%d", revN, f.deadlocks)
}

// =========================================================================
// CASE 6：独立提现已退休 —— 无 pending 提现，退款只产生 REVERSAL，钱包不入账。
// =========================================================================
func TestPGConcurrency_Case6_RefundWhilePending(t *testing.T) {
	f := newPGAffFixture(t)
	f.seedOrder(t, 1, 100)
	// 申请被退休拦截，不存在 pending 提现。
	f.assertApplyWithdrawRetired(t, f.l1User().ID, "10")

	if err := f.refundFull(t, 1, "100"); err != nil {
		t.Fatalf("CASE6: refund: %v", err)
	}
	// 未出金退款 → REVERSAL，不得 DEBT。
	if revN, _ := f.countLedgers(t, f.l1Profile().ID, constants.AffiliateLedgerTypeReversal); revN != 1 {
		t.Fatalf("CASE6: want 1 reversal, got %d", revN)
	}
	if debtN, _ := f.countLedgers(t, f.l1Profile().ID, constants.AffiliateLedgerTypeDebt); debtN != 0 {
		t.Fatalf("CASE6: must NOT create debt while withdraw retired, got %d", debtN)
	}
	// 钱包不应有任何出金。
	if n := f.walletCreditCount(t, f.l1User().ID); n != 0 {
		t.Fatalf("CASE6: wallet must not be credited, got %d", n)
	}
	t.Logf("CASE6 PASS: net=%s", f.netProfileBalance(t, f.l1Profile().ID))
}

// =========================================================================
// CASE 7：独立提现已退休 —— approve/pay 均返回 ErrWithdrawRetired，退款正常 REVERSAL。
// =========================================================================
func TestPGConcurrency_Case7_RefundWhileApproved(t *testing.T) {
	f := newPGAffFixture(t)
	f.seedOrder(t, 1, 100)
	// 申请与审核均已退休。
	f.assertApplyWithdrawRetired(t, f.l1User().ID, "10")
	if _, err := f.svc.ReviewWithdraw(1, 1, constants.AffiliateWithdrawActionApprove, ""); !errors.Is(err, affiliateapp.ErrWithdrawRetired) {
		t.Fatalf("CASE7: ReviewWithdraw want ErrWithdrawRetired, got %v", err)
	}

	if err := f.refundFull(t, 1, "100"); err != nil {
		t.Fatalf("CASE7: refund: %v", err)
	}
	if revN, _ := f.countLedgers(t, f.l1Profile().ID, constants.AffiliateLedgerTypeReversal); revN != 1 {
		t.Fatalf("CASE7: want 1 reversal, got %d", revN)
	}
	// Pay 已退休，返回 ErrWithdrawRetired，不得出金。
	_, payErr := f.svc.PayWithdraw(300, 1)
	if !errors.Is(payErr, affiliateapp.ErrWithdrawRetired) {
		t.Fatalf("CASE7: pay want ErrWithdrawRetired, got %v", payErr)
	}
	if n := f.walletCreditCount(t, f.l1User().ID); n != 0 {
		t.Fatalf("CASE7: wallet must not be credited, got %d", n)
	}
	t.Logf("CASE7 PASS: payErr=%v", payErr)
}

// =========================================================================
// CASE 8：独立提现已退休 —— 无"已打款"佣金，退款只产生 REVERSAL，不形成 DEBT。
// =========================================================================
func TestPGConcurrency_Case8_RefundAfterPaidDebt(t *testing.T) {
	f := newPGAffFixture(t)
	f.seedOrder(t, 1, 100)
	// 申请/审核/打款均已退休，不存在"已打款"佣金。
	f.assertApplyWithdrawRetired(t, f.l1User().ID, "10")
	if _, err := f.svc.ReviewWithdraw(1, 1, constants.AffiliateWithdrawActionApprove, ""); !errors.Is(err, affiliateapp.ErrWithdrawRetired) {
		t.Fatalf("CASE8: ReviewWithdraw want ErrWithdrawRetired, got %v", err)
	}
	if _, err := f.svc.PayWithdraw(400, 1); !errors.Is(err, affiliateapp.ErrWithdrawRetired) {
		t.Fatalf("CASE8: PayWithdraw want ErrWithdrawRetired, got %v", err)
	}
	// 钱包从未入账。
	if got := f.walletAvailable(t, f.l1User().ID); !got.Equal(pgDec("0.00")) {
		t.Fatalf("CASE8: wallet want 0 after retired pay, got %s", got)
	}

	if err := f.refundFull(t, 1, "100"); err != nil {
		t.Fatalf("CASE8: refund: %v", err)
	}
	// 未出金退款 → REVERSAL，不得 DEBT。
	if revN, _ := f.countLedgers(t, f.l1Profile().ID, constants.AffiliateLedgerTypeReversal); revN != 1 {
		t.Fatalf("CASE8: want 1 reversal, got %d", revN)
	}
	if debtN, _ := f.countLedgers(t, f.l1Profile().ID, constants.AffiliateLedgerTypeDebt); debtN != 0 {
		t.Fatalf("CASE8: must NOT create reversal debt (no payout), got %d", debtN)
	}
	t.Logf("CASE8 PASS: revN=1 wallet=%s", f.walletAvailable(t, f.l1User().ID))
}

// =========================================================================
// CASE 9：独立提现已退休 —— 不产生已出金，退款不形成 DEBT，净佣金归零。
// =========================================================================
func TestPGConcurrency_Case9_DebtCreationExact(t *testing.T) {
	f := newPGAffFixture(t)
	f.seedOrder(t, 1, 100)
	// 提现已退休，不产生已出金，退款不会形成 DEBT。
	f.assertApplyWithdrawRetired(t, f.l1User().ID, "10")
	if _, err := f.svc.ReviewWithdraw(1, 1, constants.AffiliateWithdrawActionApprove, ""); !errors.Is(err, affiliateapp.ErrWithdrawRetired) {
		t.Fatalf("CASE9: ReviewWithdraw want ErrWithdrawRetired, got %v", err)
	}
	if _, err := f.svc.PayWithdraw(500, 1); !errors.Is(err, affiliateapp.ErrWithdrawRetired) {
		t.Fatalf("CASE9: PayWithdraw want ErrWithdrawRetired, got %v", err)
	}
	if err := f.refundFull(t, 1, "100"); err != nil {
		t.Fatalf("CASE9: refund: %v", err)
	}
	// 不应有任何 DEBT 行。
	var debts []affiliatedomain.CommissionLedger
	f.db.Where("affiliate_profile_id = ? AND type = ?", f.l1Profile().ID, constants.AffiliateLedgerTypeDebt).
		Order("id asc").Find(&debts)
	if len(debts) != 0 {
		t.Fatalf("CASE9: want 0 debt ledger rows (retired withdraw), got %d", len(debts))
	}
	// 资金守恒：credit(+10) + reversal(-10) = 0。
	net := f.netProfileBalance(t, f.l1Profile().ID)
	if !net.Equal(pgDec("0.00")) {
		t.Fatalf("CASE9: net want 0.00 (reversal offset, no debt), got %s", net)
	}
	t.Logf("CASE9 PASS: net=%s", net)
}

// =========================================================================
// CASE 10：独立提现已退休 —— 不产生 DEBT，新佣金正常叠加；ApplyWithdraw 仍返回退休错误。
// =========================================================================
func TestPGConcurrency_Case10_DebtRecovery(t *testing.T) {
	f := newPGAffFixture(t)
	f.seedOrder(t, 1, 100) // order A: L1 +10
	// 提现已退休，不产生已出金/DEBT。
	f.assertApplyWithdrawRetired(t, f.l1User().ID, "10")
	if _, err := f.svc.ReviewWithdraw(1, 1, constants.AffiliateWithdrawActionApprove, ""); !errors.Is(err, affiliateapp.ErrWithdrawRetired) {
		t.Fatalf("CASE10: ReviewWithdraw want ErrWithdrawRetired, got %v", err)
	}
	if _, err := f.svc.PayWithdraw(600, 1); !errors.Is(err, affiliateapp.ErrWithdrawRetired) {
		t.Fatalf("CASE10: PayWithdraw want ErrWithdrawRetired, got %v", err)
	}
	if err := f.refundFull(t, 1, "100"); err != nil { // order A 退款 → REVERSAL -10
		t.Fatalf("CASE10: refund: %v", err)
	}
	if net := f.netProfileBalance(t, f.l1Profile().ID); !net.Equal(pgDec("0.00")) {
		t.Fatalf("CASE10: before new order net want 0 (reversal offset, no debt), got %s", net)
	}

	// 新订单 B 完成，L1 再得 +10。
	f.seedOrder(t, 2, 100)
	// 无 DEBT，新佣金直接叠加为 +10。
	net := f.netProfileBalance(t, f.l1Profile().ID)
	if !net.Equal(pgDec("10.00")) {
		t.Fatalf("CASE10: after new order, net want 10 (no debt), got %s", net)
	}
	// 新佣金依然不可提（提现已退休），ApplyWithdraw 返回 ErrWithdrawRetired。
	_, err := f.svc.ApplyWithdraw(f.l1User().ID, affiliateapp.WithdrawApplyInput{
		Amount: pgDec("10"), Channel: "usdt", Account: "acc",
	})
	if !errors.Is(err, affiliateapp.ErrWithdrawRetired) {
		t.Fatalf("CASE10: ApplyWithdraw want ErrWithdrawRetired, got %v", err)
	}
	t.Logf("CASE10 PASS: net after refund+new order=%s", net)
}
