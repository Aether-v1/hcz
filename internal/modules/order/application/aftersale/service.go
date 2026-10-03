package aftersale

import (
	"errors"
	"time"

	"github.com/Aether-v1/hcz/internal/constants"
	ordercontract "github.com/Aether-v1/hcz/internal/modules/order/contract"
	orderdomain "github.com/Aether-v1/hcz/internal/modules/order/domain"
	walletdomain "github.com/Aether-v1/hcz/internal/modules/wallet/domain"
	"github.com/shopspring/decimal"
)

var (
	ErrNotCompleted   = errors.New("order not completed")
	ErrNotOwner       = errors.New("not order owner")
	ErrTicketNotFound = errors.New("after-sale ticket not found")
	ErrPendingExists  = errors.New("pending after-sale already exists")
	ErrInvalidAction  = errors.New("invalid after-sale action")
)

// OrderStore 售后所需的订单持久化端口（由 order store 实现，支持 WithinTransaction）。
type OrderStore interface {
	GetByID(id uint) (*orderdomain.Order, error)
	GetByIDAndUser(id, userID uint) (*orderdomain.Order, error)
	UpdateFields(id uint, updates map[string]interface{}) error
	GetAfterSaleTicketByOrderID(orderID uint) (*orderdomain.AfterSaleTicket, error)
	WithinTransaction(fn func(ordercontract.Transaction) error) error
}

// Refunder 售后退款端口（由 WalletRefunderAdapter 实现，内部调 AdminRefundToWalletInTx）。
type Refunder interface {
	RefundInTx(tx ordercontract.Transaction, orderID uint, amount decimal.Decimal, remark string) (*walletdomain.Transaction, *orderdomain.OrderRefundRecord, error)
}

type Service struct {
	orders OrderStore
	refund Refunder
}

func NewService(orders OrderStore, refund Refunder) *Service {
	return &Service{orders: orders, refund: refund}
}

// Request 用户发起未收到。创建 ticket + order.after_sale_status=pending。
func (s *Service) Request(userID uint, orderID uint, reason, desc string) (*orderdomain.AfterSaleTicket, error) {
	order, err := s.orders.GetByIDAndUser(orderID, userID)
	if err != nil {
		return nil, err
	}
	if order == nil {
		return nil, ErrNotOwner
	}
	if order.Status != constants.OrderStatusCompleted {
		return nil, ErrNotCompleted
	}
	t := &orderdomain.AfterSaleTicket{
		OrderID: orderID, UserID: userID, Type: orderdomain.AfterSaleTypeNotReceived,
		Reason: reason, Description: desc, Status: orderdomain.AfterSaleStatusPending,
	}
	if err := s.orders.WithinTransaction(func(tx ordercontract.Transaction) error {
		orders := tx.Orders()
		existing, err := orders.LockAfterSalePendingByOrderIDForUpdate(orderID)
		if err != nil {
			return err
		}
		if existing != nil {
			return ErrPendingExists
		}
		if err := orders.CreateAfterSaleTicket(t); err != nil {
			return err
		}
		return orders.UpdateFields(orderID, map[string]interface{}{"after_sale_status": orderdomain.AfterSaleStatusPending})
	}); err != nil {
		return nil, err
	}
	return t, nil
}

func (s *Service) Reject(orderID uint, note string) error {
	return s.resolveAction(orderID, orderdomain.AfterSaleStatusRejected, note, decimal.NullDecimal{})
}

func (s *Service) Resolve(orderID uint, note string) error {
	return s.resolveAction(orderID, orderdomain.AfterSaleStatusResolved, note, decimal.NullDecimal{})
}

func (s *Service) resolveAction(orderID uint, status, note string, refundAmt decimal.NullDecimal) error {
	return s.orders.WithinTransaction(func(tx ordercontract.Transaction) error {
		orders := tx.Orders()
		t, err := orders.LockAfterSalePendingByOrderIDForUpdate(orderID)
		if err != nil {
			return err
		}
		if t == nil {
			return ErrInvalidAction
		}
		now := time.Now()
		if err := orders.UpdateAfterSaleTicket(t.ID, map[string]interface{}{
			"status": status, "admin_note": note, "refund_amount": refundAmt, "resolved_at": now, "updated_at": now,
		}); err != nil {
			return err
		}
		return orders.UpdateFields(orderID, map[string]interface{}{"after_sale_status": status})
	})
}

// PartialRefund / FullRefund 统一走共享事务：锁 ticket → 钱包退款 → 关单。
func (s *Service) PartialRefund(orderID uint, amount decimal.Decimal) (*orderdomain.AfterSaleTicket, error) {
	return s.doRefund(orderID, amount)
}

func (s *Service) FullRefund(orderID uint) (*orderdomain.AfterSaleTicket, error) {
	order, err := s.orders.GetByID(orderID)
	if err != nil || order == nil {
		return nil, ErrTicketNotFound
	}
	return s.doRefund(orderID, order.WalletPaidAmount.Decimal)
}

func (s *Service) doRefund(orderID uint, amount decimal.Decimal) (*orderdomain.AfterSaleTicket, error) {
	if s.refund == nil {
		return nil, errors.New("wallet refund service not wired")
	}
	var ticket *orderdomain.AfterSaleTicket
	if err := s.orders.WithinTransaction(func(tx ordercontract.Transaction) error {
		orders := tx.Orders()
		t, err := orders.LockAfterSalePendingByOrderIDForUpdate(orderID)
		if err != nil {
			return err
		}
		if t == nil {
			return ErrInvalidAction
		}
		// AdminRefundToWalletInTx 内部会再锁 order、wallet credit、ledger、refund_status、commission。
		if _, _, err := s.refund.RefundInTx(tx, orderID, amount, "after_sale_not_received"); err != nil {
			return err
		}
		now := time.Now()
		nd := decimal.NullDecimal{Decimal: amount.Round(2), Valid: true}
		if err := orders.UpdateAfterSaleTicket(t.ID, map[string]interface{}{
			"status": orderdomain.AfterSaleStatusResolved, "refund_amount": nd, "resolved_at": now, "updated_at": now,
		}); err != nil {
			return err
		}
		if err := orders.UpdateFields(orderID, map[string]interface{}{"after_sale_status": orderdomain.AfterSaleStatusResolved}); err != nil {
			return err
		}
		ticket = t
		return nil
	}); err != nil {
		return nil, err
	}
	return ticket, nil
}

// Get 查询工单（事务外只读）。
func (s *Service) Get(orderID uint) (*orderdomain.AfterSaleTicket, error) {
	return s.orders.GetAfterSaleTicketByOrderID(orderID)
}
