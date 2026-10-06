package integrationtest

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Aether-v1/hcz/internal/constants"

	affiliateapp "github.com/Aether-v1/hcz/internal/modules/affiliate/application"
	affiliatedomain "github.com/Aether-v1/hcz/internal/modules/affiliate/domain"
	affiliategormstore "github.com/Aether-v1/hcz/internal/modules/affiliate/infrastructure/gormstore"

	userdomain "github.com/Aether-v1/hcz/internal/modules/identity/user/domain"
	userstore "github.com/Aether-v1/hcz/internal/modules/identity/user/infrastructure/gormstore"
	orderdomain "github.com/Aether-v1/hcz/internal/modules/order/domain"

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

// financeTestEnv 资金安全测试环境
type financeTestEnv struct {
	svc         *affiliateapp.Service
	db          *gorm.DB
	affiliateRepo *affiliategormstore.Store
	walletRepo  *walletgormstore.Store
	walletSvc   *walletapp.Service
	orderReader *fakeOrderReader
	users       []userdomain.User
	profiles    []affiliatedomain.Profile
}

// setupFinanceTest 搭建完整的资金安全测试环境（含真实 wallet）
func setupFinanceTest(t *testing.T, rates map[int]float64) *financeTestEnv {
	t.Helper()

	dsn := fmt.Sprintf("file:finance_%d?mode=memory&cache=shared", time.Now().UnixNano())
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
		&walletdomain.Account{},
		&walletdomain.Transaction{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	setting := settingsintegration.AffiliateSetting{
		Enabled: true, MaxLevel: 3, LevelRates: levelRates(rates),
		ConfirmDays: 0, // 佣金直接 available
		MinWithdrawAmount: 1,
		WithdrawChannels: []string{"alipay", "usdt"},
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

	// 构建 4 人邀请链：users[0] 是顶层，users[3] 是下单用户
	users := buildChain(t, db, 4, "finance")
	profiles := activateProfiles(t, db, users[0], users[1], users[2])

	return &financeTestEnv{
		svc:           svc,
		db:            db,
		affiliateRepo: affiliateRepo,
		walletRepo:    walletRepo,
		walletSvc:     walletSvc,
		orderReader:   orderReader,
		users:         users,
		profiles:      profiles,
	}
}

// seedOrderWithCommissions 创建一个订单并生成佣金
func (env *financeTestEnv) seedOrderWithCommissions(t *testing.T, orderID uint, amount float64) {
	t.Helper()
	order := newUSDTPaidOrder(orderID, env.users[3].ID, amount)
	env.orderReader.orders[orderID] = order
	if err := env.svc.HandleOrderCompleted(orderID); err != nil {
		t.Fatalf("seed order commissions: %v", err)
	}
}

// l1User 返回 L1 邀请人（users[2]），即佣金最多的人
func (env *financeTestEnv) l1User() userdomain.User {
	return env.users[2]
}

// l1Profile 返回 L1 推广档案（profiles[2]）
func (env *financeTestEnv) l1Profile() affiliatedomain.Profile {
	return env.profiles[2]
}

// getCommissionNet 从 ledger 计算某 profile 的净佣金余额
func (env *financeTestEnv) getProfileNetBalance(t *testing.T, profileID uint) decimal.Decimal {
	t.Helper()
	net, err := env.affiliateRepo.SumLedgerByProfile(profileID, nil)
	if err != nil {
		t.Fatalf("sum ledger: %v", err)
	}
	return net.Round(2)
}

// getWalletBalance 获取用户钱包余额
func (env *financeTestEnv) getWalletBalance(t *testing.T, userID uint) decimal.Decimal {
	t.Helper()
	acct, err := env.walletRepo.GetAccountByUserID(userID)
	if err != nil {
		t.Fatalf("get wallet account: %v", err)
	}
	if acct == nil {
		return decimal.Zero
	}
	return acct.AvailableBalance.Decimal.Round(2)
}

// assertWithdrawRetired 验证独立提现已退休：ApplyWithdraw 必须返回 ErrWithdrawRetired，
// 且不得创建任何 withdraw_request 记录、不得向钱包入账。
func (env *financeTestEnv) assertWithdrawRetired(t *testing.T) {
	t.Helper()
	if _, err := env.svc.ApplyWithdraw(env.l1User().ID, affiliateapp.WithdrawApplyInput{
		Amount:  decimal.NewFromFloat(5),
		Channel: "alipay",
		Account: "test@alipay",
	}); !errors.Is(err, affiliateapp.ErrWithdrawRetired) {
		t.Fatalf("ApplyWithdraw want ErrWithdrawRetired, got %v", err)
	}

	var withdrawCount int64
	if err := env.db.Model(&affiliatedomain.WithdrawRequest{}).Count(&withdrawCount).Error; err != nil {
		t.Fatalf("count withdraw requests: %v", err)
	}
	if withdrawCount != 0 {
		t.Fatalf("retired withdraw must not create withdraw_request rows, got %d", withdrawCount)
	}

	if balance := env.getWalletBalance(t, env.l1User().ID); !balance.Equal(decimal.Zero) {
		t.Fatalf("retired withdraw must not credit wallet, got %s", balance.String())
	}
}

// ---------------------------------------------------------------------------
// 测试用例
// ---------------------------------------------------------------------------

// 1. 订单 completed 后 commission + CREDIT ledger 同时创建
func TestCommissionCreate_LedgerCreditRecord(t *testing.T) {
	env := setupFinanceTest(t, map[int]float64{1: 10, 2: 5, 3: 2})
	env.seedOrderWithCommissions(t, 1, 100)

	rows := commissionsForOrder(t, env.affiliateRepo, 1)
	if len(rows) != 3 {
		t.Fatalf("expected 3 commissions, got %d", len(rows))
	}

	// 检查每条 commission 都有对应的 CREDIT ledger
	for _, c := range rows {
		ledgers, err := env.affiliateRepo.ListLedgersByCommission(c.ID)
		if err != nil {
			t.Fatalf("list ledgers: %v", err)
		}
		if len(ledgers) != 1 {
			t.Fatalf("commission %d: expected 1 credit ledger, got %d", c.ID, len(ledgers))
		}
		if ledgers[0].Type != constants.AffiliateLedgerTypeCredit {
			t.Fatalf("commission %d: expected type=credit, got %s", c.ID, ledgers[0].Type)
		}
		if !ledgers[0].Amount.Decimal.Equal(c.CommissionAmount.Decimal) {
			t.Fatalf("commission %d: expected ledger amount=%s, got %s", c.ID, c.CommissionAmount.String(), ledgers[0].Amount.String())
		}
	}
}

// 2. 重复 HandleOrderCompleted 不重复创建
func TestDuplicateCommission_Idempotent(t *testing.T) {
	env := setupFinanceTest(t, map[int]float64{1: 10})
	env.seedOrderWithCommissions(t, 1, 100)

	if err := env.svc.HandleOrderCompleted(1); err != nil {
		t.Fatalf("second HandleOrderCompleted: %v", err)
	}

	rows := commissionsForOrder(t, env.affiliateRepo, 1)
	if len(rows) != 1 {
		t.Fatalf("expected 1 commission after duplicate, got %d", len(rows))
	}

	ledgers, err := env.affiliateRepo.ListLedgersByCommission(rows[0].ID)
	if err != nil {
		t.Fatalf("list ledgers: %v", err)
	}
	if len(ledgers) != 1 {
		t.Fatalf("expected 1 ledger after duplicate, got %d", len(ledgers))
	}
}

// 3. 全额退款创建 REVERSAL，净佣金=0，commission 金额不变
func TestFullRefund_ReversalLedger_NetZero(t *testing.T) {
	env := setupFinanceTest(t, map[int]float64{1: 10})
	env.seedOrderWithCommissions(t, 1, 100)

	rows := commissionsForOrder(t, env.affiliateRepo, 1)
	originalAmount := rows[0].CommissionAmount.Decimal

	order := env.orderReader.orders[1]
	if err := env.svc.HandleOrderRefunded(env.affiliateRepo, order, decimal.NewFromFloat(100), decimal.Zero, "full refund"); err != nil {
		t.Fatalf("HandleOrderRefunded: %v", err)
	}

	// commission amount 保持原始值不变
	rows = commissionsForOrder(t, env.affiliateRepo, 1)
	if !rows[0].CommissionAmount.Decimal.Equal(originalAmount) {
		t.Fatalf("expected commission amount immutable, original=%s, now=%s", originalAmount.String(), rows[0].CommissionAmount.String())
	}

	// 净余额为 0
	net := env.getProfileNetBalance(t, env.profiles[0].ID)
	if !net.Equal(decimal.Zero) {
		t.Fatalf("expected net balance=0 after full refund, got %s", net.String())
	}

	// 状态为 rejected
	if rows[0].Status != constants.AffiliateCommissionStatusRejected {
		t.Fatalf("expected status=rejected, got %s", rows[0].Status)
	}
}

// 4. 部分退款按比例冲正
func TestPartialRefund_ReversalByRemainingRatio(t *testing.T) {
	env := setupFinanceTest(t, map[int]float64{1: 5})
	env.seedOrderWithCommissions(t, 1, 100) // 佣金 5

	// 退 40%（40/100），应冲正 5 * 40/100 = 2
	order := env.orderReader.orders[1]
	if err := env.svc.HandleOrderRefunded(env.affiliateRepo, order, decimal.NewFromFloat(40), decimal.Zero, "partial 40%"); err != nil {
		t.Fatalf("HandleOrderRefunded: %v", err)
	}

	net := env.getProfileNetBalance(t, env.l1Profile().ID)
	expected := decimal.NewFromFloat(3) // 5 - 2 = 3
	if !net.Equal(expected) {
		t.Fatalf("expected net=3 after 40%% refund, got %s", net.String())
	}
}

// 5. 两次部分退款累计计算
func TestRepeatedRefund_CumulativeCalculation(t *testing.T) {
	env := setupFinanceTest(t, map[int]float64{1: 10})
	env.seedOrderWithCommissions(t, 1, 100) // 佣金 10

	order := env.orderReader.orders[1]

	// 第一次退 30
	if err := env.svc.HandleOrderRefunded(env.affiliateRepo, order, decimal.NewFromFloat(30), decimal.Zero, "first 30"); err != nil {
		t.Fatalf("first refund: %v", err)
	}
	net1 := env.getProfileNetBalance(t, env.l1Profile().ID)
	expected1 := decimal.NewFromFloat(7) // 10 - 10*30/100 = 7
	if !net1.Equal(expected1) {
		t.Fatalf("after first refund: expected net=7, got %s", net1.String())
	}

	// 第二次退 35（基于剩余 70）
	if err := env.svc.HandleOrderRefunded(env.affiliateRepo, order, decimal.NewFromFloat(35), decimal.NewFromFloat(30), "second 35"); err != nil {
		t.Fatalf("second refund: %v", err)
	}
	net2 := env.getProfileNetBalance(t, env.l1Profile().ID)
	// 第二次冲正 = 10 * 35/70 = 5
	// 总净余额 = 10 - 3 - 5 = 2
	expected2 := decimal.NewFromFloat(2)
	if !net2.Equal(expected2) {
		t.Fatalf("after second refund: expected net=2, got %s", net2.String())
	}
}

// 6. 多级佣金每级独立按比例冲正
func TestMultiLevelCommissionReversal_AllLevels(t *testing.T) {
	env := setupFinanceTest(t, map[int]float64{1: 10, 2: 5, 3: 2})
	env.seedOrderWithCommissions(t, 1, 100) // L1=10, L2=5, L3=2

	order := env.orderReader.orders[1]
	// 全额退款
	if err := env.svc.HandleOrderRefunded(env.affiliateRepo, order, decimal.NewFromFloat(100), decimal.Zero, "full"); err != nil {
		t.Fatalf("HandleOrderRefunded: %v", err)
	}

	for i, p := range env.profiles {
		net := env.getProfileNetBalance(t, p.ID)
		if !net.Equal(decimal.Zero) {
			t.Fatalf("level %d profile %d: expected net=0, got %s", i+1, p.ID, net.String())
		}
	}
}

// 7. 独立提现已退休：ApplyWithdraw 返回 ErrWithdrawRetired，不创建 withdraw_request / lock ledger / 钱包入账
func TestWithdrawRequest_CreatesLockLedger(t *testing.T) {
	env := setupFinanceTest(t, map[int]float64{1: 10})
	env.seedOrderWithCommissions(t, 1, 100) // 佣金 10（L1）

	env.assertWithdrawRetired(t)

	// 净佣金余额保持 10，未被锁定
	if net := env.getProfileNetBalance(t, env.l1Profile().ID); !net.Equal(decimal.NewFromFloat(10)) {
		t.Fatalf("expected net balance unchanged=10, got %s", net.String())
	}
}

// 8. 独立提现已退休：重复申请同样返回 ErrWithdrawRetired（不再有余额锁定概念）
func TestDuplicateWithdrawRequest_BlockedByLock(t *testing.T) {
	env := setupFinanceTest(t, map[int]float64{1: 10})
	env.seedOrderWithCommissions(t, 1, 100) // 佣金 10

	// 两次申请都应直接退休，不创建任何记录
	for i, amt := range []float64{8, 5} {
		if _, err := env.svc.ApplyWithdraw(env.l1User().ID, affiliateapp.WithdrawApplyInput{
			Amount:  decimal.NewFromFloat(amt),
			Channel: "alipay",
			Account: "test@alipay",
		}); !errors.Is(err, affiliateapp.ErrWithdrawRetired) {
			t.Fatalf("withdraw #%d want ErrWithdrawRetired, got %v", i, err)
		}
	}
	var withdrawCount int64
	if err := env.db.Model(&affiliatedomain.WithdrawRequest{}).Count(&withdrawCount).Error; err != nil {
		t.Fatalf("count withdraw requests: %v", err)
	}
	if withdrawCount != 0 {
		t.Fatalf("expected 0 withdraw requests, got %d", withdrawCount)
	}
}

// 9. 独立提现已退休：ReviewWithdraw 直接返回 ErrWithdrawRetired，不创建 release ledger
func TestRejectWithdraw_ReleaseLockLedger(t *testing.T) {
	env := setupFinanceTest(t, map[int]float64{1: 10})
	env.seedOrderWithCommissions(t, 1, 100)

	env.assertWithdrawRetired(t)

	// ReviewWithdraw（拒绝/批准）也已退休
	if _, err := env.svc.ReviewWithdraw(1, 1, constants.AffiliateWithdrawActionReject, "test reject"); !errors.Is(err, affiliateapp.ErrWithdrawRetired) {
		t.Fatalf("ReviewWithdraw want ErrWithdrawRetired, got %v", err)
	}
}

// 10. 独立提现已退休：PayWithdraw 返回 ErrWithdrawRetired，钱包不增加、无 settle ledger
func TestWithdrawPay_WalletCreditAndSettleLedger(t *testing.T) {
	env := setupFinanceTest(t, map[int]float64{1: 10})
	env.seedOrderWithCommissions(t, 1, 100)

	env.assertWithdrawRetired(t)

	// PayWithdraw 也已退休
	if _, err := env.svc.PayWithdraw(1, 1); !errors.Is(err, affiliateapp.ErrWithdrawRetired) {
		t.Fatalf("PayWithdraw want ErrWithdrawRetired, got %v", err)
	}

	// 钱包余额仍为 0
	if balance := env.getWalletBalance(t, env.l1User().ID); !balance.Equal(decimal.Zero) {
		t.Fatalf("expected wallet balance=0, got %s", balance.String())
	}
}

// 11. 独立提现已退休：重复 PayWithdraw 同样返回 ErrWithdrawRetired，绝不重复入账
func TestDuplicatePay_Idempotent_NoDoubleCredit(t *testing.T) {
	env := setupFinanceTest(t, map[int]float64{1: 10})
	env.seedOrderWithCommissions(t, 1, 100)

	env.assertWithdrawRetired(t)

	for i := 0; i < 2; i++ {
		if _, err := env.svc.PayWithdraw(1, 1); !errors.Is(err, affiliateapp.ErrWithdrawRetired) {
			t.Fatalf("PayWithdraw #%d want ErrWithdrawRetired, got %v", i, err)
		}
	}
	if balance := env.getWalletBalance(t, env.l1User().ID); !balance.Equal(decimal.Zero) {
		t.Fatalf("wallet must stay 0, got %s", balance.String())
	}
}

// 12. 独立提现已退休：没有任何佣金被标记为已出金，退款只产生 REVERSAL 而非 DEBT
func TestRefundAfterWithdrawPaid_CreatesDebt(t *testing.T) {
	env := setupFinanceTest(t, map[int]float64{1: 10})
	env.seedOrderWithCommissions(t, 1, 100)

	// 申请提现被退休拦截，不存在"已打款"状态
	env.assertWithdrawRetired(t)

	// 全额退款：佣金仍为 available（从未出金），应走 REVERSAL 而非 DEBT
	order := env.orderReader.orders[1]
	if err := env.svc.HandleOrderRefunded(env.affiliateRepo, order, decimal.NewFromFloat(100), decimal.Zero, "full refund"); err != nil {
		t.Fatalf("HandleOrderRefunded: %v", err)
	}

	rows := commissionsForOrder(t, env.affiliateRepo, 1)
	ledgers, err := env.affiliateRepo.ListLedgersByCommission(rows[0].ID)
	if err != nil {
		t.Fatalf("list ledgers: %v", err)
	}
	for _, l := range ledgers {
		if l.Type == constants.AffiliateLedgerTypeDebt {
			t.Fatalf("retired withdraw: must NOT create DEBT (no commission ever withdrawn), ledger=%+v", l)
		}
	}
	// 净余额为 0（reversal 冲正），不出现负数债务
	if net := env.getProfileNetBalance(t, env.l1Profile().ID); !net.Equal(decimal.Zero) {
		t.Fatalf("expected net=0 after reversal, got %s", net.String())
	}
}

// 13. 管理员调整创建 ADJUSTMENT ledger
func TestAdminAdjustment_CreatesAdjustmentLedger(t *testing.T) {
	env := setupFinanceTest(t, map[int]float64{1: 10})
	env.seedOrderWithCommissions(t, 1, 100) // 佣金 10

	// 管理员调整 + 5
	err := env.svc.AdminAdjustCommission(affiliateapp.AdminAdjustCommissionInput{
		ProfileID: env.l1Profile().ID,
		Amount:    decimal.NewFromFloat(5),
		Remark:    "manual adjustment",
		AdminID:   1,
	})
	if err != nil {
		t.Fatalf("AdminAdjustCommission: %v", err)
	}

	net := env.getProfileNetBalance(t, env.l1Profile().ID)
	expected := decimal.NewFromFloat(15) // 10 + 5 = 15
	if !net.Equal(expected) {
		t.Fatalf("expected net=15 after adjustment, got %s", net.String())
	}
}

// 14. commission amount 创建后不可变
func TestCommissionImmutable_NoAmountUpdateAfterCreate(t *testing.T) {
	env := setupFinanceTest(t, map[int]float64{1: 10})
	env.seedOrderWithCommissions(t, 1, 100)

	rows := commissionsForOrder(t, env.affiliateRepo, 1)
	originalAmount := rows[0].CommissionAmount.Decimal

	// 部分退款
	order := env.orderReader.orders[1]
	if err := env.svc.HandleOrderRefunded(env.affiliateRepo, order, decimal.NewFromFloat(50), decimal.Zero, "partial"); err != nil {
		t.Fatalf("HandleOrderRefunded: %v", err)
	}

	// 检查 commission amount 仍然是原始值
	rows = commissionsForOrder(t, env.affiliateRepo, 1)
	if !rows[0].CommissionAmount.Decimal.Equal(originalAmount) {
		t.Fatalf("commission amount mutated: original=%s, after refund=%s", originalAmount.String(), rows[0].CommissionAmount.String())
	}
}

// 15. 精度测试：0.01 级别无精度损失
func TestDecimalPrecision_NoFloatLoss(t *testing.T) {
	env := setupFinanceTest(t, map[int]float64{1: 33.33})
	env.seedOrderWithCommissions(t, 1, 100) // 佣金 33.33

	rows := commissionsForOrder(t, env.affiliateRepo, 1)
	if !rows[0].CommissionAmount.Decimal.Equal(decimal.NewFromFloat(33.33)) {
		t.Fatalf("expected commission=33.33, got %s", rows[0].CommissionAmount.String())
	}

	// 部分退款 1
	order := env.orderReader.orders[1]
	if err := env.svc.HandleOrderRefunded(env.affiliateRepo, order, decimal.NewFromFloat(1), decimal.Zero, "small refund"); err != nil {
		t.Fatalf("HandleOrderRefunded: %v", err)
	}

	net := env.getProfileNetBalance(t, env.l1Profile().ID)
	// 冲正 = 33.33 * 1/100 = 0.3333 → 0.33
	expected := decimal.NewFromFloat(33.00)
	if !net.Equal(expected) {
		t.Fatalf("expected net=33.00, got %s", net.String())
	}
}

// 16. 独立提现已退休：不存在 settle，wallet 与 affiliate ledger 均无提现相关变动
func TestWalletLedgerConsistency_AfterPay(t *testing.T) {
	env := setupFinanceTest(t, map[int]float64{1: 10})
	env.seedOrderWithCommissions(t, 1, 100)

	env.assertWithdrawRetired(t)
	if _, err := env.svc.PayWithdraw(1, 1); !errors.Is(err, affiliateapp.ErrWithdrawRetired) {
		t.Fatalf("PayWithdraw want ErrWithdrawRetired, got %v", err)
	}

	// wallet 余额 = 0（从未出金）
	if balance := env.getWalletBalance(t, env.l1User().ID); !balance.Equal(decimal.Zero) {
		t.Fatalf("wallet must be 0, got %s", balance.String())
	}
	// affiliate settle ledger 总额 = 0
	settled, err := env.affiliateRepo.SumLedgerByProfile(env.l1Profile().ID, []string{constants.AffiliateLedgerTypeWithdrawSettle})
	if err != nil {
		t.Fatalf("sum settle: %v", err)
	}
	if !settled.Equal(decimal.Zero) {
		t.Fatalf("expected no settle ledger, got %s", settled.String())
	}
}

// 17. 独立提现已退休：没有 pending 提现，PayWithdraw 直接返回退休错误
func TestRefundWhileWithdrawPending_ReducesAvailable_BlocksPay(t *testing.T) {
	env := setupFinanceTest(t, map[int]float64{1: 10})
	env.seedOrderWithCommissions(t, 1, 100)

	// 申请被退休拦截，不存在 pending 提现
	env.assertWithdrawRetired(t)

	// 订单退款正常冲正
	order := env.orderReader.orders[1]
	if err := env.svc.HandleOrderRefunded(env.affiliateRepo, order, decimal.NewFromFloat(100), decimal.Zero, "refund"); err != nil {
		t.Fatalf("HandleOrderRefunded: %v", err)
	}

	// PayWithdraw 已退休，返回 ErrWithdrawRetired（而非旧的 InsufficientAfterRefund）
	_, err := env.svc.PayWithdraw(1, 1)
	if !errors.Is(err, affiliateapp.ErrWithdrawRetired) {
		t.Fatalf("expected ErrWithdrawRetired, got %v", err)
	}
}

// 18. 独立提现已退休：不会产生 DEBT，新佣金正常叠加（无债务抵扣场景）
func TestNegativeDebtHandling_FutureCreditOffsets(t *testing.T) {
	env := setupFinanceTest(t, map[int]float64{1: 10})
	env.seedOrderWithCommissions(t, 1, 100)

	// 申请+支付均被退休拦截，不产生已出金，退款不会形成 DEBT
	env.assertWithdrawRetired(t)
	if _, err := env.svc.PayWithdraw(1, 1); !errors.Is(err, affiliateapp.ErrWithdrawRetired) {
		t.Fatalf("PayWithdraw want ErrWithdrawRetired, got %v", err)
	}

	// 订单全额退款 → REVERSAL，净余额归 0（不是负数 DEBT）
	order := env.orderReader.orders[1]
	if err := env.svc.HandleOrderRefunded(env.affiliateRepo, order, decimal.NewFromFloat(100), decimal.Zero, "refund"); err != nil {
		t.Fatalf("HandleOrderRefunded: %v", err)
	}
	if net := env.getProfileNetBalance(t, env.l1Profile().ID); !net.Equal(decimal.Zero) {
		t.Fatalf("expected net=0 (no debt), got %s", net.String())
	}

	// 新订单产生佣金 5，正常叠加为 5（无债务抵扣）
	env.seedOrderWithCommissions(t, 2, 50)
	netAfter := env.getProfileNetBalance(t, env.l1Profile().ID)
	if !netAfter.Equal(decimal.NewFromFloat(5)) {
		t.Fatalf("expected net=5 after new commission, got %s", netAfter.String())
	}
}

// 19. 独立提现已退休：申请/审核/打款全部返回 ErrWithdrawRetired，无状态流转
func TestWithdrawApprove_StatusTransition(t *testing.T) {
	env := setupFinanceTest(t, map[int]float64{1: 10})
	env.seedOrderWithCommissions(t, 1, 100)

	env.assertWithdrawRetired(t)

	if _, err := env.svc.ReviewWithdraw(1, 1, constants.AffiliateWithdrawActionApprove, ""); !errors.Is(err, affiliateapp.ErrWithdrawRetired) {
		t.Fatalf("ReviewWithdraw want ErrWithdrawRetired, got %v", err)
	}
	if _, err := env.svc.PayWithdraw(1, 1); !errors.Is(err, affiliateapp.ErrWithdrawRetired) {
		t.Fatalf("PayWithdraw want ErrWithdrawRetired, got %v", err)
	}
}

// 20. 独立提现已退休：调用 ApplyWithdraw 不影响 commission amount
func TestWithdrawDoesNotModifyCommissionAmount(t *testing.T) {
	env := setupFinanceTest(t, map[int]float64{1: 10})
	env.seedOrderWithCommissions(t, 1, 100)

	rows := commissionsForOrder(t, env.affiliateRepo, 1)
	originalAmount := rows[0].CommissionAmount.Decimal

	// 申请被退休拦截
	env.assertWithdrawRetired(t)

	// commission amount 不变
	rows = commissionsForOrder(t, env.affiliateRepo, 1)
	if !rows[0].CommissionAmount.Decimal.Equal(originalAmount) {
		t.Fatalf("commission amount mutated after withdraw: original=%s, now=%s", originalAmount.String(), rows[0].CommissionAmount.String())
	}
}

// ---------------------------------------------------------------------------
// 补充专项测试（L10 / refund-to-full / duplicate reversal / refund while approved / reconciliation / race）
// ---------------------------------------------------------------------------

// 21. L10 佣金：10 级邀请链全部生成佣金
func TestL10Commission_AllLevelsGenerated(t *testing.T) {
	setting := settingsintegration.AffiliateSetting{
		Enabled: true, MaxLevel: 10,
		LevelRates: levelRates(map[int]float64{1: 10, 2: 9, 3: 8, 4: 7, 5: 6, 6: 5, 7: 4, 8: 3, 9: 2, 10: 1}),
		ConfirmDays: 0, MinWithdrawAmount: 1,
		WithdrawChannels: []string{"usdt"},
	}
	svc, db, repo, orders := setupMultilevelTest(t, setting, "l10")

	// 11 人链：users[0] 顶层(L10)，users[10] 下单
	users := buildChain(t, db, 11, "l10")
	activateProfiles(t, db, users[:10]...) // 前 10 人开通推广

	order := newUSDTPaidOrder(1, users[10].ID, 100)
	orders.orders[1] = order
	if err := svc.HandleOrderCompleted(1); err != nil {
		t.Fatalf("HandleOrderCompleted: %v", err)
	}

	rows := commissionsForOrder(t, repo, 1)
	if len(rows) != 10 {
		t.Fatalf("expected 10 commissions (L1-L10), got %d", len(rows))
	}

	expectedRates := map[int]float64{1: 10, 2: 9, 3: 8, 4: 7, 5: 6, 6: 5, 7: 4, 8: 3, 9: 2, 10: 1}
	for _, c := range rows {
		expected := decimal.NewFromFloat(100).Mul(decimal.NewFromFloat(expectedRates[c.Level])).Div(decimal.NewFromInt(100)).Round(2)
		if !c.CommissionAmount.Decimal.Equal(expected) {
			t.Fatalf("level %d: expected commission=%s, got %s", c.Level, expected.String(), c.CommissionAmount.String())
		}
	}
}

// 22. 部分退款后退至全额：累计冲正后净佣金为 0
func TestRefundToFull_CumulativeNetZero(t *testing.T) {
	env := setupFinanceTest(t, map[int]float64{1: 10})
	env.seedOrderWithCommissions(t, 1, 100)

	order := env.orderReader.orders[1]

	if err := env.svc.HandleOrderRefunded(env.affiliateRepo, order, decimal.NewFromFloat(60), decimal.Zero, "first 60"); err != nil {
		t.Fatalf("first refund: %v", err)
	}
	net1 := env.getProfileNetBalance(t, env.l1Profile().ID)
	if !net1.Equal(decimal.NewFromFloat(4)) {
		t.Fatalf("after first refund: expected net=4, got %s", net1.String())
	}

	if err := env.svc.HandleOrderRefunded(env.affiliateRepo, order, decimal.NewFromFloat(40), decimal.NewFromFloat(60), "remaining 40"); err != nil {
		t.Fatalf("second refund: %v", err)
	}
	net2 := env.getProfileNetBalance(t, env.l1Profile().ID)
	if !net2.Equal(decimal.Zero) {
		t.Fatalf("after full refund: expected net=0, got %s", net2.String())
	}
}

// 23. 重复退款冲正幂等：同一 refund event 不产生重复 REVERSAL
func TestDuplicateReversal_Idempotent(t *testing.T) {
	env := setupFinanceTest(t, map[int]float64{1: 10})
	env.seedOrderWithCommissions(t, 1, 100)

	order := env.orderReader.orders[1]

	if err := env.svc.HandleOrderRefunded(env.affiliateRepo, order, decimal.NewFromFloat(30), decimal.Zero, "partial"); err != nil {
		t.Fatalf("first refund: %v", err)
	}
	net1 := env.getProfileNetBalance(t, env.l1Profile().ID)

	if err := env.svc.HandleOrderRefunded(env.affiliateRepo, order, decimal.NewFromFloat(30), decimal.Zero, "partial"); err != nil {
		t.Fatalf("duplicate refund: %v", err)
	}
	net2 := env.getProfileNetBalance(t, env.l1Profile().ID)

	if !net1.Equal(net2) {
		t.Fatalf("duplicate reversal changed balance: before=%s, after=%s", net1.String(), net2.String())
	}

	rows := commissionsForOrder(t, env.affiliateRepo, 1)
	ledgers, err := env.affiliateRepo.ListLedgersByCommission(rows[0].ID)
	if err != nil {
		t.Fatalf("list ledgers: %v", err)
	}
	reversalCount := 0
	for _, l := range ledgers {
		if l.Type == constants.AffiliateLedgerTypeReversal {
			reversalCount++
		}
	}
	if reversalCount != 1 {
		t.Fatalf("expected exactly 1 reversal ledger, got %d", reversalCount)
	}
}

// 24. 独立提现已退休：approve/pay 均返回 ErrWithdrawRetired
func TestRefundWhileWithdrawApproved_BlocksPay(t *testing.T) {
	env := setupFinanceTest(t, map[int]float64{1: 10})
	env.seedOrderWithCommissions(t, 1, 100)

	env.assertWithdrawRetired(t)

	if _, err := env.svc.ReviewWithdraw(1, 1, constants.AffiliateWithdrawActionApprove, ""); !errors.Is(err, affiliateapp.ErrWithdrawRetired) {
		t.Fatalf("ReviewWithdraw want ErrWithdrawRetired, got %v", err)
	}

	order := env.orderReader.orders[1]
	if err := env.svc.HandleOrderRefunded(env.affiliateRepo, order, decimal.NewFromFloat(100), decimal.Zero, "refund"); err != nil {
		t.Fatalf("HandleOrderRefunded: %v", err)
	}

	_, err := env.svc.PayWithdraw(1, 1)
	if !errors.Is(err, affiliateapp.ErrWithdrawRetired) {
		t.Fatalf("expected ErrWithdrawRetired, got %v", err)
	}
}

// 25. 资金对账：提现已退休，settled/lock 恒为 0，available = gross + reversal
func TestReconciliation_BalanceInvariant(t *testing.T) {
	env := setupFinanceTest(t, map[int]float64{1: 10, 2: 5})
	env.seedOrderWithCommissions(t, 1, 100)

	// 提现被退休拦截，不产生 settle/lock
	env.assertWithdrawRetired(t)

	order := env.orderReader.orders[1]
	if err := env.svc.HandleOrderRefunded(env.affiliateRepo, order, decimal.NewFromFloat(30), decimal.Zero, "partial 30"); err != nil {
		t.Fatalf("HandleOrderRefunded: %v", err)
	}

	profileID := env.l1Profile().ID
	gross, _ := env.affiliateRepo.SumLedgerByProfile(profileID, []string{constants.AffiliateLedgerTypeCredit})
	reversal, _ := env.affiliateRepo.SumLedgerByProfile(profileID, []string{constants.AffiliateLedgerTypeReversal})
	settled, _ := env.affiliateRepo.SumLedgerByProfile(profileID, []string{constants.AffiliateLedgerTypeWithdrawSettle})
	lockNet, _ := env.affiliateRepo.SumLedgerByProfile(profileID, []string{
		constants.AffiliateLedgerTypeWithdrawLock,
		constants.AffiliateLedgerTypeWithdrawRelease,
	})

	available := gross.Add(reversal).Add(settled).Add(lockNet).Round(2)
	// credit 10 + reversal(-3) = 7（无 settle/lock）
	expected := decimal.NewFromFloat(7)
	if !available.Equal(expected) {
		t.Fatalf("reconciliation failed: gross=%s reversal=%s settled=%s lock=%s => available=%s, expected %s",
			gross.String(), reversal.String(), settled.String(), lockNet.String(), available.String(), expected.String())
	}
	if !settled.Equal(decimal.Zero) || !lockNet.Equal(decimal.Zero) {
		t.Fatalf("retired withdraw must leave settled=0 and lock=0, got settled=%s lock=%s", settled.String(), lockNet.String())
	}
}

// 26. 独立提现已退休：PayWithdraw 恒返回退休错误，并发退款不导致任何钱包出金
func TestConcurrentRefundAndPay_NoOverpay(t *testing.T) {
	env := setupFinanceTest(t, map[int]float64{1: 10})
	env.seedOrderWithCommissions(t, 1, 100)

	// 申请被退休拦截，无提现单
	env.assertWithdrawRetired(t)

	order := env.orderReader.orders[1]

	var wg sync.WaitGroup
	wg.Add(2)
	var payErr, refundErr error
	go func() {
		defer wg.Done()
		_, payErr = env.svc.PayWithdraw(1, 1)
	}()
	go func() {
		defer wg.Done()
		refundErr = env.svc.HandleOrderRefunded(env.affiliateRepo, order, decimal.NewFromFloat(100), decimal.Zero, "concurrent refund")
	}()
	wg.Wait()

	// PayWithdraw 必须恒为退休错误，绝不出金
	if !errors.Is(payErr, affiliateapp.ErrWithdrawRetired) {
		t.Fatalf("PayWithdraw want ErrWithdrawRetired, got %v", payErr)
	}
	if refundErr != nil {
		t.Fatalf("concurrent refund error: %v", refundErr)
	}
	// 钱包始终为 0，无超发
	if walletBalance := env.getWalletBalance(t, env.l1User().ID); !walletBalance.Equal(decimal.Zero) {
		t.Fatalf("wallet must stay 0, got %s", walletBalance.String())
	}
}
