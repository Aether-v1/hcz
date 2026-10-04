package gormstore

import (
	"errors"
	"strings"
	"time"

	walletcontract "github.com/Aether-v1/hcz/internal/modules/wallet/contract"
	walletgormstore "github.com/Aether-v1/hcz/internal/modules/wallet/infrastructure/gormstore"
	withdrawalcontract "github.com/Aether-v1/hcz/internal/modules/walletwithdrawal/contract"
	withdrawaldomain "github.com/Aether-v1/hcz/internal/modules/walletwithdrawal/domain"
	"github.com/Aether-v1/hcz/internal/persistence/gormutil"
	"github.com/shopspring/decimal"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Store 同时实现 withdrawal Repository 与 UnitOfWork。
// 持有 walletStore 引用，以便在 WithinTransaction 内绑定同一 *gorm.DB，
// 实现提现事务与钱包 ledger/账户行锁的共享事务。
type Store struct {
	db          *gorm.DB
	walletStore *walletgormstore.Store
}

var (
	_ withdrawalcontract.Repository = (*Store)(nil)
	_ withdrawalcontract.UnitOfWork = (*Store)(nil)
)

// New 创建提现 Store。walletStore 用于共享事务绑定。
func New(db *gorm.DB, walletStore *walletgormstore.Store) *Store {
	return &Store{db: db, walletStore: walletStore}
}

// Bind 返回绑定到新事务 tx 的 Store。
func (s *Store) Bind(tx *gorm.DB) *Store {
	if tx == nil {
		return s
	}
	return New(tx, s.walletStore)
}

type transaction struct {
	withdrawals *Store
	wallets     walletcontract.Repository
}

func (tx transaction) Withdrawals() withdrawalcontract.Repository { return tx.withdrawals }
func (tx transaction) Wallets() walletcontract.Repository         { return tx.wallets }

// UseTransaction 用已开启的 *gorm.DB 构造 withdrawal Transaction，并绑定钱包仓库。
func UseTransaction(tx *gorm.DB, walletStore *walletgormstore.Store) withdrawalcontract.Transaction {
	if tx == nil {
		return nil
	}
	return transaction{
		withdrawals: New(tx, walletStore),
		wallets:     walletStore.Bind(tx),
	}
}

// WithinTransaction 开启提现事务，回调内 Wallets() 与 Withdrawals() 共享同一 *gorm.DB。
func (s *Store) WithinTransaction(fn func(withdrawalcontract.Transaction) error) error {
	if fn == nil {
		return nil
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		return fn(UseTransaction(tx, s.walletStore))
	})
}

func (s *Store) CreateWithdrawal(w *withdrawaldomain.Withdrawal) error {
	return s.db.Create(w).Error
}

func (s *Store) UpdateWithdrawal(w *withdrawaldomain.Withdrawal) error {
	return s.db.Save(w).Error
}

func (s *Store) GetWithdrawalByID(id uint) (*withdrawaldomain.Withdrawal, error) {
	if id == 0 {
		return nil, nil
	}
	var w withdrawaldomain.Withdrawal
	if err := s.db.Where("id = ? AND deleted_at IS NULL", id).First(&w).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &w, nil
}

func (s *Store) GetWithdrawalByIDForUpdate(id uint) (*withdrawaldomain.Withdrawal, error) {
	if id == 0 {
		return nil, nil
	}
	var w withdrawaldomain.Withdrawal
	if err := s.db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND deleted_at IS NULL", id).
		First(&w).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &w, nil
}

func (s *Store) GetWithdrawalByReference(reference string) (*withdrawaldomain.Withdrawal, error) {
	reference = strings.TrimSpace(reference)
	if reference == "" {
		return nil, nil
	}
	var w withdrawaldomain.Withdrawal
	if err := s.db.Where("reference = ? AND deleted_at IS NULL", reference).First(&w).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &w, nil
}

func (s *Store) ListWithdrawals(filter withdrawalcontract.WithdrawalListFilter) ([]withdrawaldomain.Withdrawal, int64, error) {
	query := s.db.Model(&withdrawaldomain.Withdrawal{}).Where("deleted_at IS NULL")
	if filter.UserID != 0 {
		query = query.Where("user_id = ?", filter.UserID)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []withdrawaldomain.Withdrawal
	if err := gormutil.ApplyPagination(query, filter.Page, filter.PageSize).Order("id desc").Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (s *Store) ListAdminWithdrawals(filter withdrawalcontract.AdminWithdrawalListFilter) ([]withdrawaldomain.Withdrawal, int64, error) {
	query := s.db.Model(&withdrawaldomain.Withdrawal{}).Where("deleted_at IS NULL")
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.UserID != 0 {
		query = query.Where("user_id = ?", filter.UserID)
	}
	if filter.WithdrawalNo != "" {
		query = query.Where("withdrawal_no LIKE ?", "%"+filter.WithdrawalNo+"%")
	}
	if filter.CreatedFrom != nil {
		query = query.Where("created_at >= ?", *filter.CreatedFrom)
	}
	if filter.CreatedTo != nil {
		query = query.Where("created_at <= ?", *filter.CreatedTo)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []withdrawaldomain.Withdrawal
	if err := gormutil.ApplyPagination(query, filter.Page, filter.PageSize).Order("id desc").Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// activeStatuses 是计入日限额/日笔数的状态（已扣款且未退款）。
var activeStatuses = []string{
	withdrawaldomain.StatusPending,
	withdrawaldomain.StatusApproved,
	withdrawaldomain.StatusProcessing,
	withdrawaldomain.StatusCompleted,
}

func (s *Store) SumUserActiveSince(userID uint, since time.Time) (decimal.Decimal, error) {
	if userID == 0 {
		return decimal.Zero, nil
	}
	var result struct {
		Sum string
	}
	if err := s.db.Model(&withdrawaldomain.Withdrawal{}).
		Where("user_id = ? AND created_at >= ? AND status IN ? AND deleted_at IS NULL", userID, since, activeStatuses).
		Select("COALESCE(SUM(request_amount), 0) as sum").
		Scan(&result).Error; err != nil {
		return decimal.Zero, err
	}
	if strings.TrimSpace(result.Sum) == "" {
		return decimal.Zero, nil
	}
	parsed, err := decimal.NewFromString(result.Sum)
	if err != nil {
		return decimal.Zero, err
	}
	return parsed.Round(2), nil
}

func (s *Store) CountUserActiveSince(userID uint, since time.Time) (int64, error) {
	if userID == 0 {
		return 0, nil
	}
	var count int64
	if err := s.db.Model(&withdrawaldomain.Withdrawal{}).
		Where("user_id = ? AND created_at >= ? AND status IN ? AND deleted_at IS NULL", userID, since, activeStatuses).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (s *Store) CountUserCompletedBefore(userID uint, before time.Time) (int64, error) {
	if userID == 0 {
		return 0, nil
	}
	var count int64
	if err := s.db.Model(&withdrawaldomain.Withdrawal{}).
		Where("user_id = ? AND status = ? AND created_at < ? AND deleted_at IS NULL", userID, withdrawaldomain.StatusCompleted, before).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (s *Store) CountCreatedOnDay(userID uint, dayStart time.Time) (int64, error) {
	var count int64
	if err := s.db.Model(&withdrawaldomain.Withdrawal{}).
		Where("created_at >= ? AND deleted_at IS NULL", dayStart).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// ---- Addresses ----

func (s *Store) CreateAddress(a *withdrawaldomain.Address) error {
	return s.db.Create(a).Error
}

func (s *Store) UpdateAddress(a *withdrawaldomain.Address) error {
	return s.db.Save(a).Error
}

func (s *Store) DeleteAddress(id uint) error {
	if id == 0 {
		return nil
	}
	now := time.Now()
	return s.db.Model(&withdrawaldomain.Address{}).Where("id = ?", id).Update("deleted_at", now).Error
}

func (s *Store) GetAddressByID(id uint) (*withdrawaldomain.Address, error) {
	if id == 0 {
		return nil, nil
	}
	var a withdrawaldomain.Address
	if err := s.db.Where("id = ? AND deleted_at IS NULL", id).First(&a).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &a, nil
}

func (s *Store) ListAddressesByUserID(userID uint) ([]withdrawaldomain.Address, error) {
	if userID == 0 {
		return []withdrawaldomain.Address{}, nil
	}
	var rows []withdrawaldomain.Address
	if err := s.db.Where("user_id = ? AND deleted_at IS NULL", userID).
		Order("is_default desc, id desc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *Store) GetAddressByUserIDNetworkAddress(userID uint, network, address string) (*withdrawaldomain.Address, error) {
	if userID == 0 || strings.TrimSpace(network) == "" || strings.TrimSpace(address) == "" {
		return nil, nil
	}
	var a withdrawaldomain.Address
	if err := s.db.Where("user_id = ? AND network = ? AND address = ? AND deleted_at IS NULL", userID, strings.ToUpper(strings.TrimSpace(network)), strings.TrimSpace(address)).
		First(&a).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &a, nil
}

func (s *Store) ClearDefaultAddress(userID uint) error {
	if userID == 0 {
		return nil
	}
	return s.db.Model(&withdrawaldomain.Address{}).
		Where("user_id = ? AND is_default = ? AND deleted_at IS NULL", userID, true).
		Update("is_default", false).Error
}
