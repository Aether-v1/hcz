package application

import (
	"time"

	affiliatedomain "github.com/Aether-v1/hcz/internal/modules/affiliate/domain"
	"github.com/Aether-v1/hcz/internal/shared/money"

	"github.com/shopspring/decimal"
)

// TrackClickInput 推广点击记录输入。
type TrackClickInput struct {
	AffiliateCode string
	VisitorKey    string
	LandingPath   string
	Referrer      string
	ClientIP      string
	UserAgent     string
}

// WithdrawApplyInput 提现申请输入。
type WithdrawApplyInput struct {
	Amount  decimal.Decimal
	Channel string
	Account string
}

// TransferToWalletInput 佣金划转至主钱包输入。
type TransferToWalletInput struct {
	Amount decimal.Decimal // 划转金额（All=true 时忽略）
	All    bool            // 是否划转全部可划转余额
}

// Dashboard 推广用户中心数据。
// Opened = commission account exists（profile 存在，含懒创建）；不再等价于"申请已通过"。
// TransferEnabled = 申请已通过且 profile active，前端据此控制划转按钮。
type Dashboard struct {
	Opened                   bool         `json:"opened"`
	ApplicationStatus        string       `json:"application_status"` // not_applied / pending / approved / rejected
	TransferEnabled          bool         `json:"transfer_enabled"`   // true 仅当 application approved + profile active
	AffiliateCode            string       `json:"affiliate_code"`
	PromotionPath            string       `json:"promotion_path"`
	ClickCount               int64        `json:"click_count"`
	ValidOrderCount          int64        `json:"valid_order_count"`
	ConversionRate           float64      `json:"conversion_rate"`
	PendingCommission        money.Amount `json:"pending_commission"`
	AvailableCommission      money.Amount `json:"available_commission"`       // 兼容旧前端：=可划转余额
	WithdrawnCommission      money.Amount `json:"withdrawn_commission"`       // 兼容旧前端：=累计已出金（历史提现+划转）
	AvailableTransferBalance money.Amount `json:"available_transfer_balance"` // 可划转余额
	DebtAmount               money.Amount `json:"debt_amount"`                // 欠款（正数表示欠多少，0=无欠款）
	TransferredAmount        money.Amount `json:"transferred_amount"`         // 累计已划转至主钱包
}

// Stats 推广统计数据。
type Stats struct {
	ClickCount               int64
	ValidOrderCount          int64
	ConversionRate           float64
	PendingCommission        money.Amount
	AvailableCommission      money.Amount
	WithdrawnCommission      money.Amount
	AvailableTransferBalance money.Amount
	DebtAmount               money.Amount
	TransferredAmount        money.Amount
}

// AdminUserItem 后台推广用户列表项。
type AdminUserItem struct {
	Profile affiliatedomain.Profile `json:"profile"`
	Stats   Stats                   `json:"stats"`
}

// AdminProfileListFilter 后台推广用户列表过滤。
type AdminProfileListFilter struct {
	Page     int
	PageSize int
	UserID   uint
	Status   string
	Code     string
	Keyword  string
}

// AdminCommissionListFilter 后台佣金列表过滤。
type AdminCommissionListFilter struct {
	Page               int
	PageSize           int
	AffiliateProfileID uint
	OrderNo            string
	Status             string
	Keyword            string
	Level              int
}

// AdminWithdrawListFilter 后台提现列表过滤。
type AdminWithdrawListFilter struct {
	Page               int
	PageSize           int
	AffiliateProfileID uint
	Status             string
	Keyword            string
}

// AffiliateApplicationListFilter 推广申请列表过滤。
type AffiliateApplicationListFilter struct {
	Page        int
	PageSize    int
	UserID      uint
	Status      string
	Keyword     string
	CreatedFrom *time.Time
	CreatedTo   *time.Time
}

// TransferRecord 划转历史记录（从 affiliate ledger 的 transfer_to_wallet 类型读取）。
type TransferRecord struct {
	ID        uint         `json:"id"`
	Amount    money.Amount `json:"amount"` // 划转金额（正数展示）
	Type      string       `json:"type"`
	Reference string       `json:"reference"`
	Remark    string       `json:"remark"`
	CreatedAt time.Time    `json:"created_at"`
}
