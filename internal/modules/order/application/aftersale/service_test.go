package aftersale

import (
	"errors"
	"testing"
	"time"

	ordercontract "github.com/Aether-v1/hcz/internal/modules/order/contract"
	orderdomain "github.com/Aether-v1/hcz/internal/modules/order/domain"
	walletdomain "github.com/Aether-v1/hcz/internal/modules/wallet/domain"
	"github.com/Aether-v1/hcz/internal/shared/money"
	"github.com/shopspring/decimal"
)

func moneyAmt(v string) money.Amount { return money.FromDecimal(decimal.RequireFromString(v)) }

// fakeOrders 同时满足 OrderStore（外层）与事务内 tx.Orders()（通过 fakeTx）。
type fakeOrders struct {
	ordercontract.Store
	order        *orderdomain.Order
	pending      *orderdomain.AfterSaleTicket
	ticket       *orderdomain.AfterSaleTicket
	afterSaleErr error
}

func (o *fakeOrders) GetByID(uint) (*orderdomain.Order, error)              { return o.order, nil }
func (o *fakeOrders) GetByIDAndUser(uint, uint) (*orderdomain.Order, error) { return o.order, nil }
func (o *fakeOrders) UpdateFields(uint, map[string]interface{}) error       { return nil }
func (o *fakeOrders) GetAfterSaleTicketByOrderID(uint) (*orderdomain.AfterSaleTicket, error) {
	return o.ticket, nil
}
func (o *fakeOrders) WithinTransaction(fn func(ordercontract.Transaction) error) error {
	return fn(&fakeTx{orders: o})
}

type fakeTx struct {
	ordercontract.Transaction
	orders *fakeOrders
}

func (t *fakeTx) Orders() ordercontract.Store { return t.orders }

// fakeOrders 事务内 after-sale 方法：
func (o *fakeOrders) CreateAfterSaleTicket(t *orderdomain.AfterSaleTicket) error {
	if o.afterSaleErr != nil {
		return o.afterSaleErr
	}
	o.ticket = t
	o.pending = t
	return nil
}
func (o *fakeOrders) LockAfterSalePendingByOrderIDForUpdate(uint) (*orderdomain.AfterSaleTicket, error) {
	return o.pending, nil
}
func (o *fakeOrders) UpdateAfterSaleTicket(uint, map[string]interface{}) error { return nil }

type fakeRefunder struct {
	calls int
	fail  bool
}

func (r *fakeRefunder) RefundInTx(tx ordercontract.Transaction, orderID uint, amount decimal.Decimal, remark string) (*walletdomain.Transaction, *orderdomain.OrderRefundRecord, error) {
	r.calls++
	if r.fail {
		return nil, nil, errors.New("refund failed")
	}
	return &walletdomain.Transaction{}, &orderdomain.OrderRefundRecord{}, nil
}

func completedOrder() *orderdomain.Order {
	return &orderdomain.Order{Status: "completed", WalletPaidAmount: moneyAmt("10")}
}

func TestRequestSuccess(t *testing.T) {
	o := &fakeOrders{order: completedOrder()}
	if _, err := NewService(o, &fakeRefunder{}).Request(1, 1, "r", "d"); err != nil {
		t.Fatal(err)
	}
	if o.ticket == nil || o.ticket.Status != orderdomain.AfterSaleStatusPending {
		t.Fatal("ticket not pending")
	}
}

func TestRequestNonCompletedRejected(t *testing.T) {
	o := &fakeOrders{order: &orderdomain.Order{Status: "processing"}}
	if _, err := NewService(o, &fakeRefunder{}).Request(1, 1, "r", "d"); err != ErrNotCompleted {
		t.Fatalf("want ErrNotCompleted got %v", err)
	}
}

func TestRequestPendingDuplicateRejected(t *testing.T) {
	o := &fakeOrders{order: completedOrder(), pending: &orderdomain.AfterSaleTicket{Status: orderdomain.AfterSaleStatusPending}}
	if _, err := NewService(o, &fakeRefunder{}).Request(1, 1, "r", "d"); err != ErrPendingExists {
		t.Fatalf("want ErrPendingExists got %v", err)
	}
}

func TestFullRefundCallsOnceAndIdempotent(t *testing.T) {
	o := &fakeOrders{order: completedOrder(), pending: &orderdomain.AfterSaleTicket{Status: orderdomain.AfterSaleStatusPending}}
	r := &fakeRefunder{}
	svc := NewService(o, r)
	if _, err := svc.FullRefund(1); err != nil {
		t.Fatal(err)
	}
	if r.calls != 1 {
		t.Fatalf("want 1 refund call got %d", r.calls)
	}
	// 退款成功后 pending 被清空（模拟真实），再次请求应拒绝
	o.pending = nil
	if _, err := svc.FullRefund(1); err != ErrInvalidAction {
		t.Fatalf("want ErrInvalidAction got %v", err)
	}
	if r.calls != 1 {
		t.Fatalf("duplicate refund! calls=%d", r.calls)
	}
}

func TestRefundFailureKeepsPending(t *testing.T) {
	ticket := &orderdomain.AfterSaleTicket{Status: orderdomain.AfterSaleStatusPending}
	o := &fakeOrders{order: completedOrder(), pending: ticket}
	r := &fakeRefunder{fail: true}
	if _, err := NewService(o, r).FullRefund(1); err == nil {
		t.Fatal("want refund error")
	}
	if ticket.Status != orderdomain.AfterSaleStatusPending {
		t.Fatalf("ticket must stay pending on refund failure, got %s", ticket.Status)
	}
}

func TestRejectResolve(t *testing.T) {
	o := &fakeOrders{order: completedOrder(), pending: &orderdomain.AfterSaleTicket{Status: orderdomain.AfterSaleStatusPending}}
	if err := NewService(o, &fakeRefunder{}).Reject(1, "note"); err != nil {
		t.Fatal(err)
	}
	o.pending = &orderdomain.AfterSaleTicket{Status: orderdomain.AfterSaleStatusPending}
	if err := NewService(o, &fakeRefunder{}).Resolve(1, "note"); err != nil {
		t.Fatal(err)
	}
}

var _ = time.Now
