package gormstore

import (
	supportdomain "github.com/Aether-v1/hcz/internal/modules/supportticket/domain"
)

// CreateAttachment 创建附件记录。
func (s *Store) CreateAttachment(a *supportdomain.Attachment) error {
	return s.db.Create(a).Error
}

// GetAttachmentByID 按 ID 取附件。
func (s *Store) GetAttachmentByID(id uint) (*supportdomain.Attachment, error) {
	if id == 0 {
		return nil, nil
	}
	var a supportdomain.Attachment
	if err := s.db.Where("id = ?", id).First(&a).Error; err != nil {
		return nil, firstErr(err)
	}
	return &a, nil
}

// ListAttachmentsByTicket 列出工单全部附件。
func (s *Store) ListAttachmentsByTicket(ticketID uint) ([]supportdomain.Attachment, error) {
	var rows []supportdomain.Attachment
	if err := s.db.Where("ticket_id = ?", ticketID).Order("id asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// LinkAttachmentToTicket 把已上传但未关联工单的附件挂到工单。
func (s *Store) LinkAttachmentToTicket(attachmentID, ticketID uint) error {
	if attachmentID == 0 || ticketID == 0 {
		return nil
	}
	return s.db.Model(&supportdomain.Attachment{}).
		Where("id = ? AND ticket_id = 0", attachmentID).
		UpdateColumn("ticket_id", ticketID).Error
}

// LinkAttachmentToMessage 把附件挂到具体消息（附件应已先挂到工单）。
func (s *Store) LinkAttachmentToMessage(attachmentID, messageID uint) error {
	if attachmentID == 0 || messageID == 0 {
		return nil
	}
	return s.db.Model(&supportdomain.Attachment{}).
		Where("id = ?", attachmentID).
		UpdateColumn("message_id", messageID).Error
}
