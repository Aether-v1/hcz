package integrationtest

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/Aether-v1/hcz/internal/constants"
	"github.com/Aether-v1/hcz/internal/shared/money"

	affiliateapp "github.com/Aether-v1/hcz/internal/modules/affiliate/application"
	affiliatedomain "github.com/Aether-v1/hcz/internal/modules/affiliate/domain"
	affiliategormstore "github.com/Aether-v1/hcz/internal/modules/affiliate/infrastructure/gormstore"

	userdomain "github.com/Aether-v1/hcz/internal/modules/identity/user/domain"
	userstore "github.com/Aether-v1/hcz/internal/modules/identity/user/infrastructure/gormstore"

	walletapp "github.com/Aether-v1/hcz/internal/modules/wallet/application"
	walletdomain "github.com/Aether-v1/hcz/internal/modules/wallet/domain"
	walletgormstore "github.com/Aether-v1/hcz/internal/modules/wallet/infrastructure/gormstore"

	settingsapp "github.com/Aether-v1/hcz/internal/modules/settings/application"
	settingsintegration "github.com/Aether-v1/hcz/internal/modules/settings/schema/integration"

	"github.com/Aether-v1/hcz/internal/testkit/memorysettings"

	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// transferFixture 是划转专项测试的 SQLite 夹具（复用真实 gormstore + 真实 wallet service）。
type transferFixture struct {
	svc           *affiliateapp.Service
	db            *gorm.DB
	affiliateRepo *affiliategormstore.Store
	walletRepo    *walletgormstore.Store
	walletSvc     *walletapp.Service
}

func setupTransferTest(t *testing.T) *transferFixture {
	t.Helper()

	dsn := fmt.Sprintf("file:transfer_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&userdomain.User{},
		&affiliatedomain.Profile{},
		&affiliatedomain.Click{},
		&affiliatedomain.Commission{},
		&affiliatedomain.CommissionLedger{},
		&affiliatedomain.WithdrawRequest{},
		&affiliatedomain.Application{},
		&walletdomain.Account{},
		&walletdomain.Transaction{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	settingRepo := memorysettings.New()
	settingSvc := settingsapp.NewService(settingRepo)
	if _, err := settingSvc.UpdateAffiliateSetting(settingsintegration.AffiliateSetting{
		Enabled:           true,
		ConfirmDays:       0,
		MaxLevel:          2,
		LevelRates:        levelRates(map[int]float64{1: 10, 2: 5}),
		MinWithdrawAmount: 1,
		WithdrawChannels:  []string{"usdt"},
	}); err != nil {
		t.Fatalf("init affiliate setting: %v", err)
	}

	affiliateRepo := affiliategormstore.New(db)
	walletRepo := walletgormstore.New(db)
	walletSvc := walletapp.NewService(walletapp.Options{
		Repository:   walletRepo,
		Transactions: walletRepo,
	})
	svc := affiliateapp.NewService(affiliateRepo, userstore.New(db), nil, nil, settingSvc)
	svc.SetWalletService(walletSvc, walletRepo)

	return &transferFixture{
		svc: svc, db: db, affiliateRepo: affiliateRepo,
		walletRepo: walletRepo, walletSvc: walletSvc,
	}
}

// ---- 小工具 ----

func tdDec(s string) decimal.Decimal {
	d, err := decimal.NewFromString(s)
	if err != nil {
		panic("bad decimal: " + s)
	}
	return d.Round(2)
}

// insertLedger 直接插入一条 affiliate ledger（自动维护 BalanceAfter 快照）。
// 用于在不经过 HandleOrderCompleted 的情况下构造 credit/reversal/debt 等场景。
func (f *transferFixture) insertLedger(t *testing.T, profile affiliatedomain.Profile, typ string, amount string, ref string) affiliatedomain.CommissionLedger {
	t.Helper()
	amt := tdDec(amount)
	var last affiliatedomain.CommissionLedger
	bal := decimal.Zero
	if err := f.db.Where("affiliate_profile_id = ?", profile.ID).
		Order("id desc").First(&last).Error; err == nil {
		bal = last.BalanceAfter.Decimal
	}
	bal = bal.Add(amt).Round(2)
	row := affiliatedomain.CommissionLedger{
		AffiliateProfileID: profile.ID,
		BeneficiaryUserID:  profile.UserID,
		Type:               typ,
		Amount:             money.FromDecimal(amt),
		BalanceAfter:       money.FromDecimal(bal),
		Reference:          ref,
		CreatedAt:          time.Now(),
	}
	if err := f.db.Create(&row).Error; err != nil {
		t.Fatalf("insert ledger type=%s ref=%s: %v", typ, ref, err)
	}
	return row
}

func (f *transferFixture) walletAvail(t *testing.T, userID uint) decimal.Decimal {
	t.Helper()
	var acc walletdomain.Account
	if err := f.db.Where("user_id = ?", userID).First(&acc).Error; err != nil {
		return decimal.Zero
	}
	return acc.AvailableBalance.Decimal.Round(2)
}

func (f *transferFixture) countWalletTxn(t *testing.T, userID uint, typ string) int64 {
	t.Helper()
	var n int64
	f.db.Model(&walletdomain.Transaction{}).
		Where("user_id = ? AND type = ?", userID, typ).Count(&n)
	return n
}

func (f *transferFixture) countLedger(t *testing.T, profileID uint, typ string) int64 {
	t.Helper()
	var n int64
	f.db.Model(&affiliatedomain.CommissionLedger{}).
		Where("affiliate_profile_id = ? AND type = ?", profileID, typ).Count(&n)
	return n
}

func (f *transferFixture) sumLedger(t *testing.T, profileID uint, typ string) decimal.Decimal {
	t.Helper()
	var row struct {
		Total decimal.Decimal `gorm:"column:total"`
	}
	f.db.Model(&affiliatedomain.CommissionLedger{}).
		Where("affiliate_profile_id = ? AND type = ?", profileID, typ).
		Select("COALESCE(SUM(amount), 0) AS total").Scan(&row)
	return row.Total.Round(2)
}

func (f *transferFixture) netLedger(t *testing.T, profileID uint) decimal.Decimal {
	t.Helper()
	var row struct {
		Total decimal.Decimal `gorm:"column:total"`
	}
	f.db.Model(&affiliatedomain.CommissionLedger{}).
		Where("affiliate_profile_id = ?", profileID).
		Select("COALESCE(SUM(amount), 0) AS total").Scan(&row)
	return row.Total.Round(2)
}

// newActivePromoter 创建一个 active 推广用户及其 profile，并插入 approved 申请（划转资格真源）。
func (f *transferFixture) newActivePromoter(t *testing.T, email string, code string) (userdomain.User, affiliatedomain.Profile) {
	t.Helper()
	u := createAffiliateTestUser(t, f.db, email)
	p := createAffiliateTestProfile(t, f.db, u.ID, code, constants.AffiliateProfileStatusActive)
	createAffiliateTestApprovedApplication(t, f.db, u.ID)
	return u, p
}

// =========================================================================
// 1. TestTransfer_CommissionAvailable
// =========================================================================
func TestTransfer_CommissionAvailable(t *testing.T) {
	f := setupTransferTest(t)
	user, profile := f.newActivePromoter(t, "avail@test.com", "AVAIL0001")
	f.insertLedger(t, profile, constants.AffiliateLedgerTypeCredit, "10.00", "ref-credit-1")

	got, err := f.svc.GetAvailableTransferBalance(profile.ID)
	if err != nil {
		t.Fatalf("GetAvailableTransferBalance: %v", err)
	}
	if !got.Equal(tdDec("10.00")) {
		t.Fatalf("available want 10.00, got %s", got)
	}
	_ = user
}

// =========================================================================
// 2. TestTransfer_Partial
// =========================================================================
func TestTransfer_Partial(t *testing.T) {
	f := setupTransferTest(t)
	user, profile := f.newActivePromoter(t, "partial@test.com", "PARTL0001")
	f.insertLedger(t, profile, constants.AffiliateLedgerTypeCredit, "10.00", "ref-credit-p")

	ledger, txn, err := f.svc.TransferToWallet(user.ID, affiliateapp.TransferToWalletInput{Amount: tdDec("4.00")})
	if err != nil {
		t.Fatalf("transfer partial: %v", err)
	}
	if ledger == nil || txn == nil {
		t.Fatalf("expect ledger+txn, got nil")
	}

	// affiliate 余额净 = 10 - 4 = 6
	if net := f.netLedger(t, profile.ID); !net.Equal(tdDec("6.00")) {
		t.Fatalf("affiliate net want 6.00, got %s", net)
	}
	// wallet 增加 4
	if w := f.walletAvail(t, user.ID); !w.Equal(tdDec("4.00")) {
		t.Fatalf("wallet want 4.00, got %s", w)
	}
	// transfer_to_wallet ledger 一条，金额 -4
	if n := f.countLedger(t, profile.ID, constants.AffiliateLedgerTypeTransferToWallet); n != 1 {
		t.Fatalf("transfer ledger count want 1, got %d", n)
	}
	if s := f.sumLedger(t, profile.ID, constants.AffiliateLedgerTypeTransferToWallet); !s.Equal(tdDec("-4.00")) {
		t.Fatalf("transfer ledger sum want -4.00, got %s", s)
	}
}

// =========================================================================
// 3. TestTransfer_All
// =========================================================================
func TestTransfer_All(t *testing.T) {
	f := setupTransferTest(t)
	user, profile := f.newActivePromoter(t, "all@test.com", "ALL000001")
	f.insertLedger(t, profile, constants.AffiliateLedgerTypeCredit, "10.00", "ref-credit-all")

	if _, _, err := f.svc.TransferToWallet(user.ID, affiliateapp.TransferToWalletInput{All: true}); err != nil {
		t.Fatalf("transfer all: %v", err)
	}
	if net := f.netLedger(t, profile.ID); !net.Equal(tdDec("0.00")) {
		t.Fatalf("affiliate net want 0.00 after all, got %s", net)
	}
	if w := f.walletAvail(t, user.ID); !w.Equal(tdDec("10.00")) {
		t.Fatalf("wallet want 10.00 after all, got %s", w)
	}
	// 再划转应失败（可用=0）
	if _, _, err := f.svc.TransferToWallet(user.ID, affiliateapp.TransferToWalletInput{All: true}); err == nil {
		t.Fatalf("second all transfer must fail after balance exhausted")
	}
}

// =========================================================================
// 4. TestTransfer_AmountExceedsAvailable
// =========================================================================
func TestTransfer_AmountExceedsAvailable(t *testing.T) {
	f := setupTransferTest(t)
	user, profile := f.newActivePromoter(t, "over@test.com", "OVER00001")
	f.insertLedger(t, profile, constants.AffiliateLedgerTypeCredit, "5.00", "ref-credit-over")

	_, _, err := f.svc.TransferToWallet(user.ID, affiliateapp.TransferToWalletInput{Amount: tdDec("10.00")})
	if !errors.Is(err, affiliateapp.ErrTransferInsufficient) {
		t.Fatalf("want ErrTransferInsufficient, got %v", err)
	}
	if w := f.walletAvail(t, user.ID); !w.Equal(tdDec("0.00")) {
		t.Fatalf("wallet must remain 0, got %s", w)
	}
}

// =========================================================================
// 5. TestTransfer_ZeroAmount
// =========================================================================
func TestTransfer_ZeroAmount(t *testing.T) {
	f := setupTransferTest(t)
	user, profile := f.newActivePromoter(t, "zero@test.com", "ZERO00001")
	f.insertLedger(t, profile, constants.AffiliateLedgerTypeCredit, "10.00", "ref-credit-zero")

	_, _, err := f.svc.TransferToWallet(user.ID, affiliateapp.TransferToWalletInput{Amount: tdDec("0.00")})
	if !errors.Is(err, affiliateapp.ErrTransferAmountInvalid) {
		t.Fatalf("want ErrTransferAmountInvalid, got %v", err)
	}
}

// =========================================================================
// 6. TestTransfer_NegativeAmount
// =========================================================================
func TestTransfer_NegativeAmount(t *testing.T) {
	f := setupTransferTest(t)
	user, profile := f.newActivePromoter(t, "neg@test.com", "NEG000001")
	f.insertLedger(t, profile, constants.AffiliateLedgerTypeCredit, "10.00", "ref-credit-neg")

	_, _, err := f.svc.TransferToWallet(user.ID, affiliateapp.TransferToWalletInput{Amount: tdDec("-5.00")})
	if !errors.Is(err, affiliateapp.ErrTransferAmountInvalid) {
		t.Fatalf("want ErrTransferAmountInvalid, got %v", err)
	}
}

// =========================================================================
// 7. TestTransfer_DuplicateReference（连续两次划转各自产生独立 reference）
// =========================================================================
func TestTransfer_DuplicateReference(t *testing.T) {
	f := setupTransferTest(t)
	user, profile := f.newActivePromoter(t, "dup@test.com", "DUP000001")
	f.insertLedger(t, profile, constants.AffiliateLedgerTypeCredit, "10.00", "ref-credit-dup")

	l1, _, err := f.svc.TransferToWallet(user.ID, affiliateapp.TransferToWalletInput{Amount: tdDec("3.00")})
	if err != nil {
		t.Fatalf("first transfer: %v", err)
	}
	l2, _, err := f.svc.TransferToWallet(user.ID, affiliateapp.TransferToWalletInput{Amount: tdDec("3.00")})
	if err != nil {
		t.Fatalf("second transfer: %v", err)
	}
	if l1.Reference == "" || l2.Reference == "" {
		t.Fatalf("references must be non-empty: %q %q", l1.Reference, l2.Reference)
	}
	if l1.Reference == l2.Reference {
		t.Fatalf("two transfers must produce independent references, both=%q", l1.Reference)
	}
	if n := f.countLedger(t, profile.ID, constants.AffiliateLedgerTypeTransferToWallet); n != 2 {
		t.Fatalf("want 2 transfer ledgers, got %d", n)
	}
}

// =========================================================================
// 8. TestTransfer_WalletCreditExactlyOnce
// =========================================================================
func TestTransfer_WalletCreditExactlyOnce(t *testing.T) {
	f := setupTransferTest(t)
	user, profile := f.newActivePromoter(t, "oncew@test.com", "ONCEW0001")
	f.insertLedger(t, profile, constants.AffiliateLedgerTypeCredit, "10.00", "ref-credit-oncew")

	if _, _, err := f.svc.TransferToWallet(user.ID, affiliateapp.TransferToWalletInput{Amount: tdDec("4.00")}); err != nil {
		t.Fatalf("transfer: %v", err)
	}
	if n := f.countWalletTxn(t, user.ID, constants.WalletTxnTypeAffiliateTransferIn); n != 1 {
		t.Fatalf("want exactly 1 affiliate_transfer_in wallet txn, got %d", n)
	}
}

// =========================================================================
// 9. TestTransfer_AffiliateLedgerExactlyOnce
// =========================================================================
func TestTransfer_AffiliateLedgerExactlyOnce(t *testing.T) {
	f := setupTransferTest(t)
	user, profile := f.newActivePromoter(t, "oncel@test.com", "ONCEL0001")
	f.insertLedger(t, profile, constants.AffiliateLedgerTypeCredit, "10.00", "ref-credit-oncel")

	if _, _, err := f.svc.TransferToWallet(user.ID, affiliateapp.TransferToWalletInput{Amount: tdDec("4.00")}); err != nil {
		t.Fatalf("transfer: %v", err)
	}
	if n := f.countLedger(t, profile.ID, constants.AffiliateLedgerTypeTransferToWallet); n != 1 {
		t.Fatalf("want exactly 1 transfer_to_wallet ledger, got %d", n)
	}
}

// =========================================================================
// 10. TestTransfer_AfterReversal
// =========================================================================
func TestTransfer_AfterReversal(t *testing.T) {
	f := setupTransferTest(t)
	user, profile := f.newActivePromoter(t, "rev@test.com", "REV000001")
	f.insertLedger(t, profile, constants.AffiliateLedgerTypeCredit, "10.00", "ref-credit-rev")
	f.insertLedger(t, profile, constants.AffiliateLedgerTypeReversal, "-6.00", "ref-rev-1")

	// 可划转 = 10 - 6 = 4
	got, err := f.svc.GetAvailableTransferBalance(profile.ID)
	if err != nil {
		t.Fatalf("available: %v", err)
	}
	if !got.Equal(tdDec("4.00")) {
		t.Fatalf("available after reversal want 4.00, got %s", got)
	}
	if _, _, err := f.svc.TransferToWallet(user.ID, affiliateapp.TransferToWalletInput{Amount: tdDec("4.00")}); err != nil {
		t.Fatalf("transfer 4 after reversal: %v", err)
	}
	if w := f.walletAvail(t, user.ID); !w.Equal(tdDec("4.00")) {
		t.Fatalf("wallet want 4.00, got %s", w)
	}
}

// =========================================================================
// 11. TestTransfer_WithDebt
// =========================================================================
func TestTransfer_WithDebt(t *testing.T) {
	f := setupTransferTest(t)
	user, profile := f.newActivePromoter(t, "debt@test.com", "DEBT00001")
	f.insertLedger(t, profile, constants.AffiliateLedgerTypeCredit, "10.00", "ref-credit-debt")
	f.insertLedger(t, profile, constants.AffiliateLedgerTypeDebt, "-15.00", "ref-debt-1")

	// 净余额 = -5 < 0 → ErrTransferInDebt
	_, _, err := f.svc.TransferToWallet(user.ID, affiliateapp.TransferToWalletInput{Amount: tdDec("1.00")})
	if !errors.Is(err, affiliateapp.ErrTransferInDebt) {
		t.Fatalf("want ErrTransferInDebt, got %v", err)
	}
	if w := f.walletAvail(t, user.ID); !w.Equal(tdDec("0.00")) {
		t.Fatalf("wallet must remain 0, got %s", w)
	}
}

// =========================================================================
// 12. TestTransfer_DebtRecovery
// =========================================================================
func TestTransfer_DebtRecovery(t *testing.T) {
	f := setupTransferTest(t)
	user, profile := f.newActivePromoter(t, "recov@test.com", "RECOV0001")
	// DEBT=-10，新佣金=6 → 净=-4，可划转=0
	f.insertLedger(t, profile, constants.AffiliateLedgerTypeDebt, "-10.00", "ref-debt-rec")
	f.insertLedger(t, profile, constants.AffiliateLedgerTypeCredit, "6.00", "ref-credit-rec1")
	if got, _ := f.svc.GetAvailableTransferBalance(profile.ID); !got.Equal(tdDec("0.00")) {
		t.Fatalf("available after debt+6 want 0, got %s", got)
	}
	// 新佣金=5 → 净=1，可划转=1
	f.insertLedger(t, profile, constants.AffiliateLedgerTypeCredit, "5.00", "ref-credit-rec2")
	if got, _ := f.svc.GetAvailableTransferBalance(profile.ID); !got.Equal(tdDec("1.00")) {
		t.Fatalf("available after debt+11 want 1, got %s", got)
	}
	if _, _, err := f.svc.TransferToWallet(user.ID, affiliateapp.TransferToWalletInput{Amount: tdDec("1.00")}); err != nil {
		t.Fatalf("transfer recovered 1: %v", err)
	}
	if w := f.walletAvail(t, user.ID); !w.Equal(tdDec("1.00")) {
		t.Fatalf("wallet want 1.00, got %s", w)
	}
}

// =========================================================================
// 13. TestTransfer_RefundBeforeTransfer
// =========================================================================
func TestTransfer_RefundBeforeTransfer(t *testing.T) {
	f := setupTransferTest(t)
	user, profile := f.newActivePromoter(t, "rfb@test.com", "RFB000001")
	f.insertLedger(t, profile, constants.AffiliateLedgerTypeCredit, "10.00", "ref-credit-rfb")
	f.insertLedger(t, profile, constants.AffiliateLedgerTypeReversal, "-10.00", "ref-rev-rfb")

	// 可划转=0，划转必须失败
	if _, _, err := f.svc.TransferToWallet(user.ID, affiliateapp.TransferToWalletInput{All: true}); err == nil {
		t.Fatalf("transfer after full refund must fail")
	}
	if w := f.walletAvail(t, user.ID); !w.Equal(tdDec("0.00")) {
		t.Fatalf("wallet must remain 0, got %s", w)
	}
}

// =========================================================================
// 14. TestTransfer_RefundAfterTransfer
// =========================================================================
func TestTransfer_RefundAfterTransfer(t *testing.T) {
	f := setupTransferTest(t)
	user, profile := f.newActivePromoter(t, "rfa@test.com", "RFA000001")
	f.insertLedger(t, profile, constants.AffiliateLedgerTypeCredit, "10.00", "ref-credit-rfa")
	if _, _, err := f.svc.TransferToWallet(user.ID, affiliateapp.TransferToWalletInput{All: true}); err != nil {
		t.Fatalf("transfer: %v", err)
	}
	// 退款后创建 DEBT，wallet 历史入账不变，affiliate 净余额为负
	f.insertLedger(t, profile, constants.AffiliateLedgerTypeDebt, "-10.00", "ref-debt-rfa")

	if w := f.walletAvail(t, user.ID); !w.Equal(tdDec("10.00")) {
		t.Fatalf("wallet history must stay 10.00, got %s", w)
	}
	if n := f.countWalletTxn(t, user.ID, constants.WalletTxnTypeAffiliateTransferIn); n != 1 {
		t.Fatalf("wallet txn count must stay 1, got %d", n)
	}
	if net := f.netLedger(t, profile.ID); !net.Equal(tdDec("-10.00")) {
		t.Fatalf("affiliate net want -10.00 after debt, got %s", net)
	}
}

// =========================================================================
// 15. TestTransfer_WalletBalanceCorrect
// =========================================================================
func TestTransfer_WalletBalanceCorrect(t *testing.T) {
	f := setupTransferTest(t)
	user, profile := f.newActivePromoter(t, "wbc@test.com", "WBC000001")
	f.insertLedger(t, profile, constants.AffiliateLedgerTypeCredit, "10.00", "ref-credit-wbc")

	before := f.walletAvail(t, user.ID)
	const amount = "4.00"
	if _, _, err := f.svc.TransferToWallet(user.ID, affiliateapp.TransferToWalletInput{Amount: tdDec(amount)}); err != nil {
		t.Fatalf("transfer: %v", err)
	}
	after := f.walletAvail(t, user.ID)
	want := before.Add(tdDec(amount))
	if !after.Equal(want) {
		t.Fatalf("wallet after want before(%s)+%s=%s, got %s", before, amount, want, after)
	}
}

// =========================================================================
// 16. TestTransfer_WalletLedgerCorrect
// =========================================================================
func TestTransfer_WalletLedgerCorrect(t *testing.T) {
	f := setupTransferTest(t)
	user, profile := f.newActivePromoter(t, "wlc@test.com", "WLC000001")
	f.insertLedger(t, profile, constants.AffiliateLedgerTypeCredit, "10.00", "ref-credit-wlc")

	_, txn, err := f.svc.TransferToWallet(user.ID, affiliateapp.TransferToWalletInput{Amount: tdDec("4.00")})
	if err != nil {
		t.Fatalf("transfer: %v", err)
	}
	if txn.Type != constants.WalletTxnTypeAffiliateTransferIn {
		t.Fatalf("txn type want %s, got %s", constants.WalletTxnTypeAffiliateTransferIn, txn.Type)
	}
	if txn.Direction != constants.WalletTxnDirectionIn {
		t.Fatalf("txn direction want in, got %s", txn.Direction)
	}
	if !txn.Amount.Decimal.Equal(tdDec("4.00")) {
		t.Fatalf("txn amount want 4.00, got %s", txn.Amount.Decimal)
	}
	if txn.Reference == "" {
		t.Fatalf("txn reference must be non-empty")
	}
}

// =========================================================================
// 17. TestTransfer_IDOR
// =========================================================================
func TestTransfer_IDOR(t *testing.T) {
	f := setupTransferTest(t)
	userA, profileA := f.newActivePromoter(t, "ida@test.com", "IDA000001")
	f.insertLedger(t, profileA, constants.AffiliateLedgerTypeCredit, "10.00", "ref-credit-ida")
	// userB 没有推广档案
	userB := createAffiliateTestUser(t, f.db, "idb@test.com")

	// userB 尝试划转（自己没有 profile，也碰不到 userA 的佣金）
	_, _, err := f.svc.TransferToWallet(userB.ID, affiliateapp.TransferToWalletInput{Amount: tdDec("4.00")})
	if err == nil {
		t.Fatalf("IDOR: userB without profile must not be able to transfer")
	}
	// userA 的 wallet 不变
	if w := f.walletAvail(t, userA.ID); !w.Equal(tdDec("0.00")) {
		t.Fatalf("userA wallet must remain 0, got %s", w)
	}
	// userA 的 affiliate 净佣金不变
	if net := f.netLedger(t, profileA.ID); !net.Equal(tdDec("10.00")) {
		t.Fatalf("userA net must remain 10.00, got %s", net)
	}
}

// =========================================================================
// 18. TestTransfer_RollbackOnWalletFailure
// 强制 affiliate ledger 写入失败（注入唯一索引冲突），验证 wallet credit 随事务回滚。
// =========================================================================
func TestTransfer_RollbackOnWalletFailure(t *testing.T) {
	f := setupTransferTest(t)
	user, profile := f.newActivePromoter(t, "roll@test.com", "ROLL00001")
	f.insertLedger(t, profile, constants.AffiliateLedgerTypeCredit, "10.00", "ref-credit-roll")

	// 预置一条 transfer_to_wallet ledger（模拟已划转 4）。
	f.insertLedger(t, profile, constants.AffiliateLedgerTypeTransferToWallet, "-4.00", "ref-transfer-seed")

	// 注入 (profile_id, type) 唯一索引，使下一次 transfer_to_wallet INSERT 必然冲突失败。
	if err := f.db.Exec("CREATE UNIQUE INDEX idx_force_txn_fail ON affiliate_commission_ledgers (affiliate_profile_id, type)").Error; err != nil {
		t.Fatalf("create force-fail index: %v", err)
	}

	// 可划转 = totalEarned(credit 10) - settled 0 - transferred(4) = 6。划转 4 应先 wallet 入账，
	// 随后 affiliate ledger INSERT 撞唯一索引失败 → 整个事务回滚。
	_, _, err := f.svc.TransferToWallet(user.ID, affiliateapp.TransferToWalletInput{Amount: tdDec("4.00")})
	if err == nil {
		t.Fatalf("expect txn to fail after forced ledger error, got nil")
	}

	// wallet 必须回滚：余额仍 0，没有 affiliate_transfer_in 流水。
	if w := f.walletAvail(t, user.ID); !w.Equal(tdDec("0.00")) {
		t.Fatalf("wallet must rollback to 0, got %s", w)
	}
	if n := f.countWalletTxn(t, user.ID, constants.WalletTxnTypeAffiliateTransferIn); n != 0 {
		t.Fatalf("wallet txn must rollback to 0 rows, got %d", n)
	}
	// affiliate transfer_to_wallet ledger 仍只有预置的那一条。
	if n := f.countLedger(t, profile.ID, constants.AffiliateLedgerTypeTransferToWallet); n != 1 {
		t.Fatalf("transfer ledger must stay at seeded 1 row, got %d", n)
	}
}

// =========================================================================
// 19. TestWithdraw_Retired
// =========================================================================
func TestWithdraw_Retired(t *testing.T) {
	f := setupTransferTest(t)
	user, _ := f.newActivePromoter(t, "ret@test.com", "RET000001")

	if _, err := f.svc.ApplyWithdraw(user.ID, affiliateapp.WithdrawApplyInput{
		Amount: tdDec("1.00"), Channel: "usdt", Account: "acc",
	}); !errors.Is(err, affiliateapp.ErrWithdrawRetired) {
		t.Fatalf("ApplyWithdraw want ErrWithdrawRetired, got %v", err)
	}
	if _, err := f.svc.ReviewWithdraw(1, 1, constants.AffiliateWithdrawActionApprove, ""); !errors.Is(err, affiliateapp.ErrWithdrawRetired) {
		t.Fatalf("ReviewWithdraw want ErrWithdrawRetired, got %v", err)
	}
}

// =========================================================================
// 20. TestTransfer_History
// =========================================================================
func TestTransfer_History(t *testing.T) {
	f := setupTransferTest(t)
	user, profile := f.newActivePromoter(t, "hist@test.com", "HIST00001")
	f.insertLedger(t, profile, constants.AffiliateLedgerTypeCredit, "10.00", "ref-credit-hist")

	if _, _, err := f.svc.TransferToWallet(user.ID, affiliateapp.TransferToWalletInput{Amount: tdDec("4.00")}); err != nil {
		t.Fatalf("transfer: %v", err)
	}
	rows, total, err := f.svc.ListTransferHistory(user.ID, 1, 10)
	if err != nil {
		t.Fatalf("ListTransferHistory: %v", err)
	}
	if total != 1 || len(rows) != 1 {
		t.Fatalf("want 1 history row, got total=%d len=%d", total, len(rows))
	}
	if rows[0].Type != constants.AffiliateLedgerTypeTransferToWallet {
		t.Fatalf("history type want transfer_to_wallet, got %s", rows[0].Type)
	}
	if rows[0].Amount.Decimal.Abs().Equal(tdDec("4.00")) == false {
		t.Fatalf("history amount abs want 4.00, got %s", rows[0].Amount.Decimal)
	}
}

// appStatusOf 读取某用户最新推广申请状态（真源 = affiliate_applications）。
func (f *transferFixture) appStatusOf(t *testing.T, userID uint) string {
	t.Helper()
	app, err := f.affiliateRepo.GetLatestApplicationByUserID(userID)
	if err != nil {
		t.Fatalf("GetLatestApplicationByUserID: %v", err)
	}
	if app == nil {
		return constants.AffiliateAppStatusNotApplied
	}
	return app.Status
}

// countApplications 统计某用户的推广申请条数（用于验证划转不会新增申请）。
func (f *transferFixture) countApplications(t *testing.T, userID uint) int64 {
	t.Helper()
	var n int64
	f.db.Model(&affiliatedomain.Application{}).Where("user_id = ?", userID).Count(&n)
	return n
}

// =========================================================================
// 21. TestTransfer_MultipleAfterOneApproval
// 一次审批 = 永久划转资格：批准一次后可反复划转，划转绝不新增/修改申请。
// =========================================================================
func TestTransfer_MultipleAfterOneApproval(t *testing.T) {
	f := setupTransferTest(t)
	user, profile := f.newActivePromoter(t, "multi@test.com", "MULTI0001")
	f.insertLedger(t, profile, constants.AffiliateLedgerTypeCredit, "20.00", "ref-credit-multi")

	if got := f.appStatusOf(t, user.ID); got != constants.AffiliateAppStatusApproved {
		t.Fatalf("precondition: application want approved, got %s", got)
	}
	if n := f.countApplications(t, user.ID); n != 1 {
		t.Fatalf("precondition: want exactly 1 application, got %d", n)
	}

	for _, amt := range []string{"5.00", "5.00", "10.00"} {
		if _, _, err := f.svc.TransferToWallet(user.ID, affiliateapp.TransferToWalletInput{Amount: tdDec(amt)}); err != nil {
			t.Fatalf("transfer %s: %v", amt, err)
		}
		// 每次划转后申请状态必须仍是 approved（资格未被消耗）。
		if got := f.appStatusOf(t, user.ID); got != constants.AffiliateAppStatusApproved {
			t.Fatalf("after transfer %s: application must stay approved, got %s", amt, got)
		}
		// 划转绝不新增申请记录。
		if n := f.countApplications(t, user.ID); n != 1 {
			t.Fatalf("after transfer %s: application count must stay 1, got %d", amt, n)
		}
	}

	// 累计划转 20，affiliate 净额归零，钱包 +20，资金守恒。
	if net := f.netLedger(t, profile.ID); !net.Equal(tdDec("0.00")) {
		t.Fatalf("affiliate net want 0.00, got %s", net)
	}
	if w := f.walletAvail(t, user.ID); !w.Equal(tdDec("20.00")) {
		t.Fatalf("wallet want 20.00, got %s", w)
	}
	if s := f.sumLedger(t, profile.ID, constants.AffiliateLedgerTypeTransferToWallet); !s.Equal(tdDec("-20.00")) {
		t.Fatalf("transfer ledger sum want -20.00, got %s", s)
	}
	if n := f.countLedger(t, profile.ID, constants.AffiliateLedgerTypeTransferToWallet); n != 3 {
		t.Fatalf("transfer ledger count want 3, got %d", n)
	}
}

// =========================================================================
// 22. TestTransfer_DisableReenableNoReapplication
// Profile disabled 暂停划转但保留 approved 历史；重新 active 后自动恢复，无需重新申请。
// =========================================================================
func TestTransfer_DisableReenableNoReapplication(t *testing.T) {
	f := setupTransferTest(t)
	user, profile := f.newActivePromoter(t, "toggle@test.com", "TOGGL0001")
	f.insertLedger(t, profile, constants.AffiliateLedgerTypeCredit, "10.00", "ref-credit-toggle")

	// approved + active：首次划转成功。
	if _, _, err := f.svc.TransferToWallet(user.ID, affiliateapp.TransferToWalletInput{Amount: tdDec("4.00")}); err != nil {
		t.Fatalf("first transfer (active+approved): %v", err)
	}

	// 管理员禁用 profile：划转被拒。
	if _, err := f.svc.UpdateAffiliateProfileStatus(profile.ID, 1, constants.AffiliateProfileStatusDisabled); err != nil {
		t.Fatalf("disable profile: %v", err)
	}
	if _, _, err := f.svc.TransferToWallet(user.ID, affiliateapp.TransferToWalletInput{Amount: tdDec("1.00")}); err == nil {
		t.Fatalf("transfer must be denied while profile disabled")
	}
	// 禁用不影响申请状态：仍为 approved，且未新增申请。
	if got := f.appStatusOf(t, user.ID); got != constants.AffiliateAppStatusApproved {
		t.Fatalf("disabled profile: application must stay approved, got %s", got)
	}
	if n := f.countApplications(t, user.ID); n != 1 {
		t.Fatalf("disabled profile: application count must stay 1, got %d", n)
	}

	// 管理员重新启用 profile：划转自动恢复，无需重新申请。
	if _, err := f.svc.UpdateAffiliateProfileStatus(profile.ID, 1, constants.AffiliateProfileStatusActive); err != nil {
		t.Fatalf("re-enable profile: %v", err)
	}
	if _, _, err := f.svc.TransferToWallet(user.ID, affiliateapp.TransferToWalletInput{Amount: tdDec("3.00")}); err != nil {
		t.Fatalf("transfer after re-enable must succeed without reapplication: %v", err)
	}
	if got := f.appStatusOf(t, user.ID); got != constants.AffiliateAppStatusApproved {
		t.Fatalf("after re-enable: application must stay approved, got %s", got)
	}
	if n := f.countApplications(t, user.ID); n != 1 {
		t.Fatalf("after re-enable: application count must stay 1, got %d", n)
	}
	// 净额 10 - 4 - 3 = 3，钱包 +7。
	if net := f.netLedger(t, profile.ID); !net.Equal(tdDec("3.00")) {
		t.Fatalf("affiliate net want 3.00, got %s", net)
	}
	if w := f.walletAvail(t, user.ID); !w.Equal(tdDec("7.00")) {
		t.Fatalf("wallet want 7.00, got %s", w)
	}
}
