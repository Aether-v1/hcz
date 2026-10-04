package application

import (
	"context"
	"errors"

	"github.com/Aether-v1/hcz/internal/logger"
	"github.com/Aether-v1/hcz/internal/modules/usernotification/contract"
	"github.com/Aether-v1/hcz/internal/modules/usernotification/domain"
)

// Service 用户站内通知用例。
// 同时实现 contract.Creator（业务模块写入）与 contract.UseCase（用户收件箱读取/标记）。
type Service struct {
	repo contract.Repository
}

var (
	_ contract.Creator = (*Service)(nil)
	_ contract.UseCase = (*Service)(nil)
)

// NewService 构造用户通知服务。
func NewService(repo contract.Repository) *Service {
	if repo == nil {
		panic("usernotification service: repo is nil")
	}
	return &Service{repo: repo}
}

// CreateNotification 写入一条用户通知（幂等）。
// 唯一约束 (user_id,biz_type,biz_id,type) 冲突时静默返回 nil（回调重放不报错、不重复）；
// 其余错误返回给调用方，由业务侧 log.Warnw 降级，不阻塞主业务事务。
func (s *Service) CreateNotification(ctx context.Context, in contract.CreateInput) error {
	if in.UserID == 0 || in.Type == "" {
		// 游客（UserID=0）或无类型通知不入库，静默忽略。
		return nil
	}
	n := &domain.UserNotification{
		UserID:  in.UserID,
		Type:    in.Type,
		Title:   in.Title,
		Body:    in.Body,
		Data:    in.Data,
		BizType: in.BizType,
		BizID:   in.BizID,
	}
	if err := s.repo.Create(ctx, n); err != nil {
		if errors.Is(err, contract.ErrAlreadyExists) {
			return nil
		}
		logger.Warnw("usernotification_create_failed",
			"user_id", in.UserID,
			"type", in.Type,
			"biz_type", in.BizType,
			"biz_id", in.BizID,
			"error", err,
		)
		return err
	}
	return nil
}

// ListByUser 分页拉取当前用户通知。
func (s *Service) ListByUser(ctx context.Context, userID uint, page, pageSize int) ([]domain.UserNotification, int64, error) {
	return s.repo.ListByUser(ctx, userID, page, pageSize)
}

// CountUnread 未读数。
func (s *Service) CountUnread(ctx context.Context, userID uint) (int64, error) {
	return s.repo.CountUnread(ctx, userID)
}

// MarkRead 标记单条已读（幂等）；通知不存在或不属于当前用户时返回 ErrNotFound（404，不暴露存在性）。
func (s *Service) MarkRead(ctx context.Context, id, userID uint) error {
	rows, err := s.repo.MarkRead(ctx, id, userID)
	if err != nil {
		return err
	}
	if rows == 0 {
		return contract.ErrNotFound
	}
	return nil
}

// MarkAllRead 一键全部已读。
func (s *Service) MarkAllRead(ctx context.Context, userID uint) (int64, error) {
	return s.repo.MarkAllRead(ctx, userID)
}
