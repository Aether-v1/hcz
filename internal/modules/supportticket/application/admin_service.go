package application

import (
	"context"
	"encoding/json"
	"strings"

	supportcontract "github.com/Aether-v1/hcz/internal/modules/supportticket/contract"
	supportdomain "github.com/Aether-v1/hcz/internal/modules/supportticket/domain"
	"github.com/Aether-v1/hcz/internal/modules/supportticket/statemachine"
)

// ==================== 客服侧用例 ====================

// Overview 后台概览。
func (s *Service) Overview(myAdminID uint) (supportcontract.OverviewStats, error) {
	return s.repo.OverviewStats(myAdminID)
}

// ListAdminTickets 后台工单列表。
func (s *Service) ListAdminTickets(filter supportcontract.AdminTicketListFilter) ([]supportdomain.Ticket, int64, error) {
	return s.repo.ListAdminTickets(filter)
}

// GetAdminTicket 后台工单详情（含消息/附件/审计 + 清零客服未读）。
func (s *Service) GetAdminTicket(ticketID uint) (*supportdomain.Ticket, []supportdomain.Message, int64, []supportdomain.Attachment, []supportdomain.Audit, error) {
	ticket, err := s.repo.GetTicketByID(ticketID)
	if err != nil {
		return nil, nil, 0, nil, nil, err
	}
	if ticket == nil {
		return nil, nil, 0, nil, nil, supportcontract.ErrTicketNotFound
	}
	messages, total, err := s.repo.ListMessagesByTicket(ticketID, 1, 100)
	if err != nil {
		return nil, nil, 0, nil, nil, err
	}
	attachments, err := s.repo.ListAttachmentsByTicket(ticketID)
	if err != nil {
		return nil, nil, 0, nil, nil, err
	}
	audits, err := s.repo.ListAuditsByTicket(ticketID)
	if err != nil {
		return nil, nil, 0, nil, nil, err
	}
	if err := s.repo.ResetAdminUnread(ticketID); err != nil {
		return nil, nil, 0, nil, nil, err
	}
	ticket.AdminUnreadCount = 0
	return ticket, messages, total, attachments, audits, nil
}

// AdminReply 客服回复。
func (s *Service) AdminReply(ctx context.Context, in supportcontract.AdminReplyInput) (*supportdomain.Ticket, error) {
	in.Body = strings.TrimSpace(in.Body)
	if len(in.Body) < 1 || len(in.Body) > 10000 {
		return nil, supportcontract.ErrInvalidBody
	}

	var (
		updated  *supportdomain.Ticket
		userID   uint
		ticketNo string
	)
	err := s.uow.WithinTransaction(func(tx supportcontract.Transaction) error {
		repo := tx.Tickets()
		ticket, err := repo.GetTicketByIDForUpdate(in.TicketID)
		if err != nil {
			return err
		}
		if ticket == nil {
			return supportcontract.ErrTicketNotFound
		}
		if statemachine.IsTerminal(ticket.Status) {
			return supportcontract.ErrTicketClosed
		}
		next, err := statemachine.Transition(ticket.Status, statemachine.EventAdminReply)
		if err != nil {
			return supportcontract.ErrTicketStatusInvalid
		}
		now := nowTime()
		msg := &supportdomain.Message{
			TicketID:      ticket.ID,
			SenderType:    statemachine.SenderAdmin,
			SenderAdminID: &in.OperatorAdminID,
			Body:          in.Body,
			MessageType:   statemachine.MessageTypeText,
			CreatedAt:     now,
		}
		if err := repo.CreateMessage(msg); err != nil {
			return err
		}
		if err := linkAttachmentsToMessage(repo, in.AttachmentIDs, ticket.ID, msg.ID, statemachine.SenderAdmin); err != nil {
			return err
		}
		ticket.Status = next
		ticket.LastReplyBy = statemachine.SenderAdmin
		ticket.LastRepliedAt = &now
		if err := repo.UpdateTicket(ticket); err != nil {
			return err
		}
		if err := repo.IncrUserUnread(ticket.ID, 1); err != nil {
			return err
		}
		if err := repo.ResetAdminUnread(ticket.ID); err != nil {
			return err
		}
		updated = ticket
		userID = ticket.UserID
		ticketNo = ticket.TicketNo
		return nil
	})
	if err != nil {
		return nil, err
	}
	// 通知（尽力而为，事务提交后）。
	s.notifySafely(ctx, supportcontract.NotificationInput{
		UserID:  userID,
		Type:    "ticket_admin_replied",
		Title:   "客服回复了您的工单",
		Body:    "工单 " + ticketNo + " 有新回复：" + truncateBody(in.Body, 200),
		BizType: "support_ticket",
		BizID:   in.TicketID,
	})
	return updated, nil
}

// AssignTicket 认领或指派。adminID=0 表示认领给自己。
func (s *Service) AssignTicket(ctx context.Context, in supportcontract.AssignTicketInput) (*supportdomain.Ticket, error) {
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
		if statemachine.IsTerminal(ticket.Status) {
			return supportcontract.ErrTicketClosed
		}

		targetAdminID := in.AdminID
		if targetAdminID == 0 {
			// 认领：CAS，仅当未指派时成功。
			rows, err := repo.ClaimTicket(ticket.ID, in.OperatorAdminID)
			if err != nil {
				return err
			}
			if rows == 0 {
				return supportcontract.ErrAlreadyAssigned
			}
			targetAdminID = in.OperatorAdminID
		} else {
			// 指派：校验目标管理员存在。
			if s.admins != nil {
				if _, _, err := s.admins.GetAdminByID(targetAdminID); err != nil {
					return err
				}
			}
			if err := repo.AssignTicket(ticket.ID, targetAdminID); err != nil {
				return err
			}
		}

		before := ""
		after := uintToStr(targetAdminID)
		if ticket.AssignedAdminID != nil {
			before = uintToStr(*ticket.AssignedAdminID)
		}
		audit := &supportdomain.Audit{
			TicketID: ticket.ID,
			AdminID:  in.OperatorAdminID,
			Action:   statemachine.AuditActionAssign,
			Before:   before,
			After:    after,
		}
		if err := repo.CreateAudit(audit); err != nil {
			return err
		}
		ticket.AssignedAdminID = &targetAdminID
		updated = ticket
		return nil
	})
	return updated, err
}

// ChangePriority 修改优先级。
func (s *Service) ChangePriority(ctx context.Context, in supportcontract.ChangePriorityInput) (*supportdomain.Ticket, error) {
	if !statemachine.ValidPriority(in.Priority) {
		return nil, supportcontract.ErrInvalidPriority
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
		if statemachine.IsTerminal(ticket.Status) {
			return supportcontract.ErrTicketClosed
		}
		before := ticket.Priority
		ticket.Priority = in.Priority
		if err := repo.UpdateTicket(ticket); err != nil {
			return err
		}
		audit := &supportdomain.Audit{
			TicketID: ticket.ID,
			AdminID:  in.OperatorAdminID,
			Action:   statemachine.AuditActionPriorityChange,
			Before:   before,
			After:    in.Priority,
		}
		if err := repo.CreateAudit(audit); err != nil {
			return err
		}
		updated = ticket
		return nil
	})
	return updated, err
}

// ResolveTicket 解决工单。
func (s *Service) ResolveTicket(ctx context.Context, in supportcontract.ResolveTicketInput) (*supportdomain.Ticket, error) {
	var (
		updated  *supportdomain.Ticket
		userID   uint
		ticketNo string
	)
	err := s.uow.WithinTransaction(func(tx supportcontract.Transaction) error {
		repo := tx.Tickets()
		ticket, err := repo.GetTicketByIDForUpdate(in.TicketID)
		if err != nil {
			return err
		}
		if ticket == nil {
			return supportcontract.ErrTicketNotFound
		}
		if statemachine.IsTerminal(ticket.Status) {
			return supportcontract.ErrTicketClosed
		}
		next, err := statemachine.Transition(ticket.Status, statemachine.EventAdminResolve)
		if err != nil {
			return err
		}
		now := nowTime()
		ticket.Status = next
		ticket.ResolvedAt = &now
		ticket.LastReplyBy = statemachine.SenderAdmin
		ticket.LastRepliedAt = &now
		if err := repo.UpdateTicket(ticket); err != nil {
			return err
		}
		sysMsg := &supportdomain.Message{
			TicketID:    ticket.ID,
			SenderType:  statemachine.SenderSystem,
			Body:        "Ticket resolved by support",
			MessageType: statemachine.MessageTypeSystemNotice,
			CreatedAt:   now,
		}
		if err := repo.CreateMessage(sysMsg); err != nil {
			return err
		}
		audit := &supportdomain.Audit{
			TicketID: ticket.ID,
			AdminID:  in.OperatorAdminID,
			Action:   statemachine.AuditActionResolve,
			Before:   "",
			After:    next,
			Reason:   in.Reason,
		}
		if err := repo.CreateAudit(audit); err != nil {
			return err
		}
		updated = ticket
		userID = ticket.UserID
		ticketNo = ticket.TicketNo
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.notifySafely(ctx, supportcontract.NotificationInput{
		UserID:  userID,
		Type:    "ticket_resolved",
		Title:   "您的工单已处理完成",
		Body:    "工单 " + ticketNo + " 已被客服标记为已解决",
		BizType: "support_ticket",
		BizID:   in.TicketID,
	})
	return updated, nil
}

// CloseTicket 客服关闭工单。
func (s *Service) CloseTicket(ctx context.Context, in supportcontract.CloseTicketInput) (*supportdomain.Ticket, error) {
	var (
		updated  *supportdomain.Ticket
		userID   uint
		ticketNo string
	)
	err := s.uow.WithinTransaction(func(tx supportcontract.Transaction) error {
		repo := tx.Tickets()
		ticket, err := repo.GetTicketByIDForUpdate(in.TicketID)
		if err != nil {
			return err
		}
		if ticket == nil {
			return supportcontract.ErrTicketNotFound
		}
		if statemachine.IsTerminal(ticket.Status) {
			return supportcontract.ErrTicketClosed
		}
		next, err := statemachine.Transition(ticket.Status, statemachine.EventAdminClose)
		if err != nil {
			return err
		}
		now := nowTime()
		ticket.Status = next
		ticket.ClosedAt = &now
		ticket.LastReplyBy = statemachine.SenderAdmin
		ticket.LastRepliedAt = &now
		if err := repo.UpdateTicket(ticket); err != nil {
			return err
		}
		sysMsg := &supportdomain.Message{
			TicketID:    ticket.ID,
			SenderType:  statemachine.SenderSystem,
			Body:        "Ticket closed by support",
			MessageType: statemachine.MessageTypeSystemNotice,
			CreatedAt:   now,
		}
		if err := repo.CreateMessage(sysMsg); err != nil {
			return err
		}
		audit := &supportdomain.Audit{
			TicketID: ticket.ID,
			AdminID:  in.OperatorAdminID,
			Action:   statemachine.AuditActionClose,
			Before:   "",
			After:    next,
			Reason:   in.Reason,
		}
		if err := repo.CreateAudit(audit); err != nil {
			return err
		}
		updated = ticket
		userID = ticket.UserID
		ticketNo = ticket.TicketNo
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.notifySafely(ctx, supportcontract.NotificationInput{
		UserID:  userID,
		Type:    "ticket_closed",
		Title:   "您的工单已关闭",
		Body:    "工单 " + ticketNo + " 已被客服关闭",
		BizType: "support_ticket",
		BizID:   in.TicketID,
	})
	return updated, nil
}

// ReopenTicket 客服重开工单（resolved 且 7 天内）。
func (s *Service) ReopenTicket(ctx context.Context, in supportcontract.ReopenTicketInput) (*supportdomain.Ticket, error) {
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
		if ticket.Status != statemachine.StatusResolved {
			return supportcontract.ErrTicketStatusInvalid
		}
		if !statemachine.CanReopen(*ticket.ResolvedAt, nowTime()) {
			return supportcontract.ErrReopenExpired
		}
		next, err := statemachine.Transition(ticket.Status, statemachine.EventAdminReopen)
		if err != nil {
			return err
		}
		now := nowTime()
		ticket.Status = next
		ticket.ResolvedAt = nil
		ticket.LastReplyBy = statemachine.SenderAdmin
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
			Body:        "Ticket reopened by support",
			MessageType: statemachine.MessageTypeSystemNotice,
			CreatedAt:   now,
		}
		if err := repo.CreateMessage(sysMsg); err != nil {
			return err
		}
		audit := &supportdomain.Audit{
			TicketID: ticket.ID,
			AdminID:  in.OperatorAdminID,
			Action:   statemachine.AuditActionReopen,
			Before:   statemachine.StatusResolved,
			After:    next,
			Reason:   in.Reason,
		}
		if err := repo.CreateAudit(audit); err != nil {
			return err
		}
		updated = ticket
		return nil
	})
	return updated, err
}

// ListAudits 列出工单审计日志。
func (s *Service) ListAudits(ticketID uint) ([]supportdomain.Audit, error) {
	return s.repo.ListAuditsByTicket(ticketID)
}

// uintToStr 把 uint 转字符串（审计 before/after）。
func uintToStr(v uint) string {
	b, _ := json.Marshal(v)
	return string(b)
}
