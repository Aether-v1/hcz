package container

import (
	"fmt"

	affiliateapp "github.com/Aether-v1/hcz/internal/modules/affiliate/application"
	auditlogapp "github.com/Aether-v1/hcz/internal/modules/auditlog/application"
	c2ccontract "github.com/Aether-v1/hcz/internal/modules/c2c/contract"
	"github.com/Aether-v1/hcz/internal/shared/jsonmap"
)

// c2cArbitrationAuditWriter 把 C2C 仲裁审计写入 AuthzAuditLog。
type c2cArbitrationAuditWriter struct {
	audit *auditlogapp.AuthzService
}

func (w c2cArbitrationAuditWriter) WriteC2CArbitration(entry c2ccontract.ArbitrationAuditEntry) error {
	if w.audit == nil {
		return nil
	}
	return w.audit.Record(auditlogapp.AuthzRecord{
		OperatorAdminID: entry.AdminID,
		Action:          "c2c_arbitration",
		Object:          fmt.Sprintf("c2c_trade:%d", entry.TradeID),
		Detail: jsonmap.JSON{
			"trade_id":   entry.TradeID,
			"result":     entry.Result,
			"reason":     entry.Reason,
			"admin_note": entry.AdminNote,
		},
	})
}

// c2cPaymentMethodAuditWriter 把收款方式变更审计写入 AuthzAuditLog（复用现有审计基础设施）。
// 用户作为 actor，UserID 落入 OperatorAdminID 列；detail 仅记录非敏感的 user_id/pm_id/type。
type c2cPaymentMethodAuditWriter struct {
	audit *auditlogapp.AuthzService
}

var _ c2ccontract.PaymentMethodAuditWriter = c2cPaymentMethodAuditWriter{}

func (w c2cPaymentMethodAuditWriter) WritePaymentMethodAudit(entry c2ccontract.PaymentMethodAuditEntry) error {
	if w.audit == nil {
		return nil
	}
	return w.audit.Record(auditlogapp.AuthzRecord{
		OperatorAdminID: entry.UserID,
		Action:          entry.Action,
		Object:          fmt.Sprintf("c2c_payment_method:%d", entry.PaymentMethodID),
		Detail: jsonmap.JSON{
			"user_id":           entry.UserID,
			"payment_method_id": entry.PaymentMethodID,
			"type":              entry.PMType,
		},
	})
}

// affiliateAuditRecorder 把推广业务审计写入 AuthzAuditLog（复用现有审计基础设施）。
type affiliateAuditRecorder struct {
	audit *auditlogapp.AuthzService
}

var _ affiliateapp.AuditRecorder = affiliateAuditRecorder{}

func (r affiliateAuditRecorder) RecordAffiliateAudit(action string, actorID uint, targetUserID uint, metadata map[string]interface{}) error {
	if r.audit == nil {
		return nil
	}
	detail := jsonmap.JSON{"target_user_id": targetUserID}
	for k, v := range metadata {
		detail[k] = v
	}
	return r.audit.Record(auditlogapp.AuthzRecord{
		OperatorAdminID: actorID,
		Action:          action,
		Object:          "affiliate",
		Detail:          detail,
	})
}
