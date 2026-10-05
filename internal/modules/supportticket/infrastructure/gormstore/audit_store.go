package gormstore

import (
	supportdomain "github.com/Aether-v1/hcz/internal/modules/supportticket/domain"
)

// CreateAudit 创建审计记录。
func (s *Store) CreateAudit(a *supportdomain.Audit) error {
	return s.db.Create(a).Error
}

// ListAuditsByTicket 列出工单审计日志（创建时间倒序）。
func (s *Store) ListAuditsByTicket(ticketID uint) ([]supportdomain.Audit, error) {
	if ticketID == 0 {
		return []supportdomain.Audit{}, nil
	}
	var rows []supportdomain.Audit
	if err := s.db.Where("ticket_id = ?", ticketID).
		Order("created_at DESC, id DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}
