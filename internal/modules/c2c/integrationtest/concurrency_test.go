//go:build integration
// +build integration

package integrationtest

import (
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	c2capp "github.com/Aether-v1/hcz/internal/modules/c2c/application"
	c2ccontract "github.com/Aether-v1/hcz/internal/modules/c2c/contract"
	c2cdomain "github.com/Aether-v1/hcz/internal/modules/c2c/domain"
	c2cgormstore "github.com/Aether-v1/hcz/internal/modules/c2c/infrastructure/gormstore"
	userdomain "github.com/Aether-v1/hcz/internal/modules/identity/user/domain"
	settingsintegration "github.com/Aether-v1/hcz/internal/modules/settings/schema/integration"
	walletapp "github.com/Aether-v1/hcz/internal/modules/wallet/application"
	walletdomain "github.com/Aether-v1/hcz/internal/modules/wallet/domain"
	walletgormstore "github.com/Aether-v1/hcz/internal/modules/wallet/infrastructure/gormstore"
	"github.com/Aether-v1/hcz/internal/shared/money"

	"github.com/shopspring/decimal"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	glogger "gorm.io/gorm/logger"
)

// pgFixture 是 PostgreSQL 版测试夹具（仅 integration tag 下编译）。
type pgFixture struct {
	db        *gorm.DB
	walletDB  *walletgormstore.Store
	c2cDB     *c2cgormstore.Store
	svc       *c2capp.Service
	users     *mockUserReader
	notifier  *mockNotifier
	scheduler *mockScheduler
	audit     *mockAudit
}

func newPGFixture(t *testing.T) *pgFixture {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("TEST_POSTGRES_DSN"))
	if dsn == "" {
		t.Skip("skip postgres integration test: TEST_POSTGRES_DSN is empty")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: glogger.Default.LogMode(glogger.Silent),
	})
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	// 清理并重建
	cleanup := []interface{}{
		&c2cdomain.RiskSignal{},
		&c2cdomain.Dispute{},
		&c2cdomain.Trade{},
		&c2cdomain.Listing{},
		&c2cdomain.PaymentMethod{},
		&walletdomain.Transaction{},
		&walletdomain.Account{},
		&userdomain.User{},
	}
	_ = db.Migrator().DropTable(cleanup...)
	if err := db.AutoMigrate(cleanup...); err != nil {
		t.Fatalf("migrate postgres: %v", err)
	}

	walletDB := walletgormstore.New(db)
	c2cDB := c2cgormstore.New(db, walletDB)
	walletSvc := walletapp.NewService(walletapp.Options{Repository: walletDB, Transactions: walletDB})

	cfg := settingsintegration.C2CSetting{
		Enabled:              true,
		TradeTimeoutMinutes:  30,
		NewUserCooldownHours: 0,
		MinTradeUSDT:         0,
		MaxTradeUSDT:         0,
		DailyTradeLimitUSDT:  0,
		MaxCancelCount:       5,
		FeeRate:              0,
	}
	users := &mockUserReader{users: map[uint]*userdomain.User{}}
	notifier := &mockNotifier{}
	scheduler := &mockScheduler{}
	audit := &mockAudit{}
	svc := c2capp.NewService(c2capp.Options{
		Repository:    c2cDB,
		UnitOfWork:    c2cDB,
		WalletService: walletSvc,
		Users:         users,
		UserAdmin:     users,
		Config:        &mockC2CConfig{cfg: cfg},
		Scheduler:     scheduler,
		Notifier:      notifier,
		Audit:         audit,
	})

	t.Cleanup(func() {
		_ = db.Migrator().DropTable(cleanup...)
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})

	return &pgFixture{
		db: db, walletDB: walletDB, c2cDB: c2cDB, svc: svc,
		users: users, notifier: notifier, scheduler: scheduler, audit: audit,
	}
}

// pgCreateUser 登记一个合格用户。
func (f *pgFixture) pgCreateUser(t *testing.T, id uint) {
	t.Helper()
	now := time.Now()
	f.users.users[id] = &userdomain.User{
		ID: id, Status: "active", TOTPEnabledAt: &now, CreatedAt: now.Add(-48 * time.Hour),
	}
}

// pgSetBalance 写入钱包账户。
func (f *pgFixture) pgSetBalance(t *testing.T, userID uint, amount string) {
	t.Helper()
	acc := &walletdomain.Account{
		UserID:           userID,
		AvailableBalance: money.FromDecimal(mustDec(amount)),
		FrozenBalance:    money.FromDecimal(decimal.Zero),
	}
	if err := f.walletDB.CreateAccount(acc); err != nil {
		t.Fatalf("pg create account %d: %v", userID, err)
	}
}

// pgCreatePM 为用户创建启用支付方式。
func (f *pgFixture) pgCreatePM(t *testing.T, userID uint) {
	t.Helper()
	pm := &c2cdomain.PaymentMethod{
		UserID: userID, Type: "bank_card",
		AccountName: "T", AccountIdentifier: "acc", Enabled: true,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := f.c2cDB.CreatePaymentMethod(pm); err != nil {
		t.Fatalf("pg create pm %d: %v", userID, err)
	}
}

// pgCreateListing 直接用 service 创建挂单。
func (f *pgFixture) pgCreateListing(t *testing.T, sellerID uint, total string) *c2cdomain.Listing {
	t.Helper()
	l, err := f.svc.CreateListing(c2ccontract.CreateListingInput{
		UserID: sellerID, FiatCurrency: "CNY",
		Price:         money.FromDecimal(mustDec("7.00")),
		MinFiatAmount: money.FromDecimal(mustDec("1.00")),
		MaxFiatAmount: money.FromDecimal(mustDec("100000.00")),
		TotalUSDT:     money.FromDecimal(mustDec(total)),
	})
	if err != nil {
		t.Fatalf("pg create listing seller=%d: %v", sellerID, err)
	}
	return l
}

func (f *pgFixture) pgAvail(t *testing.T, userID uint) decimal.Decimal {
	t.Helper()
	acc, err := f.walletDB.GetAccountByUserID(userID)
	if err != nil {
		t.Fatalf("pg get account %d: %v", userID, err)
	}
	if acc == nil {
		return decimal.Zero
	}
	return acc.AvailableBalance.Decimal.Round(2)
}

func (f *pgFixture) pgFrozen(t *testing.T, userID uint) decimal.Decimal {
	t.Helper()
	acc, err := f.walletDB.GetAccountByUserID(userID)
	if err != nil {
		t.Fatalf("pg get account %d: %v", userID, err)
	}
	if acc == nil {
		return decimal.Zero
	}
	return acc.FrozenBalance.Decimal.Round(2)
}

// TestConcurrent_ListingNoOversell 两人并发各买 80，listing total=100，不超卖。
func TestConcurrent_ListingNoOversell(t *testing.T) {
	f := newPGFixture(t)
	t.Parallel()

	const sellerID, buyerA, buyerB = uint(1), uint(2), uint(3)
	f.pgCreateUser(t, sellerID)
	f.pgCreateUser(t, buyerA)
	f.pgCreateUser(t, buyerB)
	f.pgSetBalance(t, sellerID, "1000.00")
	f.pgCreatePM(t, sellerID)
	listing := f.pgCreateListing(t, sellerID, "100.00")

	var wg sync.WaitGroup
	results := make([]error, 2)
	ids := make([]uint, 2)
	keys := []string{"concurrent-buy-a", "concurrent-buy-b"}
	buyers := []uint{buyerA, buyerB}
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			tr, err := f.svc.CreateTrade(c2ccontract.CreateTradeInput{
				BuyerUserID:    buyers[i],
				ListingID:      listing.ID,
				USDTAmount:     money.FromDecimal(mustDec("80.00")),
				IdempotencyKey: keys[i],
			})
			results[i] = err
			if tr != nil {
				ids[i] = tr.ID
			}
		}(i)
	}
	wg.Wait()

	successCount := 0
	for _, err := range results {
		if err == nil {
			successCount++
		}
	}
	// 80+80=160 > 100，应该恰好 1 个成功，1 个失败
	if successCount != 1 {
		t.Fatalf("want exactly 1 success (80<=100, second fails), got %d (errors: %v)", successCount, results)
	}

	// 重新读 listing 余量，>=0 且 <=100
	reloaded, err := f.c2cDB.GetListingByID(listing.ID)
	if err != nil || reloaded == nil {
		t.Fatalf("reload listing: %v", err)
	}
	avail := reloaded.AvailableUSDT.Decimal.Round(2)
	if avail.LessThan(decimal.Zero) {
		t.Fatalf("listing available must not be negative, got %s", avail)
	}
	if avail.GreaterThan(mustDec("100.00")) {
		t.Fatalf("listing available want <=100, got %s", avail)
	}
	// 成交量 = 100 - avail = 80（仅成功那笔）
	volume := mustDec("100.00").Sub(avail)
	if !volume.Equal(mustDec("80.00")) {
		t.Fatalf("traded volume want 80.00, got %s", volume)
	}
}

// TestConcurrent_SellerMultipleTradesFreeze 同一 seller 两个 listing 并发创建 trade，freeze 正确累加。
func TestConcurrent_SellerMultipleTradesFreeze(t *testing.T) {
	f := newPGFixture(t)
	t.Parallel()

	const sellerID, buyerA, buyerB = uint(1), uint(2), uint(3)
	f.pgCreateUser(t, sellerID)
	f.pgCreateUser(t, buyerA)
	f.pgCreateUser(t, buyerB)
	f.pgSetBalance(t, sellerID, "1000.00")
	f.pgCreatePM(t, sellerID)
	l1 := f.pgCreateListing(t, sellerID, "500.00")
	l2 := f.pgCreateListing(t, sellerID, "500.00")

	var wg sync.WaitGroup
	errs := make([]error, 2)
	type args struct {
		buyerID uint
		listing uint
		amount  string
		idemKey string
	}
	cases := []args{
		{buyerA, l1.ID, "500.00", "freeze-a"},
		{buyerB, l2.ID, "500.00", "freeze-b"},
	}
	for i := range cases {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := f.svc.CreateTrade(c2ccontract.CreateTradeInput{
				BuyerUserID:    cases[i].buyerID,
				ListingID:      cases[i].listing,
				USDTAmount:     money.FromDecimal(mustDec(cases[i].amount)),
				IdempotencyKey: cases[i].idemKey,
			})
			errs[i] = err
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("trade %d failed: %v", i, err)
		}
	}

	// seller: available = 1000 - 500 - 500 = 0, frozen = 1000
	avail := f.pgAvail(t, sellerID)
	frozen := f.pgFrozen(t, sellerID)
	if !avail.Equal(mustDec("0.00")) {
		t.Fatalf("seller available want 0.00, got %s", avail)
	}
	if !frozen.Equal(mustDec("1000.00")) {
		t.Fatalf("seller frozen want 1000.00, got %s", frozen)
	}
}

// TestConcurrent_CancelVsConfirm 同一 trade，一个 cancel 一个 confirm，只有一个成功，状态一致。
func TestConcurrent_CancelVsConfirm(t *testing.T) {
	f := newPGFixture(t)
	t.Parallel()

	const sellerID, buyerID = uint(1), uint(2)
	f.pgCreateUser(t, sellerID)
	f.pgCreateUser(t, buyerID)
	f.pgSetBalance(t, sellerID, "1000.00")
	f.pgCreatePM(t, sellerID)
	listing := f.pgCreateListing(t, sellerID, "1000.00")
	f.pgSetBalance(t, buyerID, "0.00")

	tr := mustCreateTradePG(t, f, buyerID, listing.ID, "100.00", "cvsc-base")
	// 先 mark-paid（让 confirm 可走）
	if _, err := f.svc.MarkPaid(c2ccontract.MarkPaidInput{TradeID: tr.ID, BuyerUserID: buyerID, PaymentReference: "x"}); err != nil {
		t.Fatalf("mark paid: %v", err)
	}

	type outcome struct {
		tradeID uint
		err     error
	}
	var wg sync.WaitGroup
	ch := make(chan outcome, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, err := f.svc.Cancel(c2ccontract.CancelTradeInput{TradeID: tr.ID, BuyerUserID: buyerID})
		ch <- outcome{tradeID: tr.ID, err: err}
	}()
	go func() {
		defer wg.Done()
		_, err := f.svc.Confirm(c2ccontract.ConfirmTradeInput{TradeID: tr.ID, SellerID: sellerID})
		ch <- outcome{tradeID: tr.ID, err: err}
	}()
	wg.Wait()
	close(ch)

	successCount := 0
	for o := range ch {
		if o.err == nil {
			successCount++
		}
	}
	if successCount != 1 {
		t.Fatalf("want exactly 1 winner between cancel/confirm, got %d", successCount)
	}

	// 最终状态必须是终态：canceled 或 completed，不能是 paid
	reloaded, err := f.c2cDB.GetTradeByID(tr.ID)
	if err != nil || reloaded == nil {
		t.Fatalf("reload trade: %v", err)
	}
	if reloaded.Status != "canceled" && reloaded.Status != "completed" {
		t.Fatalf("trade must be terminal (canceled/completed), got %s", reloaded.Status)
	}

	// 资金一致性：若 canceled → seller available=1000, frozen=0；若 completed → seller available=900, frozen=0, buyer available=100
	sellerAvail := f.pgAvail(t, sellerID)
	sellerFrozen := f.pgFrozen(t, sellerID)
	if !sellerFrozen.Equal(mustDec("0.00")) {
		t.Fatalf("seller frozen must be 0 after cancel/confirm race, got %s", sellerFrozen)
	}
	if reloaded.Status == "canceled" {
		if !sellerAvail.Equal(mustDec("1000.00")) {
			t.Fatalf("canceled: seller avail want 1000, got %s", sellerAvail)
		}
	} else {
		if !sellerAvail.Equal(mustDec("900.00")) {
			t.Fatalf("completed: seller avail want 900, got %s", sellerAvail)
		}
		if got := f.pgAvail(t, buyerID); !got.Equal(mustDec("100.00")) {
			t.Fatalf("completed: buyer avail want 100, got %s", got)
		}
	}
}

// TestConcurrent_ExpireVsMarkPaid 同一 trade，一个 mark-paid 一个 expire，只有一个成功。
func TestConcurrent_ExpireVsMarkPaid(t *testing.T) {
	f := newPGFixture(t)
	t.Parallel()

	const sellerID, buyerID = uint(1), uint(2)
	f.pgCreateUser(t, sellerID)
	f.pgCreateUser(t, buyerID)
	f.pgSetBalance(t, sellerID, "1000.00")
	f.pgCreatePM(t, sellerID)
	listing := f.pgCreateListing(t, sellerID, "1000.00")

	tr := mustCreateTradePG(t, f, buyerID, listing.ID, "100.00", "evmp-base")

	var wg sync.WaitGroup
	ch := make(chan error, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, err := f.svc.MarkPaid(c2ccontract.MarkPaidInput{TradeID: tr.ID, BuyerUserID: buyerID, PaymentReference: "x"})
		ch <- err
	}()
	go func() {
		defer wg.Done()
		err := f.svc.ExpireTrade(tr.ID)
		ch <- err
	}()
	wg.Wait()
	close(ch)

	for err := range ch {
		// 任一失败都允许（状态机拒绝），但不能 panic / 双写
		_ = err
	}

	reloaded, err := f.c2cDB.GetTradeByID(tr.ID)
	if err != nil || reloaded == nil {
		t.Fatalf("reload trade: %v", err)
	}
	// 最终状态必须是 paid 或 expired（不能停留在 pending_payment）
	if reloaded.Status != "paid" && reloaded.Status != "expired" {
		t.Fatalf("trade must be paid or expired after race, got %s", reloaded.Status)
	}

	// 资金一致性：
	// 若 paid → seller frozen=100, available=900
	// 若 expired → seller frozen=0, available=1000
	sellerAvail := f.pgAvail(t, sellerID)
	sellerFrozen := f.pgFrozen(t, sellerID)
	if reloaded.Status == "paid" {
		if !sellerAvail.Equal(mustDec("900.00")) || !sellerFrozen.Equal(mustDec("100.00")) {
			t.Fatalf("paid: seller avail=900 frozen=100, got avail=%s frozen=%s", sellerAvail, sellerFrozen)
		}
	} else {
		if !sellerAvail.Equal(mustDec("1000.00")) || !sellerFrozen.Equal(mustDec("0.00")) {
			t.Fatalf("expired: seller avail=1000 frozen=0, got avail=%s frozen=%s", sellerAvail, sellerFrozen)
		}
	}
}

// TestConcurrent_DisputeVsConfirm 同一 paid 交易，一个 dispute 一个 confirm，只有一个成功。
func TestConcurrent_DisputeVsConfirm(t *testing.T) {
	f := newPGFixture(t)
	t.Parallel()

	const sellerID, buyerID = uint(1), uint(2)
	f.pgCreateUser(t, sellerID)
	f.pgCreateUser(t, buyerID)
	f.pgSetBalance(t, sellerID, "1000.00")
	f.pgCreatePM(t, sellerID)
	listing := f.pgCreateListing(t, sellerID, "1000.00")
	f.pgSetBalance(t, buyerID, "0.00")

	tr := mustCreateTradePG(t, f, buyerID, listing.ID, "100.00", "dvsc-base")
	if _, err := f.svc.MarkPaid(c2ccontract.MarkPaidInput{TradeID: tr.ID, BuyerUserID: buyerID, PaymentReference: "x"}); err != nil {
		t.Fatalf("mark paid: %v", err)
	}

	var wg sync.WaitGroup
	ch := make(chan error, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, err := f.svc.InitiateDispute(c2ccontract.InitiateDisputeInput{UserID: buyerID, TradeID: tr.ID, Reason: "r"})
		ch <- err
	}()
	go func() {
		defer wg.Done()
		_, err := f.svc.Confirm(c2ccontract.ConfirmTradeInput{TradeID: tr.ID, SellerID: sellerID})
		ch <- err
	}()
	wg.Wait()
	close(ch)

	for err := range ch {
		_ = err
	}

	reloaded, err := f.c2cDB.GetTradeByID(tr.ID)
	if err != nil || reloaded == nil {
		t.Fatalf("reload trade: %v", err)
	}
	// 状态必须是 completed 或 disputed，不能停留在 paid
	if reloaded.Status != "completed" && reloaded.Status != "disputed" {
		t.Fatalf("trade must be completed or disputed, got %s", reloaded.Status)
	}

	// 资金一致性：
	// completed → buyer available=100, seller frozen=0
	// disputed → seller frozen=100, buyer available=0
	sellerFrozen := f.pgFrozen(t, sellerID)
	buyerAvail := f.pgAvail(t, buyerID)
	if reloaded.Status == "completed" {
		if !sellerFrozen.Equal(mustDec("0.00")) || !buyerAvail.Equal(mustDec("100.00")) {
			t.Fatalf("completed: seller frozen=0 buyer avail=100, got f=%s b=%s", sellerFrozen, buyerAvail)
		}
	} else {
		if !sellerFrozen.Equal(mustDec("100.00")) || !buyerAvail.Equal(mustDec("0.00")) {
			t.Fatalf("disputed: seller frozen=100 buyer avail=0, got f=%s b=%s", sellerFrozen, buyerAvail)
		}
	}
}

// TestConcurrent_ArbitrationRetry 同一 dispute 并发两个 arbitration（同 result），资金动作只执行一次。
func TestConcurrent_ArbitrationRetry(t *testing.T) {
	f := newPGFixture(t)
	t.Parallel()

	const sellerID, buyerID = uint(1), uint(2)
	f.pgCreateUser(t, sellerID)
	f.pgCreateUser(t, buyerID)
	f.pgSetBalance(t, sellerID, "1000.00")
	f.pgCreatePM(t, sellerID)
	listing := f.pgCreateListing(t, sellerID, "1000.00")
	f.pgSetBalance(t, buyerID, "0.00")

	tr := mustCreateTradePG(t, f, buyerID, listing.ID, "100.00", "retry-base")
	if _, err := f.svc.MarkPaid(c2ccontract.MarkPaidInput{TradeID: tr.ID, BuyerUserID: buyerID, PaymentReference: "x"}); err != nil {
		t.Fatalf("mark paid: %v", err)
	}
	if _, err := f.svc.InitiateDispute(c2ccontract.InitiateDisputeInput{UserID: buyerID, TradeID: tr.ID, Reason: "r"}); err != nil {
		t.Fatalf("dispute: %v", err)
	}

	var wg sync.WaitGroup
	ch := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(adminID uint) {
			defer wg.Done()
			_, _, err := f.svc.Arbitrate(c2ccontract.ArbitrateInput{
				AdminID: adminID, TradeID: tr.ID,
				Result: c2ccontract.ArbitrationResultReleaseToBuyer, Reason: "r",
			})
			ch <- err
		}(uint(100 + i))
	}
	wg.Wait()
	close(ch)

	for err := range ch {
		if err != nil {
			t.Fatalf("arbitration call should be idempotent, got %v", err)
		}
	}

	reloaded, err := f.c2cDB.GetTradeByID(tr.ID)
	if err != nil || reloaded == nil {
		t.Fatalf("reload trade: %v", err)
	}
	if reloaded.Status != "completed" {
		t.Fatalf("trade should be completed, got %s", reloaded.Status)
	}

	// buyer available 必须恰好 100（只 receive 一次）
	if got := f.pgAvail(t, buyerID); !got.Equal(mustDec("100.00")) {
		t.Fatalf("buyer avail want 100.00 (single receive), got %s", got)
	}
	// seller frozen = 0
	if got := f.pgFrozen(t, sellerID); !got.Equal(mustDec("0.00")) {
		t.Fatalf("seller frozen want 0, got %s", got)
	}
}

// mustCreateTradePG 在 PG 上创建交易，失败直接 fatal。
func mustCreateTradePG(t *testing.T, f *pgFixture, buyerID, listingID uint, amount, idemKey string) *c2cdomain.Trade {
	t.Helper()
	tr, err := f.svc.CreateTrade(c2ccontract.CreateTradeInput{
		BuyerUserID:    buyerID,
		ListingID:      listingID,
		USDTAmount:     money.FromDecimal(mustDec(amount)),
		IdempotencyKey: idemKey,
	})
	if err != nil {
		t.Fatalf("pg create trade buyer=%d: %v", buyerID, err)
	}
	return tr
}
