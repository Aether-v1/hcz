package application

import (
	pointscontract "github.com/Aether-v1/hcz/internal/modules/points/contract"
)

// RewardOrderCompleted 订单进入 COMPLETED 后发放积分（P1）。
//
// 幂等防线（双层）：
//  1. reference（points:order_reward:{order_id}）预查：已存在 → 直接返回，不重复入账；
//  2. points_ledger.reference 唯一索引：并发/重放撞索引 → 识别冲突后静默跳过。
//
// 必须在调用方（订单完成事务）事务内执行；账户更新 + Ledger 写入同事务。
func (s *Service) RewardOrderCompleted(tx pointscontract.Transaction, input pointscontract.OrderRewardInput) error {
	if tx == nil {
		return pointscontract.ErrTransactionRequired
	}
	if input.OrderID == 0 || input.UserID == 0 || input.Amount <= 0 {
		return pointscontract.ErrInvalidAmount
	}
	if input.Reference == "" {
		return pointscontract.ErrReferenceRequired
	}
	existing, err := tx.Points().GetLedgerEntryByReference(input.Reference)
	if err != nil {
		return err
	}
	if existing != nil {
		return nil // 幂等：同订单已奖励
	}
	orderID := input.OrderID
	if _, _, err := s.applyMutation(tx, mutationInput{
		userID:       input.UserID,
		actionType:   pointscontract.ActionOrderReward,
		sourceType:   pointscontract.SourceOrderReward,
		sourceID:     input.OrderID,
		amount:       input.Amount,
		reference:    input.Reference,
		reason:       input.Reason,
		operatorType: pointscontract.OperatorSystem,
		orderID:      &orderID,
	}); err != nil {
		if isDuplicateKeyError(err) {
			return nil // 并发兜底：另一事务已写入
		}
		return err
	}
	return nil
}

// ReverseOrderReward 订单退款时按比例冲正积分（P1）。
//
// 冲正是 append-only：绝不修改/删除原始 ORDER_REWARD 流水，只新增
// ORDER_REWARD_REVERSAL 流水（amount 为负）。reference =
// points:order_refund:{refund_record_id}，同一退款记录最多冲正一次。
//
// 必须在调用方（退款事务）事务内执行。
func (s *Service) ReverseOrderReward(tx pointscontract.Transaction, input pointscontract.OrderReversalInput) error {
	if tx == nil {
		return pointscontract.ErrTransactionRequired
	}
	if input.OrderID == 0 || input.UserID == 0 || input.RefundRecordID == 0 || input.Amount <= 0 {
		return pointscontract.ErrInvalidAmount
	}
	if input.Reference == "" {
		return pointscontract.ErrReferenceRequired
	}
	existing, err := tx.Points().GetLedgerEntryByReference(input.Reference)
	if err != nil {
		return err
	}
	if existing != nil {
		return nil // 幂等：同退款记录已冲正
	}
	orderID := input.OrderID
	if _, _, err := s.applyMutation(tx, mutationInput{
		userID:       input.UserID,
		actionType:   pointscontract.ActionOrderRewardReversal,
		sourceType:   pointscontract.SourceOrderRefund,
		sourceID:     input.RefundRecordID,
		amount:       -input.Amount, // 冲正恒为负
		reference:    input.Reference,
		reason:       input.Reason,
		operatorType: pointscontract.OperatorSystem,
		orderID:      &orderID,
	}); err != nil {
		if isDuplicateKeyError(err) {
			return nil // 并发兜底
		}
		return err
	}
	return nil
}

// OrderRewardSummary 返回订单已发放奖励与已冲正累计（P1 部分退款累计算法用）。
// reward 恒 >= 0；reversed 恒 >= 0（绝对值）。必须在调用方事务内执行。
func (s *Service) OrderRewardSummary(tx pointscontract.Transaction, orderID uint) (reward int64, reversed int64, err error) {
	if tx == nil {
		return 0, 0, pointscontract.ErrTransactionRequired
	}
	return tx.Points().SumOrderRewards(orderID)
}
