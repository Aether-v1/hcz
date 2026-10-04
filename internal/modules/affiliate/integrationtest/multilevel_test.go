package integrationtest

import (
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/Aether-v1/hcz/internal/constants"
	"github.com/Aether-v1/hcz/internal/shared/money"

	affiliateapp "github.com/Aether-v1/hcz/internal/modules/affiliate/application"
	affiliatecontract "github.com/Aether-v1/hcz/internal/modules/affiliate/contract"
	affiliatedomain "github.com/Aether-v1/hcz/internal/modules/affiliate/domain"
	affiliategormstore "github.com/Aether-v1/hcz/internal/modules/affiliate/infrastructure/gormstore"

	userdomain "github.com/Aether-v1/hcz/internal/modules/identity/user/domain"
	userstore "github.com/Aether-v1/hcz/internal/modules/identity/user/infrastructure/gormstore"
	orderdomain "github.com/Aether-v1/hcz/internal/modules/order/domain"

	settingsapp "github.com/Aether-v1/hcz/internal/modules/settings/application"
	settingsintegration "github.com/Aether-v1/hcz/internal/modules/settings/schema/integration"

	"github.com/Aether-v1/hcz/internal/testkit/memorysettings"

	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// fakeOrderReader 是多级别佣金测试用的最小订单读取端口。
type fakeOrderReader struct {
	orders map[uint]*orderdomain.Order
}

func (f *fakeOrderReader) GetByID(id uint) (*orderdomain.Order, error) {
	if f == nil {
		return nil, nil
	}
	if o, ok := f.orders[id]; ok {
		return o, nil
	}
	return nil, nil
}

// setupMultilevelTest 搭建一个带 USDT 订单源的多级别佣金测试环境。
func setupMultilevelTest(t *testing.T, setting settingsintegration.AffiliateSetting, dsnTag string) (*affiliateapp.Service, *gorm.DB, affiliatecontract.Store, *fakeOrderReader) {
	t.Helper()

	dsn := fmt.Sprintf("file:ml_%s_%d?mode=memory&cache=shared", dsnTag, time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&userdomain.User{}, &affiliatedomain.Profile{}, &affiliatedomain.Click{}, &affiliatedomain.Commission{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	settingRepo := memorysettings.New()
	settingSvc := settingsapp.NewService(settingRepo)
	if _, err := settingSvc.UpdateAffiliateSetting(setting); err != nil {
		t.Fatalf("init affiliate setting: %v", err)
	}

	affiliateRepo := affiliategormstore.New(db)
	orderReader := &fakeOrderReader{orders: map[uint]*orderdomain.Order{}}
	svc := affiliateapp.NewService(affiliateRepo, userstore.New(db), orderReader, nil, settingSvc)
	return svc, db, affiliateRepo, orderReader
}

// levelRates 构造固定长度 10 的层级费率；未在 rates 中出现的层级为 disabled/0。
func levelRates(rates map[int]float64) []settingsintegration.LevelRate {
	out := make([]settingsintegration.LevelRate, 10)
	for i := 1; i <= 10; i++ {
		rate, ok := rates[i]
		out[i-1] = settingsintegration.LevelRate{Level: i, Enabled: ok, Rate: rate}
	}
	return out
}

// buildChain 构造一条邀请链：users[0] 是最顶层祖先，users[n-1] 是最末端（下单用户）。
// users[i].inviter_id = users[i-1]。
func buildChain(t *testing.T, db *gorm.DB, n int, prefix string) []userdomain.User {
	t.Helper()
	users := make([]userdomain.User, 0, n)
	for i := 0; i < n; i++ {
		u := createAffiliateTestUser(t, db, prefix+"-"+itoa(i)+"@ml.test")
		users = append(users, u)
	}
	for i := 1; i < n; i++ {
		inviterID := users[i-1].ID
		if err := db.Model(&userdomain.User{}).Where("id = ?", users[i].ID).
			Update("inviter_id", inviterID).Error; err != nil {
			t.Fatalf("link inviter_id: %v", err)
		}
	}
	return users
}

// activateProfiles 为给定用户创建 active 的推广档案，返回与入参顺序一致的 profile 列表。
func activateProfiles(t *testing.T, db *gorm.DB, users ...userdomain.User) []affiliatedomain.Profile {
	t.Helper()
	out := make([]affiliatedomain.Profile, 0, len(users))
	for _, u := range users {
		p := createAffiliateTestProfile(t, db, u.ID, "CODEU"+itoa(int(u.ID)), constants.AffiliateProfileStatusActive)
		out = append(out, p)
	}
	return out
}

func newUSDTPaidOrder(id, userID uint, walletUSDT float64) *orderdomain.Order {
	return &orderdomain.Order{
		ID:               id,
		UserID:           userID,
		WalletPaidAmount: money.FromDecimal(decimal.NewFromFloat(walletUSDT)),
		TotalAmount:      money.FromDecimal(decimal.NewFromFloat(walletUSDT)),
	}
}

func commissionsForOrder(t *testing.T, repo affiliatecontract.Store, orderID uint) []affiliatedomain.Commission {
	t.Helper()
	rows, err := repo.ListCommissionsByOrder(orderID, nil)
	if err != nil {
		t.Fatalf("list commissions by order: %v", err)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Level < rows[j].Level })
	return rows
}

// expectCommission 校验某条佣金的关键字段。
func expectCommission(t *testing.T, c affiliatedomain.Commission, level int, beneficiaryID uint, amount float64) {
	t.Helper()
	if c.Level != level {
		t.Fatalf("expected level=%d, got %d", level, c.Level)
	}
	if c.BeneficiaryUserID != beneficiaryID {
		t.Fatalf("expected beneficiary=%d, got %d", beneficiaryID, c.BeneficiaryUserID)
	}
	if !c.CommissionAmount.Decimal.Equal(decimal.NewFromFloat(amount)) {
		t.Fatalf("expected commission amount=%.2f, got %s", amount, c.CommissionAmount.String())
	}
	if c.CommissionType != constants.AffiliateCommissionTypeOrder {
		t.Fatalf("expected commission_type=order, got %q", c.CommissionType)
	}
}

// ---------------------------------------------------------------------------
// A. 佣金生成
// ---------------------------------------------------------------------------

// 1. 单级链：max_level=1，仅生成 L1。
func TestHandleOrderCompleted_SingleLevel(t *testing.T) {
	setting := settingsintegration.AffiliateSetting{
		Enabled: true, MaxLevel: 1, LevelRates: levelRates(map[int]float64{1: 5}),
	}
	svc, db, repo, orders := setupMultilevelTest(t, setting, "single")

	chain := buildChain(t, db, 2, "single")        // [U1, U2(order)]
	activateProfiles(t, db, chain[0])              // U1 active
	order := newUSDTPaidOrder(1, chain[1].ID, 100) // U2 下单
	orders.orders[1] = order

	if err := svc.HandleOrderCompleted(1); err != nil {
		t.Fatalf("HandleOrderCompleted: %v", err)
	}
	rows := commissionsForOrder(t, repo, 1)
	if len(rows) != 1 {
		t.Fatalf("expected 1 commission, got %d: %+v", len(rows), rows)
	}
	expectCommission(t, rows[0], 1, chain[0].ID, 5)
}

// 2. 三级链 U1→U2→U3→U4(order)，生成 3 条，beneficiary 逐级上移。
func TestHandleOrderCompleted_ThreeLevels(t *testing.T) {
	setting := settingsintegration.AffiliateSetting{
		Enabled: true, MaxLevel: 3, LevelRates: levelRates(map[int]float64{1: 5, 2: 5, 3: 5}),
	}
	svc, db, repo, orders := setupMultilevelTest(t, setting, "three")

	chain := buildChain(t, db, 4, "three") // [U1,U2,U3,U4(order)]
	activateProfiles(t, db, chain[0], chain[1], chain[2])
	order := newUSDTPaidOrder(1, chain[3].ID, 100)
	orders.orders[1] = order

	if err := svc.HandleOrderCompleted(1); err != nil {
		t.Fatalf("HandleOrderCompleted: %v", err)
	}
	rows := commissionsForOrder(t, repo, 1)
	if len(rows) != 3 {
		t.Fatalf("expected 3 commissions, got %d: %+v", len(rows), rows)
	}
	// level1 -> U3, level2 -> U2, level3 -> U1
	expectCommission(t, rows[0], 1, chain[2].ID, 5)
	expectCommission(t, rows[1], 2, chain[1].ID, 5)
	expectCommission(t, rows[2], 3, chain[0].ID, 5)
}

// 3. 十级完整链，生成 10 条 level 1~10。
func TestHandleOrderCompleted_TenLevels(t *testing.T) {
	setting := settingsintegration.AffiliateSetting{
		Enabled: true, MaxLevel: 10, LevelRates: levelRates(map[int]float64{
			1: 1, 2: 1, 3: 1, 4: 1, 5: 1, 6: 1, 7: 1, 8: 1, 9: 1, 10: 1,
		}),
	}
	svc, db, repo, orders := setupMultilevelTest(t, setting, "ten")

	chain := buildChain(t, db, 11, "ten") // 10 inviters + order user
	activateProfiles(t, db, chain[0], chain[1], chain[2], chain[3], chain[4], chain[5], chain[6], chain[7], chain[8], chain[9])
	order := newUSDTPaidOrder(1, chain[10].ID, 1000)
	orders.orders[1] = order

	if err := svc.HandleOrderCompleted(1); err != nil {
		t.Fatalf("HandleOrderCompleted: %v", err)
	}
	rows := commissionsForOrder(t, repo, 1)
	if len(rows) != 10 {
		t.Fatalf("expected 10 commissions, got %d", len(rows))
	}
	for i, c := range rows {
		level := i + 1
		// 链上第 i 个收益人是 chain[9-i]（level1 = 直接上级 = chain[9]）
		beneficiary := chain[9-i].ID
		expectCommission(t, c, level, beneficiary, 10)
	}
}

// 4. 链有 10 级但 max_level=3，只生成 3 条。
func TestHandleOrderCompleted_MaxLevelLimit(t *testing.T) {
	setting := settingsintegration.AffiliateSetting{
		Enabled: true, MaxLevel: 3, LevelRates: levelRates(map[int]float64{1: 5, 2: 5, 3: 5, 4: 5, 5: 5, 6: 5, 7: 5, 8: 5, 9: 5, 10: 5}),
	}
	svc, db, repo, orders := setupMultilevelTest(t, setting, "maxlevel")

	chain := buildChain(t, db, 11, "maxlevel")
	activateProfiles(t, db, chain[0], chain[1], chain[2], chain[3], chain[4], chain[5], chain[6], chain[7], chain[8], chain[9])
	order := newUSDTPaidOrder(1, chain[10].ID, 100)
	orders.orders[1] = order

	if err := svc.HandleOrderCompleted(1); err != nil {
		t.Fatalf("HandleOrderCompleted: %v", err)
	}
	rows := commissionsForOrder(t, repo, 1)
	if len(rows) != 3 {
		t.Fatalf("expected 3 commissions (max_level=3), got %d", len(rows))
	}
	expectCommission(t, rows[0], 1, chain[9].ID, 5)
	expectCommission(t, rows[1], 2, chain[8].ID, 5)
	expectCommission(t, rows[2], 3, chain[7].ID, 5)
}

// 5. 中间层 inactive 跳过但不压缩层级：L2 仍为 level=2。
func TestHandleOrderCompleted_InactiveAffiliateSkippedButContinue(t *testing.T) {
	setting := settingsintegration.AffiliateSetting{
		Enabled: true, MaxLevel: 3, LevelRates: levelRates(map[int]float64{1: 5, 2: 5, 3: 5}),
	}
	svc, db, repo, orders := setupMultilevelTest(t, setting, "inactive")

	chain := buildChain(t, db, 4, "inactive") // [U1,U2,U3,U4(order)]
	activateProfiles(t, db, chain[0], chain[2])
	// U2 (chain[1]) 显式 disabled
	createAffiliateTestProfile(t, db, chain[1].ID, "CODEU"+itoa(int(chain[1].ID)), constants.AffiliateProfileStatusDisabled)

	order := newUSDTPaidOrder(1, chain[3].ID, 100)
	orders.orders[1] = order

	if err := svc.HandleOrderCompleted(1); err != nil {
		t.Fatalf("HandleOrderCompleted: %v", err)
	}
	rows := commissionsForOrder(t, repo, 1)
	if len(rows) != 2 {
		t.Fatalf("expected 2 commissions (L2 skipped), got %d: %+v", len(rows), rows)
	}
	// level1 = U3(chain[2]), level3 = U1(chain[0])，层级不压缩
	expectCommission(t, rows[0], 1, chain[2].ID, 5)
	expectCommission(t, rows[1], 3, chain[0].ID, 5)
}

// 6. 某级 rate=0 时该级不创建佣金。
func TestHandleOrderCompleted_RateZeroSkipsLevel(t *testing.T) {
	setting := settingsintegration.AffiliateSetting{
		Enabled: true, MaxLevel: 3, LevelRates: levelRates(map[int]float64{1: 5, 3: 5}), // L2 disabled
	}
	svc, db, repo, orders := setupMultilevelTest(t, setting, "ratezero")

	chain := buildChain(t, db, 4, "ratezero")
	activateProfiles(t, db, chain[0], chain[1], chain[2])
	order := newUSDTPaidOrder(1, chain[3].ID, 100)
	orders.orders[1] = order

	if err := svc.HandleOrderCompleted(1); err != nil {
		t.Fatalf("HandleOrderCompleted: %v", err)
	}
	rows := commissionsForOrder(t, repo, 1)
	if len(rows) != 2 {
		t.Fatalf("expected 2 commissions (L2 rate=0 skipped), got %d", len(rows))
	}
	expectCommission(t, rows[0], 1, chain[2].ID, 5)
	expectCommission(t, rows[1], 3, chain[0].ID, 5)
}

// 7. 计算出的佣金 < 0.01 USDT 时不创建。
func TestHandleOrderCompleted_CommissionBelowOneCent(t *testing.T) {
	setting := settingsintegration.AffiliateSetting{
		Enabled: true, MaxLevel: 1, LevelRates: levelRates(map[int]float64{1: 1}),
	}
	svc, db, repo, orders := setupMultilevelTest(t, setting, "belowcent")

	chain := buildChain(t, db, 2, "belowcent")
	activateProfiles(t, db, chain[0])
	// 0.20 * 1% = 0.002 -> round 0.00 < 0.01
	order := newUSDTPaidOrder(1, chain[1].ID, 0.20)
	orders.orders[1] = order

	if err := svc.HandleOrderCompleted(1); err != nil {
		t.Fatalf("HandleOrderCompleted: %v", err)
	}
	if rows := commissionsForOrder(t, repo, 1); len(rows) != 0 {
		t.Fatalf("expected 0 commissions (below 0.01), got %d", len(rows))
	}
}

// 8. settings.Enabled=false 时不生成任何佣金。
func TestHandleOrderCompleted_DisabledAffiliate(t *testing.T) {
	setting := settingsintegration.AffiliateSetting{
		Enabled: false, MaxLevel: 1, LevelRates: levelRates(map[int]float64{1: 5}),
	}
	svc, db, repo, orders := setupMultilevelTest(t, setting, "disabled")

	chain := buildChain(t, db, 2, "disabled")
	activateProfiles(t, db, chain[0])
	order := newUSDTPaidOrder(1, chain[1].ID, 100)
	orders.orders[1] = order

	if err := svc.HandleOrderCompleted(1); err != nil {
		t.Fatalf("HandleOrderCompleted: %v", err)
	}
	if rows := commissionsForOrder(t, repo, 1); len(rows) != 0 {
		t.Fatalf("expected 0 commissions when affiliate disabled, got %d", len(rows))
	}
}

// ---------------------------------------------------------------------------
// B. 幂等与防重
// ---------------------------------------------------------------------------

// 9. 同一订单重复调用 HandleOrderCompleted，不重复生成。
func TestHandleOrderCompleted_IdempotentRetry(t *testing.T) {
	setting := settingsintegration.AffiliateSetting{
		Enabled: true, MaxLevel: 3, LevelRates: levelRates(map[int]float64{1: 5, 2: 5, 3: 5}),
	}
	svc, db, repo, orders := setupMultilevelTest(t, setting, "idem")

	chain := buildChain(t, db, 4, "idem")
	activateProfiles(t, db, chain[0], chain[1], chain[2])
	order := newUSDTPaidOrder(1, chain[3].ID, 100)
	orders.orders[1] = order

	if err := svc.HandleOrderCompleted(1); err != nil {
		t.Fatalf("first: %v", err)
	}
	if err := svc.HandleOrderCompleted(1); err != nil {
		t.Fatalf("second: %v", err)
	}
	rows := commissionsForOrder(t, repo, 1)
	if len(rows) != 3 {
		t.Fatalf("expected 3 commissions after retry, got %d", len(rows))
	}
}

// 10. 唯一索引兜底：插入 (order,beneficiary,level,type) 重复行应失败。
func TestHandleOrderCompleted_SameBeneficiaryNoDuplicate(t *testing.T) {
	setting := settingsintegration.AffiliateSetting{
		Enabled: true, MaxLevel: 1, LevelRates: levelRates(map[int]float64{1: 5}),
	}
	svc, db, repo, orders := setupMultilevelTest(t, setting, "uniqueidx")

	chain := buildChain(t, db, 2, "uniqueidx")
	profiles := activateProfiles(t, db, chain[0])
	order := newUSDTPaidOrder(1, chain[1].ID, 100)
	orders.orders[1] = order

	if err := svc.HandleOrderCompleted(1); err != nil {
		t.Fatalf("HandleOrderCompleted: %v", err)
	}
	rows := commissionsForOrder(t, repo, 1)
	if len(rows) != 1 {
		t.Fatalf("expected 1 commission, got %d", len(rows))
	}

	// 直接插入一条与已存在佣金相同 (order_id, beneficiary_user_id, level, commission_type='order') 的行，
	// 应被唯一索引拦截。
	dupe := affiliatedomain.Commission{
		AffiliateProfileID: profiles[0].ID,
		OrderID:            1,
		CommissionType:     constants.AffiliateCommissionTypeOrder,
		BeneficiaryUserID:  chain[0].ID,
		Level:              1,
		BaseAmount:         money.FromDecimal(decimal.NewFromFloat(100)),
		RatePercent:        money.FromDecimal(decimal.NewFromFloat(5)),
		CommissionAmount:   money.FromDecimal(decimal.NewFromFloat(5)),
		Status:             constants.AffiliateCommissionStatusAvailable,
	}
	err := db.Create(&dupe).Error
	if err == nil {
		t.Fatalf("expected unique index violation on duplicate (order,beneficiary,level,type)")
	}
}

// ---------------------------------------------------------------------------
// C. Self / Cycle 防护
// ---------------------------------------------------------------------------

// 11. 自邀请（inviter 指向自己）不生成佣金，不 panic。
func TestHandleOrderCompleted_SelfInviteProtection(t *testing.T) {
	setting := settingsintegration.AffiliateSetting{
		Enabled: true, MaxLevel: 1, LevelRates: levelRates(map[int]float64{1: 5}),
	}
	svc, db, repo, orders := setupMultilevelTest(t, setting, "self")

	u := createAffiliateTestUser(t, db, "self@ml.test")
	self := u.ID
	if err := db.Model(&userdomain.User{}).Where("id = ?", u.ID).Update("inviter_id", self).Error; err != nil {
		t.Fatalf("set self inviter: %v", err)
	}
	activateProfiles(t, db, u)
	order := newUSDTPaidOrder(1, u.ID, 100)
	orders.orders[1] = order

	if err := svc.HandleOrderCompleted(1); err != nil {
		t.Fatalf("HandleOrderCompleted: %v", err)
	}
	if rows := commissionsForOrder(t, repo, 1); len(rows) != 0 {
		t.Fatalf("expected 0 commissions for self-invite, got %d", len(rows))
	}
}

// 12. 邀请链成环（U1↔U2），检测到 cycle 后停止向上，不无限循环。
func TestHandleOrderCompleted_CycleDetection(t *testing.T) {
	setting := settingsintegration.AffiliateSetting{
		Enabled: true, MaxLevel: 3, LevelRates: levelRates(map[int]float64{1: 5, 2: 5, 3: 5}),
	}
	svc, db, repo, orders := setupMultilevelTest(t, setting, "cycle")

	u1 := createAffiliateTestUser(t, db, "cycle-u1@ml.test")
	u2 := createAffiliateTestUser(t, db, "cycle-u2@ml.test")
	u3 := createAffiliateTestUser(t, db, "cycle-u3@ml.test")
	// U3.inviter=U2, U2.inviter=U1, U1.inviter=U2 (cycle U1<->U2)
	if err := db.Model(&userdomain.User{}).Where("id = ?", u3.ID).Update("inviter_id", u2.ID).Error; err != nil {
		t.Fatalf("link u3->u2: %v", err)
	}
	if err := db.Model(&userdomain.User{}).Where("id = ?", u2.ID).Update("inviter_id", u1.ID).Error; err != nil {
		t.Fatalf("link u2->u1: %v", err)
	}
	if err := db.Model(&userdomain.User{}).Where("id = ?", u1.ID).Update("inviter_id", u2.ID).Error; err != nil {
		t.Fatalf("link u1->u2: %v", err)
	}
	activateProfiles(t, db, u1, u2)

	order := newUSDTPaidOrder(1, u3.ID, 100)
	orders.orders[1] = order

	if err := svc.HandleOrderCompleted(1); err != nil {
		t.Fatalf("HandleOrderCompleted: %v", err)
	}
	// chain: [U2(level1), U1(level2)] then cycle U1->U2 visited -> stop
	rows := commissionsForOrder(t, repo, 1)
	if len(rows) != 2 {
		t.Fatalf("expected 2 commissions before cycle stop, got %d: %+v", len(rows), rows)
	}
	expectCommission(t, rows[0], 1, u2.ID, 5)
	expectCommission(t, rows[1], 2, u1.ID, 5)
}

// ---------------------------------------------------------------------------
// D. 退款反冲
// ---------------------------------------------------------------------------

// seedThreeLevelCommissions 构造 3 条各 5 USDT 的 available 佣金，返回订单与佣金。
func seedThreeLevelCommissions(t *testing.T) (*affiliateapp.Service, affiliatecontract.Store, *orderdomain.Order) {
	setting := settingsintegration.AffiliateSetting{
		Enabled: true, MaxLevel: 3, LevelRates: levelRates(map[int]float64{1: 5, 2: 5, 3: 5}),
	}
	svc, db, repo, orders := setupMultilevelTest(t, setting, "refund")
	chain := buildChain(t, db, 4, "refund")
	activateProfiles(t, db, chain[0], chain[1], chain[2])
	order := newUSDTPaidOrder(1, chain[3].ID, 100)
	orders.orders[1] = order
	if err := svc.HandleOrderCompleted(1); err != nil {
		t.Fatalf("seed commissions: %v", err)
	}
	if got := commissionsForOrder(t, repo, 1); len(got) != 3 {
		t.Fatalf("seed: expected 3 commissions, got %d", len(got))
	}
	return svc, repo, order
}

// 13. partial refund delta=20/original=100，每条佣金按比例扣减 5 -> 4。
func TestHandleOrderRefunded_PartialRefund_MultiLevel(t *testing.T) {
	svc, repo, order := seedThreeLevelCommissions(t)

	if err := svc.HandleOrderRefunded(repo, order, decimal.NewFromFloat(20), decimal.Zero, "partial"); err != nil {
		t.Fatalf("HandleOrderRefunded: %v", err)
	}
	rows := commissionsForOrder(t, repo, 1)
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(rows))
	}
	for _, c := range rows {
		if !c.CommissionAmount.Decimal.Equal(decimal.NewFromFloat(4)) {
			t.Fatalf("level %d: expected commission=4 after partial refund, got %s", c.Level, c.CommissionAmount.String())
		}
		if c.Status != constants.AffiliateCommissionStatusAvailable {
			t.Fatalf("level %d: expected still available, got %s", c.Level, c.Status)
		}
	}
}

// 14. full refund 后所有级别佣金归零且 rejected。
func TestHandleOrderRefunded_FullRefund_AllLevelsZero(t *testing.T) {
	svc, repo, order := seedThreeLevelCommissions(t)

	if err := svc.HandleOrderRefunded(repo, order, decimal.NewFromFloat(100), decimal.Zero, "full"); err != nil {
		t.Fatalf("HandleOrderRefunded: %v", err)
	}
	rows := commissionsForOrder(t, repo, 1)
	for _, c := range rows {
		if !c.CommissionAmount.Decimal.Equal(decimal.Zero) {
			t.Fatalf("level %d: expected commission=0 after full refund, got %s", c.Level, c.CommissionAmount.String())
		}
		if c.Status != constants.AffiliateCommissionStatusRejected {
			t.Fatalf("level %d: expected rejected, got %s", c.Level, c.Status)
		}
	}
}

// 15. 同一退款重复调用不重复扣减：全退后再次调用为 no-op。
func TestHandleOrderRefunded_RetryNoDuplicateReversal(t *testing.T) {
	svc, repo, order := seedThreeLevelCommissions(t)

	if err := svc.HandleOrderRefunded(repo, order, decimal.NewFromFloat(100), decimal.Zero, "full"); err != nil {
		t.Fatalf("first refund: %v", err)
	}
	// 再次重复调用（retry），不应再改动任何佣金。
	if err := svc.HandleOrderRefunded(repo, order, decimal.NewFromFloat(30), decimal.Zero, "full-retry"); err != nil {
		t.Fatalf("retry refund: %v", err)
	}
	rows := commissionsForOrder(t, repo, 1)
	for _, c := range rows {
		if !c.CommissionAmount.Decimal.Equal(decimal.Zero) {
			t.Fatalf("level %d: retry must not re-process, expected 0, got %s", c.Level, c.CommissionAmount.String())
		}
		if c.Status != constants.AffiliateCommissionStatusRejected {
			t.Fatalf("level %d: expected rejected after retry, got %s", c.Level, c.Status)
		}
	}
}

// 16. full refund 兜底归零，不留 0.01 以下残差。
func TestHandleOrderRefunded_NoRoundingResidue(t *testing.T) {
	setting := settingsintegration.AffiliateSetting{
		Enabled: true, MaxLevel: 1, LevelRates: levelRates(map[int]float64{1: 33.33}),
	}
	svc, db, repo, orders := setupMultilevelTest(t, setting, "residue")
	chain := buildChain(t, db, 2, "residue")
	activateProfiles(t, db, chain[0])
	order := newUSDTPaidOrder(1, chain[1].ID, 100)
	orders.orders[1] = order
	if err := svc.HandleOrderCompleted(1); err != nil {
		t.Fatalf("seed: %v", err)
	}
	// commission = 100 * 33.33% = 33.33
	rows := commissionsForOrder(t, repo, 1)
	if len(rows) != 1 || !rows[0].CommissionAmount.Decimal.Equal(decimal.NewFromFloat(33.33)) {
		t.Fatalf("seed: unexpected commissions: %+v", rows)
	}
	// 先部分退 33.33，再退剩余 -> 全退兜底
	if err := svc.HandleOrderRefunded(repo, order, decimal.NewFromFloat(33.33), decimal.Zero, "partial"); err != nil {
		t.Fatalf("partial refund: %v", err)
	}
	if err := svc.HandleOrderRefunded(repo, order, decimal.NewFromFloat(66.67), decimal.NewFromFloat(33.33), "full"); err != nil {
		t.Fatalf("final refund: %v", err)
	}
	rows = commissionsForOrder(t, repo, 1)
	if !rows[0].CommissionAmount.Decimal.Equal(decimal.Zero) {
		t.Fatalf("expected exact zero (no rounding residue), got %s", rows[0].CommissionAmount.String())
	}
	if rows[0].Status != constants.AffiliateCommissionStatusRejected {
		t.Fatalf("expected rejected, got %s", rows[0].Status)
	}
}

// ---------------------------------------------------------------------------
// E. 配置校验
// ---------------------------------------------------------------------------

// 17. enabled 层级费率之和 >100 被拒绝。
func TestAffiliateSetting_TotalRateExceeds100_Rejected(t *testing.T) {
	setting := settingsintegration.AffiliateSetting{
		Enabled: true, MaxLevel: 2, LevelRates: levelRates(map[int]float64{1: 60, 2: 60}),
	}
	if err := settingsintegration.ValidateAffiliateSetting(setting); err == nil {
		t.Fatalf("expected error when enabled rate sum > 100")
	}
}

// 18. max_level 越界被 Normalize 修正到 [1,10]。
func TestAffiliateSetting_MaxLevelOutOfRange(t *testing.T) {
	low := settingsintegration.NormalizeAffiliateSetting(settingsintegration.AffiliateSetting{MaxLevel: 0})
	if low.MaxLevel != 1 {
		t.Fatalf("expected max_level clamped to 1, got %d", low.MaxLevel)
	}
	high := settingsintegration.NormalizeAffiliateSetting(settingsintegration.AffiliateSetting{MaxLevel: 11})
	if high.MaxLevel != 10 {
		t.Fatalf("expected max_level clamped to 10, got %d", high.MaxLevel)
	}
}

// 19. LevelRates 长度不足时 Normalize 补全到 10。
func TestAffiliateSetting_LevelRatesNormalizedToLength10(t *testing.T) {
	n := settingsintegration.NormalizeAffiliateSetting(settingsintegration.AffiliateSetting{LevelRates: nil})
	if len(n.LevelRates) != 10 {
		t.Fatalf("expected LevelRates length 10, got %d", len(n.LevelRates))
	}
	for i, item := range n.LevelRates {
		if item.Level != i+1 {
			t.Fatalf("expected index %d -> level %d, got %d", i, i+1, item.Level)
		}
	}
}

// ---------------------------------------------------------------------------
// F. 历史数据兼容
// ---------------------------------------------------------------------------

// 20. 历史 commission level=1 可被正确读取。
func TestHistoricalCommission_LevelDefaultsTo1(t *testing.T) {
	setting := settingsintegration.AffiliateSetting{
		Enabled: true, MaxLevel: 1, LevelRates: levelRates(map[int]float64{1: 5}),
	}
	svc, db, repo, _ := setupMultilevelTest(t, setting, "hist")
	_ = svc

	c := affiliatedomain.Commission{
		AffiliateProfileID: 1,
		OrderID:            99,
		CommissionType:     constants.AffiliateCommissionTypeOrder,
		BeneficiaryUserID:  55,
		Level:              1,
		BaseAmount:         money.FromDecimal(decimal.NewFromFloat(100)),
		RatePercent:        money.FromDecimal(decimal.NewFromFloat(5)),
		CommissionAmount:   money.FromDecimal(decimal.NewFromFloat(5)),
		Status:             constants.AffiliateCommissionStatusAvailable,
	}
	if err := db.Create(&c).Error; err != nil {
		t.Fatalf("insert historical commission: %v", err)
	}
	rows, err := repo.ListCommissionsByOrder(99, nil)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 1 || rows[0].Level != 1 || rows[0].BeneficiaryUserID != 55 {
		t.Fatalf("unexpected historical commission readback: %+v", rows)
	}
}

// 21. 多级别 available commission 正确汇总到同一 affiliate profile（withdraw 不关心 level）。
func TestAffiliateWithdraw_AggregateAcrossLevels(t *testing.T) {
	setting := settingsintegration.AffiliateSetting{
		Enabled: true, MaxLevel: 2, LevelRates: levelRates(map[int]float64{1: 5, 2: 5}),
	}
	svc, db, repo, orders := setupMultilevelTest(t, setting, "agg")

	u1 := createAffiliateTestUser(t, db, "agg-u1@ml.test")
	u2 := createAffiliateTestUser(t, db, "agg-u2@ml.test")
	ua := createAffiliateTestUser(t, db, "agg-ua@ml.test")
	ub := createAffiliateTestUser(t, db, "agg-ub@ml.test")
	// Ua.inviter=U1 (U1 is L1 for order A)
	mustLink(t, db, ua.ID, u1.ID)
	// Ub.inviter=U2, U2.inviter=U1 (U1 is L2 for order B)
	mustLink(t, db, ub.ID, u2.ID)
	mustLink(t, db, u2.ID, u1.ID)

	p1 := createAffiliateTestProfile(t, db, u1.ID, "CODEU"+itoa(int(u1.ID)), constants.AffiliateProfileStatusActive)
	createAffiliateTestProfile(t, db, u2.ID, "CODEU"+itoa(int(u2.ID)), constants.AffiliateProfileStatusActive)

	orderA := newUSDTPaidOrder(1, ua.ID, 100)
	orderB := newUSDTPaidOrder(2, ub.ID, 100)
	orders.orders[1] = orderA
	orders.orders[2] = orderB

	if err := svc.HandleOrderCompleted(1); err != nil {
		t.Fatalf("order A: %v", err)
	}
	if err := svc.HandleOrderCompleted(2); err != nil {
		t.Fatalf("order B: %v", err)
	}

	// U1 的 profile 应汇总到 level=1 (orderA, 5) + level=2 (orderB, 5) = 10
	sum, err := repo.SumCommissionByProfile(p1.ID, []string{constants.AffiliateCommissionStatusAvailable}, false)
	if err != nil {
		t.Fatalf("sum: %v", err)
	}
	if !sum.Equal(decimal.NewFromFloat(10)) {
		t.Fatalf("expected aggregate across levels = 10, got %s", sum.String())
	}
}

// ---------------------------------------------------------------------------
// G. 货币
// ---------------------------------------------------------------------------

// 22. commission 基于 WalletPaidAmount(USDT)，不使用 site currency TotalAmount 或汇率。
func TestCommissionCurrencyIsUSDT(t *testing.T) {
	setting := settingsintegration.AffiliateSetting{
		Enabled: true, MaxLevel: 1, LevelRates: levelRates(map[int]float64{1: 5}),
	}
	svc, db, repo, orders := setupMultilevelTest(t, setting, "usdt")

	chain := buildChain(t, db, 2, "usdt")
	activateProfiles(t, db, chain[0])
	// site currency total=700 (CNY), but wallet paid=100 USDT.
	order := &orderdomain.Order{
		ID:               1,
		UserID:           chain[1].ID,
		TotalAmount:      money.FromDecimal(decimal.NewFromFloat(700)),
		WalletPaidAmount: money.FromDecimal(decimal.NewFromFloat(100)),
	}
	orders.orders[1] = order

	if err := svc.HandleOrderCompleted(1); err != nil {
		t.Fatalf("HandleOrderCompleted: %v", err)
	}
	rows := commissionsForOrder(t, repo, 1)
	if len(rows) != 1 {
		t.Fatalf("expected 1 commission, got %d", len(rows))
	}
	// BaseAmount 必须是 USDT 实付 100（不是 700），佣金 = 100*5% = 5（不是 35）。
	if !rows[0].BaseAmount.Decimal.Equal(decimal.NewFromFloat(100)) {
		t.Fatalf("expected base=100 USDT, got %s", rows[0].BaseAmount.String())
	}
	if !rows[0].CommissionAmount.Decimal.Equal(decimal.NewFromFloat(5)) {
		t.Fatalf("expected commission=5 USDT, got %s", rows[0].CommissionAmount.String())
	}
}

// ---------------------------------------------------------------------------
// 小工具
// ---------------------------------------------------------------------------

func mustLink(t *testing.T, db *gorm.DB, userID, inviterID uint) {
	t.Helper()
	if err := db.Model(&userdomain.User{}).Where("id = ?", userID).Update("inviter_id", inviterID).Error; err != nil {
		t.Fatalf("link inviter: %v", err)
	}
}

// 避免在本文件引入 strconv/strings 的额外差异；以下为极简实现。
func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var buf [12]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
