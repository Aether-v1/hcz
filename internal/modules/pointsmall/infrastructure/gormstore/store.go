package pointsmallgormstore

import (
	"errors"
	"strings"
	"time"

	pointscontract "github.com/Aether-v1/hcz/internal/modules/points/contract"
	pointsgormstore "github.com/Aether-v1/hcz/internal/modules/points/infrastructure/gormstore"
	pointsmallcontract "github.com/Aether-v1/hcz/internal/modules/pointsmall/contract"
	pointsmalldomain "github.com/Aether-v1/hcz/internal/modules/pointsmall/domain"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Store 是积分商城模块（商品 + 兑换订单）的 GORM 持久化实现。
type Store struct {
	db *gorm.DB
}

var _ pointsmallcontract.Repository = (*Store)(nil)
var _ pointsmallcontract.UnitOfWork = (*Store)(nil)

func New(db *gorm.DB) *Store {
	return &Store{db: db}
}

// UseTransaction 将已打开事务的 *gorm.DB 绑定为商城域事务视图。
func UseTransaction(tx *gorm.DB) pointsmallcontract.Transaction {
	return storeTransaction{db: tx}
}

type storeTransaction struct {
	db *gorm.DB
}

func (t storeTransaction) Products() pointsmallcontract.Repository {
	return &Store{db: t.db}
}

func (t storeTransaction) Points() pointscontract.Transaction {
	return pointsgormstore.UseTransaction(t.db)
}

func (s *Store) WithinTransaction(fn func(pointsmallcontract.Transaction) error) error {
	if fn == nil {
		return nil
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		return fn(UseTransaction(tx))
	})
}

// ---------------------------------------------------------------------------
// Products
// ---------------------------------------------------------------------------

func (s *Store) ListProducts(filter pointsmallcontract.ProductListFilter) ([]pointsmalldomain.PointsProduct, int64, error) {
	query := s.db.Model(&pointsmalldomain.PointsProduct{})
	if filter.Enabled != nil {
		query = query.Where("enabled = ?", *filter.Enabled)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page, pageSize := normalizePage(filter.Page, filter.PageSize)
	var products []pointsmalldomain.PointsProduct
	if err := query.Order("sort ASC, id ASC").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Find(&products).Error; err != nil {
		return nil, 0, err
	}
	return products, total, nil
}

func (s *Store) GetProductByID(id uint) (*pointsmalldomain.PointsProduct, error) {
	if id == 0 {
		return nil, nil
	}
	var product pointsmalldomain.PointsProduct
	if err := s.db.Where("id = ?", id).First(&product).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &product, nil
}

func (s *Store) GetProductByIDForUpdate(id uint) (*pointsmalldomain.PointsProduct, error) {
	if id == 0 {
		return nil, nil
	}
	var product pointsmalldomain.PointsProduct
	if err := s.db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", id).First(&product).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &product, nil
}

func (s *Store) CreateProduct(product *pointsmalldomain.PointsProduct) error {
	return s.db.Create(product).Error
}

func (s *Store) UpdateProduct(product *pointsmalldomain.PointsProduct) error {
	return s.db.Save(product).Error
}

// ---------------------------------------------------------------------------
// Exchange orders
// ---------------------------------------------------------------------------

// CountUserActiveOrders 统计用户对某商品的有效兑换数（PENDING/PROCESSING/COMPLETED；
// FAILED/CANCELLED 已返还积分与库存，不计入限购）。excludeOrderID>0 时排除自身订单。
func (s *Store) CountUserActiveOrders(userID, productID, excludeOrderID uint) (int64, error) {
	query := s.db.Model(&pointsmalldomain.ExchangeOrder{}).
		Where("user_id = ? AND product_id = ? AND status IN ?",
			userID, productID,
			[]string{
				pointsmalldomain.ExchangeStatusPending,
				pointsmalldomain.ExchangeStatusProcessing,
				pointsmalldomain.ExchangeStatusCompleted,
			})
	if excludeOrderID > 0 {
		query = query.Where("id <> ?", excludeOrderID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (s *Store) CreateExchangeOrder(order *pointsmalldomain.ExchangeOrder) error {
	return s.db.Create(order).Error
}

func (s *Store) GetExchangeOrderByID(id uint) (*pointsmalldomain.ExchangeOrder, error) {
	if id == 0 {
		return nil, nil
	}
	var order pointsmalldomain.ExchangeOrder
	if err := s.db.Where("id = ?", id).First(&order).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &order, nil
}

func (s *Store) GetExchangeOrderByIDForUpdate(id uint) (*pointsmalldomain.ExchangeOrder, error) {
	if id == 0 {
		return nil, nil
	}
	var order pointsmalldomain.ExchangeOrder
	if err := s.db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", id).First(&order).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &order, nil
}

// GetExchangeOrderByIDAndUser 按 id + user_id 查询（IDOR 防御：用户只能取自己的订单）。
func (s *Store) GetExchangeOrderByIDAndUser(id, userID uint) (*pointsmalldomain.ExchangeOrder, error) {
	if id == 0 || userID == 0 {
		return nil, nil
	}
	var order pointsmalldomain.ExchangeOrder
	if err := s.db.Where("id = ? AND user_id = ?", id, userID).First(&order).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &order, nil
}

func (s *Store) GetExchangeOrderByIdempotencyKey(userID uint, key string) (*pointsmalldomain.ExchangeOrder, error) {
	if userID == 0 || strings.TrimSpace(key) == "" {
		return nil, nil
	}
	var order pointsmalldomain.ExchangeOrder
	if err := s.db.Where("user_id = ? AND idempotency_key = ?", userID, strings.TrimSpace(key)).
		First(&order).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &order, nil
}

func (s *Store) UpdateExchangeOrder(order *pointsmalldomain.ExchangeOrder) error {
	return s.db.Save(order).Error
}

func (s *Store) ListExchangeOrders(filter pointsmallcontract.ExchangeOrderListFilter) ([]pointsmalldomain.ExchangeOrder, int64, error) {
	query := s.db.Model(&pointsmalldomain.ExchangeOrder{})
	if filter.UserID != 0 {
		query = query.Where("user_id = ?", filter.UserID)
	}
	if status := strings.TrimSpace(filter.Status); status != "" {
		query = query.Where("status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page, pageSize := normalizePage(filter.Page, filter.PageSize)
	var orders []pointsmalldomain.ExchangeOrder
	if err := query.Order("id DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Find(&orders).Error; err != nil {
		return nil, 0, err
	}
	return orders, total, nil
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

// ---------------------------------------------------------------------------
// 运营统计只读聚合（P4：直接聚合事实表，不维护累计总表）
// ---------------------------------------------------------------------------

// CountExchangeOrdersCreated 统计 [from,to) 内创建的兑换单数。
func (s *Store) CountExchangeOrdersCreated(from, to time.Time) (int64, error) {
	var count int64
	err := s.db.Model(&pointsmalldomain.ExchangeOrder{}).
		Where("created_at >= ? AND created_at < ?", from, to).
		Count(&count).Error
	return count, err
}

// CountExchangeOrdersByStatus 按状态统计兑换单数（待处理 / 处理中看板指标）。
func (s *Store) CountExchangeOrdersByStatus(status string) (int64, error) {
	var count int64
	err := s.db.Model(&pointsmalldomain.ExchangeOrder{}).
		Where("status = ?", status).
		Count(&count).Error
	return count, err
}

// CountPendingExchangeOrders 实现积分模块的 ExchangeStatsReader 端口。
func (s *Store) CountPendingExchangeOrders() (int64, error) {
	return s.CountExchangeOrdersByStatus(pointsmalldomain.ExchangeStatusPending)
}

// CountProcessingExchangeOrders 实现积分模块的 ExchangeStatsReader 端口。
func (s *Store) CountProcessingExchangeOrders() (int64, error) {
	return s.CountExchangeOrdersByStatus(pointsmalldomain.ExchangeStatusProcessing)
}

var _ pointscontract.ExchangeStatsReader = (*Store)(nil)
