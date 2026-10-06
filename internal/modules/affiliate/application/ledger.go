package application

import (
	"fmt"
	"time"

	affiliatecontract "github.com/Aether-v1/hcz/internal/modules/affiliate/contract"
	affiliatedomain "github.com/Aether-v1/hcz/internal/modules/affiliate/domain"
	"github.com/Aether-v1/hcz/internal/shared/money"

	"github.com/shopspring/decimal"
)

// LedgerEntry 是 appendLedger 的输入参数。
type LedgerEntry struct {
	CommissionID       uint
	AffiliateProfileID uint
	BeneficiaryUserID  uint
	OrderID            uint
	Type               string
	Amount             decimal.Decimal // 有符号值
	Reference          string
	WithdrawRequestID  *uint
	Remark             string
}

// appendLedger 在事务内创建一条 ledger 记录，自动计算 BalanceAfter。
// 核心原则：ledger 只 INSERT，永远不 UPDATE/DELETE。
// 通过 reference 唯一索引保证幂等。
func (s *Service) appendLedger(repoTx affiliatecontract.Store, input LedgerEntry) (*affiliatedomain.CommissionLedger, error) {
	if repoTx == nil {
		return nil, fmt.Errorf("appendLedger: repoTx is nil")
	}
	if input.AffiliateProfileID == 0 {
		return nil, fmt.Errorf("appendLedger: affiliate_profile_id is required")
	}
	if input.BeneficiaryUserID == 0 {
		return nil, fmt.Errorf("appendLedger: beneficiary_user_id is required")
	}
	if input.Type == "" {
		return nil, fmt.Errorf("appendLedger: type is required")
	}
	reference := input.Reference
	if reference == "" {
		return nil, fmt.Errorf("appendLedger: reference is required for idempotency")
	}

	// 幂等检查：reference 已存在则直接返回已有记录。
	existing, err := repoTx.GetCommissionLedgerByReference(reference)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}

	// 行锁取最新一条 ledger，计算 BalanceAfter 快照。
	last, err := repoTx.GetLastLedgerByProfileForUpdate(input.AffiliateProfileID)
	if err != nil {
		return nil, err
	}
	var balanceAfter decimal.Decimal
	if last != nil {
		balanceAfter = last.BalanceAfter.Decimal
	}
	balanceAfter = balanceAfter.Add(input.Amount).Round(2)

	now := time.Now()
	ledger := &affiliatedomain.CommissionLedger{
		CommissionID:       input.CommissionID,
		AffiliateProfileID: input.AffiliateProfileID,
		BeneficiaryUserID:  input.BeneficiaryUserID,
		OrderID:            input.OrderID,
		Type:               input.Type,
		Amount:             money.FromDecimal(input.Amount.Round(2)),
		BalanceAfter:       money.FromDecimal(balanceAfter),
		Reference:          reference,
		WithdrawRequestID:  input.WithdrawRequestID,
		Remark:             input.Remark,
		CreatedAt:          now,
	}
	if err := repoTx.CreateCommissionLedger(ledger); err != nil {
		if isDuplicateKeyError(err) {
			existing, queryErr := repoTx.GetCommissionLedgerByReference(reference)
			if queryErr == nil && existing != nil {
				return existing, nil
			}
		}
		return nil, err
	}
	return ledger, nil
}

// sumLedgerByProfile 计算某 profile 指定类型的 ledger 金额总和。
func (s *Service) sumLedgerByProfile(repoTx affiliatecontract.Store, profileID uint, types []string) (decimal.Decimal, error) {
	if repoTx == nil || profileID == 0 {
		return decimal.Zero, nil
	}
	return repoTx.SumLedgerByProfile(profileID, types)
}

// getTotalEarned 计算累计佣金收益 = credit + reversal + adjustment + debt
// debt 本身是负金额（已出金后退款产生的欠款），直接相加即可。
func (s *Service) getTotalEarned(repoTx affiliatecontract.Store, profileID uint) (decimal.Decimal, error) {
	return s.sumLedgerByProfile(repoTx, profileID, []string{
		"credit", "reversal", "adjustment", "debt",
	})
}

// getSettledAmount 计算已出金总额 = ABS(SUM(settle))，返回正数。
func (s *Service) getSettledAmount(repoTx affiliatecontract.Store, profileID uint) (decimal.Decimal, error) {
	settleSum, err := s.sumLedgerByProfile(repoTx, profileID, []string{"withdraw_settle"})
	if err != nil {
		return decimal.Zero, err
	}
	// settle 是负数，取绝对值
	return settleSum.Abs().Round(2), nil
}
