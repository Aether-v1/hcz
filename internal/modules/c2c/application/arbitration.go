package application

import (
	"strings"
	"time"

	"github.com/Aether-v1/hcz/internal/constants"
	c2ccontract "github.com/Aether-v1/hcz/internal/modules/c2c/contract"
	c2cdomain "github.com/Aether-v1/hcz/internal/modules/c2c/domain"
	"github.com/Aether-v1/hcz/internal/modules/c2c/statemachine"
	walletcontract "github.com/Aether-v1/hcz/internal/modules/wallet/contract"
)

// Arbitrate 管理员仲裁一笔 open 申诉。同一事务内完成状态机流转、资金动作与申诉结案。
//   - release_to_buyer：disputed -> completed，SettleFrozen(卖家冻结 USDT -> 买家可用)
//   - return_to_seller：disputed -> canceled，Unfreeze 卖家并恢复挂单可售余量
//
// 幂等：申诉已结案（resolved）直接返回，不重复资金动作；底层 SettleFrozen/Unfreeze
// 亦按 ledger reference 幂等。
func (s *Service) Arbitrate(input c2ccontract.ArbitrateInput) (*c2cdomain.Trade, *c2cdomain.Dispute, error) {
	if input.AdminID == 0 || input.TradeID == 0 {
		return nil, nil, c2ccontract.ErrTradeNotFound
	}
	switch input.Result {
	case c2ccontract.ArbitrationResultReleaseToBuyer, c2ccontract.ArbitrationResultReturnToSeller:
	default:
		return nil, nil, c2ccontract.ErrInvalidArbitrationResult
	}
	if strings.TrimSpace(input.Reason) == "" {
		return nil, nil, c2ccontract.ErrArbitrationReasonRequired
	}

	var (
		outTrade   *c2cdomain.Trade
		outDispute *c2cdomain.Dispute
	)
	err := s.uow.WithinTransaction(func(tx c2ccontract.Transaction) error {
		// 1. 行锁交易单
		t, err := tx.C2C().GetTradeByIDForUpdate(input.TradeID)
		if err != nil {
			return err
		}
		if t == nil {
			return c2ccontract.ErrTradeNotFound
		}
		// 2. 行锁申诉单
		d, err := tx.C2C().GetDisputeByTradeIDForUpdate(input.TradeID)
		if err != nil {
			return err
		}
		if d == nil {
			return c2ccontract.ErrDisputeNotFound
		}

		// 幂等：已结案的申诉不重复资金动作。
		if d.Status == "resolved" {
			outTrade = t
			outDispute = d
			return nil
		}
		if d.Status != "open" {
			return c2ccontract.ErrDisputeAlreadyResolved
		}
		// 交易必须处于 disputed
		if t.Status != statemachine.StatusDisputed {
			return c2ccontract.ErrTradeStatusInvalid
		}

		now := time.Now()

		switch input.Result {
		case c2ccontract.ArbitrationResultReleaseToBuyer:
			next, err := statemachine.Transition(t.Status, statemachine.EventArbitrateRelease)
			if err != nil {
				return c2ccontract.ErrTradeStatusInvalid
			}
			// 冻结资金从卖家结算到买家（SettleFrozen 内部按 user_id 升序锁双账户，reference 幂等）
			if err := s.wallet.SettleFrozen(tx, walletcontract.SettleInput{
				SourceUserID:    t.SellerUserID,
				TargetUserID:    t.BuyerUserID,
				Amount:          t.USDTAmount,
				SourceReference: "c2c_settle:trade:" + t.TradeNo,
				TargetReference: "c2c_receive:trade:" + t.TradeNo,
				Remark:          "C2C仲裁结算放行",
			}); err != nil {
				return err
			}
			t.Status = next
			t.CompletedAt = &now
			t.BuyerReceiveUSDT = t.USDTAmount
			t.UpdatedAt = now

		case c2ccontract.ArbitrationResultReturnToSeller:
			next, err := statemachine.Transition(t.Status, statemachine.EventArbitrateReturn)
			if err != nil {
				return c2ccontract.ErrTradeStatusInvalid
			}
			// 解冻卖家（reference 与 cancel/expire 区分开，使用仲裁专用 reference，幂等）
			if _, _, err := s.wallet.Unfreeze(tx, walletcontract.UnfreezeInput{
				UserID:    t.SellerUserID,
				Amount:    t.USDTAmount,
				Reference: "c2c_unfreeze:trade:" + t.TradeNo,
				Remark:    "C2C仲裁退回解冻",
			}); err != nil {
				return err
			}
			// 恢复挂单可售余量
			if err := tx.C2C().IncrementListingAvailableUSDT(t.ListingID, t.USDTAmount.Decimal.Round(2)); err != nil {
				return err
			}
			t.Status = next
			t.CanceledAt = &now
			t.UpdatedAt = now
		}

		if err := tx.C2C().UpdateTrade(t); err != nil {
			return err
		}

		d.Status = "resolved"
		d.AdminResult = input.Result
		d.AdminNote = strings.TrimSpace(input.AdminNote)
		d.ResolvedAt = &now
		d.UpdatedAt = now
		if err := tx.C2C().UpdateDispute(d); err != nil {
			return err
		}

		outTrade = t
		outDispute = d
		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	// 事务提交后：写审计 + 异步通知（均失败不影响仲裁结果）。
	if s.audit != nil {
		_ = s.audit.WriteC2CArbitration(c2ccontract.ArbitrationAuditEntry{
			AdminID:   input.AdminID,
			TradeID:   input.TradeID,
			Result:    input.Result,
			Reason:    input.Reason,
			AdminNote: input.AdminNote,
		})
	}
	s.notifySafely(constants.NotificationEventC2CArbitrated, outTrade.ID, map[string]interface{}{
		"trade_no":    outTrade.TradeNo,
		"buyer_id":    outTrade.BuyerUserID,
		"seller_id":   outTrade.SellerUserID,
		"result":      outDispute.AdminResult,
		"usdt_amount": outTrade.USDTAmount.String(),
	})
	return outTrade, outDispute, nil
}
