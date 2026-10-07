package integrationtest

// application_gate_test.go — transfer-to-wallet 权限门回归：
// not_applied / pending / rejected / disabled 的用户都不得出金。
// 这些用户调用 TransferToWallet 必须被拒绝（ErrNotOpened）。

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/Aether-v1/hcz/internal/constants"

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

// setupAppGateTest 搭建"可出金服务"夹具，并额外迁移 affiliate_applications。
func setupAppGateTest(t *testing.T) (*affiliateapp.Service, *gorm.DB) {
	t.Helper()

	dsn := fmt.Sprintf("file:appgate_%d?mode=memory&cache=shared", time.Now().UnixNano())
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
		MaxLevel:          1,
		LevelRates:        levelRates(map[int]float64{1: 5}),
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
	return svc, db
}

// expectTransferBlocked 断言该用户的 transfer-to-wallet 被拒绝。
func expectTransferBlocked(t *testing.T, svc *affiliateapp.Service, userID uint) {
	t.Helper()
	req := affiliateapp.TransferToWalletInput{
		Amount: decimal.Zero, All: true,
	}
	if _, _, err := svc.TransferToWallet(userID, req); !errors.Is(err, affiliateapp.ErrNotOpened) {
		t.Fatalf("user=%d transfer-to-wallet must be blocked with ErrNotOpened, got %v", userID, err)
	}
}

// B21. TestPendingCannotTransferToWallet
func TestPendingCannotTransferToWallet(t *testing.T) {
	svc, db := setupAppGateTest(t)
	u := createAffiliateTestUser(t, db, "gate-pending@hcz.test")
	mustApply(t, svc, u.ID, "x") // pending, no profile

	expectTransferBlocked(t, svc, u.ID)
}

// B22. TestRejectedCannotTransferToWallet
func TestRejectedCannotTransferToWallet(t *testing.T) {
	svc, db := setupAppGateTest(t)
	u := createAffiliateTestUser(t, db, "gate-rejected@hcz.test")
	app := mustApply(t, svc, u.ID, "x")
	_ = svc.RejectApplication(app.ID, 1, "no") // rejected, no profile

	expectTransferBlocked(t, svc, u.ID)
}

// B23. TestDisabledCannotTransferToWallet
func TestDisabledCannotTransferToWallet(t *testing.T) {
	svc, db := setupAppGateTest(t)
	u := createAffiliateTestUser(t, db, "gate-disabled@hcz.test")
	createAffiliateTestProfile(t, db, u.ID, "GATEDISC01", constants.AffiliateProfileStatusDisabled)

	expectTransferBlocked(t, svc, u.ID)
}

// B24. TestNotAppliedCannotTransferToWallet
func TestNotAppliedCannotTransferToWallet(t *testing.T) {
	svc, db := setupAppGateTest(t)
	u := createAffiliateTestUser(t, db, "gate-new@hcz.test")
	// 无申请、无 profile。

	expectTransferBlocked(t, svc, u.ID)
}
