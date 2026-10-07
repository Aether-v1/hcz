package refund

import (
	"fmt"
	"strings"
	"time"

	ordercontract "github.com/Aether-v1/hcz/internal/modules/order/contract"
	orderdomain "github.com/Aether-v1/hcz/internal/modules/order/domain"

	"github.com/Aether-v1/hcz/internal/constants"
	settingsapp "github.com/Aether-v1/hcz/internal/modules/settings/application"
	walletcontract "github.com/Aether-v1/hcz/internal/modules/wallet/contract"
	walletdomain "github.com/Aether-v1/hcz/internal/modules/wallet/domain"
	"github.com/Aether-v1/hcz/internal/shared/money"

	"github.com/shopspring/decimal"
)

// AdminRefundToWalletInput describes an order refund whose settlement target
// is the registered user's wallet.
type AdminRefundToWalletInput struct {
	OrderID uint
	Amount  money.Amount
	Remark  string
}

// AdminRefundToWallet executes the order refund workflow and delegates only
// the account credit to Wallet. Order state, refund records, affiliate
// reversals, and reseller accounting remain owned by the order context.
func (s *Service) AdminRefundToWallet(
	input AdminRefundToWalletInput,
) (*orderdomain.Order, *walletdomain.Transaction, *orderdomain.OrderRefundRecord, error) {
	if input.OrderID == 0 {
		return nil, nil, nil, ErrOrderNotFound
	}
	amount := input.Amount.Decimal.Round(2)
	if amount.LessThanOrEqual(decimal.Zero) {
		return nil, nil, nil, walletcontract.ErrInvalidAmount
	}
	if s == nil || s.wallets == nil {
		return nil, nil, nil, walletcontract.ErrAccountNotFound
	}
	var (
		transactionResult  *walletdomain.Transaction
		refundRecordResult *orderdomain.OrderRefundRecord
	)
	if err := s.orderStore.WithinTransaction(func(tx ordercontract.Transaction) error {
		txRes, rec, err := s.AdminRefundToWalletInTx(tx, input)
		if err != nil {
			return err
		}
		transactionResult = txRes
		refundRecordResult = rec
		return nil
	}); err != nil {
		return nil, nil, nil, err
	}
	order, err := s.orderStore.GetByID(input.OrderID)
	if err != nil {
		return nil, nil, nil, ErrOrderFetchFailed
	}
	if order == nil {
		return nil, nil, nil, ErrOrderNotFound
	}
	return order, transactionResult, refundRecordResult, nil
}

// AdminRefundToWalletInTx 在调用方提供的事务内执行钱包退款核心，不自己开事务。
// 唯一实现：wallet credit + ledger + order refunded_amount/status/refund_status +
// refund record + affiliate reversal + reseller。调用方须保证已在事务内。
func (s *Service) AdminRefundToWalletInTx(
	tx ordercontract.Transaction,
	input AdminRefundToWalletInput,
) (*walletdomain.Transaction, *orderdomain.OrderRefundRecord, error) {
	if input.OrderID == 0 {
		return nil, nil, ErrOrderNotFound
	}
	amount := input.Amount.Decimal.Round(2)
	if amount.LessThanOrEqual(decimal.Zero) {
		return nil, nil, walletcontract.ErrInvalidAmount
	}
	if s == nil || s.wallets == nil {
		return nil, nil, walletcontract.ErrAccountNotFound
	}
	remark := strings.TrimSpace(input.Remark)
	walletRemark := remark
	if walletRemark == "" {
		walletRemark = "管理员退款到余额"
	}
	reference := fmt.Sprintf("order:%d:admin_refund:%d", input.OrderID, time.Now().UnixNano())

	config := settingsapp.DefaultOrderRefundConfig()
	if s.settingService != nil {
		loaded, err := s.settingService.GetOrderRefundConfig()
		if err != nil {
			return nil, nil, err
		}
		config = loaded
	}

	orderRepository := tx.Orders()
	locked, err := orderRepository.GetByIDForUpdate(input.OrderID)
	if err != nil {
		return nil, nil, err
	}
	if locked == nil {
		return nil, nil, ErrOrderNotFound
	}
	order := *locked
	if order.UserID == 0 {
		return nil, nil, walletcontract.ErrNotSupportedForGuest
	}
	if order.PaidAt == nil {
		return nil, nil, ErrOrderStatusInvalid
	}
	now := time.Now()
	if settingsapp.IsOrderRefundWindowExpired(order.CreatedAt, order.PaidAt, config.MaxRefundDays, now) {
		return nil, nil, ErrOrderRefundExpired
	}
	// P0-2: USDT 结算单按实际 USDT 实付（WalletPaidAmount）退款，退款金额单位为 USDT。
	paidBase := order.TotalAmount.Decimal
	refundCurrency := order.Currency
	if order.UsdtTotalAmount.Decimal.GreaterThan(decimal.Zero) {
		paidBase = order.WalletPaidAmount.Decimal
		refundCurrency = "USDT"
	}
	if paidBase.LessThanOrEqual(decimal.Zero) {
		return nil, nil, ErrOrderStatusInvalid
	}
	refundedBefore := order.RefundedAmount.Decimal.Round(2)
	refundable := paidBase.Sub(refundedBefore).Round(2)
	if amount.GreaterThan(refundable) {
		return nil, nil, walletcontract.ErrRefundExceeded
	}

	_, transaction, err := s.wallets.CreditInTransaction(
		tx.Wallets(),
		walletcontract.CreditInput{
			UserID:    order.UserID,
			Amount:    money.FromDecimal(amount),
			Currency:  refundCurrency,
			Type:      constants.WalletTxnTypeAdminRefund,
			Reference: reference,
			Remark:    walletRemark,
			OrderID:   &order.ID,
		},
	)
	if err != nil {
		return nil, nil, err
	}

	newRefunded := refundedBefore.Add(amount).Round(2)
	updates := map[string]interface{}{
		"refunded_amount": money.FromDecimal(newRefunded),
		"updated_at":      now,
	}
	// P0 合同修复：Refund Service 只写 refund_status，永不写 Business Order 主状态。
	// 主状态唯一权威是 ordermachine；completed/failed/canceled 不得被退款覆盖。
	markRefunded := newRefunded.GreaterThanOrEqual(paidBase)
	if markRefunded {
		updates["refund_status"] = constants.OrderRefundStatusFull
	} else {
		updates["refund_status"] = constants.OrderRefundStatusPartial
	}
	if err := orderRepository.UpdateFields(order.ID, updates); err != nil {
		return nil, nil, ErrOrderUpdateFailed
	}
	// parent/child 主状态由 ordermachine 决定，退款不再驱动子订单主状态。
	// refund_status 各自独立；此处不做父子状态同步。
	if s.affiliateRefund != nil {
		if err := s.affiliateRefund.HandleOrderRefunded(
			tx.Affiliates(),
			&order,
			amount,
			refundedBefore,
			"order_refunded_to_wallet",
		); err != nil {
			return nil, nil, err
		}
	}

	currency := strings.ToUpper(strings.TrimSpace(refundCurrency))
	if currency == "" {
		currency = "CNY"
	}
	record := &orderdomain.OrderRefundRecord{
		UserID:                   order.UserID,
		GuestEmail:               order.GuestEmail,
		OrderID:                  order.ID,
		Type:                     constants.OrderRefundTypeWallet,
		Amount:                   money.FromDecimal(amount),
		PaymentFeeRefunded:       false,
		PaymentFeeRefundedAmount: money.FromDecimal(decimal.Zero),
		Currency:                 currency,
		Remark:                   remark,
		CreatedAt:                now,
		UpdatedAt:                now,
	}
	if err := orderRepository.CreateRefundRecord(record); err != nil {
		return nil, nil, ErrRefundRecordCreateFailed
	}
	// P1：退款积分冲正（floor 累计算法；仅父单；append-only ORDER_REWARD_REVERSAL）。
	if err := s.reverseOrderRewardInTx(tx, &order, amount, refundedBefore, record.ID); err != nil {
		return nil, nil, err
	}
	if s.resellerAccounting != nil {
		if err := s.resellerAccounting.HandleRefundDeduct(tx.ResellerAccounting(), &order, record, refundedBefore); err != nil {
			return nil, nil, err
		}
	}
	return transaction, record, nil
}
