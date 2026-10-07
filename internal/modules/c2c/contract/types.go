package contract

import (
	"time"

	"github.com/Aether-v1/hcz/internal/shared/money"
	"github.com/shopspring/decimal"
)

// ---- 支付方式 ----

// CreatePaymentMethodInput 新增支付方式输入。
// UserID 永远来自认证上下文，禁止前端传入。敏感字段以明文传入，service 层加密存储。
type CreatePaymentMethodInput struct {
	UserID uint // 来自认证上下文

	Type              string // USDT_TRC20/BANK_CARD/ALIPAY/WECHAT
	Currency          string // 仅 USDT_TRC20 必须为 USDT
	Network           string // 仅 USDT_TRC20 必须为 TRC20
	Address           string // USDT 地址（明文传入，service 加密）
	Label             string // 可选标签
	AccountName       string // 收款人姓名（明文传入，service 加密）
	BankName          string // 银行名称（仅 BANK_CARD）
	BankAccount       string // 银行卡号（明文传入，service 加密）
	BranchName        string // 开户行（可选，仅 BANK_CARD）
	AccountIdentifier string // 支付宝/微信号（明文传入，service 加密）
	QRCodeFileID      string // 上传服务返回的文件 ID
	QRCodeURL         string // 上传服务返回的 URL
	QRImage           string `json:"-"` // Deprecated：保留旧字段兼容
	Instructions      string

	// Step-Up 安全验证：2FA 用户传 TOTPCode；未开 2FA 用户传 Password。
	TOTPCode string
	Password string
}

// UpdatePaymentMethodInput 修改支付方式输入。空字段表示不更新该列。
type UpdatePaymentMethodInput struct {
	ID      uint
	UserID  uint // 来自认证上下文
	Type    string
	Label   string

	Address           string
	AccountName       string
	BankName          string
	BankAccount       string
	BranchName        string
	AccountIdentifier string
	QRCodeFileID      string
	QRCodeURL         string
	QRImage           string // Deprecated
	Instructions      string
	Enabled           *bool

	// Step-Up 安全验证。
	TOTPCode string
	Password string
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
	// PaymentMethodID 买家选择的卖家收款方式；为 0 时回退到卖家默认/首个启用方式。
	PaymentMethodID uint
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

// PaymentMethodAuditEntry 收款方式变更审计记录输入。
// 禁止携带完整地址/卡号/账号/TOTP/密码等敏感数据。
type PaymentMethodAuditEntry struct {
	UserID          uint
	Action          string // payment_method_created/updated/deleted/default_changed
	PaymentMethodID uint
	PMType          string
}

// UserC2CStatusView 用户 C2C 状态视图。
type UserC2CStatusView struct {
	UserID    uint   `json:"user_id"`
	C2CBanned bool   `json:"c2c_banned"`
	Status    string `json:"status"`
}
