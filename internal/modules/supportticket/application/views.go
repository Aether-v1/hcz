package application

import (
	"context"

	supportcontract "github.com/Aether-v1/hcz/internal/modules/supportticket/contract"
	supportdomain "github.com/Aether-v1/hcz/internal/modules/supportticket/domain"
)

// ==================== 视图装配（domain -> view，含 join 名称）====================

// buildTicketView 装配单个工单视图，按需补充分类名/用户邮箱/指派人昵称。
func (s *Service) buildTicketView(t *supportdomain.Ticket, withUser, withAssignee bool) supportcontract.TicketView {
	v := supportcontract.TicketView{
		ID:               t.ID,
		TicketNo:         t.TicketNo,
		UserID:           t.UserID,
		CategoryID:       t.CategoryID,
		Subject:          t.Subject,
		Status:           t.Status,
		Priority:         t.Priority,
		AssignedAdminID:  t.AssignedAdminID,
		BizType:          t.BizType,
		BizID:            t.BizID,
		UserUnreadCount:  t.UserUnreadCount,
		AdminUnreadCount: t.AdminUnreadCount,
		LastReplyBy:      t.LastReplyBy,
		LastRepliedAt:    t.LastRepliedAt,
		CreatedAt:        t.CreatedAt,
	}
	if cat, err := s.repo.GetCategoryByID(t.CategoryID); err == nil && cat != nil {
		v.CategoryName = cat.Name
	}
	if withUser && s.users != nil {
		if _, email, name, err := s.users.GetByID(t.UserID); err == nil {
			v.UserEmail = email
			v.UserName = name
		}
	}
	if withAssignee && t.AssignedAdminID != nil && s.admins != nil {
		if _, username, err := s.admins.GetAdminByID(*t.AssignedAdminID); err == nil {
			v.AssignedAdminName = username
		}
	}
	return v
}

// buildTicketListViews 批量装配工单列表视图。
func (s *Service) buildTicketListViews(rows []supportdomain.Ticket, withUser, withAssignee bool) []supportcontract.TicketView {
	out := make([]supportcontract.TicketView, 0, len(rows))
	for i := range rows {
		out = append(out, s.buildTicketView(&rows[i], withUser, withAssignee))
	}
	return out
}

// buildAttachmentView 装配附件视图。
func buildAttachmentView(a *supportdomain.Attachment) supportcontract.AttachmentView {
	return supportcontract.AttachmentView{
		ID:           a.ID,
		TicketID:     a.TicketID,
		MessageID:    a.MessageID,
		UploaderType: a.UploaderType,
		FileName:     a.FileName,
		MimeType:     a.MimeType,
		FileSize:     a.FileSize,
		CreatedAt:    a.CreatedAt,
	}
}

// buildMessageViews 装配消息视图（按 message_id 分组附件）。
func buildMessageViews(messages []supportdomain.Message, attachments []supportdomain.Attachment) []supportcontract.MessageView {
	attByMsg := make(map[uint][]supportcontract.AttachmentView)
	for i := range attachments {
		a := attachments[i]
		if a.MessageID != nil {
			attByMsg[*a.MessageID] = append(attByMsg[*a.MessageID], buildAttachmentView(&a))
		}
	}
	out := make([]supportcontract.MessageView, 0, len(messages))
	for i := range messages {
		m := messages[i]
		out = append(out, supportcontract.MessageView{
			ID:            m.ID,
			TicketID:      m.TicketID,
			SenderType:    m.SenderType,
			SenderUserID:  m.SenderUserID,
			SenderAdminID: m.SenderAdminID,
			Body:          m.Body,
			MessageType:   m.MessageType,
			CreatedAt:     m.CreatedAt,
			Attachments:   attByMsg[m.ID],
		})
	}
	return out
}

// ==================== 用户侧视图用例 ====================

// ListMyTicketsView 用户侧列表视图。
func (s *Service) ListMyTicketsView(userID uint, filter supportcontract.TicketListFilter) ([]supportcontract.TicketView, int64, error) {
	rows, total, err := s.ListMyTickets(userID, filter)
	if err != nil {
		return nil, 0, err
	}
	return s.buildTicketListViews(rows, false, false), total, nil
}

// GetMyTicketView 用户侧详情视图。
func (s *Service) GetMyTicketView(ctx context.Context, userID, ticketID uint) (*supportcontract.TicketDetailView, error) {
	ticket, messages, total, attachments, err := s.GetMyTicket(userID, ticketID)
	if err != nil {
		return nil, err
	}
	return &supportcontract.TicketDetailView{
		Ticket:        s.buildTicketView(ticket, false, false),
		Messages:      buildMessageViews(messages, attachments),
		MessagesTotal: total,
		Attachments:   buildAttachmentViewList(attachments),
	}, nil
}

// ==================== 客服侧视图用例 ====================

// ListAdminTicketsView 后台列表视图。
func (s *Service) ListAdminTicketsView(filter supportcontract.AdminTicketListFilter) ([]supportcontract.TicketView, int64, error) {
	rows, total, err := s.ListAdminTickets(filter)
	if err != nil {
		return nil, 0, err
	}
	return s.buildTicketListViews(rows, true, true), total, nil
}

// GetAdminTicketView 后台详情视图。
func (s *Service) GetAdminTicketView(ticketID uint) (*supportcontract.TicketDetailView, error) {
	ticket, messages, total, attachments, audits, err := s.GetAdminTicket(ticketID)
	if err != nil {
		return nil, err
	}
	auditViews := make([]supportcontract.AuditView, 0, len(audits))
	for i := range audits {
		a := audits[i]
		auditViews = append(auditViews, supportcontract.AuditView{
			ID:        a.ID,
			TicketID:  a.TicketID,
			AdminID:   a.AdminID,
			Action:    a.Action,
			Before:    a.Before,
			After:     a.After,
			Reason:    a.Reason,
			CreatedAt: a.CreatedAt,
		})
	}
	return &supportcontract.TicketDetailView{
		Ticket:        s.buildTicketView(ticket, true, true),
		Messages:      buildMessageViews(messages, attachments),
		MessagesTotal: total,
		Attachments:   buildAttachmentViewList(attachments),
		Audits:        auditViews,
	}, nil
}

func buildAttachmentViewList(attachments []supportdomain.Attachment) []supportcontract.AttachmentView {
	out := make([]supportcontract.AttachmentView, 0, len(attachments))
	for i := range attachments {
		out = append(out, buildAttachmentView(&attachments[i]))
	}
	return out
}
