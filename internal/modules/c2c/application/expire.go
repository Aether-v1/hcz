package application

import (
	"time"

	"github.com/Aether-v1/hcz/internal/constants"
	c2ccontract "github.com/Aether-v1/hcz/internal/modules/c2c/contract"
	c2cdomain "github.com/Aether-v1/hcz/internal/modules/c2c/domain"
	"github.com/Aether-v1/hcz/internal/modules/c2c/statemachine"
	walletcontract "github.com/Aether-v1/hcz/internal/modules/wallet/contract"
)

// ExpireTrade 由 asynq 超时任务调用：pending_payment -> expired，解冻卖家并恢复挂单余量。
// 幂等：已 paid/completed/canceled/disputed 的交易直接返回，绝不动资金。
func (s *Service) ExpireTrade(tradeID uint) error {
	if tradeID == 0 {
		return nil
	}
	var expired *c2cdomain.Trade
	err := s.uow.WithinTransaction(func(tx c2ccontract.Transaction) error {
		t, err := tx.C2C().GetTradeByIDForUpdate(tradeID)
		if err != nil {
			return err
		}
		if t == nil {
			return nil // 已删除/不存在，幂等返回
		}
		// 非待付款状态一律不动资金（已付款后由卖家 Confirm/仲裁处理）
		if t.Status != statemachine.StatusPendingPayment {
			return nil
		}
		next, err := statemachine.Transition(t.Status, statemachine.EventExpire)
		if err != nil {
			return err
		}
		// 解冻卖家（reference 与买家 Cancel 共用，幂等：若已取消则 Unfreeze 命中既有 ledger 直接返回）
		if _, _, err := s.wallet.Unfreeze(tx, walletcontract.UnfreezeInput{
			UserID:    t.SellerUserID,
			Amount:    t.USDTAmount,
			Reference: "c2c_unfreeze:trade:" + t.TradeNo,
			Remark:    "C2C交易超时解冻",
		}); err != nil {
			return err
		}
		// 恢复挂单可售余量
		if err := tx.C2C().IncrementListingAvailableUSDT(t.ListingID, t.USDTAmount.Decimal.Round(2)); err != nil {
			return err
		}
		now := time.Now()
		t.Status = next
		t.UpdatedAt = now
		expired = t
		return tx.C2C().UpdateTrade(t)
	})
	if err != nil {
		return err
	}
	if expired != nil {
		s.notifySafely(constants.NotificationEventC2CTradeExpired, expired.ID, map[string]interface{}{
			"trade_no":    expired.TradeNo,
			"buyer_id":    expired.BuyerUserID,
			"seller_id":   expired.SellerUserID,
			"usdt_amount": expired.USDTAmount.String(),
		})
	}
	return nil
}
