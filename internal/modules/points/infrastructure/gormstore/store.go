package gormstore

import (
	"errors"
	"strings"
	"time"

	pointscontract "github.com/Aether-v1/hcz/internal/modules/points/contract"
	pointsdomain "github.com/Aether-v1/hcz/internal/modules/points/domain"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Store 是积分模块的 GORM 持久化实现。
type Store struct {
	db *gorm.DB
}

var _ pointscontract.Repository = (*Store)(nil)
var _ pointscontract.UnitOfWork = (*Store)(nil)

func New(db *gorm.DB) *Store {
	return &Store{db: db}
}

// UseTransaction 将已打开事务的 *gorm.DB 绑定为积分域事务视图。
func UseTransaction(tx *gorm.DB) pointscontract.Transaction {
	return storeTransaction{db: tx}
}

// storeTransaction 是积分域的事务视图，不暴露 ORM 原语。
type storeTransaction struct {
	db *gorm.DB
}

func (t storeTransaction) Points() pointscontract.Repository {
	return &Store{db: t.db}
}

func (s *Store) WithinTransaction(fn func(pointscontract.Transaction) error) error {
	if fn == nil {
		return nil
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		return fn(UseTransaction(tx))
	})
}

func (s *Store) GetAccountByUserID(userID uint) (*pointsdomain.Account, error) {
	if userID == 0 {
		return nil, nil
	}
	var account pointsdomain.Account
	if err := s.db.Where("user_id = ?", userID).First(&account).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &account, nil
}

func (s *Store) GetAccountByUserIDForUpdate(userID uint) (*pointsdomain.Account, error) {
	if userID == 0 {
		return nil, nil
	}
	var account pointsdomain.Account
	if err := s.db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_id = ?", userID).
		First(&account).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &account, nil
}

func (s *Store) CreateAccount(account *pointsdomain.Account) error {
	return s.db.Create(account).Error
}

func (s *Store) UpdateAccount(account *pointsdomain.Account) error {
	return s.db.Save(account).Error
}

func (s *Store) CreateLedgerEntry(entry *pointsdomain.LedgerEntry) error {
	return s.db.Create(entry).Error
}

func (s *Store) GetLedgerEntryByReference(reference string) (*pointsdomain.LedgerEntry, error) {
	if strings.TrimSpace(reference) == "" {
		return nil, nil
	}
	var entry pointsdomain.LedgerEntry
	if err := s.db.Where("reference = ?", strings.TrimSpace(reference)).First(&entry).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &entry, nil
}

// SumOrderRewards 聚合订单奖励与已冲正积分（P1 部分退款累计算法）。
// reward  = Σ amount（ORDER_REWARD，amount 为正）
// reversed = Σ |amount|（ORDER_REWARD_REVERSAL，amount 为负）
func (s *Store) SumOrderRewards(orderID uint) (reward int64, reversed int64, err error) {
	if err := s.db.Model(&pointsdomain.LedgerEntry{}).
		Where("order_id = ? AND action_type = ?", orderID, pointscontract.ActionOrderReward).
		Select("COALESCE(SUM(amount),0)").
		Scan(&reward).Error; err != nil {
		return 0, 0, err
	}
	if err := s.db.Model(&pointsdomain.LedgerEntry{}).
		Where("order_id = ? AND action_type = ?", orderID, pointscontract.ActionOrderRewardReversal).
		Select("COALESCE(SUM(-amount),0)").
		Scan(&reversed).Error; err != nil {
		return 0, 0, err
	}
	return reward, reversed, nil
}

func (s *Store) ListLedgerEntries(filter pointscontract.LedgerListFilter) ([]pointsdomain.LedgerEntry, int64, error) {
	query := s.db.Model(&pointsdomain.LedgerEntry{})
	if filter.UserID != 0 {
		query = query.Where("user_id = ?", filter.UserID)
	}
	if action := strings.TrimSpace(filter.ActionType); action != "" {
		query = query.Where("action_type = ?", action)
	}
	if source := strings.TrimSpace(filter.SourceType); source != "" {
		query = query.Where("source_type = ?", source)
	}
	if reference := strings.TrimSpace(filter.Reference); reference != "" {
		query = query.Where("reference = ?", reference)
	}
	switch strings.ToLower(strings.TrimSpace(filter.Direction)) {
	case "income":
		query = query.Where("amount > ?", 0)
	case "expense":
		query = query.Where("amount < ?", 0)
	}
	if !filter.CreatedFrom.IsZero() {
		query = query.Where("created_at >= ?", filter.CreatedFrom)
	}
	if !filter.CreatedTo.IsZero() {
		query = query.Where("created_at < ?", filter.CreatedTo)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	page, pageSize := normalizePage(filter.Page, filter.PageSize)
	var entries []pointsdomain.LedgerEntry
	if err := query.
		Order("created_at DESC, id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&entries).Error; err != nil {
		return nil, 0, err
	}
	return entries, total, nil
}

// ListAccounts 分页查询积分账户。NegativeOnly 对应运营排查"退款冲正后欠积分"的用户；
// 余额负值是真实结果，查询侧不做任何钳制。
func (s *Store) ListAccounts(filter pointscontract.AccountListFilter) ([]pointscontract.AccountWithUser, int64, error) {
	query := s.db.Model(&pointsdomain.Account{})
	if filter.UserID != 0 {
		query = query.Where("user_id = ?", filter.UserID)
	}
	if filter.NegativeOnly {
		query = query.Where("balance < ?", 0)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	page, pageSize := normalizePage(filter.Page, filter.PageSize)
	var accounts []pointsdomain.Account
	if err := query.
		Order("id ASC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&accounts).Error; err != nil {
		return nil, 0, err
	}

	items := make([]pointscontract.AccountWithUser, 0, len(accounts))
	for _, account := range accounts {
		items = append(items, pointscontract.AccountWithUser{
			UserID:      account.UserID,
			Balance:     account.Balance,
			TotalEarned: account.TotalEarned,
			TotalSpent:  account.TotalSpent,
			UpdatedAt:   account.UpdatedAt,
		})
	}
	return items, total, nil
}

// AggregateLedgerRange 聚合 [from,to) 区间内流水的入账/出账/订单奖励与条数。
func (s *Store) AggregateLedgerRange(from, to time.Time) (pointscontract.LedgerAggregate, error) {
	var agg pointscontract.LedgerAggregate
	base := s.db.Model(&pointsdomain.LedgerEntry{}).Where("created_at >= ? AND created_at < ?", from, to)
	if err := base.Session(&gorm.Session{}).
		Where("amount > ?", 0).
		Select("COALESCE(SUM(amount),0)").Scan(&agg.Granted).Error; err != nil {
		return agg, err
	}
	if err := base.Session(&gorm.Session{}).
		Where("amount < ?", 0).
		Select("COALESCE(SUM(-amount),0)").Scan(&agg.Spent).Error; err != nil {
		return agg, err
	}
	if err := base.Session(&gorm.Session{}).
		Where("action_type = ?", pointscontract.ActionOrderReward).
		Select("COALESCE(SUM(amount),0)").Scan(&agg.OrderReward).Error; err != nil {
		return agg, err
	}
	if err := base.Session(&gorm.Session{}).
		Select("COUNT(*)").Scan(&agg.Mutations).Error; err != nil {
		return agg, err
	}
	return agg, nil
}

// SumBalances 返回全平台余额总量与负余额账户数。
func (s *Store) SumBalances() (total int64, negativeAccounts int64, err error) {
	if err = s.db.Model(&pointsdomain.Account{}).
		Select("COALESCE(SUM(balance),0)").Scan(&total).Error; err != nil {
		return 0, 0, err
	}
	if err = s.db.Model(&pointsdomain.Account{}).
		Where("balance < ?", 0).
		Select("COUNT(*)").Scan(&negativeAccounts).Error; err != nil {
		return 0, 0, err
	}
	return total, negativeAccounts, nil
}

func normalizePage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 200 {
		pageSize = 200
	}
	return page, pageSize
}
