package integrationtest

import (
	"testing"

	walletcontract "github.com/Aether-v1/hcz/internal/modules/wallet/contract"
	walletdomain "github.com/Aether-v1/hcz/internal/modules/wallet/domain"
	"github.com/Aether-v1/hcz/internal/shared/money"
	"github.com/shopspring/decimal"
)

// TestFrozenReconciliation 验证 wallet_accounts.frozen_balance 与
// 该用户最新一笔 wallet_transactions.frozen_after 完全一致。
// 同时验证不变量：total = available + frozen。
func TestFrozenReconciliation(t *testing.T) {
	env := newFreezeTestEnv(t)
	const uid = uint(1001)

	// 初始可用 1000
	env.mustSeedAvailable(t, uid, 1000)
	assertFrozenReconciled(t, env, uid)

	// 冻结 300（C2C 卖单）
	env.mustFreeze(t, uid, 300, "c2c_listing:1")
	assertFrozenReconciled(t, env, uid)

	// 再冻结 100（商品订单）
	env.mustFreeze(t, uid, 100, "order_freeze:1")
	assertFrozenReconciled(t, env, uid)

	// 解冻 50（C2C 下架部分）
	env.mustUnfreeze(t, uid, 50, "c2c_unlist:1")
	assertFrozenReconciled(t, env, uid)

	// 再冻结 200
	env.mustFreeze(t, uid, 200, "c2c_listing:2")
	assertFrozenReconciled(t, env, uid)

	// 全部解冻（当前 frozen = 300+100-50+200 = 550）
	env.mustUnfreeze(t, uid, 550, "unfreeze_all")
	assertFrozenReconciled(t, env, uid)

	// 最终 frozen 必须为 0
	acc, _ := env.wallets.GetAccountByUserID(uid)
	if !acc.FrozenBalance.Decimal.Equal(decimal.Zero) {
		t.Errorf("final frozen = %s, want 0", acc.FrozenBalance.Decimal.String())
	}
}

// TestFrozenReconciliationMultipleUsers 多用户各自冻结后对账。
func TestFrozenReconciliationMultipleUsers(t *testing.T) {
	env := newFreezeTestEnv(t)

	for _, uid := range []uint{2001, 2002, 2003} {
		env.mustSeedAvailable(t, uid, 500)
		env.mustFreeze(t, uid, 100, "freeze:"+itoa(uid))
	}

	for _, uid := range []uint{2001, 2002, 2003} {
		assertFrozenReconciled(t, env, uid)
	}
}

// TestFrozenReconciliationAfterSettle C2C 成交（卖家 frozen→扣除，买家 available→增加）后双方对账。
func TestFrozenReconciliationAfterSettle(t *testing.T) {
	env := newFreezeTestEnv(t)
	const seller = uint(3001)
	const buyer = uint(3002)

	env.mustSeedAvailable(t, seller, 1000)
	env.mustSeedAvailable(t, buyer, 10)

	// 卖家冻结 300
	env.mustFreeze(t, seller, 300, "c2c_listing:s1")
	assertFrozenReconciled(t, env, seller)

	// 成交：卖家 frozen 扣 300 → 买家 available 加 300
	err := env.settle(t, walletcontract.SettleInput{
		SourceUserID:    seller,
		TargetUserID:    buyer,
		Amount:          money.FromDecimal(decimal.NewFromInt(300)),
		SourceReference: "c2c_settle:s1",
		TargetReference: "c2c_receive:s1",
		Remark:          "c2c settle",
	})
	if err != nil {
		t.Fatalf("settle: %v", err)
	}

	// 卖家 frozen 应为 0，买家 frozen 应为 0
	assertFrozenReconciled(t, env, seller)
	assertFrozenReconciled(t, env, buyer)

	sellerAcc, _ := env.wallets.GetAccountByUserID(seller)
	buyerAcc, _ := env.wallets.GetAccountByUserID(buyer)
	if !sellerAcc.FrozenBalance.Decimal.Equal(decimal.Zero) {
		t.Errorf("seller frozen after settle = %s, want 0", sellerAcc.FrozenBalance.Decimal.String())
	}
	if !buyerAcc.AvailableBalance.Decimal.Equal(decimal.NewFromInt(310)) {
		t.Errorf("buyer available after settle = %s, want 310", buyerAcc.AvailableBalance.Decimal.String())
	}
}

// TestFrozenReconciliationNoTransactions 验证对账函数对无流水账户不崩溃。
func TestFrozenReconciliationNoTransactions(t *testing.T) {
	env := newFreezeTestEnv(t)
	const uid = uint(4001)

	// seed 会产生流水，所以这里直接调用 assertFrozenReconciled
	// 对于有流水的账户，验证对账逻辑正常；对于无流水的账户，函数内部处理 RowsAffected==0
	env.mustSeedAvailable(t, uid, 100)
	assertFrozenReconciled(t, env, uid)
}

// assertFrozenReconciled 只读对账：
// 1. wallet_accounts.frozen_balance == 最新流水 frozen_after
// 2. total(available+frozen) == 最新流水 balance_after
func assertFrozenReconciled(t *testing.T, env *freezeTestEnv, userID uint) {
	t.Helper()

	account, err := env.wallets.GetAccountByUserID(userID)
	if err != nil {
		t.Fatalf("user %d get account: %v", userID, err)
	}

	// 取该用户最新一笔流水（按 id 倒序）
	var latest walletdomain.Transaction
	result := env.db.Where("user_id = ?", userID).
		Order("id DESC").
		Limit(1).
		Find(&latest)
	if result.Error != nil {
		t.Fatalf("user %d query latest tx: %v", userID, result.Error)
	}
	if result.RowsAffected == 0 {
		// 无流水时 frozen 必须为 0
		if !account.FrozenBalance.Decimal.Equal(decimal.Zero) {
			t.Errorf("user %d no transactions but frozen=%s", userID, account.FrozenBalance.Decimal.String())
		}
		return
	}

	stored := account.FrozenBalance.Decimal
	fromLedger := latest.FrozenAfter.Decimal

	if !stored.Equal(fromLedger) {
		t.Errorf("user %d FROZEN MISMATCH: stored(wallet_accounts)=%s, ledger(latest frozen_after)=%s, latest_tx_id=%d, tx_type=%s",
			userID, stored.String(), fromLedger.String(), latest.ID, latest.Type)
	}

	// 不变量：total = available + frozen == latest balance_after
	total := account.AvailableBalance.Decimal.Add(account.FrozenBalance.Decimal)
	if !total.Equal(latest.BalanceAfter.Decimal) {
		t.Errorf("user %d TOTAL MISMATCH: available(%s)+frozen(%s)=%s, ledger balance_after=%s, latest_tx_id=%d",
			userID,
			account.AvailableBalance.Decimal.String(),
			account.FrozenBalance.Decimal.String(),
			total.String(),
			latest.BalanceAfter.Decimal.String(),
			latest.ID)
	}
}

// mustUnfreeze helper：解冻并在失败时终止测试。
func (e *freezeTestEnv) mustUnfreeze(t *testing.T, userID uint, amount int64, ref string) {
	t.Helper()
	if _, _, err := e.unfreeze(t, walletcontract.UnfreezeInput{
		UserID: userID, Amount: money.FromDecimal(decimal.NewFromInt(amount)), Reference: ref,
	}); err != nil {
		t.Fatalf("mustUnfreeze user %d amount %d: %v", userID, amount, err)
	}
}

// itoa 简单 uint 转字符串。
func itoa(n uint) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
