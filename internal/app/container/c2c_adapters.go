package container

import (
	"fmt"

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
