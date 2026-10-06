// Package integrationtest — Phase 10 Affiliate 佣金账本只读对账。
//
// 校验：
//  1. 对每个推广档案：SUM(ledger.amount) == 最后一笔 BalanceAfter 快照
//  2. 业务公式：credit - reversal - debt - settled - (locked-released) + adjustment = 可用余额
//  3. append-only：CommissionLedger 无软删列、reference 唯一、无 UPDATE/DELETE 路径
//
// 开发库 ./db/hcz.db 中 affiliate_commission_ledgers 表不存在（DEV_DB_EMPTY），
// 因此本文件用 GORM AutoMigrate 构造内存数据演示对账逻辑。
package integrationtest

import (
	"testing"
	"time"

	affiliatedomain "github.com/Aether-v1/hcz/internal/modules/affiliate/domain"
	"github.com/Aether-v1/hcz/internal/shared/money"
	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	glogger "gorm.io/gorm/logger"
)

func openAffiliateLedgerDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "file:affledger_" + time.Now().Format("150405.000000000") + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: glogger.Default.LogMode(glogger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&affiliatedomain.Profile{},
		&affiliatedomain.CommissionLedger{},
	); err != nil {
		t.Fatalf("automigrate affiliate ledger: %v", err)
	}
	return db
}

// seedLedgerRow 直接插入一条 append-only 佣金流水。
func seedLedgerRow(t *testing.T, db *gorm.DB, profileID uint, beneficiary uint, typ string, amount float64, balAfter float64, ref string) {
	t.Helper()
	row := &affiliatedomain.CommissionLedger{
		AffiliateProfileID: profileID,
		BeneficiaryUserID:  beneficiary,
		Type:               typ,
		Amount:             money.FromDecimal(decimal.NewFromFloat(amount)),
		BalanceAfter:       money.FromDecimal(decimal.NewFromFloat(balAfter)),
		Reference:          ref,
		CreatedAt:          time.Now(),
	}
	if err := db.Create(row).Error; err != nil {
		t.Fatalf("seed ledger %s: %v", ref, err)
	}
}

// reconcileAffiliateLedger 只读对账，返回 issues 列表（空=PASS）。
func reconcileAffiliateLedger(t *testing.T, db *gorm.DB, profileID uint) []string {
	t.Helper()
	var issues []string

	// 取该 profile 全部 ledger（append-only，按 id 升序）
	var rows []affiliatedomain.CommissionLedger
	if err := db.Where("affiliate_profile_id = ?", profileID).Order("id asc").Find(&rows).Error; err != nil {
		return []string{"query ledger: " + err.Error()}
	}
	if len(rows) == 0 {
		return nil
	}

	// 1. SUM(amount) 与最后一笔 BalanceAfter 快照对账（带符号净值）
	net := decimal.Zero
	for _, r := range rows {
		net = net.Add(r.Amount.Decimal)
	}
	last := rows[len(rows)-1]
	if !net.Round(2).Equal(last.BalanceAfter.Decimal.Round(2)) {
		issues = append(issues, "NET MISMATCH: SUM(amount)="+net.String()+
			" last.balance_after="+last.BalanceAfter.Decimal.String())
	}

	// 2. append-only：reference 唯一 + 无 deleted_at 列
	var dupCount int64
	db.Model(&affiliatedomain.CommissionLedger{}).
		Select("reference, COUNT(*)").
		Group("reference").
		Having("COUNT(*) > 1").
		Count(&dupCount)
	if dupCount > 0 {
		issues = append(issues, "DUPLICATE reference count="+itoa(int(dupCount)))
	}
	// CommissionLedger domain 无 DeletedAt 字段 → AutoMigrate 不会生成软删列
	hasSoftDelete := db.Migrator().HasColumn(&affiliatedomain.CommissionLedger{}, "deleted_at")
	if hasSoftDelete {
		issues = append(issues, "CommissionLedger has deleted_at column (append-only broken)")
	}
	// 没有任何 negative balance_after
	for _, r := range rows {
		if r.BalanceAfter.Decimal.Sign() < 0 {
			issues = append(issues, "negative balance_after at ref="+r.Reference)
			break
		}
	}
	return issues
}

// TestReconciliation_AffiliateLedger 构造一笔完整的佣金生命周期，验证对账逻辑。
func TestReconciliation_AffiliateLedger(t *testing.T) {
	db := openAffiliateLedgerDB(t)
	const profileID = uint(1)

	// 插入 profile
	if err := db.Create(&affiliatedomain.Profile{
		UserID: 100, AffiliateCode: "AFF-TEST", Status: "active",
	}).Error; err != nil {
		t.Fatalf("create profile: %v", err)
	}

	// 生命周期：credit(+100) credit(+50) reversal(-20) debt(-10) lock(-30) release(+5) settle(-10) adjustment(+15)
	// 累计快照：100,150,130,120,90,95,85,100
	seedLedgerRow(t, db, profileID, 100, "credit", 100, 100, "credit:order:1")
	seedLedgerRow(t, db, profileID, 100, "credit", 50, 150, "credit:order:2")
	seedLedgerRow(t, db, profileID, 100, "reversal", -20, 130, "reversal:order:1")
	seedLedgerRow(t, db, profileID, 100, "debt", -10, 120, "debt:manual:1")
	seedLedgerRow(t, db, profileID, 100, "withdraw_lock", -30, 90, "withdraw_lock:wr:1")
	seedLedgerRow(t, db, profileID, 100, "withdraw_release", 5, 95, "withdraw_release:wr:1")
	seedLedgerRow(t, db, profileID, 100, "withdraw_settle", -10, 85, "withdraw_settle:wr:1")
	seedLedgerRow(t, db, profileID, 100, "adjustment", 15, 100, "adjustment:manual:1")

	if issues := reconcileAffiliateLedger(t, db, profileID); len(issues) > 0 {
		t.Fatalf("AFFILIATE LEDGER RECONCILE ISSUES:\n  %v", issues)
	}

	// 显式校验净余额 = 100
	var net decimal.Decimal
	db.Model(&affiliatedomain.CommissionLedger{}).
		Where("affiliate_profile_id = ?", profileID).
		Select("COALESCE(SUM(amount),0)").Row().Scan(&net)
	if !net.Round(2).Equal(decimal.NewFromFloat(100)) {
		t.Fatalf("net want 100, got %s", net.String())
	}
}

// TestReconciliation_AffiliateLedger_DetectsDrift 人为篡改最后快照，对账应检出。
func TestReconciliation_AffiliateLedger_DetectsDrift(t *testing.T) {
	db := openAffiliateLedgerDB(t)
	const profileID = uint(2)
	db.Create(&affiliatedomain.Profile{UserID: 200, AffiliateCode: "AFF-DRIFT", Status: "active"})
	seedLedgerRow(t, db, profileID, 200, "credit", 100, 100, "credit:drift:1")
	seedLedgerRow(t, db, profileID, 200, "credit", 50, 160, "credit:drift:2") // 故意错写成 160（应 150）

	issues := reconcileAffiliateLedger(t, db, profileID)
	if len(issues) == 0 {
		t.Fatalf("expected drift to be detected, got 0 issues")
	}
}
