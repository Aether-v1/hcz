package contract

import (
	"fmt"
	"strings"
	"time"
)

// ActionType 是积分账本动作类型（禁止散落 magic string）。
//
// 全量清单（P4 收口，与 actionMetas 一一对应）：
//
//	ADMIN_ADD / ADMIN_DEDUCT          Admin 常规调整（客服补偿、纠错）
//	ADMIN_COMPENSATION                Admin 人工补偿（明确区别于系统奖励，见 P4 决策）
//	ORDER_REWARD / ORDER_REWARD_REVERSAL  订单奖励与退款冲正（系统）
//	CHECKIN_REWARD                    每日签到奖励（系统）
//	REDEEM / REDEEM_REFUND            商城兑换扣减与失败/取消返还
const (
	ActionAdminAdd          = "ADMIN_ADD"          // 管理员增加积分（计入 total_earned）
	ActionAdminDeduct       = "ADMIN_DEDUCT"       // 管理员扣减积分（仅影响 balance，不计入 total_spent）
	ActionAdminCompensation = "ADMIN_COMPENSATION" // 管理员人工补偿（计入 total_earned，审计可与正常奖励区分）

	ActionOrderReward         = "ORDER_REWARD"
	ActionOrderRewardReversal = "ORDER_REWARD_REVERSAL"

	ActionCheckinReward = "CHECKIN_REWARD"

	ActionRedeem       = "REDEEM"
	ActionRedeemRefund = "REDEEM_REFUND"
)

// 积分系统统一边界（P4 收口）。
//
// MaxPointsAmount 是"单笔 mutation / 单个配置项"的硬上限，用于挡住后台误输入
// （例如 100000000000000）。签到奖励上限与商城/商品积分价上限与此同口径。
// 真正的 int64 溢出保护在 mutation 内独立执行，本上限只作用于业务输入。
const (
	MaxPointsAmount          = int64(1_000_000) // 单笔调整/补偿、签到单日奖励、商品积分价、订单奖励上限
	MaxReasonLength          = 255              // points_ledger.reason 列宽
	MaxIdempotencyKeyLength  = 64               // 客户端提交的 Idempotency-Key
	MaxReferenceLength       = 120              // points_ledger.reference 列宽（含派生前缀）
	MaxLedgerPageSizeDefault = 200              // 与 store normalizePage 上限一致
)

// 流水查询白名单（P4）：过滤字段只接受已登记枚举，未知值直接拒绝，
// 避免把受限枚举退化成自由文本查询；排序固定，不接受客户端指定字段。
const (
	DirectionIncome  = "income"  // 只看入账（amount > 0）
	DirectionExpense = "expense" // 只看出账（amount < 0）
)

var registeredActionTypes = []string{
	ActionAdminAdd, ActionAdminDeduct, ActionAdminCompensation,
	ActionOrderReward, ActionOrderRewardReversal,
	ActionCheckinReward,
	ActionRedeem, ActionRedeemRefund,
}

var registeredSourceTypes = []string{
	SourceAdminAdjust, SourceAdminCompensation,
	SourceOrderReward, SourceOrderRefund,
	SourceCheckin,
	SourceRedeem, SourceRedeemRefund,
}

// IsValidActionType 判断 action_type 过滤值是否属于登记枚举。
func IsValidActionType(v string) bool {
	return containsString(registeredActionTypes, v)
}

// IsValidSourceType 判断 source_type 过滤值是否属于登记枚举。
func IsValidSourceType(v string) bool {
	return containsString(registeredSourceTypes, v)
}

// IsValidLedgerDirection 判断收支方向过滤值是否合法（空值表示不限）。
func IsValidLedgerDirection(v string) bool {
	return v == "" || v == DirectionIncome || v == DirectionExpense
}

func containsString(list []string, v string) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}

// SourceType 是积分来源类型（与 action 组合表达"一次业务事件的一次入账"）。
const (
	SourceAdminAdjust       = "admin_adjust"       // Admin 手动调整（幂等键 = reference）
	SourceAdminCompensation = "admin_compensation" // Admin 人工补偿（幂等键 = reference）

	// 预留（P1+）：
	SourceOrderReward = "order_reward"
	SourceOrderRefund = "order_refund"
	// 预留（P2）：
	SourceCheckin = "checkin"
	// 预留（P3）：
	SourceRedeem       = "redeem"
	SourceRedeemRefund = "redeem_refund"
)

// OperatorType 标识积分变更的操作者类型。
const (
	OperatorAdmin  = "admin"  // 管理员（OperatorID = admin_id）
	OperatorSystem = "system" // 系统自动（OperatorID = 0）
	OperatorUser   = "user"   // 用户主动（OperatorID = user_id）
)

// AdminAdjustReference 由 Idempotency-Key 派生，保证同一次调整在数据库层唯一。
// 前缀 "admin_adjust:" 与钱包的 "admin_adjust:" 对齐，避免跨资产混淆。
func AdminAdjustReference(idempotencyKey string) string {
	return "admin_adjust:" + strings.TrimSpace(idempotencyKey)
}

// AdminCompensationReference 由 Idempotency-Key 派生。
// 与 AdminAdjustReference 分属不同前缀，人工补偿与常规调整在幂等空间和审计口径上都不混用。
func AdminCompensationReference(idempotencyKey string) string {
	return "admin_compensation:" + strings.TrimSpace(idempotencyKey)
}

// 预留 reference 规则（P1+ 直接使用，本轮不产生业务实现）：
//
//	OrderRewardReference(orderID)         = "points:order_reward:{order_id}"
//	OrderRefundReversalReference(refundID) = "points:order_refund:{refund_record_id}"
//	CheckinReference(userID, date)        = "points:checkin:{user_id}:{YYYY-MM-DD}"
//	RedeemReference(exchangeOrderID)      = "points:redeem:{exchange_order_id}"
//	RedeemRefundReference(exchangeOrderID)= "points:redeem_refund:{exchange_order_id}"

// OrderRewardReference 生成订单奖励幂等键（P1 使用）。
func OrderRewardReference(orderID uint) string {
	return fmt.Sprintf("points:order_reward:%d", orderID)
}

// OrderRefundReversalReference 生成订单退款积分冲正幂等键（P1 使用）。
// 每个退款记录（order_refund_records.id）唯一；部分退款多次 → 多个记录 → 各自独立冲正流水。
func OrderRefundReversalReference(refundRecordID uint) string {
	return fmt.Sprintf("points:order_refund:%d", refundRecordID)
}

// CheckinReference 生成签到幂等键（P2 使用）。
func CheckinReference(userID uint, date time.Time) string {
	return fmt.Sprintf("points:checkin:%d:%s", userID, date.Format("2006-01-02"))
}

// OrderRewardInput 是订单完成积分奖励的领域输入（P1）。
// Amount 恒为正数（>0）；action=ORDER_REWARD，source=order_reward，source_id=OrderID。
type OrderRewardInput struct {
	UserID    uint
	OrderID   uint
	Amount    int64 // > 0（父单积分快照）
	Reason    string
	Reference string // OrderRewardReference(orderID)
}

// OrderReversalInput 是订单退款积分冲正的领域输入（P1）。
// Amount 恒为正数（>0）；内部转为负值入账（ORDER_REWARD_REVERSAL），
// source=order_refund，source_id=RefundRecordID，order_id=OrderID。
type OrderReversalInput struct {
	UserID         uint
	OrderID        uint
	RefundRecordID uint
	Amount         int64 // > 0（本次应冲正积分）
	Reason         string
	Reference      string // OrderRefundReversalReference(refundRecordID)
}

// CheckinRewardInput 是每日签到奖励的领域输入（P2）。
// Amount 恒为正数（>0）；action=CHECKIN_REWARD，source=checkin，
// source_id=CheckinID（签到记录 ID），checkin_date=业务日（UTC 零点）。
// 零积分日（奖励=0）不调用本方法，不产生 0 amount Ledger。
type CheckinRewardInput struct {
	UserID    uint
	CheckinID uint
	Amount    int64     // > 0
	Date      time.Time // Asia/Shanghai 业务日（UTC 零点表示）
	Reason    string
	Reference string // CheckinReference(userID, date)
}

// RedeemReference 生成积分商城兑换扣减幂等键（P3）。
// 每个兑换订单（points_exchange_orders.id）唯一；同订单最多 1 个 REDEEM。
func RedeemReference(exchangeOrderID uint) string {
	return fmt.Sprintf("points:redeem:%d", exchangeOrderID)
}

// RedeemRefundReference 生成积分商城兑换返还幂等键（P3）。
// 同订单最多 1 个正式 REDEEM_REFUND（数据库唯一索引是最终防线）。
func RedeemRefundReference(exchangeOrderID uint) string {
	return fmt.Sprintf("points:redeem_refund:%d", exchangeOrderID)
}

// RedeemInput 是积分商城兑换扣减的领域输入（P3）。
// Amount 恒为正数（>0）；内部转为负值入账（REDEEM，允许余额为负 = false，
// 用户主动消费禁止负余额）；source=redeem，source_id=ExchangeOrderID，
// exchange_order_id=ExchangeOrderID。
type RedeemInput struct {
	UserID          uint
	ExchangeOrderID uint
	Amount          int64 // > 0（= 兑换订单 total_points）
	Reason          string
	Reference       string // RedeemReference(exchangeOrderID)
}

// RedeemRefundInput 是积分商城兑换返还的领域输入（P3）。
// Amount 恒为正数（>0）；内部转为正数入账（REDEEM_REFUND，返还计入 total_earned，
// 与 P0 已锁定语义一致：total_spent 是历史累计消费，不因返还回滚）；
// source=redeem_refund，source_id=ExchangeOrderID，exchange_order_id=ExchangeOrderID。
type RedeemRefundInput struct {
	UserID          uint
	ExchangeOrderID uint
	Amount          int64 // > 0（= 兑换订单 total_points）
	Reason          string
	Reference       string // RedeemRefundReference(exchangeOrderID)
}

// AdjustInput 是 Admin 积分调整的领域输入。
// Amount 恒为正数（>0）；方向由 Operation 表达（add/subtract），
// 禁止通过负数 Amount 表达扣减，避免双重负数错误。
type AdjustInput struct {
	UserID          uint
	OperatorAdminID uint
	Operation       string // add / subtract
	Amount          int64  // > 0
	Reason          string // 必填
	Reference       string // 幂等键（Idempotency-Key 派生），必填
}

// CompensateInput 是 Admin 人工补偿的领域输入（P4）。
// 语义：系统生命周期之外的一次性补发（例如历史订单漏发积分的客服补偿）。
// 一律为入账（正数），action=ADMIN_COMPENSATION，source=admin_compensation。
// 不复用 ORDER_REWARD：补偿必须在账本上可辨认出"这是人工行为，不是正常订单奖励"。
type CompensateInput struct {
	UserID          uint
	OperatorAdminID uint
	Amount          int64 // > 0
	Reason          string
	Reference       string // AdminCompensationReference(idempotencyKey)
	OrderID         uint   // 可选：关联订单（仅作为流水维度列，不改变 action 语义）
}

// LedgerListFilter 是积分流水查询过滤条件。
// UserID=0 表示不限用户（仅 Admin 跨用户查询使用）；排序固定 created_at DESC, id DESC，
// 不接受客户端指定排序字段（禁止任意字段 SQL sort）。
type LedgerListFilter struct {
	UserID      uint
	ActionType  string
	SourceType  string
	Reference   string
	Direction   string // "" 不限 / income 只看正数 / expense 只看负数
	CreatedFrom time.Time
	CreatedTo   time.Time
	Page        int
	PageSize    int
}

// AccountListFilter 是积分账户查询过滤条件（P4 负余额排查）。
type AccountListFilter struct {
	UserID       uint
	NegativeOnly bool // true: 仅 balance < 0（退款冲正后欠积分用户排查）
	Page         int
	PageSize     int
}

// AccountWithUser 是带余额排查口径的账户视图（Admin 列表用）。
type AccountWithUser struct {
	UserID      uint      `json:"user_id"`
	Balance     int64     `json:"balance"`
	TotalEarned int64     `json:"total_earned"`
	TotalSpent  int64     `json:"total_spent"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// StatsRange 是统计聚合的时间区间（含首不含尾，UTC）。
type StatsRange struct {
	From time.Time
	To   time.Time
}

// LedgerAggregate 是积分流水在给定区间内的聚合结果。
type LedgerAggregate struct {
	Granted     int64 // Σ amount（amount > 0）
	Spent       int64 // Σ |amount|（amount < 0）
	OrderReward int64 // Σ amount WHERE action=ORDER_REWARD
	Mutations   int64 // 流水条数
}

// StatsResult 是后台积分运营看板的基础指标（P4）。
// 全部直接聚合自 points_ledger / user_checkins / points_exchange_orders，
// 不引入"实时累计总表"作为第二事实源。
type StatsResult struct {
	TotalBalance         int64  `json:"total_balance"`          // 全平台当前余额总量（含负值）
	TodayGranted         int64  `json:"today_granted"`          // 今日新增积分
	TodaySpent           int64  `json:"today_spent"`            // 今日消费积分
	TodayOrderReward     int64  `json:"today_order_reward"`     // 今日订单奖励发放
	TodayCheckinUsers    int64  `json:"today_checkin_users"`    // 今日签到人数
	TodayExchanges       int64  `json:"today_exchanges"`        // 今日兑换单数
	PendingExchanges     int64  `json:"pending_exchanges"`      // 待处理兑换单数
	ProcessingExchanges  int64  `json:"processing_exchanges"`   // 处理中兑换单数
	NegativeBalanceUsers int64  `json:"negative_balance_users"` // 负余额账户数
	TodayStart           string `json:"today_start"`            // 业务日边界（RFC3339，便于运营核对口径）
}
