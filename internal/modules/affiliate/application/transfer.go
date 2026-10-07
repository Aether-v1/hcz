package application

import (
	"fmt"
	"strings"
	"time"

	affiliatecontract "github.com/Aether-v1/hcz/internal/modules/affiliate/contract"
	affiliatedomain "github.com/Aether-v1/hcz/internal/modules/affiliate/domain"
	walletcontract "github.com/Aether-v1/hcz/internal/modules/wallet/contract"
	walletdomain "github.com/Aether-v1/hcz/internal/modules/wallet/domain"

	"github.com/Aether-v1/hcz/internal/constants"
	"github.com/Aether-v1/hcz/internal/shared/money"

	"github.com/shopspring/decimal"
)

// computeAvailableTransferBalance 在指定 repoTx 内计算可划转余额。
// 公式：available = totalEarned - settled(历史withdraw_settle) - transferred(transfer_to_wallet)
//   - totalEarned = SUM(credit + reversal + adjustment + debt)
//   - settled     = ABS(SUM(withdraw_settle))（历史独立提现已出金）
//   - transferred = ABS(SUM(transfer_to_wallet))
//
// 若结果 < 0（DEBT 场景），返回 0。
func (s *Service) computeAvailableTransferBalance(repoTx affiliatecontract.Store, profileID uint) (decimal.Decimal, error) {
	if repoTx == nil || profileID == 0 {
		return decimal.Zero, nil
	}
	totalEarned, err := s.getTotalEarned(repoTx, profileID)
	if err != nil {
		return decimal.Zero, err
	}
	settled, err := s.getSettledAmount(repoTx, profileID)
	if err != nil {
		return decimal.Zero, err
	}
	transferSum, err := s.sumLedgerByProfile(repoTx, profileID, []string{
		constants.AffiliateLedgerTypeTransferToWallet,
	})
	if err != nil {
		return decimal.Zero, err
	}
	transferred := transferSum.Abs().Round(2)
	available := totalEarned.Sub(settled).Sub(transferred).Round(2)
	if available.LessThan(decimal.Zero) {
		return decimal.Zero, nil
	}
	return available, nil
}

// GetAvailableTransferBalance 计算用户可划转余额（事务外，供 dashboard 查询）。
func (s *Service) GetAvailableTransferBalance(profileID uint) (decimal.Decimal, error) {
	if profileID == 0 || s.repo == nil {
		return decimal.Zero, nil
	}
	return s.computeAvailableTransferBalance(s.repo, profileID)
}

// getNetLedgerBalance 计算 profile 全部 ledger 的净余额（正=净收益，负=欠款）。
func (s *Service) getNetLedgerBalance(repoTx affiliatecontract.Store, profileID uint) (decimal.Decimal, error) {
	if repoTx == nil || profileID == 0 {
		return decimal.Zero, nil
	}
	// 传空 types 表示不按类型过滤，SUM(amount) 即全部类型净值。
	return repoTx.SumLedgerByProfile(profileID, nil)
}

// TransferToWallet 将佣金从 affiliate 账户划转至用户主钱包。
// 原子性：affiliate ledger 扣减（负）+ wallet credit 入账 在同一个 gorm 事务内。
// 幂等：affiliate ledger reference 与 wallet txn reference 各自唯一索引。
func (s *Service) TransferToWallet(userID uint, input TransferToWalletInput) (*affiliatedomain.CommissionLedger, *walletdomain.Transaction, error) {
	if userID == 0 || s.repo == nil {
		return nil, nil, ErrNotOpened
	}
	if s.walletSvc == nil {
		return nil, nil, fmt.Errorf("wallet service not configured")
	}

	// 先到期佣金，确保 pending_confirm → available 后再计算可划转余额。
	if err := s.ConfirmDueCommissions(time.Now()); err != nil {
		return nil, nil, err
	}

	var resultLedger *affiliatedomain.CommissionLedger
	var resultWalletTxn *walletdomain.Transaction

	err := s.runCombinedTransaction(func(affiliateTx affiliatecontract.Store, walletTx walletcontract.Transaction) error {
		profile, err := affiliateTx.GetProfileByUserID(userID)
		if err != nil {
			return err
		}
		if profile == nil {
			return ErrNotOpened
		}
		if strings.TrimSpace(profile.Status) != constants.AffiliateProfileStatusActive {
			return ErrNotOpened
		}

		// 划转资格真源：affiliate_applications.status == approved（事务内读取，防止并发 approve/disable 竞态）
		latestApp, err := affiliateTx.GetLatestApplicationByUserID(userID)
		if err != nil {
			return err
		}
		if latestApp == nil || strings.TrimSpace(latestApp.Status) != constants.AffiliateAppStatusApproved {
			return ErrTransferNotApproved
		}

		// 行锁序列化同一 profile 的并发划转：先锁定该 profile 的全部 ledger 行，
		// 再计算可划转余额。这样并发请求会在此排队，后到者持锁后 SUM 已能看到先到者
		// 已提交的 transfer_to_wallet 扣减，从根上杜绝"两人都读到 available=10 导致重复入账"
		// 的 TOCTOU 竞态（与钱包账户行锁共同保证恰好一次入账）。
		if _, err := affiliateTx.ListLedgersByProfileForUpdate(profile.ID); err != nil {
			return err
		}

		// 事务内重新计算可划转余额（锁定最新 ledger）。
		available, err := s.computeAvailableTransferBalance(affiliateTx, profile.ID)
		if err != nil {
			return err
		}

		// DEBT 检查：净余额为负时禁止划转。
		netBalance, err := s.getNetLedgerBalance(affiliateTx, profile.ID)
		if err != nil {
			return err
		}
		if netBalance.LessThan(decimal.Zero) {
			return ErrTransferInDebt
		}

		amount := input.Amount.Round(2)
		if input.All {
			amount = available
		}
		if amount.LessThanOrEqual(decimal.Zero) {
			return ErrTransferAmountInvalid
		}
		if amount.GreaterThan(available) {
			return ErrTransferInsufficient
		}

		now := time.Now()
		// 两个 reference 独立：affiliate ledger 与 wallet txn 各自幂等。
		affiliateRef := fmt.Sprintf("affiliate_transfer:profile:%d:%d", profile.ID, now.UnixNano())
		walletRef := fmt.Sprintf("affiliate_transfer_in:profile:%d:%d", profile.ID, now.UnixNano())

		// 1. 钱包真实入账（available += amount）。
		_, walletTxn, err := s.walletSvc.CreditInTransaction(walletTx, walletcontract.CreditInput{
			UserID:    profile.UserID,
			Amount:    money.FromDecimal(amount),
			Currency:  "USDT",
			Type:      constants.WalletTxnTypeAffiliateTransferIn,
			Reference: walletRef,
			Remark:    "Affiliate commission transfer to wallet",
		})
		if err != nil {
			return err
		}

		// 2. affiliate ledger 扣减（负金额，append-only）。
		ledger, err := s.appendLedger(affiliateTx, LedgerEntry{
			AffiliateProfileID: profile.ID,
			BeneficiaryUserID:  profile.UserID,
			Type:               constants.AffiliateLedgerTypeTransferToWallet,
			Amount:             amount.Neg(),
			Reference:          affiliateRef,
			Remark:             "Transferred to wallet",
		})
		if err != nil {
			return err
		}

		resultLedger = ledger
		resultWalletTxn = walletTxn
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return resultLedger, resultWalletTxn, nil
}

// ListTransferHistory 查询用户划转历史（type=transfer_to_wallet 的 ledger，分页）。
func (s *Service) ListTransferHistory(userID uint, page, pageSize int) ([]affiliatedomain.CommissionLedger, int64, error) {
	if userID == 0 || s.repo == nil {
		return nil, 0, nil
	}
	profile, err := s.repo.GetProfileByUserID(userID)
	if err != nil {
		return nil, 0, err
	}
	if profile == nil {
		return nil, 0, nil
	}
	return s.repo.ListLedgersByProfileAndType(profile.ID, constants.AffiliateLedgerTypeTransferToWallet, page, pageSize)
}
