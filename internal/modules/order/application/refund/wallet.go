package refund

import (
	"fmt"
	"strings"
	"time"

	orderapp "github.com/Aether-v1/hcz/internal/modules/order/application"

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

	remark := strings.TrimSpace(input.Remark)
	walletRemark := remark
	if walletRemark == "" {
		walletRemark = "管理员退款到余额"
	}
	reference := fmt.Sprintf("order:%d:admin_refund:%d", input.OrderID, time.Now().UnixNano())
	var (
		transactionResult  *walletdomain.Transaction
		refundRecordResult *orderdomain.OrderRefundRecord
	)

	config := settingsapp.DefaultOrderRefundConfig()
	if s.settingService != nil {
		loaded, err := s.settingService.GetOrderRefundConfig()
		if err != nil {
			return nil, nil, nil, err
		}
		config = loaded
	}

	err := s.orderStore.WithinTransaction(func(tx ordercontract.Transaction) error {
		orderRepository := tx.Orders()
		locked, err := orderRepository.GetByIDForUpdate(input.OrderID)
		if err != nil {
			return err
		}
		if locked == nil {
			return ErrOrderNotFound
		}
		order := *locked
		if order.UserID == 0 {
			return walletcontract.ErrNotSupportedForGuest
		}
		if order.PaidAt == nil {
			return ErrOrderStatusInvalid
		}
		now := time.Now()
		if settingsapp.IsOrderRefundWindowExpired(order.CreatedAt, order.PaidAt, config.MaxRefundDays, now) {
			return ErrOrderRefundExpired
		}
		// P0-2: USDT 结算单按实际 USDT 实付（WalletPaidAmount）退款，退款金额单位为 USDT，
		// 不再用 Site Currency 的 TotalAmount 重算。旧单无快照时 legacy 按 Site 口径。
		paidBase := order.TotalAmount.Decimal
		refundCurrency := order.Currency
		if order.UsdtTotalAmount.Decimal.GreaterThan(decimal.Zero) {
			paidBase = order.WalletPaidAmount.Decimal
			refundCurrency = "USDT"
		}
		if paidBase.LessThanOrEqual(decimal.Zero) {
			return ErrOrderStatusInvalid
		}
		refundedBefore := order.RefundedAmount.Decimal.Round(2)
		refundable := paidBase.Sub(refundedBefore).Round(2)
		if amount.GreaterThan(refundable) {
			return walletcontract.ErrRefundExceeded
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
			return err
		}

		newRefunded := refundedBefore.Add(amount).Round(2)
		updates := map[string]interface{}{
			"refunded_amount": money.FromDecimal(newRefunded),
			"updated_at":      now,
		}
		markRefunded := newRefunded.GreaterThanOrEqual(paidBase)
		if markRefunded {
			updates["status"] = constants.OrderStatusRefunded
		} else {
			updates["status"] = constants.OrderStatusPartiallyRefunded
		}
		if err := orderRepository.UpdateFields(order.ID, updates); err != nil {
			return ErrOrderUpdateFailed
		}
		if order.ParentID == nil {
			targetStatus := constants.OrderStatusPartiallyRefunded
			if markRefunded {
				targetStatus = constants.OrderStatusRefunded
			}
			if err := applyParentRefundChildStatusUpdates(orderRepository, order.ID, targetStatus, now); err != nil {
				return ErrOrderUpdateFailed
			}
		} else if _, err := orderapp.SyncParentStatus(orderRepository, *order.ParentID, now); err != nil {
			return ErrOrderUpdateFailed
		}
		if s.affiliateRefund != nil {
			if err := s.affiliateRefund.HandleOrderRefunded(
				tx.Affiliates(),
				&order,
				amount,
				refundedBefore,
				"order_refunded_to_wallet",
			); err != nil {
				return err
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
			return ErrRefundRecordCreateFailed
		}
		if s.resellerAccounting != nil {
			if err := s.resellerAccounting.HandleRefundDeduct(tx.ResellerAccounting(), &order, record, refundedBefore); err != nil {
				return err
			}
		}
		transactionResult = transaction
		refundRecordResult = record
		return nil
	})
	if err != nil {
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
