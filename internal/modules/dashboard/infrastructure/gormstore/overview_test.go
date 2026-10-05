package gormstore

import (
	"testing"
	"time"

	orderdomain "github.com/Aether-v1/hcz/internal/modules/order/domain"

	walletdomain "github.com/Aether-v1/hcz/internal/modules/wallet/domain"

	"github.com/Aether-v1/hcz/internal/constants"
	"github.com/Aether-v1/hcz/internal/shared/money"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func TestGetOverviewUsesOrderCreationWindowForPaidGMV(t *testing.T) {
	repo, db := setupDashboardRepositoryTest(t)
	now := time.Now().UTC().Truncate(time.Second)

	paidOutsideWindow := now.Add(24 * time.Hour)
	inWindowOrder := &orderdomain.Order{
		OrderNo:        "DJ-GMV-IN-WINDOW",
		UserID:         1,
		Status:         constants.OrderStatusPaid,
		Currency:       "CNY",
		OriginalAmount: money.FromDecimal(decimal.NewFromInt(100)),
		DiscountAmount: money.FromDecimal(decimal.Zero),
		TotalAmount:    money.FromDecimal(decimal.NewFromInt(100)),
		CreatedAt:      now,
		PaidAt:         &paidOutsideWindow,
	}
	if err := db.Create(inWindowOrder).Error; err != nil {
		t.Fatalf("create in-window order failed: %v", err)
	}

	paidInsideWindow := now
	outOfWindowOrder := &orderdomain.Order{
		OrderNo:        "DJ-GMV-OUT-WINDOW",
		UserID:         1,
		Status:         constants.OrderStatusPaid,
		Currency:       "CNY",
		OriginalAmount: money.FromDecimal(decimal.NewFromInt(60)),
		DiscountAmount: money.FromDecimal(decimal.Zero),
		TotalAmount:    money.FromDecimal(decimal.NewFromInt(60)),
		CreatedAt:      now.Add(-48 * time.Hour),
		PaidAt:         &paidInsideWindow,
	}
	if err := db.Create(outOfWindowOrder).Error; err != nil {
		t.Fatalf("create out-of-window order failed: %v", err)
	}

	deletedAt := now.Add(time.Minute)
	deletedInWindowOrder := &orderdomain.Order{
		OrderNo:        "DJ-GMV-DELETED-IN-WINDOW",
		UserID:         1,
		Status:         constants.OrderStatusPaid,
		Currency:       "CNY",
		OriginalAmount: money.FromDecimal(decimal.NewFromInt(999)),
		DiscountAmount: money.FromDecimal(decimal.Zero),
		TotalAmount:    money.FromDecimal(decimal.NewFromInt(999)),
		CreatedAt:      now,
		PaidAt:         &paidInsideWindow,
		DeletedAt:      &deletedAt,
	}
	if err := db.Create(deletedInWindowOrder).Error; err != nil {
		t.Fatalf("create deleted in-window order failed: %v", err)
	}

	overview, err := repo.GetOverview(now.Add(-time.Hour), now.Add(time.Hour))
	if err != nil {
		t.Fatalf("get overview failed: %v", err)
	}
	if overview.GMVPaid != 100 {
		t.Fatalf("gmv paid want 100 got %.2f", overview.GMVPaid)
	}
}

func setupWalletBalanceTest(t *testing.T) (*Store, *gorm.DB) {
	t.Helper()
	repo, db := setupDashboardRepositoryTest(t)
	if err := db.AutoMigrate(&walletdomain.Account{}); err != nil {
		t.Fatalf("migrate wallet account failed: %v", err)
	}
	return repo, db
}

func TestGetTotalUserBalance_SumsAvailablePlusFrozen(t *testing.T) {
	repo, db := setupWalletBalanceTest(t)

	accounts := []walletdomain.Account{
		{UserID: 1, AvailableBalance: money.FromDecimal(decimal.NewFromInt(100)), FrozenBalance: money.FromDecimal(decimal.NewFromInt(50))},
		{UserID: 2, AvailableBalance: money.FromDecimal(decimal.NewFromInt(200)), FrozenBalance: money.FromDecimal(decimal.NewFromInt(0))},
		{UserID: 3, AvailableBalance: money.FromDecimal(decimal.NewFromInt(0)), FrozenBalance: money.FromDecimal(decimal.NewFromInt(75))},
	}
	for i := range accounts {
		if err := db.Create(&accounts[i]).Error; err != nil {
			t.Fatalf("create wallet account failed: %v", err)
		}
	}

	total, err := repo.GetTotalUserBalance()
	if err != nil {
		t.Fatalf("GetTotalUserBalance failed: %v", err)
	}
	// 100+50 + 200+0 + 0+75 = 425
	if total != 425 {
		t.Fatalf("total balance want 425 got %.2f", total)
	}
}

func TestGetTotalUserBalance_EmptyTableReturnsZero(t *testing.T) {
	repo, _ := setupWalletBalanceTest(t)

	total, err := repo.GetTotalUserBalance()
	if err != nil {
		t.Fatalf("GetTotalUserBalance failed: %v", err)
	}
	if total != 0 {
		t.Fatalf("empty table total want 0 got %.2f", total)
	}
}

func TestGetTotalUserBalance_ExcludesDeletedAccounts(t *testing.T) {
	repo, db := setupWalletBalanceTest(t)
	now := time.Now()

	active := walletdomain.Account{
		UserID:           1,
		AvailableBalance: money.FromDecimal(decimal.NewFromInt(300)),
		FrozenBalance:    money.FromDecimal(decimal.NewFromInt(100)),
	}
	if err := db.Create(&active).Error; err != nil {
		t.Fatalf("create active account failed: %v", err)
	}
	deleted := walletdomain.Account{
		UserID:           2,
		AvailableBalance: money.FromDecimal(decimal.NewFromInt(9999)),
		FrozenBalance:    money.FromDecimal(decimal.NewFromInt(9999)),
		DeletedAt:        &now,
	}
	if err := db.Create(&deleted).Error; err != nil {
		t.Fatalf("create deleted account failed: %v", err)
	}

	total, err := repo.GetTotalUserBalance()
	if err != nil {
		t.Fatalf("GetTotalUserBalance failed: %v", err)
	}
	// only active account: 300+100 = 400
	if total != 400 {
		t.Fatalf("total balance want 400 (excludes deleted) got %.2f", total)
	}
}
