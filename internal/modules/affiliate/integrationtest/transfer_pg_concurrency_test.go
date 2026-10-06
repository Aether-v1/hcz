//go:build integration
// +build integration

// transfer_pg_concurrency_test.go — Affiliate 划转（transfer-to-wallet）在真实 PostgreSQL 上的并发验证。
//
// 运行（PowerShell）：
//
//	$env:TEST_POSTGRES_DSN="host=127.0.0.1 port=5432 user=postgres password= dbname=hcz_test sslmode=disable TimeZone=UTC"
//	go test -tags integration -run TestPGTransfer -v -timeout 300s ./internal/modules/affiliate/integrationtest/
//
// 设计要点：
//   - 复用 newPGAffFixture：每个用例重建全部表，ConfirmDays=0（佣金直接 available），L1=10%。
//   - 并发用 sync.WaitGroup + start 栅栏保证 goroutine 同时开始。
//   - 仅对 PG 40P01 deadlock 做有限重试（runWithDeadlockRetry），其余错误一律不重试。
//   - 核心不变量：wallet 入账恰好一次、affiliate ledger 恰好一条 transfer_to_wallet、无超扣、资金守恒。
package integrationtest

import (
	"sync"
	"testing"

	"github.com/Aether-v1/hcz/internal/constants"

	affiliateapp "github.com/Aether-v1/hcz/internal/modules/affiliate/application"
	affiliatedomain "github.com/Aether-v1/hcz/internal/modules/affiliate/domain"
	walletdomain "github.com/Aether-v1/hcz/internal/modules/wallet/domain"

	"github.com/shopspring/decimal"
)

// ---- 局部统计辅助（作用于 pgAffFixture，与 pg_concurrency_test.go 的辅助互补）----

func pgLedgerCount(t *testing.T, f *pgAffFixture, profileID uint, typ string) int64 {
	t.Helper()
	var n int64
	if err := f.db.Model(&affiliatedomain.CommissionLedger{}).
		Where("affiliate_profile_id = ? AND type = ?", profileID, typ).
		Count(&n).Error; err != nil {
		t.Fatalf("count ledger type=%s: %v", typ, err)
	}
	return n
}

func pgLedgerSum(t *testing.T, f *pgAffFixture, profileID uint, typ string) decimal.Decimal {
	t.Helper()
	sum, err := f.affiliateRepo.SumLedgerByProfile(profileID, []string{typ})
	if err != nil {
		t.Fatalf("sum ledger type=%s: %v", typ, err)
	}
	return sum.Round(2)
}

func pgWalletTransferInCount(t *testing.T, f *pgAffFixture, userID uint) int64 {
	t.Helper()
	var n int64
	f.db.Model(&walletdomain.Transaction{}).
		Where("user_id = ? AND type = ? AND direction = ?",
			userID, constants.WalletTxnTypeAffiliateTransferIn, constants.WalletTxnDirectionIn).
		Count(&n)
	return n
}

// =========================================================================
// CASE 21：同一用户并发 10 个 {all:true}，只有 1 个成功入账。
// =========================================================================
func TestPGTransfer_ConcurrentAll(t *testing.T) {
	f := newPGAffFixture(t)
	f.seedOrder(t, 1, 100) // L1 available = 10

	const n = 10
	var wg sync.WaitGroup
	start := make(chan struct{})
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			errs[i] = runWithDeadlockRetry(f, func() error {
				_, _, e := f.svc.TransferToWallet(f.l1User().ID, affiliateapp.TransferToWalletInput{All: true})
				return e
			}, 3)
		}(i)
	}
	close(start)
	wg.Wait()

	success := 0
	for _, e := range errs {
		if e == nil {
			success++
		}
	}

	// 关键断言：只有 1 个成功。
	if success != 1 {
		t.Fatalf("CONC-ALL: want exactly 1 successful all-transfer (10 available x10 goroutines), got %d (errors=%v)", success, errs)
	}
	// wallet 只入账一次。
	if got := pgWalletTransferInCount(t, f, f.l1User().ID); got != 1 {
		t.Fatalf("CONC-ALL: wallet affiliate_transfer_in count want 1, got %d (errors=%v)", got, errs)
	}
	// affiliate ledger 只有一条 transfer_to_wallet。
	if got := pgLedgerCount(t, f, f.l1Profile().ID, constants.AffiliateLedgerTypeTransferToWallet); got != 1 {
		t.Fatalf("CONC-ALL: transfer_to_wallet ledger count want 1, got %d", got)
	}
	// transfer_to_wallet 金额 = -10。
	if got := pgLedgerSum(t, f, f.l1Profile().ID, constants.AffiliateLedgerTypeTransferToWallet); !got.Equal(pgDec("-10.00")) {
		t.Fatalf("CONC-ALL: transfer sum want -10.00, got %s", got)
	}
	// wallet 可用余额恰好 10。
	if got := f.walletAvailable(t, f.l1User().ID); !got.Equal(pgDec("10.00")) {
		t.Fatalf("CONC-ALL: wallet available want 10.00, got %s", got)
	}
	t.Logf("CONC-ALL PASS: success=%d errors=%v deadlocks=%d wallet=%s",
		success, errs, f.deadlocks, f.walletAvailable(t, f.l1User().ID))
}

// =========================================================================
// CASE 22：同一用户并发 5 个部分划转，总额 <= 可用，wallet 总入账 = affiliate 总扣减。
// =========================================================================
func TestPGTransfer_ConcurrentPartial(t *testing.T) {
	f := newPGAffFixture(t)
	f.seedOrder(t, 1, 100) // L1 available = 10

	const per = "3.00" // 5 x 3 = 15 > 10 → 只能成功 3 笔（9）
	const n = 5
	var wg sync.WaitGroup
	start := make(chan struct{})
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			errs[i] = runWithDeadlockRetry(f, func() error {
				_, _, e := f.svc.TransferToWallet(f.l1User().ID, affiliateapp.TransferToWalletInput{Amount: pgDec(per)})
				return e
			}, 3)
		}(i)
	}
	close(start)
	wg.Wait()

	success := 0
	for _, e := range errs {
		if e == nil {
			success++
		}
	}

	// 资金守恒：affiliate 总扣减（transfer_to_wallet 绝对值）= wallet 总入账。
	deducted := pgLedgerSum(t, f, f.l1Profile().ID, constants.AffiliateLedgerTypeTransferToWallet).Abs()
	walletBal := f.walletAvailable(t, f.l1User().ID)
	if !deducted.Equal(walletBal) {
		t.Fatalf("CONC-PARTIAL: affiliate deducted(%s) must equal wallet credited(%s), success=%d errors=%v",
			deducted, walletBal, success, errs)
	}
	// 总扣减不得超过初始可用 10。
	if deducted.GreaterThan(pgDec("10.00")) {
		t.Fatalf("CONC-PARTIAL: over-draw deducted=%s > 10, success=%d errors=%v", deducted, success, errs)
	}
	// wallet 入账笔数 == 成功笔数。
	if got := pgWalletTransferInCount(t, f, f.l1User().ID); int(got) != success {
		t.Fatalf("CONC-PARTIAL: wallet credit txn count want %d (==success), got %d", success, got)
	}
	t.Logf("CONC-PARTIAL PASS: success=%d deducted=%s wallet=%s errors=%v deadlocks=%d",
		success, deducted, walletBal, errs, f.deadlocks)
}

// =========================================================================
// CASE 23：划转 ∥ 退款 并发，验证资金守恒（无超扣、无重复入账）。
// =========================================================================
func TestPGTransfer_ConcurrentWithRefund(t *testing.T) {
	f := newPGAffFixture(t)
	f.seedOrder(t, 1, 100) // L1 available = 10

	var wg sync.WaitGroup
	start := make(chan struct{})
	var transferErr, refundErr error
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		transferErr = runWithDeadlockRetry(f, func() error {
			_, _, e := f.svc.TransferToWallet(f.l1User().ID, affiliateapp.TransferToWalletInput{All: true})
			return e
		}, 3)
	}()
	go func() {
		defer wg.Done()
		<-start
		refundErr = runWithDeadlockRetry(f, func() error { return f.refundFull(t, 1, "100") }, 3)
	}()
	close(start)
	wg.Wait()

	creditN := pgWalletTransferInCount(t, f, f.l1User().ID)
	txN := pgLedgerCount(t, f, f.l1Profile().ID, constants.AffiliateLedgerTypeTransferToWallet)
	revN := pgLedgerCount(t, f, f.l1Profile().ID, constants.AffiliateLedgerTypeReversal)
	debtN := pgLedgerCount(t, f, f.l1Profile().ID, constants.AffiliateLedgerTypeDebt)
	walletBal := f.walletAvailable(t, f.l1User().ID)
	net := f.netProfileBalance(t, f.l1Profile().ID)

	t.Logf("CONC-REFUND: transferErr=%v refundErr=%v creditN=%d txN=%d revN=%d debtN=%d wallet=%s net=%s deadlocks=%d",
		transferErr, refundErr, creditN, txN, revN, debtN, walletBal, net, f.deadlocks)

	// 不变量 1：wallet 入账至多一次（无重复入账）。
	if creditN > 1 {
		t.Fatalf("CONC-REFUND: duplicate wallet credit, count=%d", creditN)
	}
	// 不变量 2：wallet 不得超扣（最多 10）。
	if walletBal.GreaterThan(pgDec("10.00")) {
		t.Fatalf("CONC-REFUND: wallet over-drawn=%s", walletBal)
	}
	// 不变量 3：affiliate transfer ledger 与 wallet 入账笔数一致。
	if txN != creditN {
		t.Fatalf("CONC-REFUND: transfer ledger count(%d) must equal wallet credit count(%d)", txN, creditN)
	}

	if creditN == 1 {
		// 划转先赢：wallet 入账一次 + 一条 transfer_to_wallet；随后退款必须形成一条 reversal。
		if txN != 1 {
			t.Fatalf("CONC-REFUND: transfer-won path must have exactly 1 transfer ledger, got %d", txN)
		}
		if revN != 1 {
			t.Fatalf("CONC-REFUND: transfer-won path must create exactly 1 reversal, got %d", revN)
		}
		if !walletBal.Equal(pgDec("10.00")) {
			t.Fatalf("CONC-REFUND: transfer-won wallet want 10.00, got %s", walletBal)
		}
	} else {
		// 退款先赢：不得入账、不得产生 transfer ledger；必须有一条 reversal。
		if txN != 0 {
			t.Fatalf("CONC-REFUND: refund-won path must NOT create transfer ledger, got %d", txN)
		}
		if revN != 1 {
			t.Fatalf("CONC-REFUND: refund-won path must create exactly 1 reversal, got %d", revN)
		}
		if !walletBal.Equal(pgDec("0.00")) {
			t.Fatalf("CONC-REFUND: refund-won wallet must be 0, got %s", walletBal)
		}
	}
	_ = debtN
}
