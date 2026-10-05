package gormstore

import (
	"strings"

	supportcontract "github.com/Aether-v1/hcz/internal/modules/supportticket/contract"
	supportdomain "github.com/Aether-v1/hcz/internal/modules/supportticket/domain"
	"github.com/Aether-v1/hcz/internal/persistence/gormutil"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CreateTicket 创建工单。
func (s *Store) CreateTicket(t *supportdomain.Ticket) error {
	return s.db.Create(t).Error
}

// UpdateTicket 更新工单。
func (s *Store) UpdateTicket(t *supportdomain.Ticket) error {
	return s.db.Save(t).Error
}

// GetTicketByID 按 ID 取工单。
func (s *Store) GetTicketByID(id uint) (*supportdomain.Ticket, error) {
	if id == 0 {
		return nil, nil
	}
	var t supportdomain.Ticket
	if err := s.db.Where("id = ?", id).First(&t).Error; err != nil {
		return nil, firstErr(err)
	}
	return &t, nil
}

// GetTicketByIDForUpdate 按 ID 取工单并加行锁（SELECT ... FOR UPDATE）。
func (s *Store) GetTicketByIDForUpdate(id uint) (*supportdomain.Ticket, error) {
	if id == 0 {
		return nil, nil
	}
	var t supportdomain.Ticket
	if err := s.db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", id).First(&t).Error; err != nil {
		return nil, firstErr(err)
	}
	return &t, nil
}

// ListMyTickets 用户侧工单列表。
func (s *Store) ListMyTickets(filter supportcontract.TicketListFilter) ([]supportdomain.Ticket, int64, error) {
	query := s.db.Model(&supportdomain.Ticket{}).Where("user_id = ?", filter.UserID)
	if strings.TrimSpace(filter.Status) != "" {
		query = query.Where("status = ?", filter.Status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []supportdomain.Ticket
	// 最近回复优先（NULLS LAST：从未回复的排最后），再按创建时间倒序。
	if err := gormutil.ApplyPagination(query, filter.Page, filter.PageSize).
		Order("last_replied_at DESC NULLS LAST, created_at DESC, id DESC").
		Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// ListAdminTickets 后台工单列表。
func (s *Store) ListAdminTickets(filter supportcontract.AdminTicketListFilter) ([]supportdomain.Ticket, int64, error) {
	query := s.db.Model(&supportdomain.Ticket{})
	if strings.TrimSpace(filter.Status) != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.CategoryID != 0 {
		query = query.Where("category_id = ?", filter.CategoryID)
	}
	if strings.TrimSpace(filter.Priority) != "" {
		query = query.Where("priority = ?", filter.Priority)
	}
	if filter.UnassignedOnly {
		query = query.Where("assigned_admin_id IS NULL")
	}
	if filter.MyAssignedOnly != 0 {
		query = query.Where("assigned_admin_id = ?", filter.MyAssignedOnly)
	}
	if filter.UnreadOnly {
		query = query.Where("admin_unread_count > 0")
	}
	if keyword := strings.TrimSpace(filter.Search); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("subject LIKE ? OR ticket_no LIKE ?", like, like)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []supportdomain.Ticket
	// 优先级高在前，创建早在前（先处理积压）。CASE 表达式兼容 MySQL/SQLite。
	if err := gormutil.ApplyPagination(query, filter.Page, filter.PageSize).
		Order("CASE priority WHEN 'urgent' THEN 1 WHEN 'high' THEN 2 WHEN 'normal' THEN 3 ELSE 4 END ASC, created_at ASC, id ASC").
		Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// OverviewStats 概览统计。
func (s *Store) OverviewStats(myAdminID uint) (supportcontract.OverviewStats, error) {
	var out supportcontract.OverviewStats
	base := s.db.Model(&supportdomain.Ticket{})
	count := func(where string, args ...interface{}) (int64, error) {
		var n int64
		err := base.Session(&gorm.Session{}).Where(where, args...).Count(&n).Error
		return n, err
	}
	var err error
	if out.Open, err = count("status = ?", "open"); err != nil {
		return out, err
	}
	if out.WaitingUser, err = count("status = ?", "waiting_user"); err != nil {
		return out, err
	}
	if out.WaitingSupport, err = count("status = ?", "waiting_support"); err != nil {
		return out, err
	}
	if out.Resolved, err = count("status = ?", "resolved"); err != nil {
		return out, err
	}
	if out.Closed, err = count("status = ?", "closed"); err != nil {
		return out, err
	}
	if out.Unassigned, err = count("assigned_admin_id IS NULL AND status <> ?", "closed"); err != nil {
		return out, err
	}
	if myAdminID != 0 {
		if out.MyAssigned, err = count("assigned_admin_id = ? AND status <> ?", myAdminID, "closed"); err != nil {
			return out, err
		}
	}
	return out, nil
}

// IncrAdminUnread 原子累加 admin_unread_count。
func (s *Store) IncrAdminUnread(ticketID uint, delta int) error {
	if ticketID == 0 || delta == 0 {
		return nil
	}
	return s.db.Model(&supportdomain.Ticket{}).
		Where("id = ?", ticketID).
		UpdateColumn("admin_unread_count", gorm.Expr("admin_unread_count + ?", delta)).Error
}

// IncrUserUnread 原子累加 user_unread_count。
func (s *Store) IncrUserUnread(ticketID uint, delta int) error {
	if ticketID == 0 || delta == 0 {
		return nil
	}
	return s.db.Model(&supportdomain.Ticket{}).
		Where("id = ?", ticketID).
		UpdateColumn("user_unread_count", gorm.Expr("user_unread_count + ?", delta)).Error
}

// ResetUserUnread 清零 user_unread_count。
func (s *Store) ResetUserUnread(ticketID uint) error {
	if ticketID == 0 {
		return nil
	}
	return s.db.Model(&supportdomain.Ticket{}).
		Where("id = ?", ticketID).
		UpdateColumn("user_unread_count", 0).Error
}

// ResetAdminUnread 清零 admin_unread_count。
func (s *Store) ResetAdminUnread(ticketID uint) error {
	if ticketID == 0 {
		return nil
	}
	return s.db.Model(&supportdomain.Ticket{}).
		Where("id = ?", ticketID).
		UpdateColumn("admin_unread_count", 0).Error
}

// ClaimTicket 认领：仅当 assigned_admin_id IS NULL 时更新为新 admin。返回受影响行数。
func (s *Store) ClaimTicket(ticketID, adminID uint) (int64, error) {
	if ticketID == 0 || adminID == 0 {
		return 0, nil
	}
	res := s.db.Model(&supportdomain.Ticket{}).
		Where("id = ? AND assigned_admin_id IS NULL", ticketID).
		UpdateColumn("assigned_admin_id", adminID)
	return res.RowsAffected, res.Error
}

// AssignTicket 直接指派（覆盖现有 assigned_admin_id）。
func (s *Store) AssignTicket(ticketID, adminID uint) error {
	if ticketID == 0 {
		return nil
	}
	return s.db.Model(&supportdomain.Ticket{}).
		Where("id = ?", ticketID).
		UpdateColumn("assigned_admin_id", adminID).Error
}

// VerifyBizOwnership 只读校验业务资源是否属于该用户。
// 红线：仅只读原始 SQL，不注入任何资金业务 service。
func (s *Store) VerifyBizOwnership(bizType string, bizID uint, userID uint) (bool, error) {
	if bizID == 0 || userID == 0 {
		return false, nil
	}
	var query string
	switch bizType {
	case supportcontract.BizTypeOrder:
		query = "SELECT 1 FROM orders WHERE id = ? AND user_id = ? AND deleted_at IS NULL LIMIT 1"
	case supportcontract.BizTypeWithdrawal:
		query = "SELECT 1 FROM wallet_withdrawals WHERE id = ? AND user_id = ? LIMIT 1"
	case supportcontract.BizTypeC2CTrade:
		query = "SELECT 1 FROM c2c_trades WHERE id = ? AND (buyer_user_id = ? OR seller_user_id = ?) LIMIT 1"
	case supportcontract.BizTypeRecharge:
		query = "SELECT 1 FROM wallet_recharge_orders WHERE id = ? AND user_id = ? LIMIT 1"
	default:
		return false, nil
	}

	var rows int64
	var err error
	if bizType == supportcontract.BizTypeC2CTrade {
		err = s.db.Raw(query, bizID, userID, userID).Count(&rows).Error
	} else {
		err = s.db.Raw(query, bizID, userID).Count(&rows).Error
	}
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}
