package application

import (
	"fmt"
	"time"

	"github.com/Aether-v1/hcz/internal/constants"
	affiliatecontract "github.com/Aether-v1/hcz/internal/modules/affiliate/contract"

	"github.com/shopspring/decimal"
)

// AdminAdjustCommissionInput 管理员佣金调整输入。
type AdminAdjustCommissionInput struct {
	ProfileID    uint
	Amount       decimal.Decimal // 正=增加，负=减少
	Remark       string
	AdminID      uint
	CommissionID uint // 可选，关联特定佣金
	OrderID      uint // 可选，关联订单
}

// AdminAdjustCommission 管理员佣金纠错调整。
// 核心原则：commission amount 创建后不可变，修正只能通过 ADJUSTMENT ledger 新增记录。
func (s *Service) AdminAdjustCommission(input AdminAdjustCommissionInput) error {
	if s.repo == nil {
		return ErrNotOpened
	}
	if input.ProfileID == 0 {
		return fmt.Errorf("profile_id is required")
	}
	amount := input.Amount.Round(2)
	if amount.Equal(decimal.Zero) {
		return fmt.Errorf("adjustment amount cannot be zero")
	}

	return s.repo.WithinTransaction(func(tx affiliatecontract.Store) error {
		profile, err := tx.GetProfileByID(input.ProfileID)
		if err != nil {
			return err
		}
		if profile == nil {
			return ErrNotFound
		}

		now := time.Now()
		ref := fmt.Sprintf("affiliate_adjust:profile:%d:admin:%d:ts:%d", input.ProfileID, input.AdminID, now.UnixNano())
		remark := fmt.Sprintf("Admin #%d adjustment: %s", input.AdminID, input.Remark)

		_, err = s.appendLedger(tx, LedgerEntry{
			CommissionID:       input.CommissionID,
			AffiliateProfileID: input.ProfileID,
			BeneficiaryUserID:  profile.UserID,
			OrderID:            input.OrderID,
			Type:               constants.AffiliateLedgerTypeAdjustment,
			Amount:             amount,
			Reference:          ref,
			Remark:             remark,
		})
		return err
	})
}
