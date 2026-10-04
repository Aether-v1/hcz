package integrationtest

import (
	"errors"
	"testing"
	"time"

	withdrawalcontract "github.com/Aether-v1/hcz/internal/modules/walletwithdrawal/contract"
	"github.com/Aether-v1/hcz/internal/shared/money"
)

// TestCreateWithdrawal_NewUserInCooldownBlocked：注册未满 cooldown 小时，拒绝提现。
// 断言：返回 ErrNewUserCooldown，钱包不扣款，无提现单、无 ledger。
func TestCreateWithdrawal_NewUserInCooldownBlocked(t *testing.T) {
	f := newFixture(t)
	f.config.cfg.NewUserCooldownHours = 24
	f.setUserCreatedAt(t, 1, time.Now().Add(-1*time.Hour))
	f.setBalance(t, 1, "100.00")

	_, err := f.svc.CreateWithdrawal(withdrawalcontract.CreateWithdrawalInput{
		UserID:         1,
		Network:        "TRC20",
		Address:        testAddress,
		Amount:         money.FromDecimal(mustDec("50.00")),
		TOTPCode:       "123456",
		IdempotencyKey: "key-cooldown-block",
	})
	if !errors.Is(err, withdrawalcontract.ErrNewUserCooldown) {
		t.Fatalf("expected ErrNewUserCooldown, got %v", err)
	}
	// 钱包不扣款
	if !f.getBalance(t, 1).Equal(mustDec("100.00")) {
		t.Fatalf("balance should be unchanged 100.00, got %s", f.getBalance(t, 1))
	}
	// 无提现单、无 ledger
	if n := f.countWithdrawals(t, 1); n != 0 {
		t.Fatalf("expected 0 withdrawal records, got %d", n)
	}
	if n := f.countLedger(t, 1); n != 0 {
		t.Fatalf("expected 0 ledger records, got %d", n)
	}
}

// TestCreateWithdrawal_AfterCooldownAllowed：注册已满 cooldown 小时，正常提现。
func TestCreateWithdrawal_AfterCooldownAllowed(t *testing.T) {
	f := newFixture(t)
	f.config.cfg.NewUserCooldownHours = 24
	f.setUserCreatedAt(t, 1, time.Now().Add(-48*time.Hour))
	f.setBalance(t, 1, "100.00")

	w, err := f.svc.CreateWithdrawal(withdrawalcontract.CreateWithdrawalInput{
		UserID:         1,
		Network:        "TRC20",
		Address:        testAddress,
		Amount:         money.FromDecimal(mustDec("50.00")),
		TOTPCode:       "123456",
		IdempotencyKey: "key-cooldown-ok",
	})
	if err != nil {
		t.Fatalf("create withdrawal after cooldown: %v", err)
	}
	if w == nil || w.ID == 0 {
		t.Fatal("expected withdrawal created")
	}
	// 扣款成功：100 - 50 = 50
	if !f.getBalance(t, 1).Equal(mustDec("50.00")) {
		t.Fatalf("expected balance 50.00, got %s", f.getBalance(t, 1))
	}
	if n := f.countWithdrawals(t, 1); n != 1 {
		t.Fatalf("expected 1 withdrawal record, got %d", n)
	}
	if n := f.countLedger(t, 1); n != 1 {
		t.Fatalf("expected 1 ledger record, got %d", n)
	}
}

// TestCreateWithdrawal_CooldownDisabledFreshUserOK：cooldown=0 表示关闭，新注册用户也能提现。
func TestCreateWithdrawal_CooldownDisabledFreshUserOK(t *testing.T) {
	f := newFixture(t)
	f.config.cfg.NewUserCooldownHours = 0
	// 注册仅 1 分钟前
	f.setUserCreatedAt(t, 1, time.Now().Add(-1*time.Minute))
	f.setBalance(t, 1, "100.00")

	w, err := f.svc.CreateWithdrawal(withdrawalcontract.CreateWithdrawalInput{
		UserID:         1,
		Network:        "TRC20",
		Address:        testAddress,
		Amount:         money.FromDecimal(mustDec("50.00")),
		TOTPCode:       "123456",
		IdempotencyKey: "key-cooldown-off",
	})
	if err != nil {
		t.Fatalf("create withdrawal with cooldown disabled: %v", err)
	}
	if w == nil || w.ID == 0 {
		t.Fatal("expected withdrawal created")
	}
	if !f.getBalance(t, 1).Equal(mustDec("50.00")) {
		t.Fatalf("expected balance 50.00, got %s", f.getBalance(t, 1))
	}
}

// TestCreateWithdrawal_TOTPBeforeCooldown：TOTP 校验在 cooldown 之前。
// 错误 TOTP 必须先返回 TOTP 错误（而不是 cooldown）；正确 TOTP 才落到 cooldown 拒绝。
// 两种情况都不得扣款。
func TestCreateWithdrawal_TOTPBeforeCooldown(t *testing.T) {
	f := newFixture(t)
	f.config.cfg.NewUserCooldownHours = 24
	f.setUserCreatedAt(t, 1, time.Now().Add(-1*time.Hour))
	f.setBalance(t, 1, "100.00")

	// 错误 TOTP：应先被 TOTP 拦截，而非 cooldown
	_, err := f.svc.CreateWithdrawal(withdrawalcontract.CreateWithdrawalInput{
		UserID:         1,
		Network:        "TRC20",
		Address:        testAddress,
		Amount:         money.FromDecimal(mustDec("50.00")),
		TOTPCode:       "wrong",
		IdempotencyKey: "key-cd-totp-bad",
	})
	if !errors.Is(err, withdrawalcontract.ErrTOTPInvalid) {
		t.Fatalf("expected ErrTOTPInvalid before cooldown, got %v", err)
	}

	// 正确 TOTP：TOTP 通过后落到 cooldown 拒绝
	_, err = f.svc.CreateWithdrawal(withdrawalcontract.CreateWithdrawalInput{
		UserID:         1,
		Network:        "TRC20",
		Address:        testAddress,
		Amount:         money.FromDecimal(mustDec("50.00")),
		TOTPCode:       "123456",
		IdempotencyKey: "key-cd-totp-good",
	})
	if !errors.Is(err, withdrawalcontract.ErrNewUserCooldown) {
		t.Fatalf("expected ErrNewUserCooldown after valid TOTP, got %v", err)
	}

	// 全程不扣款、不写单
	if !f.getBalance(t, 1).Equal(mustDec("100.00")) {
		t.Fatalf("balance should remain 100.00, got %s", f.getBalance(t, 1))
	}
	if n := f.countWithdrawals(t, 1); n != 0 {
		t.Fatalf("expected 0 withdrawal records, got %d", n)
	}
	if n := f.countLedger(t, 1); n != 0 {
		t.Fatalf("expected 0 ledger records, got %d", n)
	}
}
