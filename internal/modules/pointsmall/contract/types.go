package contract

import (
	pointsmalldomain "github.com/Aether-v1/hcz/internal/modules/pointsmall/domain"
)

// ProductListFilter 是商品列表过滤条件。
type ProductListFilter struct {
	Enabled  *bool
	Page     int
	PageSize int
}

// ProductInput 是商品创建/更新输入（Admin）。
// 校验规则：Name 必填；PointsPrice ∈ (0, MaxPointsAmount]（BIGINT 整数，禁 float/负数/0/天文数字）；
// Stock ∈ [0, MaxStock]；PerUserLimit ∈ [0, MaxPerUserLimit]；FulfillmentType 仅 MANUAL。
// Enabled=false 时同样要求价格合法，避免脏数据。
// OperatorAdminID / Reason 只用于审计（不落商品表）：库存与价格的每一次后台变更都必须可追溯到人和原因。
type ProductInput struct {
	Name            string
	Subtitle        string
	Description     string
	Cover           string
	PointsPrice     int64
	Stock           int64
	UnlimitedStock  bool
	Enabled         bool
	Sort            int
	PerUserLimit    int64
	FulfillmentType string
	Instructions    string

	OperatorAdminID uint
	Reason          string
}

// 商品后台输入边界（P4 收口）：
// 价格上限与积分系统单笔 mutation 上限同口径（pointscontract.MaxPointsAmount），
// 库存/限购是独立维度，用本模块上限挡住误输入。
const (
	MaxStock           = int64(1_000_000)
	MaxPerUserLimit    = int64(10_000)
	MaxNameLength      = 120 // points_products.name 列宽
	MaxCoverLength     = 255 // points_products.cover 列宽
	MaxInstructionsLen = 8_000
)

// ProductEnabledInput 是上下架命令（P4）。
// 与"编辑商品"分离为显式命令：上下架必须带操作者与原因，不允许被整单保存静默携带。
type ProductEnabledInput struct {
	ProductID       uint
	Enabled         bool
	OperatorAdminID uint
	Reason          string
}

// ProductDetail 是用户端商品详情（含 UX 辅助的 can_redeem/reason_code）。
// 真正 POST 兑换时必须重新校验全部条件，前端判断仅作展示。
type ProductDetail struct {
	Product           pointsmalldomain.PointsProduct
	UserRedeemedCount int64
	CanRedeem         bool
	ReasonCode        string
}

// ReasonCode 是 can_redeem=false 的原因（稳定业务码，非 UI 文案）。
// 下架商品在详情接口直接 404（error.points_product_not_found），因此不存在 DISABLED 码。
const (
	ReasonInsufficientPoints = "INSUFFICIENT_POINTS"
	ReasonOutOfStock         = "OUT_OF_STOCK"
	ReasonLimitReached       = "LIMIT_REACHED"
)

// CreateExchangeInput 是创建兑换订单的输入（用户）。
// Quantity 固定为 1（V1 决策：数字权益无批量兑换需求，限购/库存/履约/退款均简化）；
// 客户端禁止提交 points_price / total_points / product_name / user_id。
type CreateExchangeInput struct {
	UserID         uint
	ProductID      uint
	IdempotencyKey string
}

// MaxIdempotencyKeyLength 与 points_exchange_orders.idempotency_key 列宽一致（varchar(64)）。
// 超长必须显式拒绝：否则只会得到数据库截断/报错，把 400 变成 500。
const MaxIdempotencyKeyLength = 64

// CancelInput 是用户取消兑换订单的输入（仅 PENDING 可取消）。
type CancelInput struct {
	UserID  uint
	OrderID uint
}

// AdminExchangeActionInput 是后台兑换订单操作的输入。
// Process/Complete 无需 Reason；Fail/Cancel 必须 Reason（返还会影响积分与库存）。
type AdminExchangeActionInput struct {
	AdminID uint
	OrderID uint
	Reason  string
	Note    string
}

// ExchangeResult 是兑换操作的返回（订单 + 事务后的权威余额 + 幂等标记）。
// AlreadyProcessed=true 表示本次请求是幂等重放（订单已是目标状态，未重复扣/返积分）。
type ExchangeResult struct {
	Order            *pointsmalldomain.ExchangeOrder
	CurrentBalance   int64
	AlreadyProcessed bool
}

// ExchangeOrderListFilter 是兑换订单列表过滤条件。
// UserID=0 时由 Admin 查询（无用户限定）；Status 空 = 全部。
type ExchangeOrderListFilter struct {
	UserID   uint
	Status   string
	Page     int
	PageSize int
}
