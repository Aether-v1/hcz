package application

import (
	"strings"

	pointscontract "github.com/Aether-v1/hcz/internal/modules/points/contract"
	pointsdomain "github.com/Aether-v1/hcz/internal/modules/points/domain"
)

// Redeem 积分商城兑换扣减（P3）。
//
// 语义：
//   - Amount 恒为正，内部转为负值入账（REDEEM）；
//   - REDEEM 属于用户主动消费：allowNegative=false，余额不足返回 ErrNegativeNotAllowed，
//     禁止余额变负（与 P0 审计结论一致：仅系统/Admin 冲正允许负余额）；
//   - REDEEM 计入 total_spent（绝对值，P0 已锁定语义：total_spent 为历史累计消费）；
//   - 幂等：reference（points:redeem:{exchange_order_id}）全局唯一，数据库唯一索引兜底；
//     同订单重复调用不重复扣减。
//   - 必须在调用方（兑换事务）事务内执行。
func (s *Service) Redeem(tx pointscontract.Transaction, input pointscontract.RedeemInput) (*pointsdomain.LedgerEntry, error) {
	if input.UserID == 0 || input.ExchangeOrderID == 0 {
		return nil, pointscontract.ErrAccountNotFound
	}
	if input.Amount <= 0 {
		return nil, pointscontract.ErrInvalidAmount
	}
	ref := strings.TrimSpace(input.Reference)
	if ref == "" {
		return nil, pointscontract.ErrReferenceRequired
	}

	// 幂等预查（事务内，同事务再次调用时命中即返回）。
	existing, err := tx.Points().GetLedgerEntryByReference(ref)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}

	_, entry, err := s.applyMutation(tx, mutationInput{
		userID:          input.UserID,
		actionType:      pointscontract.ActionRedeem,
		sourceType:      pointscontract.SourceRedeem,
		sourceID:        input.ExchangeOrderID,
		amount:          -input.Amount,
		reference:       ref,
		reason:          cleanReason(input.Reason, "积分商城兑换"),
		operatorType:    pointscontract.OperatorUser,
		operatorID:      input.UserID,
		exchangeOrderID: &input.ExchangeOrderID,
	})
	if err != nil {
		// 并发下同 reference 的另一事务已提交：唯一索引兜底 → 幂等返回已存在流水。
		if isDuplicateKeyError(err) {
			existing, queryErr := tx.Points().GetLedgerEntryByReference(ref)
			if queryErr != nil {
				return nil, queryErr
			}
			if existing != nil {
				return existing, nil
			}
		}
		return nil, err
	}
	return entry, nil
}

// RedeemRefund 积分商城兑换返还（P3）。
//
// 语义：
//   - Amount 恒为正，内部转为正数入账（REDEEM_REFUND）；
//   - 返还算入 total_earned（P0 已锁定：total_spent 是历史累计消费，不因返还回滚）；
//   - 幂等：reference（points:redeem_refund:{exchange_order_id}）全局唯一；
//     已存在（含并发撞唯一索引）→ 直接返回已存在流水，绝不重复返还。
//   - 必须在调用方（兑换失败/取消事务）事务内执行。
func (s *Service) RedeemRefund(tx pointscontract.Transaction, input pointscontract.RedeemRefundInput) (*pointsdomain.LedgerEntry, error) {
	if input.UserID == 0 || input.ExchangeOrderID == 0 {
		return nil, pointscontract.ErrAccountNotFound
	}
	if input.Amount <= 0 {
		return nil, pointscontract.ErrInvalidAmount
	}
	ref := strings.TrimSpace(input.Reference)
	if ref == "" {
		return nil, pointscontract.ErrReferenceRequired
	}

	existing, err := tx.Points().GetLedgerEntryByReference(ref)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}

	_, entry, err := s.applyMutation(tx, mutationInput{
		userID:          input.UserID,
		actionType:      pointscontract.ActionRedeemRefund,
		sourceType:      pointscontract.SourceRedeemRefund,
		sourceID:        input.ExchangeOrderID,
		amount:          input.Amount,
		reference:       ref,
		reason:          cleanReason(input.Reason, "积分商城兑换返还"),
		operatorType:    pointscontract.OperatorSystem,
		operatorID:      0,
		exchangeOrderID: &input.ExchangeOrderID,
	})
	if err != nil {
		if isDuplicateKeyError(err) {
			existing, queryErr := tx.Points().GetLedgerEntryByReference(ref)
			if queryErr != nil {
				return nil, queryErr
			}
			if existing != nil {
				return existing, nil
			}
		}
		return nil, err
	}
	return entry, nil
}
