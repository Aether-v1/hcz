package aftersale

import (
	ordercontract "github.com/Aether-v1/hcz/internal/modules/order/contract"
	"github.com/Aether-v1/hcz/internal/modules/order/application/refund"
	orderdomain "github.com/Aether-v1/hcz/internal/modules/order/domain"
	walletdomain "github.com/Aether-v1/hcz/internal/modules/wallet/domain"
	"github.com/Aether-v1/hcz/internal/shared/money"
	"github.com/shopspring/decimal"
)

// WalletRefunder 售后退款必须使用的钱包退款链（P0-2 正式 USDT 退款）。
// 由 refund.Service.AdminRefundToWalletInTx 满足。
type WalletRefunder interface {
	AdminRefundToWalletInTx(tx ordercontract.Transaction, input refund.AdminRefundToWalletInput) (*walletdomain.Transaction, *orderdomain.OrderRefundRecord, error)
}

// WalletRefunderAdapter 薄适配：售后层只传 orderID/amount/remark，不复制退款逻辑。
// 注意：必须在调用方事务内调用，与 ticket/order 更新共享同一 tx。
type WalletRefunderAdapter struct{ svc WalletRefunder }

func NewWalletRefunderAdapter(svc WalletRefunder) *WalletRefunderAdapter {
	return &WalletRefunderAdapter{svc: svc}
}

func (a *WalletRefunderAdapter) RefundInTx(tx ordercontract.Transaction, orderID uint, amount decimal.Decimal, remark string) (*walletdomain.Transaction, *orderdomain.OrderRefundRecord, error) {
	return a.svc.AdminRefundToWalletInTx(tx, refund.AdminRefundToWalletInput{
		OrderID: orderID,
		Amount:  money.FromDecimal(amount),
		Remark:  remark,
	})
}
