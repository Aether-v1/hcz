package application

import (
	"context"
	"errors"
	"mime/multipart"
	"strings"

	supportcontract "github.com/Aether-v1/hcz/internal/modules/supportticket/contract"
	supportdomain "github.com/Aether-v1/hcz/internal/modules/supportticket/domain"
	"github.com/Aether-v1/hcz/internal/modules/supportticket/statemachine"
)

// ==================== 用户侧用例 ====================

// CreateTicket 用户创建工单。
func (s *Service) CreateTicket(ctx context.Context, in supportcontract.CreateTicketInput) (*supportdomain.Ticket, error) {
	in.Subject = strings.TrimSpace(in.Subject)
	in.Body = strings.TrimSpace(in.Body)
	if len(in.Subject) < 1 || len(in.Subject) > 255 {
		return nil, supportcontract.ErrInvalidSubject
	}
	if len(in.Body) < 1 || len(in.Body) > 10000 {
		return nil, supportcontract.ErrInvalidBody
	}

	category, err := s.repo.GetCategoryByID(in.CategoryID)
	if err != nil {
		return nil, err
	}
	if category == nil {
		return nil, supportcontract.ErrCategoryNotFound
	}
	if !category.Enabled {
		return nil, supportcontract.ErrCategoryDisabled
	}

	// Biz 归属校验（只读原始 SQL）。
	bizType := strings.TrimSpace(in.BizType)
	bizID := in.BizID
	if bizType != "" {
		if _, ok := supportcontract.ValidBizTypes[bizType]; !ok {
			return nil, supportcontract.ErrInvalidBizOwnership
		}
		owned, err := s.repo.VerifyBizOwnership(bizType, bizID, in.UserID)
		if err != nil {
			return nil, err
		}
		if !owned {
			return nil, supportcontract.ErrInvalidBizOwnership
		}
	} else {
		bizID = 0
	}

	var created *supportdomain.Ticket
	err = s.uow.WithinTransaction(func(tx supportcontract.Transaction) error {
		repo := tx.Tickets()
		now := nowTime()
		ticket := &supportdomain.Ticket{
			TicketNo:         genTicketNo(),
			UserID:           in.UserID,
			CategoryID:       in.CategoryID,
			Subject:          in.Subject,
			Status:           statemachine.StatusOpen,
			Priority:         category.DefaultPriority,
			BizType:          bizType,
			BizID:            bizID,
			UserUnreadCount:  0,
			AdminUnreadCount: 1,
			LastReplyBy:      statemachine.SenderUser,
			LastRepliedAt:    &now,
			CreatedAt:        now,
			UpdatedAt:        now,
		}
		if err := repo.CreateTicket(ticket); err != nil {
			return err
		}
		msg := &supportdomain.Message{
			TicketID:     ticket.ID,
			SenderType:   statemachine.SenderUser,
			SenderUserID: &in.UserID,
			Body:         in.Body,
			MessageType:  statemachine.MessageTypeText,
			CreatedAt:    now,
		}
		if err := repo.CreateMessage(msg); err != nil {
			return err
		}
		if err := linkAttachmentsToMessage(repo, in.AttachmentIDs, ticket.ID, msg.ID, statemachine.SenderUser); err != nil {
			return err
		}
		created = ticket
		return nil
	})
	return created, err
}

// ListMyTickets 用户侧工单列表。
func (s *Service) ListMyTickets(userID uint, filter supportcontract.TicketListFilter) ([]supportdomain.Ticket, int64, error) {
	filter.UserID = userID
	return s.repo.ListMyTickets(filter)
}

// GetMyTicket 用户侧工单详情（IDOR 校验 + 清零用户未读）。
func (s *Service) GetMyTicket(userID, ticketID uint) (*supportdomain.Ticket, []supportdomain.Message, int64, []supportdomain.Attachment, error) {
	ticket, err := s.repo.GetTicketByID(ticketID)
	if err != nil {
		return nil, nil, 0, nil, err
	}
	if ticket == nil {
		return nil, nil, 0, nil, supportcontract.ErrTicketNotFound
	}
	if ticket.UserID != userID {
		return nil, nil, 0, nil, supportcontract.ErrNotOwner
	}
	messages, total, err := s.repo.ListMessagesByTicket(ticketID, 1, 100)
	if err != nil {
		return nil, nil, 0, nil, err
	}
	attachments, err := s.repo.ListAttachmentsByTicket(ticketID)
	if err != nil {
		return nil, nil, 0, nil, err
	}
	// 打开即清零用户未读（原子）。
	if err := s.repo.ResetUserUnread(ticketID); err != nil {
		return nil, nil, 0, nil, err
	}
	ticket.UserUnreadCount = 0
	return ticket, messages, total, attachments, nil
}

// ReplyToTicket 用户回复。
func (s *Service) ReplyToTicket(ctx context.Context, in supportcontract.ReplyTicketInput) (*supportdomain.Ticket, error) {
	in.Body = strings.TrimSpace(in.Body)
	if len(in.Body) < 1 || len(in.Body) > 10000 {
		return nil, supportcontract.ErrInvalidBody
	}

	var updated *supportdomain.Ticket
	err := s.uow.WithinTransaction(func(tx supportcontract.Transaction) error {
		repo := tx.Tickets()
		ticket, err := repo.GetTicketByIDForUpdate(in.TicketID)
		if err != nil {
			return err
		}
		if ticket == nil {
			return supportcontract.ErrTicketNotFound
		}
		if ticket.UserID != in.UserID {
			return supportcontract.ErrNotOwner
		}
		if statemachine.IsTerminal(ticket.Status) {
			return supportcontract.ErrTicketClosed
		}
		// resolved 状态下用户回复必须先重开。
		if ticket.Status == statemachine.StatusResolved {
			return supportcontract.ErrTicketStatusInvalid
		}
		next, err := statemachine.Transition(ticket.Status, statemachine.EventUserReply)
		if err != nil {
			return supportcontract.ErrTicketStatusInvalid
		}
		now := nowTime()
		msg := &supportdomain.Message{
			TicketID:     ticket.ID,
			SenderType:   statemachine.SenderUser,
			SenderUserID: &in.UserID,
			Body:         in.Body,
			MessageType:  statemachine.MessageTypeText,
			CreatedAt:    now,
		}
		if err := repo.CreateMessage(msg); err != nil {
			return err
		}
		if err := linkAttachmentsToMessage(repo, in.AttachmentIDs, ticket.ID, msg.ID, statemachine.SenderUser); err != nil {
			return err
		}
		ticket.Status = next
		ticket.LastReplyBy = statemachine.SenderUser
		ticket.LastRepliedAt = &now
		if err := repo.UpdateTicket(ticket); err != nil {
			return err
		}
		// admin 未读 +1，user 未读清零。
		if err := repo.IncrAdminUnread(ticket.ID, 1); err != nil {
			return err
		}
		if err := repo.ResetUserUnread(ticket.ID); err != nil {
			return err
		}
		updated = ticket
		return nil
	})
	return updated, err
}

// CloseMyTicket 用户关闭工单。
func (s *Service) CloseMyTicket(ctx context.Context, userID, ticketID uint) (*supportdomain.Ticket, error) {
	var updated *supportdomain.Ticket
	err := s.uow.WithinTransaction(func(tx supportcontract.Transaction) error {
		repo := tx.Tickets()
		ticket, err := repo.GetTicketByIDForUpdate(ticketID)
		if err != nil {
			return err
		}
		if ticket == nil {
			return supportcontract.ErrTicketNotFound
		}
		if ticket.UserID != userID {
			return supportcontract.ErrNotOwner
		}
		if statemachine.IsTerminal(ticket.Status) {
			return supportcontract.ErrTicketClosed
		}
		next, err := statemachine.Transition(ticket.Status, statemachine.EventUserClose)
		if err != nil {
			return supportcontract.ErrTicketStatusInvalid
		}
		now := nowTime()
		ticket.Status = next
		ticket.ClosedAt = &now
		ticket.LastReplyBy = statemachine.SenderUser
		ticket.LastRepliedAt = &now
		if err := repo.UpdateTicket(ticket); err != nil {
			return err
		}
		sysMsg := &supportdomain.Message{
			TicketID:    ticket.ID,
			SenderType:  statemachine.SenderSystem,
			Body:        "User closed the ticket",
			MessageType: statemachine.MessageTypeSystemNotice,
			CreatedAt:   now,
		}
		if err := repo.CreateMessage(sysMsg); err != nil {
			return err
		}
		updated = ticket
		return nil
	})
	return updated, err
}

// ReopenMyTicket 用户重开工单（仅 resolved 且 7 天内）。
func (s *Service) ReopenMyTicket(ctx context.Context, userID, ticketID uint) (*supportdomain.Ticket, error) {
	var updated *supportdomain.Ticket
	err := s.uow.WithinTransaction(func(tx supportcontract.Transaction) error {
		repo := tx.Tickets()
		ticket, err := repo.GetTicketByIDForUpdate(ticketID)
		if err != nil {
			return err
		}
		if ticket == nil {
			return supportcontract.ErrTicketNotFound
		}
		if ticket.UserID != userID {
			return supportcontract.ErrNotOwner
		}
		if ticket.Status != statemachine.StatusResolved {
			return supportcontract.ErrTicketStatusInvalid
		}
		if !statemachine.CanReopen(*ticket.ResolvedAt, nowTime()) {
			return supportcontract.ErrReopenExpired
		}
		next, err := statemachine.Transition(ticket.Status, statemachine.EventUserReopen)
		if err != nil {
			return supportcontract.ErrTicketStatusInvalid
		}
		now := nowTime()
		ticket.Status = next
		ticket.ResolvedAt = nil
		ticket.LastReplyBy = statemachine.SenderUser
		ticket.LastRepliedAt = &now
		if err := repo.UpdateTicket(ticket); err != nil {
			return err
		}
		if err := repo.IncrAdminUnread(ticket.ID, 1); err != nil {
			return err
		}
		sysMsg := &supportdomain.Message{
			TicketID:    ticket.ID,
			SenderType:  statemachine.SenderSystem,
			Body:        "User reopened the ticket",
			MessageType: statemachine.MessageTypeSystemNotice,
			CreatedAt:   now,
		}
		if err := repo.CreateMessage(sysMsg); err != nil {
			return err
		}
		updated = ticket
		return nil
	})
	return updated, err
}

// UploadAttachment 上传附件（用户侧）。返回附件记录。
func (s *Service) UploadAttachment(userID uint, file *multipart.FileHeader) (*supportdomain.Attachment, error) {
	if s.uploader == nil {
		return nil, errors.New("uploader not configured")
	}
	if file == nil {
		return nil, supportcontract.ErrInvalidBody
	}
	result, err := s.uploader.SaveFileWithMeta(file, supportcontract.SceneSupportTicket)
	if err != nil {
		return nil, err
	}
	// ObjectKey 存储相对 uploads 根目录的路径。
	objectKey := strings.TrimPrefix(result.URL, "/uploads/")
	att := &supportdomain.Attachment{
		TicketID:     0,
		UploaderType: statemachine.SenderUser,
		FileName:     result.Filename,
		ObjectKey:    objectKey,
		MimeType:     result.MimeType,
		FileSize:     result.Size,
	}
	if err := s.repo.CreateAttachment(att); err != nil {
		return nil, err
	}
	return att, nil
}

// DownloadAttachment 用户下载附件（IDOR：必须属于该用户参与的工单）。
func (s *Service) DownloadAttachment(userID, attachmentID uint) (*supportcontract.AttachmentDownload, error) {
	att, err := s.repo.GetAttachmentByID(attachmentID)
	if err != nil {
		return nil, err
	}
	if att == nil {
		return nil, supportcontract.ErrAttachmentNotFound
	}
	ticket, err := s.repo.GetTicketByID(att.TicketID)
	if err != nil {
		return nil, err
	}
	if ticket == nil || ticket.UserID != userID {
		return nil, supportcontract.ErrNotOwner
	}
	if s.filer == nil {
		return nil, errors.New("file store not configured")
	}
	reader, err := s.filer.ReadObject(att.ObjectKey)
	if err != nil {
		return nil, err
	}
	return &supportcontract.AttachmentDownload{
		Attachment: att,
		Reader:     reader,
		MimeType:   att.MimeType,
		FileName:   att.FileName,
	}, nil
}
