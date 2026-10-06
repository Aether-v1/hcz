package application

import (
	"fmt"
	"strings"
	"time"

	affiliatecontract "github.com/Aether-v1/hcz/internal/modules/affiliate/contract"
	affiliatedomain "github.com/Aether-v1/hcz/internal/modules/affiliate/domain"
	walletcontract "github.com/Aether-v1/hcz/internal/modules/wallet/contract"

	"github.com/Aether-v1/hcz/internal/constants"
)

// ApplyWithdraw 用户提交提现申请（已退休）。
// 旧独立提现入口已关闭，请使用 TransferToWallet 将佣金划转至主钱包后统一提现。
func (s *Service) ApplyWithdraw(userID uint, input WithdrawApplyInput) (*affiliatedomain.WithdrawRequest, error) {
	return nil, ErrWithdrawRetired
}

// ReviewWithdraw 管理端审核提现申请（已退休）。
// 旧独立提现审核入口已关闭，不再处理新的提现单。历史数据仍可通过列表查询。
func (s *Service) ReviewWithdraw(adminID, withdrawID uint, action, rejectReason string) (*affiliatedomain.WithdrawRequest, error) {
	return nil, ErrWithdrawRetired
}

// approveWithdraw 审核通过（pending_review → approved），不做资金动作。
func (s *Service) approveWithdraw(adminID, withdrawID uint) (*affiliatedomain.WithdrawRequest, error) {
	err := s.repo.WithinTransaction(func(repoTx affiliatecontract.Store) error {
		req, err := repoTx.GetWithdrawByIDForUpdate(withdrawID)
		if err != nil {
			return err
		}
		if req == nil {
			return ErrNotFound
		}
		if req.Status != constants.AffiliateWithdrawStatusPendingReview {
			return ErrWithdrawStatusInvalid
		}
		now := time.Now()
		req.Status = constants.AffiliateWithdrawStatusApproved
		req.ProcessedBy = &adminID
		req.ProcessedAt = &now
		req.UpdatedAt = now
		return repoTx.UpdateWithdraw(req)
	})
	if err != nil {
		return nil, err
	}
	return s.repo.GetWithdrawByID(withdrawID)
}

// rejectWithdraw 拒绝提现，创建 WITHDRAW_RELEASE ledger 释放锁定。
func (s *Service) rejectWithdraw(adminID, withdrawID uint, rejectReason string) (*affiliatedomain.WithdrawRequest, error) {
	err := s.repo.WithinTransaction(func(repoTx affiliatecontract.Store) error {
		req, err := repoTx.GetWithdrawByIDForUpdate(withdrawID)
		if err != nil {
			return err
		}
		if req == nil {
			return ErrNotFound
		}
		if req.Status != constants.AffiliateWithdrawStatusPendingReview &&
			req.Status != constants.AffiliateWithdrawStatusApproved {
			return ErrWithdrawStatusInvalid
		}

		now := time.Now()
		req.Status = constants.AffiliateWithdrawStatusRejected
		req.RejectReason = rejectReason
		req.ProcessedBy = &adminID
		req.ProcessedAt = &now
		req.UpdatedAt = now

		// 创建 WITHDRAW_RELEASE ledger（正金额，释放锁定）
		releaseRef := fmt.Sprintf("affiliate_release:withdraw:%d", withdrawID)
		profile, err := repoTx.GetProfileByID(req.AffiliateProfileID)
		if err != nil {
			return err
		}
		if profile == nil {
			return ErrNotFound
		}
		if _, err := s.appendLedger(repoTx, LedgerEntry{
			CommissionID:       0,
			AffiliateProfileID: profile.ID,
			BeneficiaryUserID:  profile.UserID,
			OrderID:            0,
			Type:               constants.AffiliateLedgerTypeWithdrawRelease,
			Amount:             req.Amount.Decimal,
			Reference:          releaseRef,
			WithdrawRequestID:  &withdrawID,
			Remark:             fmt.Sprintf("Withdraw rejected: %s", rejectReason),
		}); err != nil {
			return err
		}

		// 释放关联 commissions 的 withdraw_request_id
		commissions, err := repoTx.ListCommissionsByWithdrawIDForUpdate(withdrawID)
		if err != nil {
			return err
		}
		ids := make([]uint, 0, len(commissions))
		for _, c := range commissions {
			ids = append(ids, c.ID)
		}
		if len(ids) > 0 {
			if err := repoTx.BatchUpdateCommissions(ids, map[string]interface{}{
				"withdraw_request_id": nil,
				"updated_at":          now,
			}); err != nil {
				return err
			}
		}

		return repoTx.UpdateWithdraw(req)
	})
	if err != nil {
		return nil, err
	}
	return s.repo.GetWithdrawByID(withdrawID)
}

// PayWithdraw 执行真实出金（已退休）。
// 旧独立提现出金入口已关闭，佣金请通过 TransferToWallet 划转至主钱包。
func (s *Service) PayWithdraw(adminID, withdrawID uint) (*affiliatedomain.WithdrawRequest, error) {
	return nil, ErrWithdrawRetired
}

// payWithdrawInternal 是 PayWithdraw 的内部实现。
// 实际的跨模块事务在 gormstore 的 WithinCombinedTransaction 中完成。
func (s *Service) payWithdrawInternal(adminID, withdrawID uint) error {
	// 这个方法会在 WithinCombinedTransaction 的回调中被调用。
	// 但由于我们需要访问底层 gorm.DB 来创建 wallet transaction，
	// 我们需要在 gormstore 层面实现。
	// 让我换一种方式：直接在 affiliate gormstore 里实现跨模块事务。

	// 为了简化，我们先在 affiliate repo 的 WithinTransaction 里做所有操作，
	// 然后手动调用 wallet 的 CreditInTransaction（需要绑定同一个 gorm.DB）。
	// 但问题是 affiliate Store 接口没有暴露 *gorm.DB。
	//
	// 解决方案：在 gormstore 里新增一个方法 WithinCombinedTransaction，
	// 它接受一个函数，该函数同时获得 affiliatecontract.Store 和 walletcontract.Transaction。

	return s.runCombinedTransaction(func(affiliateTx affiliatecontract.Store, walletTx walletcontract.Transaction) error {
		req, err := affiliateTx.GetWithdrawByIDForUpdate(withdrawID)
		if err != nil {
			return err
		}
		if req == nil {
			return ErrNotFound
		}
		if req.Status != constants.AffiliateWithdrawStatusPendingReview &&
			req.Status != constants.AffiliateWithdrawStatusApproved {
			return ErrWithdrawStatusInvalid
		}

		// 检查可用余额是否足够（退款可能已减少余额）
		profile, err := affiliateTx.GetProfileByID(req.AffiliateProfileID)
		if err != nil {
			return err
		}
		if profile == nil {
			return ErrNotFound
		}

		// 检查总收益 - 已结算 >= 提现金额（退款可能已减少总收益）
		// 注意：不能用 available（扣除了 lock），因为这笔提现本身已经被 lock 了。
		// 正确检查：totalEarned（净佣金收入） - settled（已出金） >= 提现金额
		totalEarned, err := s.getTotalEarned(affiliateTx, profile.ID)
		if err != nil {
			return err
		}
		settled, err := s.getSettledAmount(affiliateTx, profile.ID)
		if err != nil {
			return err
		}
		netAvailable := totalEarned.Sub(settled).Round(2)

		if netAvailable.LessThan(req.Amount.Decimal) {
			return ErrWithdrawInsufficientAfterRefund
		}

		// 锁序规范：先锁定本提现单绑定的佣金行，再做钱包入账与 ledger 写入。
		// 与退款路径（order → commissions → last_ledger）保持一致，
		// 避免「Pay 先取 last_ledger 再取 commissions」与「Refund 先取 commissions 再取 last_ledger」形成的反向锁序死锁。
		lockedCommissions, err := affiliateTx.ListCommissionsByWithdrawIDForUpdate(withdrawID)
		if err != nil {
			return err
		}

		now := time.Now()

		// 1. 调用钱包服务真实入账
		walletRef := fmt.Sprintf("affiliate_payout:withdraw:%d", withdrawID)
		_, walletTxn, err := s.walletSvc.CreditInTransaction(walletTx, walletcontract.CreditInput{
			UserID:    profile.UserID,
			Amount:    req.Amount,
			Currency:  "USDT",
			Type:      constants.WalletTxnTypeAffiliatePayout,
			Reference: walletRef,
			Remark:    fmt.Sprintf("Affiliate withdrawal payout #%d", withdrawID),
		})
		if err != nil {
			return err
		}

		// 2. 创建 WITHDRAW_SETTLE ledger（负金额）
		settleRef := fmt.Sprintf("affiliate_settle:withdraw:%d", withdrawID)
		if _, err := s.appendLedger(affiliateTx, LedgerEntry{
			CommissionID:       0,
			AffiliateProfileID: profile.ID,
			BeneficiaryUserID:  profile.UserID,
			OrderID:            0,
			Type:               constants.AffiliateLedgerTypeWithdrawSettle,
			Amount:             req.Amount.Decimal.Neg(),
			Reference:          settleRef,
			WithdrawRequestID:  &withdrawID,
			Remark:             fmt.Sprintf("Withdraw paid to wallet, txn #%d", walletTxn.ID),
		}); err != nil {
			return err
		}

		// 2b. 释放 WITHDRAW_LOCK（出金完成后锁定不再需要，避免 lock 与 settle 双重扣减）
		releaseRef := fmt.Sprintf("affiliate_release:withdraw:%d:paid", withdrawID)
		if _, err := s.appendLedger(affiliateTx, LedgerEntry{
			CommissionID:       0,
			AffiliateProfileID: profile.ID,
			BeneficiaryUserID:  profile.UserID,
			OrderID:            0,
			Type:               constants.AffiliateLedgerTypeWithdrawRelease,
			Amount:             req.Amount.Decimal,
			Reference:          releaseRef,
			WithdrawRequestID:  &withdrawID,
			Remark:             "Lock released after successful payout",
		}); err != nil {
			return err
		}

		// 3. 更新提现申请状态
		req.Status = constants.AffiliateWithdrawStatusPaid
		req.ProcessedBy = &adminID
		req.ProcessedAt = &now
		req.WalletTxnID = &walletTxn.ID
		req.UpdatedAt = now
		if err := affiliateTx.UpdateWithdraw(req); err != nil {
			return err
		}

		// 4. 更新关联 commissions 状态为 withdrawn（复用步骤开头已锁定的佣金行）
		ids := make([]uint, 0, len(lockedCommissions))
		for _, c := range lockedCommissions {
			ids = append(ids, c.ID)
		}
		if len(ids) > 0 {
			if err := affiliateTx.BatchUpdateCommissions(ids, map[string]interface{}{
				"status":     constants.AffiliateCommissionStatusWithdrawn,
				"updated_at": now,
			}); err != nil {
				return err
			}
		}

		return nil
	})
}

// runCombinedTransaction 执行跨模块事务（affiliate + wallet 共享同一个 gorm.DB）。
// 这个方法在 gormstore 里实现，但为了编译通过，我们先在 application 层声明一个接口。
// 实际实现见 gormstore.WithinCombinedTransaction。
func (s *Service) runCombinedTransaction(fn func(affiliatecontract.Store, walletcontract.Transaction) error) error {
	// 类型断言：如果 repo 是 *gormstore.Store，调用其 WithinCombinedTransaction 方法。
	// 为了解耦，我们定义一个内部接口。
	type combinedTxRunner interface {
		WithinCombinedTransaction(fn func(affiliatecontract.Store, walletcontract.Transaction) error) error
	}
	runner, ok := s.repo.(combinedTxRunner)
	if !ok {
		// 回退方案：如果不支持组合事务，就分开执行（测试环境可能用 fake repo）
		// 但这样就不是原子的了。测试环境我们会用真实 gormstore。
		return fmt.Errorf("repo does not support combined transaction")
	}
	return runner.WithinCombinedTransaction(fn)
}

func containsWithdrawChannel(channels []string, channel string) bool {
	target := strings.ToLower(strings.TrimSpace(channel))
	if target == "" {
		return false
	}
	for _, item := range channels {
		if strings.ToLower(strings.TrimSpace(item)) == target {
			return true
		}
	}
	return false
}
