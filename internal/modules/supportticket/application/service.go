// Package application 实现工单系统的用例编排：用户侧、客服侧、分类管理。
// Handler 只做参数解析与鉴权，所有业务规则在本层完成。
package application

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Aether-v1/hcz/internal/logger"
	supportcontract "github.com/Aether-v1/hcz/internal/modules/supportticket/contract"

	"github.com/google/uuid"
)

// Options 装配工单用例所需依赖。
type Options struct {
	Repository supportcontract.Repository
	UnitOfWork supportcontract.UnitOfWork
	Users      supportcontract.UserReader
	Admins     supportcontract.AdminReader
	Notifier   supportcontract.NotificationCreator
	Uploader   supportcontract.FileUploader
	Filer      supportcontract.FileStreamer
}

// Service 工单用例入口。
type Service struct {
	repo    supportcontract.Repository
	uow     supportcontract.UnitOfWork
	users   supportcontract.UserReader
	admins  supportcontract.AdminReader
	notifier supportcontract.NotificationCreator
	uploader supportcontract.FileUploader
	filer   supportcontract.FileStreamer
}

// NewService 创建工单 Service。
func NewService(opts Options) *Service {
	if opts.Repository == nil || opts.UnitOfWork == nil {
		panic("supportticket service: repository/unitofwork is nil")
	}
	return &Service{
		repo:     opts.Repository,
		uow:      opts.UnitOfWork,
		users:    opts.Users,
		admins:   opts.Admins,
		notifier: opts.Notifier,
		uploader: opts.Uploader,
		filer:    opts.Filer,
	}
}

// genTicketNo 生成工单号：TKT- + uuid 前 12 位，不可预测。
func genTicketNo() string {
	return "TKT-" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
}

// truncateBody 截断通知正文（最长 200 字符）。
func truncateBody(body string, n int) string {
	body = strings.TrimSpace(body)
	if len(body) <= n {
		return body
	}
	return body[:n] + "..."
}

// notifySafely 事务提交后写入用户通知，失败仅记日志、不阻塞主流程。
func (s *Service) notifySafely(ctx context.Context, in supportcontract.NotificationInput) {
	if s == nil || s.notifier == nil {
		return
	}
	defer func() { _ = recover() }()
	if err := s.notifier.CreateNotification(ctx, in); err != nil {
		logger.Warnw("supportticket_notify_failed",
			"user_id", in.UserID, "type", in.Type, "biz_id", in.BizID, "error", err)
	}
}

// now 当前时间。
func nowTime() time.Time { return time.Now() }

// linkAttachmentsToMessage 在事务内把 attachment_ids 挂到工单与消息。
// 仅允许挂当前上传者自己的、尚未关联工单（ticket_id=0）的附件。
func linkAttachmentsToMessage(txRepo supportcontract.Repository, attachmentIDs []uint, ticketID, messageID uint, uploaderType string) error {
	for _, attID := range attachmentIDs {
		if attID == 0 {
			continue
		}
		att, err := txRepo.GetAttachmentByID(attID)
		if err != nil {
			return err
		}
		if att == nil || att.TicketID != 0 || att.UploaderType != uploaderType {
			return fmt.Errorf("%w: attachment %d", supportcontract.ErrAttachmentNotFound, attID)
		}
		if err := txRepo.LinkAttachmentToTicket(attID, ticketID); err != nil {
			return err
		}
		if err := txRepo.LinkAttachmentToMessage(attID, messageID); err != nil {
			return err
		}
	}
	return nil
}
