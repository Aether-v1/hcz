package contract

import (
	"time"

	"github.com/Aether-v1/hcz/internal/shared/money"
)

type AccountListFilter struct {
	Page     int
	PageSize int
	UserID   uint
}

type TransactionListFilter struct {
	Page        int
	PageSize    int
	UserID      uint
	OrderID     uint
	Type        string
	Direction   string
	CreatedFrom *time.Time
	CreatedTo   *time.Time
}

type RechargeListFilter struct {
	Page         int
	PageSize     int
	RechargeNo   string
	UserID       uint
	UserKeyword  string
	PaymentID    uint
	ChannelID    uint
	ProviderType string
	ChannelType  string
	Status       string
	CreatedFrom  *time.Time
	CreatedTo    *time.Time
	PaidFrom     *time.Time
	PaidTo       *time.Time
}

type RechargeInput struct {
	UserID   uint
	Amount   money.Amount
	Currency string
	Remark   string
}

type AdjustBalanceInput struct {
	UserID          uint
	OperatorAdminID uint
	Delta           money.Amount
	Currency        string
	Remark          string
}

type CreditInput struct {
	UserID    uint
	Amount    money.Amount
	Currency  string
	Type      string
	Reference string
	Remark    string
	OrderID   *uint
}

type OrderBalanceInput struct {
	OrderID          uint
	UserID           uint
	TotalAmount      money.Amount
	WalletPaidAmount money.Amount
	Currency         string
	UseBalance       bool
}

type OrderReleaseInput struct {
	OrderID          uint
	UserID           uint
	WalletPaidAmount money.Amount
	TotalAmount      money.Amount
	Currency         string
	TransactionType  string
	Remark           string
}

// FreezeInput 冻结资金输入：available -= amount, frozen += amount
type FreezeInput struct {
	UserID    uint
	Amount    money.Amount
	Reference string // 唯一键，建议格式 "c2c_freeze:<business_id>"
	Remark    string
}

// UnfreezeInput 解冻资金输入：frozen -= amount, available += amount
type UnfreezeInput struct {
	UserID    uint
	Amount    money.Amount
	Reference string // 唯一键，建议格式 "c2c_unfreeze:<business_id>"
	Remark    string
}

// SettleInput 结算输入：source.frozen -= amount, target.available += amount
type SettleInput struct {
	SourceUserID    uint
	TargetUserID    uint
	Amount          money.Amount
	SourceReference string // source 侧 ledger 唯一键，建议 "c2c_settle:<business_id>"
	TargetReference string // target 侧 ledger 唯一键，建议 "c2c_receive:<business_id>"
	Remark          string
}
