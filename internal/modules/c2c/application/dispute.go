package application

import (
	"strings"
	"time"

	"github.com/Aether-v1/hcz/internal/constants"
	c2ccontract "github.com/Aether-v1/hcz/internal/modules/c2c/contract"
	c2cdomain "github.com/Aether-v1/hcz/internal/modules/c2c/domain"
	"github.com/Aether-v1/hcz/internal/modules/c2c/statemachine"
)

// InitiateDispute 买家或卖家对一笔 paid 状态的交易发起争议：paid -> disputed。
// 冻结中的 USDT 继续冻结，不操作钱包；disputed 后买家不能 cancel、卖家不能普通 confirm
// （状态机 Transition 对 paid->cancel / paid->confirm 在 disputed 状态直接拒绝）。
func (s *Service) InitiateDispute(input c2ccontract.InitiateDisputeInput) (*c2cdomain.Dispute, error) {
	if input.UserID == 0 || input.TradeID == 0 {
		return nil, c2ccontract.ErrTradeNotFound
	}
	reason := strings.TrimSpace(input.Reason)
	if reason == "" {
		return nil, c2ccontract.ErrArbitrationReasonRequired
	}

	var (
		result    *c2cdomain.Dispute
		tradeInfo c2cdomain.Trade
	)
	err := s.uow.WithinTransaction(func(tx c2ccontract.Transaction) error {
		t, err := tx.C2C().GetTradeByIDForUpdate(input.TradeID)
		if err != nil {
			return err
		}
		if t == nil {
			return c2ccontract.ErrTradeNotFound
		}
		// 仅交易双方可发起争议
		if t.BuyerUserID != input.UserID && t.SellerUserID != input.UserID {
			return c2ccontract.ErrPermissionDenied
		}
		// 只能在 paid 状态发起
		if t.Status != statemachine.StatusPaid {
			return c2ccontract.ErrTradeStatusInvalid
		}
		next, err := statemachine.Transition(t.Status, statemachine.EventDispute)
		if err != nil {
			return c2ccontract.ErrTradeStatusInvalid
		}
		// 同一笔交易只能有一条 open 申诉（trade_id 唯一索引兜底）
		existing, err := tx.C2C().GetDisputeByTradeIDForUpdate(input.TradeID)
		if err != nil {
			return err
		}
		if existing != nil && existing.Status != "resolved" {
			return c2ccontract.ErrDisputeAlreadyOpen
		}

		now := time.Now()
		t.Status = next
		t.DisputedAt = &now
		t.UpdatedAt = now
		if err := tx.C2C().UpdateTrade(t); err != nil {
			return err
		}

		d := &c2cdomain.Dispute{
			TradeID:         input.TradeID,
			InitiatorUserID: input.UserID,
			Reason:          reason,
			Description:     strings.TrimSpace(input.Description),
			Evidence:        strings.TrimSpace(input.Evidence),
			Status:          "open",
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		if err := tx.C2C().CreateDispute(d); err != nil {
			return err
		}
		result = d
		tradeInfo = *t
		return nil
	})
	if err != nil {
		return nil, err
	}

	// 事务提交后异步通知对方与管理员。
	opponentID := uint(0)
	if result.InitiatorUserID == tradeInfo.BuyerUserID {
		opponentID = tradeInfo.SellerUserID
	} else {
		opponentID = tradeInfo.BuyerUserID
	}
	s.notifySafely(constants.NotificationEventC2CDisputed, result.TradeID, map[string]interface{}{
		"trade_id":     result.TradeID,
		"trade_no":     tradeInfo.TradeNo,
		"dispute_id":   result.ID,
		"initiator_id": result.InitiatorUserID,
		"opponent_id":  opponentID,
		"reason":       result.Reason,
	})
	return result, nil
}
