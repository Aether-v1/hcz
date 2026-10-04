package gormstore

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/Aether-v1/hcz/internal/modules/usernotification/contract"
	"github.com/Aether-v1/hcz/internal/modules/usernotification/domain"
	"github.com/Aether-v1/hcz/internal/persistence/gormutil"

	"gorm.io/gorm"
)

// Store 用户通知 GORM 持久化实现。
type Store struct {
	db *gorm.DB
}

var _ contract.Repository = (*Store)(nil)

// New 构造用户通知 store。
func New(db *gorm.DB) *Store {
	return &Store{db: db}
}

// Create 插入通知；唯一约束冲突时返回 ErrAlreadyExists。
func (s *Store) Create(ctx context.Context, n *domain.UserNotification) error {
	if n == nil {
		return errors.New("user notification is nil")
	}
	if err := s.db.WithContext(ctx).Create(n).Error; err != nil {
		if isDuplicateKeyError(err) {
			return contract.ErrAlreadyExists
		}
		return err
	}
	return nil
}

// ListByUser 按 created_at DESC 分页。
func (s *Store) ListByUser(ctx context.Context, userID uint, page, pageSize int) ([]domain.UserNotification, int64, error) {
	if userID == 0 {
		return []domain.UserNotification{}, 0, nil
	}
	query := s.db.WithContext(ctx).Model(&domain.UserNotification{}).Where("user_id = ?", userID)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []domain.UserNotification
	if err := gormutil.ApplyPagination(query.Order("created_at DESC, id DESC"), page, pageSize).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// CountUnread 统计未读数。
func (s *Store) CountUnread(ctx context.Context, userID uint) (int64, error) {
	if userID == 0 {
		return 0, nil
	}
	var total int64
	if err := s.db.WithContext(ctx).Model(&domain.UserNotification{}).
		Where("user_id = ? AND is_read = ?", userID, false).
		Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

// MarkRead 带 user_id 条件标记单条已读，返回受影响行数。
// 幂等：总是刷新 read_at（值变化），故 RowsAffected>=1 代表「该通知归属当前用户」（无论此前是否已读）；
// RowsAffected==0 代表不存在或不属于当前用户（不暴露存在性）。
func (s *Store) MarkRead(ctx context.Context, id, userID uint) (int64, error) {
	if id == 0 || userID == 0 {
		return 0, nil
	}
	now := time.Now()
	result := s.db.WithContext(ctx).Model(&domain.UserNotification{}).
		Where("id = ? AND user_id = ?", id, userID).
		Updates(map[string]interface{}{"is_read": true, "read_at": now})
	if result.Error != nil {
		return 0, result.Error
	}
	return result.RowsAffected, nil
}

// MarkAllRead 一键全部已读，返回实际标记条数。
func (s *Store) MarkAllRead(ctx context.Context, userID uint) (int64, error) {
	if userID == 0 {
		return 0, nil
	}
	now := time.Now()
	result := s.db.WithContext(ctx).Model(&domain.UserNotification{}).
		Where("user_id = ? AND is_read = ?", userID, false).
		Updates(map[string]interface{}{"is_read": true, "read_at": now})
	if result.Error != nil {
		return 0, result.Error
	}
	return result.RowsAffected, nil
}

// isDuplicateKeyError 兼容 GORM 标准化错误与底层驱动（SQLite pure-Go / MySQL）原始报错，
// 确保唯一约束冲突在测试（SQLite）与生产（MySQL）下都能被识别。
func isDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique constraint failed") || // sqlite
		strings.Contains(msg, "duplicate entry") || // mysql
		strings.Contains(msg, "error 1062")
}
