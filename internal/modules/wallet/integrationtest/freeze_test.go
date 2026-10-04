package integrationtest

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	walletapp "github.com/Aether-v1/hcz/internal/modules/wallet/application"
	walletcontract "github.com/Aether-v1/hcz/internal/modules/wallet/contract"
	walletdomain "github.com/Aether-v1/hcz/internal/modules/wallet/domain"
	walletgormstore "github.com/Aether-v1/hcz/internal/modules/wallet/infrastructure/gormstore"
	"github.com/Aether-v1/hcz/internal/shared/money"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// freezeTestEnv 封装一套独立的内存 SQLite 钱包测试环境。
type freezeTestEnv struct {
	repo    *walletgormstore.Store
	svc     *walletapp.Service
	db      *gorm.DB
	unit    walletcontract.UnitOfWork
	wallets walletcontract.Repository
}

func newFreezeTestEnv(t *testing.T) *freezeTestEnv {
	t.Helper()
	repo, db := setupWalletRepositoryTest(t)
	svc := walletapp.NewService(walletapp.Options{Repository: repo, Transactions: repo})
	return &freezeTestEnv{repo: repo, svc: svc, db: db, unit: repo, wallets: repo}
}

// newConcurrentFreezeTestEnv 用单连接串行化 SQLite 写入，避免 SQLITE_BUSY，
// 同时验证在事务交错下不出现死锁/超时、金额不超扣。
func newConcurrentFreezeTestEnv(t *testing.T) *freezeTestEnv {
	t.Helper()
	env := newFreezeTestEnv(t)
	sqlDB, err := env.db.DB()
	if err != nil {
		t.Fatalf("raw db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	return env
}

// mustSeedAvailable 通过管理员调账给账户灌入可用余额（走真实事务路径）。
func (e *freezeTestEnv) mustSeedAvailable(t *testing.T, userID uint, amount int64) {
	t.Helper()
	_, _, err := e.svc.AdminAdjustBalance(walletcontract.AdjustBalanceInput{
		UserID:          userID,
		OperatorAdminID: 1,
		Delta:           money.FromDecimal(decimal.NewFromInt(amount)),
		Currency:        "USDT",
		Remark:          "seed available",
	})
	if err != nil {
		t.Fatalf("seed available user %d: %v", userID, err)
	}
}

// freeze 打开一个事务执行 Freeze。
func (e *freezeTestEnv) freeze(t *testing.T, in walletcontract.FreezeInput) (*walletdomain.Account, *walletdomain.Transaction, error) {
	t.Helper()
	var acc *walletdomain.Account
	var txn *walletdomain.Transaction
	err := e.unit.WithinTransaction(func(tx walletcontract.Transaction) error {
		var callErr error
		acc, txn, callErr = e.svc.Freeze(tx, in)
		return callErr
	})
	return acc, txn, err
}

// unfreeze 打开一个事务执行 Unfreeze。
func (e *freezeTestEnv) unfreeze(t *testing.T, in walletcontract.UnfreezeInput) (*walletdomain.Account, *walletdomain.Transaction, error) {
	t.Helper()
	var acc *walletdomain.Account
	var txn *walletdomain.Transaction
	err := e.unit.WithinTransaction(func(tx walletcontract.Transaction) error {
		var callErr error
		acc, txn, callErr = e.svc.Unfreeze(tx, in)
		return callErr
	})
	return acc, txn, err
}

// settle 打开一个事务执行 SettleFrozen。
func (e *freezeTestEnv) settle(t *testing.T, in walletcontract.SettleInput) error {
	t.Helper()
	return e.unit.WithinTransaction(func(tx walletcontract.Transaction) error {
		return e.svc.SettleFrozen(tx, in)
	})
}

// balances 读取账户最新可用/冻结余额。
func (e *freezeTestEnv) balances(t *testing.T, userID uint) (available, frozen decimal.Decimal) {
	t.Helper()
	acc, err := e.wallets.GetAccountByUserID(userID)
	if err != nil {
		t.Fatalf("get account %d: %v", userID, err)
	}
	if acc == nil {
		t.Fatalf("account %d not found", userID)
	}
	return acc.AvailableBalance.Decimal, acc.FrozenBalance.Decimal
}

// mustFreeze helper：冻结并在失败时终止测试。
func (e *freezeTestEnv) mustFreeze(t *testing.T, userID uint, amount int64, ref string) {
	t.Helper()
	if _, _, err := e.freeze(t, walletcontract.FreezeInput{
		UserID: userID, Amount: money.FromDecimal(decimal.NewFromInt(amount)), Reference: ref,
	}); err != nil {
		t.Fatalf("mustFreeze user %d amount %d: %v", userID, amount, err)
	}
}

func waitOrFail(t *testing.T, wg *sync.WaitGroup, timeout time.Duration, what string) {
	t.Helper()
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(timeout):
		t.Fatalf("%s: timed out after %v (possible deadlock)", what, timeout)
	}
}

func dec(v int64) decimal.Decimal { return decimal.NewFromInt(v) }

// ---------- 基础功能 ----------

func TestFreeze_Success(t *testing.T) {
	env := newFreezeTestEnv(t)
	const userID = uint(1001)
	env.mustSeedAvailable(t, userID, 1000)

	acc, txn, err := env.freeze(t, walletcontract.FreezeInput{
		UserID: userID, Amount: money.FromDecimal(dec(300)), Reference: "c2c_freeze:biz-1", Remark: "lock trade",
	})
	if err != nil {
		t.Fatalf("Freeze failed: %v", err)
	}
	if got := acc.AvailableBalance.Decimal; !got.Equal(dec(700)) {
		t.Fatalf("available = %s, want 700.00", got)
	}
	if got := acc.FrozenBalance.Decimal; !got.Equal(dec(300)) {
		t.Fatalf("frozen = %s, want 300.00", got)
	}
	// total 不变
	if !acc.AvailableBalance.Decimal.Add(acc.FrozenBalance.Decimal).Equal(dec(1000)) {
		t.Fatalf("total changed: available+frozen=%s", acc.AvailableBalance.Decimal.Add(acc.FrozenBalance.Decimal))
	}
	// ledger 字段
	if txn.Type != "c2c_freeze" {
		t.Fatalf("type = %s, want c2c_freeze", txn.Type)
	}
	if txn.Direction != "out" {
		t.Fatalf("direction = %s, want out", txn.Direction)
	}
	if !txn.Amount.Decimal.Equal(dec(300)) {
		t.Fatalf("amount = %s, want 300", txn.Amount.Decimal)
	}
	if !txn.AvailableBefore.Decimal.Equal(dec(1000)) || !txn.AvailableAfter.Decimal.Equal(dec(700)) {
		t.Fatalf("available before/after = %s/%s, want 1000/700", txn.AvailableBefore.Decimal, txn.AvailableAfter.Decimal)
	}
	if !txn.FrozenBefore.Decimal.Equal(dec(0)) || !txn.FrozenAfter.Decimal.Equal(dec(300)) {
		t.Fatalf("frozen before/after = %s/%s, want 0/300", txn.FrozenBefore.Decimal, txn.FrozenAfter.Decimal)
	}
	if !txn.BalanceBefore.Decimal.Equal(dec(1000)) || !txn.BalanceAfter.Decimal.Equal(dec(1000)) {
		t.Fatalf("total balance before/after = %s/%s, want 1000/1000", txn.BalanceBefore.Decimal, txn.BalanceAfter.Decimal)
	}
	if txn.Currency != "USDT" {
		t.Fatalf("currency = %s, want USDT", txn.Currency)
	}
}

func TestFreeze_InsufficientAvailable(t *testing.T) {
	env := newFreezeTestEnv(t)
	const userID = uint(1002)
	env.mustSeedAvailable(t, userID, 100)

	_, _, err := env.freeze(t, walletcontract.FreezeInput{
		UserID: userID, Amount: money.FromDecimal(dec(300)), Reference: "c2c_freeze:insuff",
	})
	if !errors.Is(err, walletcontract.ErrInsufficientBalance) {
		t.Fatalf("err = %v, want ErrInsufficientBalance", err)
	}
	avail, frozen := env.balances(t, userID)
	if !avail.Equal(dec(100)) || !frozen.Equal(dec(0)) {
		t.Fatalf("balances mutated on failure: available=%s frozen=%s", avail, frozen)
	}
}

func TestFreeze_AmountZero(t *testing.T) {
	env := newFreezeTestEnv(t)
	const userID = uint(1003)
	env.mustSeedAvailable(t, userID, 100)

	for _, amt := range []int64{0, -5} {
		_, _, err := env.freeze(t, walletcontract.FreezeInput{
			UserID: userID, Amount: money.FromDecimal(dec(amt)), Reference: fmt.Sprintf("c2c_freeze:zero-%d", amt),
		})
		if !errors.Is(err, walletcontract.ErrInvalidAmount) {
			t.Fatalf("amount=%d err = %v, want ErrInvalidAmount", amt, err)
		}
	}
}

func TestFreeze_DuplicateIdempotent(t *testing.T) {
	env := newFreezeTestEnv(t)
	const userID = uint(1004)
	env.mustSeedAvailable(t, userID, 1000)

	firstAcc, firstTxn, err := env.freeze(t, walletcontract.FreezeInput{
		UserID: userID, Amount: money.FromDecimal(dec(200)), Reference: "c2c_freeze:dup-1",
	})
	if err != nil {
		t.Fatalf("first freeze: %v", err)
	}
	secondAcc, secondTxn, err := env.freeze(t, walletcontract.FreezeInput{
		UserID: userID, Amount: money.FromDecimal(dec(200)), Reference: "c2c_freeze:dup-1",
	})
	if err != nil {
		t.Fatalf("duplicate freeze should be idempotent, got: %v", err)
	}
	if firstTxn.ID != secondTxn.ID {
		t.Fatalf("duplicate returned different transaction id: %d vs %d", firstTxn.ID, secondTxn.ID)
	}
	if !firstAcc.AvailableBalance.Decimal.Equal(secondAcc.AvailableBalance.Decimal) {
		t.Fatalf("idempotent retry changed balances")
	}
	avail, frozen := env.balances(t, userID)
	if !avail.Equal(dec(800)) || !frozen.Equal(dec(200)) {
		t.Fatalf("after idempotent retry available=%s frozen=%s, want 800/200", avail, frozen)
	}
}

func TestUnfreeze_Success(t *testing.T) {
	env := newFreezeTestEnv(t)
	const userID = uint(1005)
	env.mustSeedAvailable(t, userID, 1000)
	env.mustFreeze(t, userID, 400, "c2c_freeze:biz-u1")

	acc, txn, err := env.unfreeze(t, walletcontract.UnfreezeInput{
		UserID: userID, Amount: money.FromDecimal(dec(150)), Reference: "c2c_unfreeze:biz-u1",
	})
	if err != nil {
		t.Fatalf("Unfreeze failed: %v", err)
	}
	if !acc.AvailableBalance.Decimal.Equal(dec(750)) {
		t.Fatalf("available = %s, want 750", acc.AvailableBalance.Decimal)
	}
	if !acc.FrozenBalance.Decimal.Equal(dec(250)) {
		t.Fatalf("frozen = %s, want 250", acc.FrozenBalance.Decimal)
	}
	if txn.Type != "c2c_unfreeze" || txn.Direction != "in" {
		t.Fatalf("type/direction = %s/%s, want c2c_unfreeze/in", txn.Type, txn.Direction)
	}
	if !txn.BalanceBefore.Decimal.Equal(dec(1000)) || !txn.BalanceAfter.Decimal.Equal(dec(1000)) {
		t.Fatalf("total should be invariant across unfreeze: %s/%s", txn.BalanceBefore.Decimal, txn.BalanceAfter.Decimal)
	}
}

func TestUnfreeze_InsufficientFrozen(t *testing.T) {
	env := newFreezeTestEnv(t)
	const userID = uint(1006)
	env.mustSeedAvailable(t, userID, 1000)
	env.mustFreeze(t, userID, 100, "c2c_freeze:biz-if1")

	_, _, err := env.unfreeze(t, walletcontract.UnfreezeInput{
		UserID: userID, Amount: money.FromDecimal(dec(500)), Reference: "c2c_unfreeze:if1",
	})
	if !errors.Is(err, walletcontract.ErrInsufficientFrozen) {
		t.Fatalf("err = %v, want ErrInsufficientFrozen", err)
	}
	avail, frozen := env.balances(t, userID)
	if !avail.Equal(dec(900)) || !frozen.Equal(dec(100)) {
		t.Fatalf("balances mutated on failure: available=%s frozen=%s", avail, frozen)
	}
}

func TestUnfreeze_DuplicateIdempotent(t *testing.T) {
	env := newFreezeTestEnv(t)
	const userID = uint(1007)
	env.mustSeedAvailable(t, userID, 1000)
	env.mustFreeze(t, userID, 200, "c2c_freeze:dup-u1")

	_, firstTxn, err := env.unfreeze(t, walletcontract.UnfreezeInput{
		UserID: userID, Amount: money.FromDecimal(dec(100)), Reference: "c2c_unfreeze:dup-u1",
	})
	if err != nil {
		t.Fatalf("first unfreeze: %v", err)
	}
	_, secondTxn, err := env.unfreeze(t, walletcontract.UnfreezeInput{
		UserID: userID, Amount: money.FromDecimal(dec(100)), Reference: "c2c_unfreeze:dup-u1",
	})
	if err != nil {
		t.Fatalf("duplicate unfreeze should be idempotent, got: %v", err)
	}
	if firstTxn.ID != secondTxn.ID {
		t.Fatalf("duplicate unfreeze returned different txn id")
	}
	avail, frozen := env.balances(t, userID)
	if !avail.Equal(dec(900)) || !frozen.Equal(dec(100)) {
		t.Fatalf("after idempotent unfreeze available=%s frozen=%s, want 900/100", avail, frozen)
	}
}

func TestSettle_Success(t *testing.T) {
	env := newFreezeTestEnv(t)
	const src, dst = uint(1008), uint(1009)
	env.mustSeedAvailable(t, src, 1000)
	env.mustFreeze(t, src, 500, "c2c_freeze:biz-s1") // src: avail 500, frozen 500
	env.mustSeedAvailable(t, dst, 200)               // dst: avail 200

	if err := env.settle(t, walletcontract.SettleInput{
		SourceUserID: src, TargetUserID: dst, Amount: money.FromDecimal(dec(300)),
		SourceReference: "c2c_settle:biz-s1", TargetReference: "c2c_receive:biz-s1",
	}); err != nil {
		t.Fatalf("SettleFrozen failed: %v", err)
	}
	srcAvail, srcFrozen := env.balances(t, src)
	dstAvail, dstFrozen := env.balances(t, dst)
	if !srcAvail.Equal(dec(500)) || !srcFrozen.Equal(dec(200)) {
		t.Fatalf("source = avail %s / frozen %s, want 500/200", srcAvail, srcFrozen)
	}
	if !dstAvail.Equal(dec(500)) || !dstFrozen.Equal(dec(0)) {
		t.Fatalf("target = avail %s / frozen %s, want 500/0", dstAvail, dstFrozen)
	}
	// 合计资产守恒：(500+200)+(500+0)=1200 == (1000)+(200)
	if !srcAvail.Add(srcFrozen).Add(dstAvail).Add(dstFrozen).Equal(dec(1200)) {
		t.Fatalf("combined assets not conserved")
	}
}

func TestSettle_DuplicateIdempotent(t *testing.T) {
	env := newFreezeTestEnv(t)
	const src, dst = uint(1010), uint(1011)
	env.mustSeedAvailable(t, src, 1000)
	env.mustFreeze(t, src, 400, "c2c_freeze:dup-s1")

	in := walletcontract.SettleInput{
		SourceUserID: src, TargetUserID: dst, Amount: money.FromDecimal(dec(100)),
		SourceReference: "c2c_settle:dup-s1", TargetReference: "c2c_receive:dup-s1",
	}
	if err := env.settle(t, in); err != nil {
		t.Fatalf("first settle: %v", err)
	}
	if err := env.settle(t, in); err != nil {
		t.Fatalf("duplicate settle should be idempotent, got: %v", err)
	}
	_, srcFrozen := env.balances(t, src)
	dstAvail, _ := env.balances(t, dst)
	if !srcFrozen.Equal(dec(300)) {
		t.Fatalf("source frozen = %s, want 300 (settled once)", srcFrozen)
	}
	if !dstAvail.Equal(dec(100)) {
		t.Fatalf("target available = %s, want 100 (received once)", dstAvail)
	}
}

func TestSettle_SourceEqualsTarget(t *testing.T) {
	env := newFreezeTestEnv(t)
	const userID = uint(1012)
	env.mustSeedAvailable(t, userID, 1000)
	env.mustFreeze(t, userID, 500, "c2c_freeze:self-settle")

	err := env.settle(t, walletcontract.SettleInput{
		SourceUserID: userID, TargetUserID: userID, Amount: money.FromDecimal(dec(100)),
		SourceReference: "c2c_settle:self", TargetReference: "c2c_receive:self",
	})
	if !errors.Is(err, walletcontract.ErrSameAccount) {
		t.Fatalf("err = %v, want ErrSameAccount", err)
	}
}

func TestSettle_InsufficientFrozen(t *testing.T) {
	env := newFreezeTestEnv(t)
	const src, dst = uint(1013), uint(1014)
	env.mustSeedAvailable(t, src, 1000)
	env.mustFreeze(t, src, 50, "c2c_freeze:insuff-settle")

	err := env.settle(t, walletcontract.SettleInput{
		SourceUserID: src, TargetUserID: dst, Amount: money.FromDecimal(dec(200)),
		SourceReference: "c2c_settle:insuff", TargetReference: "c2c_receive:insuff",
	})
	if !errors.Is(err, walletcontract.ErrInsufficientFrozen) {
		t.Fatalf("err = %v, want ErrInsufficientFrozen", err)
	}
	_, srcFrozen := env.balances(t, src)
	if !srcFrozen.Equal(dec(50)) {
		t.Fatalf("source frozen mutated on failure: %s", srcFrozen)
	}
}

// ---------- 不变量验证 ----------

func TestFreeze_TotalInvariant(t *testing.T) {
	env := newFreezeTestEnv(t)
	const userID = uint(1101)
	env.mustSeedAvailable(t, userID, 777)

	_, txn, err := env.freeze(t, walletcontract.FreezeInput{
		UserID: userID, Amount: money.FromDecimal(dec(333)), Reference: "c2c_freeze:inv-1",
	})
	if err != nil {
		t.Fatalf("freeze: %v", err)
	}
	if !txn.BalanceBefore.Decimal.Equal(txn.BalanceAfter.Decimal) {
		t.Fatalf("total not invariant: before=%s after=%s", txn.BalanceBefore.Decimal, txn.BalanceAfter.Decimal)
	}
	if !txn.BalanceBefore.Decimal.Equal(dec(777)) {
		t.Fatalf("total = %s, want 777", txn.BalanceBefore.Decimal)
	}
	// available_before + frozen_before == available_after + frozen_after
	before := txn.AvailableBefore.Decimal.Add(txn.FrozenBefore.Decimal)
	after := txn.AvailableAfter.Decimal.Add(txn.FrozenAfter.Decimal)
	if !before.Equal(after) {
		t.Fatalf("component totals differ: before=%s after=%s", before, after)
	}
}

func TestUnfreeze_TotalInvariant(t *testing.T) {
	env := newFreezeTestEnv(t)
	const userID = uint(1102)
	env.mustSeedAvailable(t, userID, 500)
	env.mustFreeze(t, userID, 200, "c2c_freeze:inv-u1")

	_, txn, err := env.unfreeze(t, walletcontract.UnfreezeInput{
		UserID: userID, Amount: money.FromDecimal(dec(50)), Reference: "c2c_unfreeze:inv-u1",
	})
	if err != nil {
		t.Fatalf("unfreeze: %v", err)
	}
	if !txn.BalanceBefore.Decimal.Equal(txn.BalanceAfter.Decimal) {
		t.Fatalf("total not invariant across unfreeze: before=%s after=%s", txn.BalanceBefore.Decimal, txn.BalanceAfter.Decimal)
	}
	before := txn.AvailableBefore.Decimal.Add(txn.FrozenBefore.Decimal)
	after := txn.AvailableAfter.Decimal.Add(txn.FrozenAfter.Decimal)
	if !before.Equal(after) {
		t.Fatalf("component totals differ: before=%s after=%s", before, after)
	}
}

func TestSettle_ConservationAcrossAccounts(t *testing.T) {
	env := newFreezeTestEnv(t)
	const src, dst = uint(1103), uint(1104)
	env.mustSeedAvailable(t, src, 1000)
	env.mustFreeze(t, src, 600, "c2c_freeze:cons-s1") // src total 1000
	env.mustSeedAvailable(t, dst, 300)                // dst total 300

	srcAvailBefore, srcFrozenBefore := env.balances(t, src)
	dstAvailBefore, dstFrozenBefore := env.balances(t, dst)
	beforeSum := srcAvailBefore.Add(srcFrozenBefore).Add(dstAvailBefore).Add(dstFrozenBefore)

	if err := env.settle(t, walletcontract.SettleInput{
		SourceUserID: src, TargetUserID: dst, Amount: money.FromDecimal(dec(400)),
		SourceReference: "c2c_settle:cons-s1", TargetReference: "c2c_receive:cons-s1",
	}); err != nil {
		t.Fatalf("settle: %v", err)
	}
	srcAvailAfter, srcFrozenAfter := env.balances(t, src)
	dstAvailAfter, dstFrozenAfter := env.balances(t, dst)
	afterSum := srcAvailAfter.Add(srcFrozenAfter).Add(dstAvailAfter).Add(dstFrozenAfter)
	if !beforeSum.Equal(afterSum) {
		t.Fatalf("combined assets changed: before=%s after=%s", beforeSum, afterSum)
	}
	if !afterSum.Equal(dec(1300)) {
		t.Fatalf("combined = %s, want 1300", afterSum)
	}
}

// ---------- 并发测试 ----------

func TestFreeze_Concurrent(t *testing.T) {
	if testing.Short() {
		t.Skip("skip concurrency in short mode")
	}
	env := newConcurrentFreezeTestEnv(t)
	const userID = uint(2001)
	env.mustSeedAvailable(t, userID, 1000)

	const workers = 10
	var wg sync.WaitGroup
	errCh := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, _, err := env.freeze(t, walletcontract.FreezeInput{
				UserID: userID, Amount: money.FromDecimal(dec(100)),
				Reference: fmt.Sprintf("c2c_freeze:conc-%d", idx),
			})
			if err != nil {
				errCh <- fmt.Errorf("worker %d: %w", idx, err)
			}
		}(i)
	}
	waitOrFail(t, &wg, 10*time.Second, "TestFreeze_Concurrent")
	close(errCh)
	for err := range errCh {
		t.Errorf("concurrent freeze: %v", err)
	}
	avail, frozen := env.balances(t, userID)
	if !avail.Equal(dec(0)) {
		t.Fatalf("available = %s, want 0 (no over-freeze)", avail)
	}
	if !frozen.Equal(dec(1000)) {
		t.Fatalf("frozen = %s, want 1000", frozen)
	}
}

func TestFreeze_ConcurrentWithOrderDebit(t *testing.T) {
	if testing.Short() {
		t.Skip("skip concurrency in short mode")
	}
	env := newConcurrentFreezeTestEnv(t)
	const userID = uint(2002)
	env.mustSeedAvailable(t, userID, 2000)

	const half = 5
	var wg sync.WaitGroup
	errCh := make(chan error, half*2)
	// 5 个冻结：available→frozen
	for i := 0; i < half; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, _, err := env.freeze(t, walletcontract.FreezeInput{
				UserID: userID, Amount: money.FromDecimal(dec(100)),
				Reference: fmt.Sprintf("c2c_freeze:mix-%d", idx),
			})
			if err != nil {
				errCh <- fmt.Errorf("freeze %d: %w", idx, err)
			}
		}(i)
	}
	// 5 个订单扣款（available 真实流出），与 freeze 交错
	for i := 0; i < half; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			err := env.unit.WithinTransaction(func(tx walletcontract.Transaction) error {
				_, e := env.svc.ApplyOrderBalance(tx, walletcontract.OrderBalanceInput{
					OrderID: uint(3000 + idx), UserID: userID,
					TotalAmount:      money.FromDecimal(dec(100)),
					WalletPaidAmount: money.FromDecimal(decimal.Zero),
					Currency:         "USDT", UseBalance: true,
				})
				return e
			})
			if err != nil {
				errCh <- fmt.Errorf("order debit %d: %w", idx, err)
			}
		}(i)
	}
	waitOrFail(t, &wg, 10*time.Second, "TestFreeze_ConcurrentWithOrderDebit")
	close(errCh)
	for err := range errCh {
		t.Errorf("mix: %v", err)
	}
	avail, frozen := env.balances(t, userID)
	// available: 2000 - 5*100(freeze) - 5*100(order) = 1000；frozen: 5*100 = 500
	if !avail.Equal(dec(1000)) {
		t.Fatalf("available = %s, want 1000", avail)
	}
	if !frozen.Equal(dec(500)) {
		t.Fatalf("frozen = %s, want 500", frozen)
	}
}

// TestFreeze_ConcurrentWithWithdrawal 用订单扣款（同为 available 流出 + 行锁同一账户）
// 模拟提现写路径的锁语义。真正的提现模块不在本包内，但其对单账户的加锁方式与
// ApplyOrderBalance 一致（ensureAccountForUpdate → 改 available），因此能覆盖
// freeze 与"提现式扣款"交错时是否死锁/超扣。
func TestFreeze_ConcurrentWithWithdrawal(t *testing.T) {
	if testing.Short() {
		t.Skip("skip concurrency in short mode")
	}
	env := newConcurrentFreezeTestEnv(t)
	const userID = uint(2003)
	env.mustSeedAvailable(t, userID, 2000)

	const half = 5
	var wg sync.WaitGroup
	errCh := make(chan error, half*2)
	for i := 0; i < half; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, _, err := env.freeze(t, walletcontract.FreezeInput{
				UserID: userID, Amount: money.FromDecimal(dec(100)),
				Reference: fmt.Sprintf("c2c_freeze:wdr-%d", idx),
			})
			if err != nil {
				errCh <- fmt.Errorf("freeze %d: %w", idx, err)
			}
		}(i)
	}
	// 模拟"提现扣款"：available 流出 100
	for i := 0; i < half; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			err := env.unit.WithinTransaction(func(tx walletcontract.Transaction) error {
				_, e := env.svc.ApplyOrderBalance(tx, walletcontract.OrderBalanceInput{
					OrderID: uint(4000 + idx), UserID: userID,
					TotalAmount:      money.FromDecimal(dec(100)),
					WalletPaidAmount: money.FromDecimal(decimal.Zero),
					Currency:         "USDT", UseBalance: true,
				})
				return e
			})
			if err != nil {
				errCh <- fmt.Errorf("withdrawal-like debit %d: %w", idx, err)
			}
		}(i)
	}
	waitOrFail(t, &wg, 10*time.Second, "TestFreeze_ConcurrentWithWithdrawal")
	close(errCh)
	for err := range errCh {
		t.Errorf("mix: %v", err)
	}
	avail, frozen := env.balances(t, userID)
	if !avail.Equal(dec(1000)) {
		t.Fatalf("available = %s, want 1000", avail)
	}
	if !frozen.Equal(dec(500)) {
		t.Fatalf("frozen = %s, want 500", frozen)
	}
}

func TestUnfreeze_ConcurrentWithSettle(t *testing.T) {
	if testing.Short() {
		t.Skip("skip concurrency in short mode")
	}
	env := newConcurrentFreezeTestEnv(t)
	// A=3000 持有 frozen，B=3001 接收
	const a, b = uint(3000), uint(3001)
	env.mustSeedAvailable(t, a, 1000)
	env.mustFreeze(t, a, 800, "c2c_freeze:us-a") // a: avail 200, frozen 800

	const half = 4
	var wg sync.WaitGroup
	errCh := make(chan error, half*2)
	// 4 个解冻：frozen→available，每个 100
	for i := 0; i < half; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, _, err := env.unfreeze(t, walletcontract.UnfreezeInput{
				UserID: a, Amount: money.FromDecimal(dec(100)),
				Reference: fmt.Sprintf("c2c_unfreeze:us-%d", idx),
			})
			if err != nil {
				errCh <- fmt.Errorf("unfreeze %d: %w", idx, err)
			}
		}(i)
	}
	// 4 个 settle: a.frozen → b.available，每个 100
	for i := 0; i < half; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			err := env.settle(t, walletcontract.SettleInput{
				SourceUserID: a, TargetUserID: b, Amount: money.FromDecimal(dec(100)),
				SourceReference: fmt.Sprintf("c2c_settle:us-%d", idx), TargetReference: fmt.Sprintf("c2c_receive:us-%d", idx),
			})
			if err != nil {
				errCh <- fmt.Errorf("settle %d: %w", idx, err)
			}
		}(i)
	}
	waitOrFail(t, &wg, 10*time.Second, "TestUnfreeze_ConcurrentWithSettle")
	close(errCh)
	for err := range errCh {
		t.Errorf("mix: %v", err)
	}
	aAvail, aFrozen := env.balances(t, a)
	bAvail, _ := env.balances(t, b)
	// frozen 800 = 4*100(unfreeze) + 4*100(settle)
	if !aFrozen.Equal(dec(0)) {
		t.Fatalf("A frozen = %s, want 0", aFrozen)
	}
	if !aAvail.Equal(dec(600)) {
		t.Fatalf("A available = %s, want 600 (200 + 400 unfrozen)", aAvail)
	}
	if !bAvail.Equal(dec(400)) {
		t.Fatalf("B available = %s, want 400", bAvail)
	}
}

// TestSettle_ReverseDirectionNoDeadlock：两个 goroutine 同时做 A→B 与 B→A 的 settle。
// 即使业务角色方向相反，锁顺序仍按 user_id 升序（先锁较小 id 再锁较大 id），
// 因此不会形成循环等待。SQLite 下单连接串行化写入，这里主要验证不超时 + 金额正确，
// 真正的防死锁保证来自 LockAccountsByUserIDOrder 的升序锁协议（在支持行锁的 RDBMS 上生效）。
func TestSettle_ReverseDirectionNoDeadlock(t *testing.T) {
	if testing.Short() {
		t.Skip("skip concurrency in short mode")
	}
	env := newConcurrentFreezeTestEnv(t)
	const a, b = uint(4000), uint(4001)
	env.mustSeedAvailable(t, a, 1000)
	env.mustFreeze(t, a, 500, "c2c_freeze:rev-a") // a: avail 500, frozen 500
	env.mustSeedAvailable(t, b, 1000)
	env.mustFreeze(t, b, 500, "c2c_freeze:rev-b") // b: avail 500, frozen 500

	const half = 3
	var wg sync.WaitGroup
	errCh := make(chan error, half*2)
	// A → B：a.frozen → b.available
	for i := 0; i < half; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			err := env.settle(t, walletcontract.SettleInput{
				SourceUserID: a, TargetUserID: b, Amount: money.FromDecimal(dec(100)),
				SourceReference: fmt.Sprintf("c2c_settle:ab-%d", idx), TargetReference: fmt.Sprintf("c2c_receive:ab-%d", idx),
			})
			if err != nil {
				errCh <- fmt.Errorf("A->B %d: %w", idx, err)
			}
		}(i)
	}
	// B → A：b.frozen → a.available
	for i := 0; i < half; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			err := env.settle(t, walletcontract.SettleInput{
				SourceUserID: b, TargetUserID: a, Amount: money.FromDecimal(dec(100)),
				SourceReference: fmt.Sprintf("c2c_settle:ba-%d", idx), TargetReference: fmt.Sprintf("c2c_receive:ba-%d", idx),
			})
			if err != nil {
				errCh <- fmt.Errorf("B->A %d: %w", idx, err)
			}
		}(i)
	}
	waitOrFail(t, &wg, 10*time.Second, "TestSettle_ReverseDirectionNoDeadlock")
	close(errCh)
	for err := range errCh {
		t.Errorf("reverse settle: %v", err)
	}
	aAvail, aFrozen := env.balances(t, a)
	bAvail, bFrozen := env.balances(t, b)
	// A: frozen 500 - 300(A->B out) = 200；avail 500 + 300(B->A in) = 800
	if !aAvail.Equal(dec(800)) || !aFrozen.Equal(dec(200)) {
		t.Fatalf("A = avail %s / frozen %s, want 800/200", aAvail, aFrozen)
	}
	// B: avail 500 + 300(A->B in) = 800；frozen 500 - 300(B->A out) = 200
	if !bAvail.Equal(dec(800)) || !bFrozen.Equal(dec(200)) {
		t.Fatalf("B = avail %s / frozen %s, want 800/200", bAvail, bFrozen)
	}
}
