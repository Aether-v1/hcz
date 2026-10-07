package contract

import "errors"

var (
	// ErrProductNotFound 商品不存在（或对用户不可见：已下架）。
	ErrProductNotFound = errors.New("points mall product not found")
	// ErrProductDisabled 商品已下架，禁止创建新兑换。
	ErrProductDisabled = errors.New("points mall product disabled")
	// ErrProductOutOfStock 可用库存不足（含并发抢最后一件后）。
	ErrProductOutOfStock = errors.New("points mall product out of stock")
	// ErrProductLimitReached 超过每用户限兑数量。
	ErrProductLimitReached = errors.New("points mall product limit reached")
	// ErrExchangeOrderNotFound 兑换订单不存在（用户视角含 IDOR 归并）。
	ErrExchangeOrderNotFound = errors.New("points mall exchange order not found")
	// ErrExchangeInvalidState 状态转换非法（终态回退 / 越权转换）。
	ErrExchangeInvalidState = errors.New("points mall exchange order invalid state transition")
	// ErrExchangeReasonRequired 失败/取消必须填写原因。
	ErrExchangeReasonRequired = errors.New("points mall exchange order reason required")
	// ErrExchangeInvalidReason 原因超长（列宽 255）：显式 400，不做静默截断。
	ErrExchangeInvalidReason = errors.New("points mall exchange order reason invalid")
	// ErrUserRequired 用户身份缺失。
	ErrUserRequired = errors.New("points mall user required")
	// ErrAdminRequired 管理员身份缺失。
	ErrAdminRequired = errors.New("points mall admin required")
	// ErrInvalidPointsPrice 积分价格非法（必须 > 0 且不超过单笔积分上限）。
	ErrInvalidPointsPrice = errors.New("points mall invalid points price")
	// ErrInvalidStock 库存非法（必须 >= 0 且不超上限）。
	ErrInvalidStock = errors.New("points mall invalid stock")
	// ErrInvalidPerUserLimit 每用户限购数非法（必须 >= 0 且不超上限；0 表示不限制）。
	ErrInvalidPerUserLimit = errors.New("points mall invalid per-user limit")
	// ErrInvalidName 商品名称/图片地址长度超出列宽。
	ErrInvalidName = errors.New("points mall invalid name or cover")
	// ErrInvalidFulfillmentType 履约类型不受支持（V1 仅 MANUAL）。
	ErrInvalidFulfillmentType = errors.New("points mall invalid fulfillment type")
	// ErrIdempotencyConflict 同一 Idempotency-Key 已存在但参数（用户/商品）不一致。
	ErrIdempotencyConflict = errors.New("points mall idempotency conflict")
	// ErrIdempotencyKeyRequired 创建兑换必须携带 Idempotency-Key。
	ErrIdempotencyKeyRequired = errors.New("points mall idempotency key required")
	// ErrIdempotencyKeyTooLong Idempotency-Key 超出列宽（64）：显式 400，不交给 DB 报错。
	ErrIdempotencyKeyTooLong = errors.New("points mall idempotency key too long")
	// ErrPointsInsufficient 积分余额不足（用户主动消费禁止负余额）。
	ErrPointsInsufficient = errors.New("points mall insufficient points balance")
	// ErrProductNameRequired 商品名称必填。
	ErrProductNameRequired = errors.New("points mall product name required")
)
