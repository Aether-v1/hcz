package contract

import (
	"time"

	"github.com/Aether-v1/hcz/internal/shared/money"
	"github.com/shopspring/decimal"
)

// ---- 支付方式 ----

// CreatePaymentMethodInput 新增支付方式输入。
type CreatePaymentMethodInput struct {
	UserID            uint
	Type              string
	AccountName       string
	AccountIdentifier string
	QRImage           string
	Instructions      string
}

// UpdatePaymentMethodInput 修改支付方式输入。
type UpdatePaymentMethodInput struct {
	ID                uint
	UserID            uint
	AccountName       string
	AccountIdentifier string
	QRImage           string
	Instructions      string
}

// ---- 挂单 ----

// ListingMarketFilter 市场挂单列表过滤。
type ListingMarketFilter struct {
	Page          int
	PageSize      int
	FiatCurrency  string
	ExcludeUserID uint // 不返回该用户自己的挂单（市场视角）
}

// ListingMyFilter 我的挂单列表过滤。
type ListingMyFilter struct {
	Page     int
	PageSize int
	UserID   uint
	Status   string
}

// CreateListingInput 发布挂单输入。
type CreateListingInput struct {
	UserID        uint
	FiatCurrency  string
	Price         money.Amount
	MinFiatAmount money.Amount
	MaxFiatAmount money.Amount
	TotalUSDT     money.Amount
	Terms         string
}

// UpdateListingInput 修改挂单输入（仅 active/paused）。
type UpdateListingInput struct {
	ID            uint
	UserID        uint
	Price         money.Amount
	MinFiatAmount money.Amount
	MaxFiatAmount money.Amount
	Terms         string
}

// ---- 交易 ----

// TradeListFilter 我的交易列表过滤。
type TradeListFilter struct {
	Page     int
	PageSize int
	UserID   uint
	Status   string
}

// CreateTradeInput 发起交易输入。
type CreateTradeInput struct {
	BuyerUserID    uint
	ListingID      uint
	USDTAmount     money.Amount
	IdempotencyKey string
}

// MarkPaidInput 买家标记已付款输入。
type MarkPaidInput struct {
	TradeID          uint
	BuyerUserID      uint
	PaymentReference string
}

// CancelTradeInput 买家取消待付款交易输入。
type CancelTradeInput struct {
	TradeID     uint
	BuyerUserID uint
}

// ConfirmTradeInput 卖家确认放行输入。
type ConfirmTradeInput struct {
	TradeID  uint
	SellerID uint
}

// RiskSignalInput 写入风控信号输入。
type RiskSignalInput struct {
	UserID     uint
	SignalType string
	TradeID    *uint
	Metadata   string
}

// ---- 争议 / 仲裁 ----

// InitiateDisputeInput 买家或卖家发起争议输入。
type InitiateDisputeInput struct {
	UserID      uint
	TradeID     uint
	Reason      string
	Description string
	Evidence    string
}

// ArbitrateInput 管理员仲裁输入。Result 仅允许 release_to_buyer / return_to_seller。
type ArbitrateInput struct {
	AdminID        uint
	TradeID        uint
	Result         string
	Reason         string
	AdminNote      string
	IdempotencyKey string
}

// 仲裁结果常量。
const (
	ArbitrationResultReleaseToBuyer = "release_to_buyer"
	ArbitrationResultReturnToSeller = "return_to_seller"
)

// DisputeListFilter 后台申诉列表过滤。
type DisputeListFilter struct {
	Page     int
	PageSize int
	Status   string
}

// RiskSignalListFilter 后台风控信号列表过滤。
type RiskSignalListFilter struct {
	Page        int
	PageSize    int
	UserID      uint
	SignalType  string
	CreatedFrom *time.Time
	CreatedTo   *time.Time
}

// AdminListingFilter 后台挂单列表过滤。
type AdminListingFilter struct {
	Page         int
	PageSize     int
	SellerUserID uint
	Status       string
	FiatCurrency string
}

// AdminTradeFilter 后台交易列表过滤。
type AdminTradeFilter struct {
	Page         int
	PageSize     int
	Status       string
	BuyerUserID  uint
	SellerUserID uint
	CreatedFrom  *time.Time
	CreatedTo    *time.Time
}

// OverviewStats C2C 概览统计。
type OverviewStats struct {
	ActiveListings  int64
	ActiveTrades    int64
	OpenDisputes    int64
	TodayVolumeUSDT decimal.Decimal
}

// ArbitrationAuditEntry 仲裁审计记录输入。
type ArbitrationAuditEntry struct {
	AdminID   uint
	TradeID   uint
	Result    string
	Reason    string
	AdminNote string
}

// UserC2CStatusView 用户 C2C 状态视图。
type UserC2CStatusView struct {
	UserID    uint   `json:"user_id"`
	C2CBanned bool   `json:"c2c_banned"`
	Status    string `json:"status"`
}
