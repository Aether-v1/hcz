package application

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Aether-v1/hcz/internal/constants"
	totpapplication "github.com/Aether-v1/hcz/internal/modules/identity/totp/application"
	walletdomain "github.com/Aether-v1/hcz/internal/modules/wallet/domain"
	withdrawalcontract "github.com/Aether-v1/hcz/internal/modules/walletwithdrawal/contract"
	withdrawaldomain "github.com/Aether-v1/hcz/internal/modules/walletwithdrawal/domain"
	"github.com/Aether-v1/hcz/internal/shared/money"

	"github.com/shopspring/decimal"
)

// CreateWithdrawal 用户发起提现：申请即扣款，同一事务内行锁账户、写 ledger、写提现单。
func (s *Service) CreateWithdrawal(input withdrawalcontract.CreateWithdrawalInput) (*withdrawaldomain.Withdrawal, error) {
	// 1. 基础入参校验
	if input.UserID == 0 {
		return nil, withdrawalcontract.ErrWithdrawalNotFound
	}
	idempotencyKey := strings.TrimSpace(input.IdempotencyKey)
	if idempotencyKey == "" {
		return nil, withdrawalcontract.ErrIdempotencyKeyRequired
	}
	amount := input.Amount.Decimal.Round(2)
	if amount.LessThanOrEqual(decimal.Zero) {
		return nil, withdrawalcontract.ErrInvalidAmount
	}

	// 2. 读取配置
	cfg := s.loadConfig()
	if !cfg.Enabled {
		return nil, withdrawalcontract.ErrWithdrawalDisabled
	}
	network := strings.ToUpper(strings.TrimSpace(input.Network))
	if network == "" {
		network = cfg.Network
	}
	if network != cfg.Network {
		return nil, withdrawalcontract.ErrUnsupportedNetwork
	}
	address := strings.TrimSpace(input.Address)
	// 3. TRC20 地址本地校验
	if network == "TRC20" {
		if !IsValidTRC20Address(address) {
			return nil, withdrawalcontract.ErrInvalidAddress
		}
	} else {
		return nil, withdrawalcontract.ErrUnsupportedNetwork
	}
	if isAddressBlacklisted(cfg, address) {
		return nil, withdrawalcontract.ErrAddressBlacklisted
	}

	// 4. min/max 校验
	minAmount := parseConfigDecimal(cfg.MinAmount)
	maxAmount := parseConfigDecimal(cfg.MaxAmount)
	if amount.LessThan(minAmount) {
		return nil, withdrawalcontract.ErrAmountTooSmall
	}
	if amount.GreaterThan(maxAmount) {
		return nil, withdrawalcontract.ErrAmountTooLarge
	}

	// 5. TOTP 校验（fail-closed）
	if cfg.Require2FA {
		if err := s.verifyTOTP(input.UserID, input.TOTPCode); err != nil {
			return nil, err
		}
	}

	// 5.5 新用户冷却校验（扣款前）：cooldown=0 关闭；users 未注入时跳过（向后兼容）。
	// 放在 TOTP 之后、事务之前：被拒绝时 TOTP 已验证，但不扣款、不写提现单、不写 ledger。
	if cfg.NewUserCooldownHours > 0 && s.users != nil {
		user, err := s.users.GetByID(input.UserID)
		if err != nil {
			return nil, withdrawalcontract.ErrUserNotFound
		}
		if user != nil {
			if time.Since(user.CreatedAt) < time.Duration(cfg.NewUserCooldownHours)*time.Hour {
				return nil, withdrawalcontract.ErrNewUserCooldown
			}
		}
	}

	// 6. 手续费计算（服务端真源）
	fixedFee := parseConfigDecimal(cfg.FixedFee)
	percentageFee := parseConfigDecimal(cfg.PercentageFee)
	quote := calculateFee(money.FromDecimal(amount), fixedFee, percentageFee)

	// 7. 幂等预检：同一 idempotency_key 已存在则直接返回，不双扣
	reference := withdrawalReference(input.UserID, idempotencyKey)
	if existing, err := s.repo.GetWithdrawalByReference(reference); err != nil {
		return nil, err
	} else if existing != nil {
		return existing, nil
	}

	// 8. 事务内扣款 + 写提现单
	now := time.Now()
	var result *withdrawaldomain.Withdrawal
	err := s.uow.WithinTransaction(func(tx withdrawalcontract.Transaction) error {
		// 8a. 行锁钱包账户
		account, err := tx.Wallets().GetAccountByUserIDForUpdate(input.UserID)
		if err != nil {
			return err
		}
		if account == nil {
			// 账户不存在则视为余额 0，直接拒绝
			return withdrawalcontract.ErrInsufficientBalance
		}
		before := account.Balance.Decimal.Round(2)
		// 8b. 余额校验
		if before.LessThan(amount) {
			return withdrawalcontract.ErrInsufficientBalance
		}
		// 8c. 日限额 / 日笔数
		dayStart, _ := dayBounds(now)
		sumToday, err := tx.Withdrawals().SumUserActiveSince(input.UserID, dayStart)
		if err != nil {
			return err
		}
		dailyLimit := parseConfigDecimal(cfg.DailyLimit)
		if dailyLimit.GreaterThan(decimal.Zero) && sumToday.Add(amount).GreaterThan(dailyLimit) {
			return withdrawalcontract.ErrDailyLimitExceeded
		}
		if cfg.DailyCountLimit > 0 {
			countToday, err := tx.Withdrawals().CountUserActiveSince(input.UserID, dayStart)
			if err != nil {
				return err
			}
			if int(countToday) >= cfg.DailyCountLimit {
				return withdrawalcontract.ErrDailyCountExceeded
			}
		}
		// 8d. 首提限额
		firstMax := parseConfigDecimal(cfg.FirstWithdrawalMax)
		if firstMax.GreaterThan(decimal.Zero) {
			completedCount, err := tx.Withdrawals().CountUserCompletedBefore(input.UserID, now)
			if err != nil {
				return err
			}
			if completedCount == 0 && amount.GreaterThan(firstMax) {
				return withdrawalcontract.ErrFirstWithdrawalLimit
			}
		}

		// 8e. 余额扣款
		after := before.Sub(amount).Round(2)
		account.Balance = money.FromDecimal(after)
		account.UpdatedAt = now
		if err := tx.Wallets().UpdateAccount(account); err != nil {
			return err
		}

		// 8f. 写 ledger（withdrawal_debit, out）
		debitTxn := &walletdomain.Transaction{
			UserID:        input.UserID,
			Type:          constants.WalletTxnTypeWithdrawalDebit,
			Direction:     constants.WalletTxnDirectionOut,
			Amount:        money.FromDecimal(amount),
			BalanceBefore: money.FromDecimal(before),
			BalanceAfter:  money.FromDecimal(after),
			Currency:      "USDT",
			Reference:     reference,
			Remark:        "用户提现扣款",
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		if err := tx.Wallets().CreateTransaction(debitTxn); err != nil {
			return err
		}

		// 8g. 写提现单
		seq, err := tx.Withdrawals().CountCreatedOnDay(input.UserID, dayStart)
		if err != nil {
			return err
		}
		withdrawalNo := fmt.Sprintf("WD%s%06d", now.Format("20060102"), seq+1)
		w := &withdrawaldomain.Withdrawal{
			WithdrawalNo:   withdrawalNo,
			UserID:         input.UserID,
			Network:        network,
			Address:        address,
			RequestAmount:  money.FromDecimal(amount),
			FeeAmount:      quote.FeeAmount,
			NetAmount:      quote.NetAmount,
			Status:         withdrawaldomain.StatusPending,
			IdempotencyKey: idempotencyKey,
			Reference:      reference,
			UserNote:       strings.TrimSpace(input.UserNote),
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		if err := tx.Withdrawals().CreateWithdrawal(w); err != nil {
			return err
		}
		result = w
		return nil
	})
	if err != nil {
		return nil, err
	}

	// 9. 事务提交后异步通知（不阻塞）
	s.notifySafely(constants.NotificationEventWithdrawalSubmitted, result.ID, map[string]interface{}{
		"withdrawal_no":  result.WithdrawalNo,
		"request_amount": result.RequestAmount.String(),
		"fee_amount":     result.FeeAmount.String(),
		"net_amount":     result.NetAmount.String(),
		"network":        result.Network,
		"address":        result.Address,
	})
	return result, nil
}

// verifyTOTP 校验 TOTP，未启用即 fail-closed。
func (s *Service) verifyTOTP(userID uint, code string) error {
	if s.totp == nil {
		return withdrawalcontract.ErrTOTPNotEnabled
	}
	if strings.TrimSpace(code) == "" {
		return withdrawalcontract.ErrTOTPRequired
	}
	if err := s.totp.VerifyChallengeCode(userID, code); err != nil {
		switch {
		case errors.Is(err, totpapplication.ErrNotEnabled):
			return withdrawalcontract.ErrTOTPNotEnabled
		default:
			return withdrawalcontract.ErrTOTPInvalid
		}
	}
	return nil
}
