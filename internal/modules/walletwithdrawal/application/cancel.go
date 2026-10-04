package application

import (
	"time"

	"github.com/Aether-v1/hcz/internal/constants"
	walletdomain "github.com/Aether-v1/hcz/internal/modules/wallet/domain"
	withdrawalcontract "github.com/Aether-v1/hcz/internal/modules/walletwithdrawal/contract"
	withdrawaldomain "github.com/Aether-v1/hcz/internal/modules/walletwithdrawal/domain"
	"github.com/Aether-v1/hcz/internal/shared/money"
)

// CancelWithdrawal 用户取消提现（仅 pending），退回原始 request_amount。
func (s *Service) CancelWithdrawal(input withdrawalcontract.CancelWithdrawalInput) (*withdrawaldomain.Withdrawal, error) {
	if input.UserID == 0 || input.ID == 0 {
		return nil, withdrawalcontract.ErrWithdrawalNotFound
	}
	// TOTP 校验（fail-closed）
	if err := s.verifyTOTP(input.UserID, input.TOTPCode); err != nil {
		return nil, err
	}

	now := time.Now()
	var result *withdrawaldomain.Withdrawal
	err := s.uow.WithinTransaction(func(tx withdrawalcontract.Transaction) error {
		// 行锁顺序：withdrawals → accounts
		w, err := tx.Withdrawals().GetWithdrawalByIDForUpdate(input.ID)
		if err != nil {
			return err
		}
		if w == nil || w.UserID != input.UserID {
			return withdrawalcontract.ErrWithdrawalNotFound
		}
		// 状态机：仅 pending 可取消
		if !withdrawaldomain.CanCancel(w.Status) {
			return withdrawalcontract.ErrWithdrawalStatusInvalid
		}

		// 行锁钱包账户
		account, err := tx.Wallets().GetAccountByUserIDForUpdate(input.UserID)
		if err != nil {
			return err
		}
		if account == nil {
			return withdrawalcontract.ErrWithdrawalNotFound
		}
		before := account.AvailableBalance.Decimal.Round(2)
		refundAmount := w.RequestAmount.Decimal.Round(2)
		after := before.Add(refundAmount).Round(2)
		frozen := account.FrozenBalance
		account.AvailableBalance = money.FromDecimal(after)
		account.UpdatedAt = now
		if err := tx.Wallets().UpdateAccount(account); err != nil {
			return err
		}

		// 写退款 ledger（reference 幂等：wd_refund:<withdrawal_id>）
		refundTxn := &walletdomain.Transaction{
			UserID:          input.UserID,
			Type:            constants.WalletTxnTypeWithdrawalRefund,
			Direction:       constants.WalletTxnDirectionIn,
			Amount:          money.FromDecimal(refundAmount),
			BalanceBefore:   money.FromDecimal(before),
			BalanceAfter:    money.FromDecimal(after),
			AvailableBefore: money.FromDecimal(before),
			AvailableAfter:  money.FromDecimal(after),
			FrozenBefore:    frozen,
			FrozenAfter:     frozen,
			Currency:        "USDT",
			Reference:       refundReference(w.ID),
			Remark:          "提现取消退款",
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		if err := tx.Wallets().CreateTransaction(refundTxn); err != nil {
			return err
		}

		// 状态机迁移：pending → canceled
		if !withdrawaldomain.CanTransition(w.Status, withdrawaldomain.StatusCanceled) {
			return withdrawalcontract.ErrWithdrawalStatusInvalid
		}
		w.Status = withdrawaldomain.StatusCanceled
		w.CanceledAt = &now
		w.UpdatedAt = now
		if err := tx.Withdrawals().UpdateWithdrawal(w); err != nil {
			return err
		}
		result = w
		return nil
	})
	if err != nil {
		return nil, err
	}

	s.notifySafely(constants.NotificationEventWithdrawalRejected, result.ID, map[string]interface{}{
		"withdrawal_no": result.WithdrawalNo,
		"reason":        "user_canceled",
	})
	return result, nil
}
