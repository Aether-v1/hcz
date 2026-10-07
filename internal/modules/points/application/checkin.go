package application

import (
	"strings"

	pointscontract "github.com/Aether-v1/hcz/internal/modules/points/contract"
)

// CheckinReward 每日签到发放积分（P2）。
//
// 幂等防线（双层，与 P1 订单奖励一致）：
//  1. reference（points:checkin:{user_id}:{YYYY-MM-DD}）预查：已存在 → 直接返回；
//  2. points_ledger.reference 唯一索引：并发/重放撞索引 → 识别冲突后静默跳过。
//
// 必须在调用方（签到事务）事务内执行；账户更新 + Ledger 写入同事务。
// 零积分日（Amount=0）禁止调用：不产生无意义的 0 amount Ledger。
func (s *Service) CheckinReward(tx pointscontract.Transaction, input pointscontract.CheckinRewardInput) error {
	if tx == nil {
		return pointscontract.ErrTransactionRequired
	}
	if input.UserID == 0 || input.CheckinID == 0 || input.Amount <= 0 {
		return pointscontract.ErrInvalidAmount
	}
	reference := strings.TrimSpace(input.Reference)
	if reference == "" {
		return pointscontract.ErrReferenceRequired
	}
	existing, err := tx.Points().GetLedgerEntryByReference(reference)
	if err != nil {
		return err
	}
	if existing != nil {
		return nil // 幂等：该用户该业务日已奖励
	}
	date := input.Date
	if _, _, err := s.applyMutation(tx, mutationInput{
		userID:       input.UserID,
		actionType:   pointscontract.ActionCheckinReward,
		sourceType:   pointscontract.SourceCheckin,
		sourceID:     input.CheckinID,
		amount:       input.Amount,
		reference:    reference,
		reason:       cleanReason(input.Reason, "daily check-in"),
		operatorType: pointscontract.OperatorSystem,
		checkinDate:  &date,
	}); err != nil {
		if isDuplicateKeyError(err) {
			return nil // 并发兜底：另一事务已写入
		}
		return err
	}
	return nil
}
