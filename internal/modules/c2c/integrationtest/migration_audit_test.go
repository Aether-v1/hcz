// Package integrationtest — Phase 6 C2C Deployment Gate 审计。
//
// 本文件只读地演示 MigrateListingsToListingFreeze 上线前应执行的 Dry Audit，
// 并通过 testkit 构造「旧模型」（CreateTrade 时 Freeze、CreateListing 不 Freeze）
// 历史数据，验证迁移规则、资金守恒与幂等性。
//
// 注意：开发库 ./db/hcz.db 当前 C2C/Wallet/Affiliate 无真实业务数据（DEV_DB_EMPTY），
// 因此所有结论均基于本文件构造的 fixture 数据；报告文档据此标注。
package integrationtest

import (
	"testing"

	"github.com/Aether-v1/hcz/internal/constants"
	c2ccontract "github.com/Aether-v1/hcz/internal/modules/c2c/contract"
	c2cdomain "github.com/Aether-v1/hcz/internal/modules/c2c/domain"
	walletcontract "github.com/Aether-v1/hcz/internal/modules/wallet/contract"
	walletdomain "github.com/Aether-v1/hcz/internal/modules/wallet/domain"

	"github.com/shopspring/decimal"
)

// ---------------------------------------------------------------------------
// 6.1 Dry Audit 只读结构
// ---------------------------------------------------------------------------

// DryAuditRow 单个 active/paused 挂单的迁移前快照。
type DryAuditRow struct {
	ListingNo       string
	RemainingUSDT    decimal.Decimal // = listing.available_usdt（待冻结本金）
	SellerUserID    uint
	SellerAvailable decimal.Decimal // seller 当前可用余额
	SellerFrozen    decimal.Decimal // seller 当前已冻结余额（含旧 trade 冻结）
	ExistingRef     string          // 已存在的 c2c_freeze:listing:{no} 引用（空=无）
	WouldSuspend    bool            // 审计预判：available < remaining → 需 suspend
}

// DryAuditReport 迁移前只读汇总。
type DryAuditReport struct {
	ActiveListingCount int64
	ActiveTradeCount  int64 // status IN (pending_payment, paid, disputed)
	Rows              []DryAuditRow
}

// runMigrationDryAudit 只读扫描，不写任何数据。
// 对每个 status IN (active,paused) 的 listing 输出：
//   - remaining = listing.available_usdt
//   - seller available / frozen
//   - 是否已存在 c2c_freeze:listing:{listingNo} 引用
//   - 预判 available < remaining → WouldSuspend
func runMigrationDryAudit(f *fixture) DryAuditReport {
	var rep DryAuditReport

	f.db.Model(&c2cdomain.Listing{}).
		Where("status IN ?", []string{"active", "paused"}).
		Count(&rep.ActiveListingCount)

	f.db.Model(&c2cdomain.Trade{}).
		Where("status IN ?", []string{"pending_payment", "paid", "disputed"}).
		Count(&rep.ActiveTradeCount)

	listings, _ := f.c2cDB.ListListingsByStatus([]string{"active", "paused"})
	for i := range listings {
		l := listings[i]
		row := DryAuditRow{
			ListingNo:    l.ListingNo,
			RemainingUSDT: l.AvailableUSDT.Decimal.Round(2),
			SellerUserID: l.SellerUserID,
		}
		acc, _ := f.walletDB.GetAccountByUserID(l.SellerUserID)
		if acc != nil {
			row.SellerAvailable = acc.AvailableBalance.Decimal.Round(2)
			row.SellerFrozen = acc.FrozenBalance.Decimal.Round(2)
		} else {
			row.SellerAvailable = decimal.Zero
			row.SellerFrozen = decimal.Zero
		}
		// 已有挂单冻结引用
		ref := "c2c_freeze:listing:" + l.ListingNo
		if txn, _ := f.walletDB.GetTransactionByReference(ref); txn != nil {
			row.ExistingRef = txn.Reference
		}
		row.WouldSuspend = row.SellerAvailable.Round(2).LessThan(row.RemainingUSDT)
		rep.Rows = append(rep.Rows, row)
	}
	return rep
}

// ---------------------------------------------------------------------------
// 旧模型 fixture 构造助手（直接写库，绕过 service 的新模型 Freeze）
// ---------------------------------------------------------------------------

// insertLegacyListing 直接落库一条旧模型挂单（不触发任何 wallet Freeze）。
func (f *fixture) insertLegacyListing(t *testing.T, sellerID uint, listingNo, total, remaining string, status string) *c2cdomain.Listing {
	t.Helper()
	l := &c2cdomain.Listing{
		ListingNo:     listingNo,
		SellerUserID:  sellerID,
		FiatCurrency:  "CNY",
		Price:         mustMoney("7.00"),
		MinFiatAmount: mustMoney("1.00"),
		MaxFiatAmount: mustMoney("100000.00"),
		TotalUSDT:     mustMoney(total),
		AvailableUSDT: mustMoney(remaining),
		Status:        status,
	}
	if err := f.c2cDB.CreateListing(l); err != nil {
		t.Fatalf("insert legacy listing %s: %v", listingNo, err)
	}
	return l
}

// oldModelFreeze 用 wallet.Freeze 模拟旧模型 CreateTrade 时对 seller 的冻结。
// 用于构造「已有 active Trade Freeze」的历史快照，验证迁移不重复冻结。
func (f *fixture) oldModelFreeze(t *testing.T, userID uint, amount string, ref string) {
	t.Helper()
	err := f.c2cDB.WithinTransaction(func(tx c2ccontract.Transaction) error {
		_, _, err := f.walletSvc.Freeze(tx, walletcontract.FreezeInput{
			UserID: userID, Amount: mustMoney(amount), Reference: ref, Remark: "旧模型trade冻结",
		})
		return err
	})
	if err != nil {
		t.Fatalf("oldModelFreeze user=%d amount=%s ref=%s: %v", userID, amount, ref, err)
	}
}

// sumAllSellerTotal 汇总所有钱包账户 available+frozen（资金守恒用）。
func (f *fixture) sumAllSellerTotal(t *testing.T) decimal.Decimal {
	t.Helper()
	var accounts []walletdomain.Account
	if err := f.db.Find(&accounts).Error; err != nil {
		t.Fatalf("sum accounts: %v", err)
	}
	sum := decimal.Zero
	for _, a := range accounts {
		sum = sum.Add(a.AvailableBalance.Decimal).Add(a.FrozenBalance.Decimal)
	}
	return sum.Round(2)
}

// countListingFreezeRefs 统计 reference LIKE 'c2c_freeze:listing:%' 的 ledger 条数与金额。
func (f *fixture) countListingFreezeRefs(t *testing.T) (int64, decimal.Decimal) {
	t.Helper()
	var txns []walletdomain.Transaction
	if err := f.db.Where("reference LIKE ?", "c2c_freeze:listing:%").Find(&txns).Error; err != nil {
		t.Fatalf("query listing freeze refs: %v", err)
	}
	sum := decimal.Zero
	for _, tx := range txns {
		sum = sum.Add(tx.Amount.Decimal)
	}
	return int64(len(txns)), sum.Round(2)
}

// ---------------------------------------------------------------------------
// 6.1 Dry Audit 测试
// ---------------------------------------------------------------------------

// TestMigrationDryAudit_EmptyFixture 开发库为空时 fixture 演示：0 active listing / 0 active trade。
// 对应真实 dev 库（./db/hcz.db）的 DEV_DB_EMPTY 标注。
func TestMigrationDryAudit_EmptyFixture(t *testing.T) {
	f := newFixture(t)
	rep := runMigrationDryAudit(f)
	if rep.ActiveListingCount != 0 || rep.ActiveTradeCount != 0 || len(rep.Rows) != 0 {
		t.Fatalf("empty fixture: want 0/0 rows, got %+v", rep)
	}
}

// TestMigrationDryAudit_ReportsLegacyListings 构造旧模型数据后，Dry Audit 正确输出每行详情。
func TestMigrationDryAudit_ReportsLegacyListings(t *testing.T) {
	f := newFixture(t)
	sellerA, sellerB := uint(101), uint(102)
	f.createUser(t, sellerA)
	f.createUser(t, sellerB)
	f.setBalance(t, sellerA, "1000.00")
	f.setBalance(t, sellerB, "50.00")

	// sellerA 有一笔旧 trade 冻结 300（模拟 active trade 已冻）
	f.oldModelFreeze(t, sellerA, "300.00", "c2c_freeze:trade:TR-OLD-1")

	f.insertLegacyListing(t, sellerA, "C2CL-OLD-A", "1000.00", "700.00", "active")
	f.insertLegacyListing(t, sellerB, "C2CL-OLD-B", "1000.00", "700.00", "active")

	rep := runMigrationDryAudit(f)
	if rep.ActiveListingCount != 2 {
		t.Fatalf("active listings want 2, got %d", rep.ActiveListingCount)
	}
	if len(rep.Rows) != 2 {
		t.Fatalf("rows want 2, got %d", len(rep.Rows))
	}

	// sellerA: available=700, frozen=300, remaining=700 → 不 suspend
	// sellerB: available=50,  remaining=700 → WouldSuspend
	var rowA, rowB DryAuditRow
	for _, r := range rep.Rows {
		if r.ListingNo == "C2CL-OLD-A" {
			rowA = r
		}
		if r.ListingNo == "C2CL-OLD-B" {
			rowB = r
		}
	}
	if !rowA.SellerAvailable.Equal(mustDec("700.00")) {
		t.Errorf("rowA available want 700, got %s", rowA.SellerAvailable)
	}
	if !rowA.SellerFrozen.Equal(mustDec("300.00")) {
		t.Errorf("rowA frozen want 300 (old trade), got %s", rowA.SellerFrozen)
	}
	if rowA.WouldSuspend {
		t.Errorf("rowA should NOT suspend (available 700 >= remaining 700)")
	}
	if rowA.ExistingRef != "" {
		t.Errorf("rowA should have no listing freeze ref yet, got %s", rowA.ExistingRef)
	}
	if !rowB.WouldSuspend {
		t.Errorf("rowB should suspend (available 50 < remaining 700)")
	}
}

// ---------------------------------------------------------------------------
// 6.2 Migration Rules 验证
// ---------------------------------------------------------------------------

// TestMigrationRules_SufficientBalanceFreezes 余额够：available→frozen，建立 listing ref，listing 保持 active。
func TestMigrationRules_SufficientBalanceFreezes(t *testing.T) {
	f := newFixture(t)
	seller := uint(201)
	f.createUser(t, seller)
	f.setBalance(t, seller, "1000.00")
	// 旧模型：300 已被 active trade 冻结
	f.oldModelFreeze(t, seller, "300.00", "c2c_freeze:trade:TR-OLD-1")
	l := f.insertLegacyListing(t, seller, "C2CL-SUFF", "1000.00", "700.00", "active")

	beforeTotal := f.sumAllSellerTotal(t)

	stats, err := f.svc.MigrateListingsToListingFreeze()
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if stats.Frozen != 1 || stats.Suspended != 0 {
		t.Fatalf("stats want Frozen=1 Suspended=0, got %+v", stats)
	}

	// seller: available 1000-300(trade)-700(listing)=0, frozen=300+700=1000
	if got := f.getAvailable(t, seller); !got.Equal(mustDec("0.00")) {
		t.Errorf("available want 0, got %s", got)
	}
	if got := f.getFrozen(t, seller); !got.Equal(mustDec("1000.00")) {
		t.Errorf("frozen want 1000 (300 trade + 700 listing), got %s", got)
	}
	// listing 仍 active
	reloaded := f.getListing(t, l.ID)
	if reloaded.Status != "active" {
		t.Errorf("listing status want active, got %s", reloaded.Status)
	}
	// 建立了 c2c_freeze:listing:C2CL-SUFF 引用
	ref := "c2c_freeze:listing:C2CL-SUFF"
	txn, err := f.walletDB.GetTransactionByReference(ref)
	if err != nil || txn == nil {
		t.Fatalf("listing freeze ref %s missing: %v", ref, err)
	}
	if !txn.Amount.Decimal.Equal(mustDec("700.00")) {
		t.Errorf("listing freeze amount want 700, got %s", txn.Amount.Decimal)
	}
	// 资金守恒
	if after := f.sumAllSellerTotal(t); !after.Equal(beforeTotal) {
		t.Errorf("conservation broken: before=%s after=%s", beforeTotal, after)
	}
}

// TestMigrationRules_InsufficientBalanceSuspends 余额不够：listing 转 suspended，禁止负余额。
func TestMigrationRules_InsufficientBalanceSuspends(t *testing.T) {
	f := newFixture(t)
	seller := uint(301)
	f.createUser(t, seller)
	f.setBalance(t, seller, "50.00")
	l := f.insertLegacyListing(t, seller, "C2CL-POOR", "1000.00", "700.00", "active")

	stats, err := f.svc.MigrateListingsToListingFreeze()
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if stats.Suspended != 1 || stats.Frozen != 0 {
		t.Fatalf("stats want Suspended=1 Frozen=0, got %+v", stats)
	}

	// 钱包不动
	if got := f.getAvailable(t, seller); !got.Equal(mustDec("50.00")) {
		t.Errorf("available must stay 50 (no negative), got %s", got)
	}
	if got := f.getFrozen(t, seller); !got.Equal(mustDec("0.00")) {
		t.Errorf("frozen must stay 0, got %s", got)
	}
	// listing 转 suspended
	reloaded := f.getListing(t, l.ID)
	if reloaded.Status != "suspended" {
		t.Errorf("listing status want suspended, got %s", reloaded.Status)
	}
	// 不应有 listing freeze ref
	if txn, _ := f.walletDB.GetTransactionByReference("c2c_freeze:listing:C2CL-POOR"); txn != nil {
		t.Errorf("no listing freeze ref should be created on suspend")
	}
}

// TestMigrationRules_NoDoubleFreezeWithActiveTrade 已有旧 active Trade Freeze：
// 迁移只冻结 AvailableUSDT(remaining)，不重复冻结 trade 金额。
func TestMigrationRules_NoDoubleFreezeWithActiveTrade(t *testing.T) {
	f := newFixture(t)
	seller := uint(401)
	f.createUser(t, seller)
	f.setBalance(t, seller, "1000.00")
	// 旧模型两笔 active trade 各冻 200 → available=600, frozen=400
	f.oldModelFreeze(t, seller, "200.00", "c2c_freeze:trade:TR-1")
	f.oldModelFreeze(t, seller, "200.00", "c2c_freeze:trade:TR-2")
	// listing 原始 1000，已被两笔 trade committed 400 → remaining=600
	f.insertLegacyListing(t, seller, "C2CL-NODBL", "1000.00", "600.00", "active")

	if got := f.getAvailable(t, seller); !got.Equal(mustDec("600.00")) {
		t.Fatalf("precondition available want 600, got %s", got)
	}
	if got := f.getFrozen(t, seller); !got.Equal(mustDec("400.00")) {
		t.Fatalf("precondition frozen want 400, got %s", got)
	}

	stats, err := f.svc.MigrateListingsToListingFreeze()
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if stats.Frozen != 1 {
		t.Fatalf("stats want Frozen=1, got %+v", stats)
	}
	// 迁移只冻 remaining=600：available=0, frozen=400(trade)+600(listing)=1000
	if got := f.getAvailable(t, seller); !got.Equal(mustDec("0.00")) {
		t.Errorf("available want 0, got %s", got)
	}
	if got := f.getFrozen(t, seller); !got.Equal(mustDec("1000.00")) {
		t.Errorf("frozen want 1000 (NO double freeze of trade 400), got %s", got)
	}
	// trade 冻结 ledger 仍是 2 条，未被迁移追加
	if n := f.countLedger(t, seller, constants.WalletTxnTypeC2CFreeze); n != 3 { // 2 trade + 1 listing
		t.Errorf("freeze ledger count want 3 (2 trade + 1 listing), got %d", n)
	}
}

// TestMigrationRules_NoAccountSuspends seller 无钱包账户 → suspend，不崩溃。
func TestMigrationRules_NoAccountSuspends(t *testing.T) {
	f := newFixture(t)
	seller := uint(501)
	f.createUser(t, seller)
	// 不 setBalance → 无账户
	l := f.insertLegacyListing(t, seller, "C2CL_NOACC", "100.00", "100.00", "active")

	stats, err := f.svc.MigrateListingsToListingFreeze()
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if stats.Suspended != 1 {
		t.Fatalf("stats want Suspended=1, got %+v", stats)
	}
	reloaded := f.getListing(t, l.ID)
	if reloaded.Status != "suspended" {
		t.Errorf("status want suspended, got %s", reloaded.Status)
	}
}

// ---------------------------------------------------------------------------
// 6.3 资金守恒
// ---------------------------------------------------------------------------

// TestMigration_ConservationMultipleSellers 多卖家：迁移前后 sum(available+frozen) 不变。
func TestMigration_ConservationMultipleSellers(t *testing.T) {
	f := newFixture(t)
	// sellerA 充足
	f.createUser(t, uint(601))
	f.setBalance(t, uint(601), "1000.00")
	f.oldModelFreeze(t, uint(601), "300.00", "c2c_freeze:trade:TR-C1")
	f.insertLegacyListing(t, uint(601), "C2CL-CONS-A", "1000.00", "700.00", "active")
	// sellerB 不足 → suspend（资金不动）
	f.createUser(t, uint(602))
	f.setBalance(t, uint(602), "50.00")
	f.insertLegacyListing(t, uint(602), "C2CL-CONS-B", "1000.00", "700.00", "active")
	// sellerC paused 也被扫描
	f.createUser(t, uint(603))
	f.setBalance(t, uint(603), "200.00")
	f.insertLegacyListing(t, uint(603), "C2CL-CONS-C", "200.00", "200.00", "paused")

	before := f.sumAllSellerTotal(t)
	stats, err := f.svc.MigrateListingsToListingFreeze()
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if stats.Scanned != 3 || stats.Frozen != 2 || stats.Suspended != 1 {
		t.Fatalf("stats want Scanned=3 Frozen=2 Suspended=1, got %+v", stats)
	}
	after := f.sumAllSellerTotal(t)
	if !before.Equal(after) {
		t.Errorf("CONSERVATION FAIL: before=%s after=%s", before, after)
	}
}

// ---------------------------------------------------------------------------
// 6.4 幂等
// ---------------------------------------------------------------------------

// TestMigration_IdempotentRepeated 重复执行迁移：wallet.Freeze 按 reference 幂等，不重复冻结。
func TestMigration_IdempotentRepeated(t *testing.T) {
	f := newFixture(t)
	seller := uint(701)
	f.createUser(t, seller)
	f.setBalance(t, seller, "1000.00")
	f.insertLegacyListing(t, seller, "C2CL-IDEM", "1000.00", "600.00", "active")

	if _, err := f.svc.MigrateListingsToListingFreeze(); err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	frozenAfterFirst := f.getFrozen(t, seller)
	availAfterFirst := f.getAvailable(t, seller)
	nAfterFirst, sumAfterFirst := f.countListingFreezeRefs(t)

	// 第二次迁移：listing 仍 active，reference 已存在 → 幂等返回，不重复冻结
	stats2, err := f.svc.MigrateListingsToListingFreeze()
	if err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	// 第二次扫描仍会扫到这条 active listing，但 Freeze 命中既有 reference → 计 Frozen
	if stats2.Scanned != 1 {
		t.Errorf("second migrate scanned want 1, got %d", stats2.Scanned)
	}
	if got := f.getFrozen(t, seller); !got.Equal(frozenAfterFirst) {
		t.Errorf("frozen changed after re-migrate: before=%s after=%s", frozenAfterFirst, got)
	}
	if got := f.getAvailable(t, seller); !got.Equal(availAfterFirst) {
		t.Errorf("available changed after re-migrate: before=%s after=%s", availAfterFirst, got)
	}
	nAfterSecond, sumAfterSecond := f.countListingFreezeRefs(t)
	if nAfterSecond != nAfterFirst {
		t.Errorf("listing freeze ref count grew: %d -> %d", nAfterFirst, nAfterSecond)
	}
	if !sumAfterSecond.Equal(sumAfterFirst) {
		t.Errorf("listing freeze ref sum grew: %s -> %s", sumAfterFirst, sumAfterSecond)
	}
	// 恰好 1 条 listing freeze ledger
	if nAfterFirst != 1 {
		t.Errorf("want exactly 1 listing freeze ref, got %d", nAfterFirst)
	}
}

// TestMigration_SkippedZeroRemaining remaining<=0 直接跳过，不 freeze。
func TestMigration_SkippedZeroRemaining(t *testing.T) {
	f := newFixture(t)
	seller := uint(801)
	f.createUser(t, seller)
	f.setBalance(t, seller, "1000.00")
	// 已全部 committed/settled，remaining=0
	f.insertLegacyListing(t, seller, "C2CL-ZERO", "1000.00", "0.00", "active")

	stats, err := f.svc.MigrateListingsToListingFreeze()
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if stats.Skipped != 1 || stats.Frozen != 0 {
		t.Fatalf("stats want Skipped=1 Frozen=0, got %+v", stats)
	}
	if got := f.getFrozen(t, seller); !got.Equal(mustDec("0.00")) {
		t.Errorf("frozen want 0 (skipped), got %s", got)
	}
}
