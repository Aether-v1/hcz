package contract

// 业务关联类型（工单创建时可携带，用于归属校验）。
const (
	BizTypeOrder      = "order"
	BizTypeWithdrawal = "withdrawal"
	BizTypeC2CTrade   = "c2c_trade"
	BizTypeRecharge   = "recharge"
)

// SceneSupportTicket 上传场景。
const SceneSupportTicket = "support_ticket"

// ValidBizTypes 合法的业务关联类型集合。
var ValidBizTypes = map[string]struct{}{
	BizTypeOrder:      {},
	BizTypeWithdrawal: {},
	BizTypeC2CTrade:   {},
	BizTypeRecharge:   {},
}
