package application

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/Aether-v1/hcz/internal/constants"
	c2ccontract "github.com/Aether-v1/hcz/internal/modules/c2c/contract"
	c2cdomain "github.com/Aether-v1/hcz/internal/modules/c2c/domain"
	"github.com/Aether-v1/hcz/internal/modules/c2c/statemachine"
	walletcontract "github.com/Aether-v1/hcz/internal/modules/wallet/contract"
	"github.com/Aether-v1/hcz/internal/shared/money"

	"github.com/shopspring/decimal"
)

// CreateTrade 买家发起一笔 C2C 交易：同一事务内行锁挂单、写交易单、原子扣减挂单余量。
// 新模型：只占用 listing 可售额度，不再 Freeze（卖家 USDT 在 CreateListing 时已冻结）。
// 任一步失败整体回滚。
func (s *Service) CreateTrade(input c2ccontract.CreateTradeInput) (*c2cdomain.Trade, error) {
	buyerID := input.BuyerUserID
	if buyerID == 0 {
		return nil, c2ccontract.ErrTradeNotFound
	}
	key := strings.TrimSpace(input.IdempotencyKey)
	if key == "" {
		return nil, c2ccontract.ErrIdempotencyConflict
	}
	amount := input.USDTAmount.Decimal.Round(2)
	if amount.LessThanOrEqual(decimal.Zero) {
		return nil, c2ccontract.ErrInvalidAmount
	}

	// 幂等预检：同买家同键已存在则直接返回，不重复冻结/建单。
	if existing, err := s.repo.GetTradeByBuyerIdempotency(buyerID, key); err != nil {
		return nil, err
	} else if existing != nil {
		return existing, nil
	}

	// 买家资格（全局开关/状态/封禁/TOTP）。
	if _, err := s.assertUserCanTrade(buyerID); err != nil {
		return nil, err
	}
	cfg := s.loadConfig()
	if cfg.MinTradeUSDT > 0 && amount.LessThan(decimal.NewFromFloat(cfg.MinTradeUSDT)) {
		return nil, c2ccontract.ErrAmountOutOfRange
	}
	if cfg.MaxTradeUSDT > 0 && amount.GreaterThan(decimal.NewFromFloat(cfg.MaxTradeUSDT)) {
		return nil, c2ccontract.ErrAmountOutOfRange
	}

	now := time.Now()
	var result *c2cdomain.Trade
	err := s.uow.WithinTransaction(func(tx c2ccontract.Transaction) error {
		// 1. 行锁挂单
		listing, err := tx.C2C().GetListingByIDForUpdate(input.ListingID)
		if err != nil {
			return err
		}
		if listing == nil {
			return c2ccontract.ErrListingNotFound
		}
		if listing.Status != listingActive {
			return c2ccontract.ErrListingNotActive
		}
		// 2. 自买自卖拒绝并记录风控信号
		if listing.SellerUserID == buyerID {
			_ = tx.C2C().CreateRiskSignal(&c2cdomain.RiskSignal{
				UserID:     buyerID,
				SignalType: "self_trade_attempt",
				CreatedAt:  now,
			})
			return c2ccontract.ErrSelfTrade
		}
		// 3. 数量校验
		avail := listing.AvailableUSDT.Decimal.Round(2)
		if amount.GreaterThan(avail) {
			return c2ccontract.ErrAmountOutOfRange
		}
		// 4. 法币金额区间校验
		price := listing.Price.Decimal.Round(2)
		fiatAmount := amount.Mul(price).Round(2)
		if fiatAmount.LessThan(listing.MinFiatAmount.Decimal.Round(2)) ||
			fiatAmount.GreaterThan(listing.MaxFiatAmount.Decimal.Round(2)) {
			return c2ccontract.ErrAmountOutOfRange
		}
		// 5. 卖家收款方式快照：仅快照买家选中（或卖家默认/首个启用）的那一条，且解密后落快照。
		selected, err := s.resolveTradePaymentMethod(tx, listing.SellerUserID, input.PaymentMethodID)
		if err != nil {
			return err
		}
		s.decryptPM(selected)
		snapshot, err := json.Marshal(buildTradePaymentSnapshot(selected, now))
		if err != nil {
			return err
		}

		tradeNo := genNo("C2CT", now)
		trade := &c2cdomain.Trade{
			TradeNo:               tradeNo,
			ListingID:             listing.ID,
			BuyerUserID:           buyerID,
			SellerUserID:          listing.SellerUserID,
			FiatCurrency:          listing.FiatCurrency,
			Price:                 money.FromDecimal(price),
			FiatAmount:            money.FromDecimal(fiatAmount),
			USDTAmount:            money.FromDecimal(amount),
			FeeAmount:             money.FromDecimal(decimal.Zero),
			BuyerReceiveUSDT:      money.FromDecimal(amount), // fee=0
			Status:                statemachine.StatusPendingPayment,
			PaymentMethodSnapshot: string(snapshot),
			IdempotencyKey:        key,
			ExpiredAt:             now.Add(time.Duration(cfg.TradeTimeoutMinutes) * time.Minute),
			CreatedAt:             now,
			UpdatedAt:             now,
		}

		// 6. 写交易单（不再 Freeze：卖家 USDT 在 CreateListing 时已冻结，这里只占用 listing 可售额度）。
		if err := tx.C2C().CreateTrade(trade); err != nil {
			return err
		}
		// 7. 原子扣减挂单可售余量
		affected, err := tx.C2C().DecrementListingAvailableUSDT(listing.ID, amount)
		if err != nil {
			return err
		}
		if affected == 0 {
			return c2ccontract.ErrAmountOutOfRange
		}
		result = trade
		return nil
	})
	if err != nil {
		return nil, err
	}

	// 事务提交后异步排期超时任务（失败不影响成交）。
	if s.scheduler != nil {
		delay := time.Duration(cfg.TradeTimeoutMinutes) * time.Minute
		_ = s.scheduler.EnqueueC2CTradeExpire(result.ID, delay)
	}
	// 事务提交后异步通知买卖双方。
	s.notifySafely(constants.NotificationEventC2CTradeCreated, result.ID, map[string]interface{}{
		"trade_no":      result.TradeNo,
		"buyer_id":      result.BuyerUserID,
		"seller_id":     result.SellerUserID,
		"usdt_amount":   result.USDTAmount.String(),
		"fiat_amount":   result.FiatAmount.String(),
		"fiat_currency": result.FiatCurrency,
	})
	return result, nil
}

// MarkPaid 买家标记已付款：pending_payment -> paid。绝对不操作钱包。
func (s *Service) MarkPaid(input c2ccontract.MarkPaidInput) (*c2cdomain.Trade, error) {
	var result *c2cdomain.Trade
	err := s.uow.WithinTransaction(func(tx c2ccontract.Transaction) error {
		t, err := tx.C2C().GetTradeByIDForUpdate(input.TradeID)
		if err != nil {
			return err
		}
		if t == nil {
			return c2ccontract.ErrTradeNotFound
		}
		if t.BuyerUserID != input.BuyerUserID {
			return c2ccontract.ErrPermissionDenied
		}
		if t.Status != statemachine.StatusPendingPayment {
			return c2ccontract.ErrTradeStatusInvalid
		}
		next, err := statemachine.Transition(t.Status, statemachine.EventMarkPaid)
		if err != nil {
			return c2ccontract.ErrTradeStatusInvalid
		}
		now := time.Now()
		t.Status = next
		t.PaymentReference = strings.TrimSpace(input.PaymentReference)
		t.BuyerPaidAt = &now
		t.UpdatedAt = now
		if err := tx.C2C().UpdateTrade(t); err != nil {
			return err
		}
		result = t
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.notifySafely(constants.NotificationEventC2CBuyerPaid, result.ID, map[string]interface{}{
		"trade_no":    result.TradeNo,
		"buyer_id":    result.BuyerUserID,
		"seller_id":   result.SellerUserID,
		"usdt_amount": result.USDTAmount.String(),
	})
	return result, nil
}

// Confirm 卖家确认放行：paid -> completed，冻结资金结算给买家（SettleFrozen）。
func (s *Service) Confirm(input c2ccontract.ConfirmTradeInput) (*c2cdomain.Trade, error) {
	var result *c2cdomain.Trade
	err := s.uow.WithinTransaction(func(tx c2ccontract.Transaction) error {
		t, err := tx.C2C().GetTradeByIDForUpdate(input.TradeID)
		if err != nil {
			return err
		}
		if t == nil {
			return c2ccontract.ErrTradeNotFound
		}
		if t.SellerUserID != input.SellerID {
			return c2ccontract.ErrPermissionDenied
		}
		if t.Status != statemachine.StatusPaid {
			return c2ccontract.ErrTradeStatusInvalid
		}
		next, err := statemachine.Transition(t.Status, statemachine.EventConfirm)
		if err != nil {
			return c2ccontract.ErrTradeStatusInvalid
		}
		// 冻结资金从卖家结算到买家可用余额（SettleFrozen 内部按 user_id 升序锁双账户，reference 幂等）
		if err := s.wallet.SettleFrozen(tx, walletcontract.SettleInput{
			SourceUserID:    t.SellerUserID,
			TargetUserID:    t.BuyerUserID,
			Amount:          t.USDTAmount,
			SourceReference: "c2c_settle:trade:" + t.TradeNo,
			TargetReference: "c2c_receive:trade:" + t.TradeNo,
			Remark:          "C2C结算放行",
		}); err != nil {
			return err
		}
		now := time.Now()
		t.Status = next
		t.SellerConfirmedAt = &now
		t.BuyerReceiveUSDT = t.USDTAmount
		t.CompletedAt = &now
		t.UpdatedAt = now
		if err := tx.C2C().UpdateTrade(t); err != nil {
			return err
		}
		result = t
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.notifySafely(constants.NotificationEventC2CTradeCompleted, result.ID, map[string]interface{}{
		"trade_no":    result.TradeNo,
		"buyer_id":    result.BuyerUserID,
		"seller_id":   result.SellerUserID,
		"usdt_amount": result.USDTAmount.String(),
		"source":      "seller_confirm",
	})
	return result, nil
}

// Cancel 买家取消待付款交易：pending_payment -> canceled。
// 新模型：seller wallet frozen 不变（资金仍属于 listing），仅恢复 listing 可售余量。
func (s *Service) Cancel(input c2ccontract.CancelTradeInput) (*c2cdomain.Trade, error) {
	var result *c2cdomain.Trade
	err := s.uow.WithinTransaction(func(tx c2ccontract.Transaction) error {
		t, err := tx.C2C().GetTradeByIDForUpdate(input.TradeID)
		if err != nil {
			return err
		}
		if t == nil {
			return c2ccontract.ErrTradeNotFound
		}
		if t.BuyerUserID != input.BuyerUserID {
			return c2ccontract.ErrPermissionDenied
		}
		if t.Status != statemachine.StatusPendingPayment {
			return c2ccontract.ErrTradeStatusInvalid
		}
		next, err := statemachine.Transition(t.Status, statemachine.EventCancel)
		if err != nil {
			return c2ccontract.ErrTradeStatusInvalid
		}
		// 恢复挂单可售余量（seller frozen 不变，资金仍属于 listing）。
		if err := tx.C2C().IncrementListingAvailableUSDT(t.ListingID, t.USDTAmount.Decimal.Round(2)); err != nil {
			return err
		}
		now := time.Now()
		t.Status = next
		t.CanceledAt = &now
		t.UpdatedAt = now
		if err := tx.C2C().UpdateTrade(t); err != nil {
			return err
		}
		result = t
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.notifySafely(constants.NotificationEventC2CTradeCanceled, result.ID, map[string]interface{}{
		"trade_no":  result.TradeNo,
		"buyer_id":  result.BuyerUserID,
		"seller_id": result.SellerUserID,
		"source":    "buyer_cancel",
	})
	return result, nil
}

// GetTradeDetail 交易详情（仅买家或卖家可见，IDOR 防护）。
func (s *Service) GetTradeDetail(userID, tradeID uint) (*c2cdomain.Trade, error) {
	t, err := s.repo.GetTradeByID(tradeID)
	if err != nil {
		return nil, err
	}
	if t == nil || (t.BuyerUserID != userID && t.SellerUserID != userID) {
		return nil, c2ccontract.ErrTradeNotFound
	}
	return t, nil
}

// ListMyTrades 我的交易（买家或卖家身份）。
func (s *Service) ListMyTrades(filter c2ccontract.TradeListFilter) ([]c2cdomain.Trade, int64, error) {
	return s.repo.ListMyTrades(filter)
}

// resolveTradePaymentMethod 解析交易使用的卖家收款方式：
//   - 指定 paymentMethodID 时校验其属于卖家且启用；
//   - 否则回退到卖家默认（或首个启用）收款方式。
func (s *Service) resolveTradePaymentMethod(tx c2ccontract.Transaction, sellerUserID, paymentMethodID uint) (*c2cdomain.PaymentMethod, error) {
	if paymentMethodID != 0 {
		pm, err := tx.C2C().GetEnabledPaymentMethodByID(paymentMethodID)
		if err != nil {
			return nil, err
		}
		if pm == nil || pm.UserID != sellerUserID {
			return nil, c2ccontract.ErrNoPaymentMethod
		}
		return pm, nil
	}
	pms, err := tx.C2C().ListEnabledPaymentMethodsByUserID(sellerUserID)
	if err != nil {
		return nil, err
	}
	if len(pms) == 0 {
		return nil, c2ccontract.ErrNoPaymentMethod
	}
	selected := pms[0]
	for i := range pms {
		if pms[i].IsDefault {
			selected = pms[i]
			break
		}
	}
	return &selected, nil
}

// tradePaymentSnapshot 交易单中固化的收款方式快照（解密后）。
type tradePaymentSnapshot struct {
	PaymentMethodID   uint   `json:"payment_method_id"`
	Type              string `json:"type"`
	Currency          string `json:"currency,omitempty"`
	Network           string `json:"network,omitempty"`
	Address           string `json:"address,omitempty"`
	AccountName       string `json:"account_name,omitempty"`
	BankName          string `json:"bank_name,omitempty"`
	BankAccount       string `json:"bank_account,omitempty"`
	BranchName        string `json:"branch_name,omitempty"`
	AccountIdentifier string `json:"account_identifier,omitempty"`
	QRCodeURL         string `json:"qr_code_url,omitempty"`
	SnapshotAt        string `json:"snapshot_at"`
}

// buildTradePaymentSnapshot 从解密后的收款方式构造交易快照。
func buildTradePaymentSnapshot(pm *c2cdomain.PaymentMethod, now time.Time) tradePaymentSnapshot {
	if pm == nil {
		return tradePaymentSnapshot{}
	}
	qr := pm.QRCodeURL
	if qr == "" {
		qr = pm.QRImage
	}
	return tradePaymentSnapshot{
		PaymentMethodID:   pm.ID,
		Type:              pm.Type,
		Currency:          pm.Currency,
		Network:           pm.Network,
		Address:           pm.Address,
		AccountName:       pm.AccountName,
		BankName:          pm.BankName,
		BankAccount:       pm.BankAccount,
		BranchName:        pm.BranchName,
		AccountIdentifier: pm.AccountIdentifier,
		QRCodeURL:         qr,
		SnapshotAt:        now.Format(time.RFC3339),
	}
}
