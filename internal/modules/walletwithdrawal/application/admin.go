package application

import (
	"strings"
	"time"

	"github.com/Aether-v1/hcz/internal/constants"
	walletdomain "github.com/Aether-v1/hcz/internal/modules/wallet/domain"
	withdrawalcontract "github.com/Aether-v1/hcz/internal/modules/walletwithdrawal/contract"
	withdrawaldomain "github.com/Aether-v1/hcz/internal/modules/walletwithdrawal/domain"
	"github.com/Aether-v1/hcz/internal/shared/money"
)

// Approve 后台审批通过：pending → approved。
func (s *Service) Approve(input withdrawalcontract.AdminReviewInput) (*withdrawaldomain.Withdrawal, error) {
	now := time.Now()
	var result *withdrawaldomain.Withdrawal
	err := s.uow.WithinTransaction(func(tx withdrawalcontract.Transaction) error {
		w, err := tx.Withdrawals().GetWithdrawalByIDForUpdate(input.ID)
		if err != nil {
			return err
		}
		if w == nil {
			return withdrawalcontract.ErrWithdrawalNotFound
		}
		if !withdrawaldomain.CanTransition(w.Status, withdrawaldomain.StatusApproved) {
			return withdrawalcontract.ErrWithdrawalStatusInvalid
		}
		w.Status = withdrawaldomain.StatusApproved
		w.AdminNote = strings.TrimSpace(input.AdminNote)
		w.ApprovedBy = &input.AdminID
		w.ApprovedAt = &now
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
	s.notifySafely(constants.NotificationEventWithdrawalApproved, result.ID, map[string]interface{}{
		"withdrawal_no": result.WithdrawalNo,
		"net_amount":    result.NetAmount.String(),
	})
	return result, nil
}

// Reject 后台拒绝：pending/approved/processing → rejected，退回原始 request_amount。
func (s *Service) Reject(input withdrawalcontract.AdminReviewInput) (*withdrawaldomain.Withdrawal, error) {
	reason := strings.TrimSpace(input.RejectReason)
	if reason == "" {
		return nil, withdrawalcontract.ErrRejectReasonRequired
	}
	now := time.Now()
	var result *withdrawaldomain.Withdrawal
	err := s.uow.WithinTransaction(func(tx withdrawalcontract.Transaction) error {
		// 行锁顺序：withdrawals → accounts
		w, err := tx.Withdrawals().GetWithdrawalByIDForUpdate(input.ID)
		if err != nil {
			return err
		}
		if w == nil {
			return withdrawalcontract.ErrWithdrawalNotFound
		}
		if !withdrawaldomain.CanTransition(w.Status, withdrawaldomain.StatusRejected) {
			return withdrawalcontract.ErrWithdrawalStatusInvalid
		}

		// 退款：仅在尚未退款时执行。终态检查 + reference 唯一索引兜底。
		if withdrawaldomain.CanRefund(w.Status) {
			account, err := tx.Wallets().GetAccountByUserIDForUpdate(w.UserID)
			if err != nil {
				return err
			}
			if account == nil {
				return withdrawalcontract.ErrWithdrawalNotFound
			}
			before := account.Balance.Decimal.Round(2)
			refundAmount := w.RequestAmount.Decimal.Round(2)
			after := before.Add(refundAmount).Round(2)
			account.Balance = money.FromDecimal(after)
			account.UpdatedAt = now
			if err := tx.Wallets().UpdateAccount(account); err != nil {
				return err
			}
			refundTxn := &walletdomain.Transaction{
				UserID:        w.UserID,
				Type:          constants.WalletTxnTypeWithdrawalRefund,
				Direction:     constants.WalletTxnDirectionIn,
				Amount:        money.FromDecimal(refundAmount),
				BalanceBefore: money.FromDecimal(before),
				BalanceAfter:  money.FromDecimal(after),
				Currency:      "USDT",
				Reference:     refundReference(w.ID),
				Remark:        "提现拒绝退款",
				CreatedAt:     now,
				UpdatedAt:     now,
			}
			if err := tx.Wallets().CreateTransaction(refundTxn); err != nil {
				return err
			}
		}

		w.Status = withdrawaldomain.StatusRejected
		w.RejectReason = reason
		w.AdminNote = strings.TrimSpace(input.AdminNote)
		w.RejectedBy = &input.AdminID
		w.RejectedAt = &now
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
		"reason":        reason,
	})
	return result, nil
}

// MarkProcessing 后台标记打款处理中：approved → processing。
func (s *Service) MarkProcessing(input withdrawalcontract.AdminProcessingInput) (*withdrawaldomain.Withdrawal, error) {
	now := time.Now()
	var result *withdrawaldomain.Withdrawal
	err := s.uow.WithinTransaction(func(tx withdrawalcontract.Transaction) error {
		w, err := tx.Withdrawals().GetWithdrawalByIDForUpdate(input.ID)
		if err != nil {
			return err
		}
		if w == nil {
			return withdrawalcontract.ErrWithdrawalNotFound
		}
		if !withdrawaldomain.CanTransition(w.Status, withdrawaldomain.StatusProcessing) {
			return withdrawalcontract.ErrWithdrawalStatusInvalid
		}
		w.Status = withdrawaldomain.StatusProcessing
		w.ProcessedBy = &input.AdminID
		w.ProcessingAt = &now
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
	return result, nil
}

// Complete 后台标记打款完成：processing → completed，txid 必填。
func (s *Service) Complete(input withdrawalcontract.AdminCompleteInput) (*withdrawaldomain.Withdrawal, error) {
	txid := strings.TrimSpace(input.Txid)
	if txid == "" {
		return nil, withdrawalcontract.ErrTxidRequired
	}
	now := time.Now()
	var result *withdrawaldomain.Withdrawal
	err := s.uow.WithinTransaction(func(tx withdrawalcontract.Transaction) error {
		w, err := tx.Withdrawals().GetWithdrawalByIDForUpdate(input.ID)
		if err != nil {
			return err
		}
		if w == nil {
			return withdrawalcontract.ErrWithdrawalNotFound
		}
		if !withdrawaldomain.CanTransition(w.Status, withdrawaldomain.StatusCompleted) {
			return withdrawalcontract.ErrWithdrawalStatusInvalid
		}
		w.Status = withdrawaldomain.StatusCompleted
		w.Txid = txid
		w.AdminNote = strings.TrimSpace(input.AdminNote)
		w.CompletedBy = &input.AdminID
		w.CompletedAt = &now
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
	s.notifySafely(constants.NotificationEventWithdrawalCompleted, result.ID, map[string]interface{}{
		"withdrawal_no": result.WithdrawalNo,
		"txid":          result.Txid,
		"net_amount":    result.NetAmount.String(),
	})
	return result, nil
}
