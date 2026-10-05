package gormstore

import (
	supportdomain "github.com/Aether-v1/hcz/internal/modules/supportticket/domain"
	"github.com/Aether-v1/hcz/internal/persistence/gormutil"
)

// CreateMessage 创建消息。
func (s *Store) CreateMessage(m *supportdomain.Message) error {
	return s.db.Create(m).Error
}

// ListMessagesByTicket 分页拉取工单消息（创建时间升序）。
func (s *Store) ListMessagesByTicket(ticketID uint, page, pageSize int) ([]supportdomain.Message, int64, error) {
	if ticketID == 0 {
		return []supportdomain.Message{}, 0, nil
	}
	query := s.db.Model(&supportdomain.Message{}).Where("ticket_id = ?", ticketID)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []supportdomain.Message
	if err := gormutil.ApplyPagination(query, page, pageSize).
		Order("created_at ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}
