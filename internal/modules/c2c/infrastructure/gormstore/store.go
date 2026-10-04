package gormstore

import (
	"errors"
	"strings"
	"time"

	c2ccontract "github.com/Aether-v1/hcz/internal/modules/c2c/contract"
	c2cdomain "github.com/Aether-v1/hcz/internal/modules/c2c/domain"
	walletcontract "github.com/Aether-v1/hcz/internal/modules/wallet/contract"
	walletgormstore "github.com/Aether-v1/hcz/internal/modules/wallet/infrastructure/gormstore"
	"github.com/Aether-v1/hcz/internal/persistence/gormutil"
	"github.com/shopspring/decimal"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Store 同时实现 C2C Repository 与 UnitOfWork。
// 持有 walletStore 引用，以便在 WithinTransaction 内绑定同一 *gorm.DB，
// 实现 C2C 事务与钱包 ledger/账户行锁的共享事务。
type Store struct {
	db          *gorm.DB
	walletStore *walletgormstore.Store
}

var (
	_ c2ccontract.Repository = (*Store)(nil)
	_ c2ccontract.UnitOfWork = (*Store)(nil)
)

// New 创建 C2C Store。walletStore 用于共享事务绑定。
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
	c2c     *Store
	wallets walletcontract.Repository
}

func (tx transaction) C2C() c2ccontract.Repository        { return tx.c2c }
func (tx transaction) Wallets() walletcontract.Repository { return tx.wallets }

// UseTransaction 用已开启的 *gorm.DB 构造 C2C Transaction，并绑定钱包仓库。
func UseTransaction(tx *gorm.DB, walletStore *walletgormstore.Store) c2ccontract.Transaction {
	if tx == nil {
		return nil
	}
	return transaction{
		c2c:     New(tx, walletStore),
		wallets: walletStore.Bind(tx),
	}
}

// WithinTransaction 开启 C2C 事务，回调内 C2C() 与 Wallets() 共享同一 *gorm.DB。
func (s *Store) WithinTransaction(fn func(c2ccontract.Transaction) error) error {
	if fn == nil {
		return nil
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		return fn(UseTransaction(tx, s.walletStore))
	})
}

// ==================== 支付方式 ====================

func (s *Store) CreatePaymentMethod(p *c2cdomain.PaymentMethod) error {
	return s.db.Create(p).Error
}

func (s *Store) UpdatePaymentMethod(p *c2cdomain.PaymentMethod) error {
	return s.db.Save(p).Error
}

func (s *Store) SoftDeletePaymentMethod(id uint) error {
	if id == 0 {
		return nil
	}
	return s.db.Delete(&c2cdomain.PaymentMethod{}, id).Error
}

func (s *Store) GetPaymentMethodByID(id uint) (*c2cdomain.PaymentMethod, error) {
	if id == 0 {
		return nil, nil
	}
	var p c2cdomain.PaymentMethod
	if err := s.db.Where("id = ?", id).First(&p).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &p, nil
}

func (s *Store) ListPaymentMethodsByUserID(userID uint) ([]c2cdomain.PaymentMethod, error) {
	if userID == 0 {
		return []c2cdomain.PaymentMethod{}, nil
	}
	var rows []c2cdomain.PaymentMethod
	if err := s.db.Where("user_id = ?", userID).Order("enabled desc, id desc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *Store) ListEnabledPaymentMethodsByUserID(userID uint) ([]c2cdomain.PaymentMethod, error) {
	if userID == 0 {
		return []c2cdomain.PaymentMethod{}, nil
	}
	var rows []c2cdomain.PaymentMethod
	if err := s.db.Where("user_id = ? AND enabled = ?", userID, true).Order("id desc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// ==================== 挂单 ====================

func (s *Store) CreateListing(l *c2cdomain.Listing) error {
	return s.db.Create(l).Error
}

func (s *Store) UpdateListing(l *c2cdomain.Listing) error {
	return s.db.Save(l).Error
}

func (s *Store) GetListingByID(id uint) (*c2cdomain.Listing, error) {
	if id == 0 {
		return nil, nil
	}
	var l c2cdomain.Listing
	if err := s.db.Where("id = ?", id).First(&l).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &l, nil
}

func (s *Store) GetListingByIDForUpdate(id uint) (*c2cdomain.Listing, error) {
	if id == 0 {
		return nil, nil
	}
	var l c2cdomain.Listing
	if err := s.db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", id).First(&l).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &l, nil
}

func (s *Store) ListMarketListings(filter c2ccontract.ListingMarketFilter) ([]c2cdomain.Listing, int64, error) {
	query := s.db.Model(&c2cdomain.Listing{}).
		Where("status = ? AND available_usdt > 0", "active")
	if strings.TrimSpace(filter.FiatCurrency) != "" {
		query = query.Where("fiat_currency = ?", strings.ToUpper(strings.TrimSpace(filter.FiatCurrency)))
	}
	if filter.ExcludeUserID != 0 {
		query = query.Where("seller_user_id <> ?", filter.ExcludeUserID)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []c2cdomain.Listing
	if err := gormutil.ApplyPagination(query, filter.Page, filter.PageSize).
		Order("price asc, id desc").Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (s *Store) ListMyListings(filter c2ccontract.ListingMyFilter) ([]c2cdomain.Listing, int64, error) {
	query := s.db.Model(&c2cdomain.Listing{}).Where("seller_user_id = ?", filter.UserID)
	if strings.TrimSpace(filter.Status) != "" {
		query = query.Where("status = ?", filter.Status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []c2cdomain.Listing
	if err := gormutil.ApplyPagination(query, filter.Page, filter.PageSize).
		Order("id desc").Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// DecrementListingAvailableUSDT 原子扣减：只有当余量足够时才扣减成功。
func (s *Store) DecrementListingAvailableUSDT(id uint, amount decimal.Decimal) (int64, error) {
	if id == 0 || amount.LessThanOrEqual(decimal.Zero) {
		return 0, nil
	}
	res := s.db.Model(&c2cdomain.Listing{}).
		Where("id = ? AND available_usdt >= ?", id, amount).
		UpdateColumn("available_usdt", gorm.Expr("available_usdt - ?", amount))
	return res.RowsAffected, res.Error
}

// IncrementListingAvailableUSDT 原子恢复余量（取消/超时退回给挂单）。
func (s *Store) IncrementListingAvailableUSDT(id uint, amount decimal.Decimal) error {
	if id == 0 || amount.LessThanOrEqual(decimal.Zero) {
		return nil
	}
	return s.db.Model(&c2cdomain.Listing{}).
		Where("id = ?", id).
		UpdateColumn("available_usdt", gorm.Expr("available_usdt + ?", amount)).Error
}

// ==================== 交易 ====================

func (s *Store) CreateTrade(t *c2cdomain.Trade) error {
	return s.db.Create(t).Error
}

func (s *Store) UpdateTrade(t *c2cdomain.Trade) error {
	return s.db.Save(t).Error
}

func (s *Store) GetTradeByID(id uint) (*c2cdomain.Trade, error) {
	if id == 0 {
		return nil, nil
	}
	var t c2cdomain.Trade
	if err := s.db.Where("id = ?", id).First(&t).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &t, nil
}

func (s *Store) GetTradeByIDForUpdate(id uint) (*c2cdomain.Trade, error) {
	if id == 0 {
		return nil, nil
	}
	var t c2cdomain.Trade
	if err := s.db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", id).First(&t).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &t, nil
}

func (s *Store) GetTradeByTradeNo(tradeNo string) (*c2cdomain.Trade, error) {
	tradeNo = strings.TrimSpace(tradeNo)
	if tradeNo == "" {
		return nil, nil
	}
	var t c2cdomain.Trade
	if err := s.db.Where("trade_no = ?", tradeNo).First(&t).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &t, nil
}

func (s *Store) GetTradeByBuyerIdempotency(buyerID uint, idempotencyKey string) (*c2cdomain.Trade, error) {
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if buyerID == 0 || idempotencyKey == "" {
		return nil, nil
	}
	var t c2cdomain.Trade
	if err := s.db.Where("buyer_user_id = ? AND idempotency_key = ?", buyerID, idempotencyKey).
		First(&t).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &t, nil
}

func (s *Store) ListMyTrades(filter c2ccontract.TradeListFilter) ([]c2cdomain.Trade, int64, error) {
	query := s.db.Model(&c2cdomain.Trade{}).
		Where("buyer_user_id = ? OR seller_user_id = ?", filter.UserID, filter.UserID)
	if strings.TrimSpace(filter.Status) != "" {
		query = query.Where("status = ?", filter.Status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []c2cdomain.Trade
	if err := gormutil.ApplyPagination(query, filter.Page, filter.PageSize).
		Order("id desc").Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (s *Store) SumUserTradedUSDTSince(userID uint, since time.Time) (decimal.Decimal, error) {
	if userID == 0 {
		return decimal.Zero, nil
	}
	var result struct {
		Sum string
	}
	if err := s.db.Model(&c2cdomain.Trade{}).
		Where("buyer_user_id = ? AND status = ? AND created_at >= ?", userID, "completed", since).
		Select("COALESCE(SUM(usdt_amount), 0) as sum").
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

// ==================== 争议 / 风控 ====================

func (s *Store) CreateDispute(d *c2cdomain.Dispute) error {
	return s.db.Create(d).Error
}

func (s *Store) UpdateDispute(d *c2cdomain.Dispute) error {
	return s.db.Save(d).Error
}

func (s *Store) GetDisputeByID(id uint) (*c2cdomain.Dispute, error) {
	if id == 0 {
		return nil, nil
	}
	var d c2cdomain.Dispute
	if err := s.db.Where("id = ?", id).First(&d).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &d, nil
}

func (s *Store) GetDisputeByTradeIDForUpdate(tradeID uint) (*c2cdomain.Dispute, error) {
	if tradeID == 0 {
		return nil, nil
	}
	var d c2cdomain.Dispute
	if err := s.db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("trade_id = ?", tradeID).First(&d).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &d, nil
}

func (s *Store) ListDisputes(filter c2ccontract.DisputeListFilter) ([]c2cdomain.Dispute, int64, error) {
	query := s.db.Model(&c2cdomain.Dispute{})
	if strings.TrimSpace(filter.Status) != "" {
		query = query.Where("status = ?", filter.Status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []c2cdomain.Dispute
	if err := gormutil.ApplyPagination(query, filter.Page, filter.PageSize).
		Order("id desc").Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (s *Store) CreateRiskSignal(r *c2cdomain.RiskSignal) error {
	return s.db.Create(r).Error
}

func (s *Store) ListRiskSignals(filter c2ccontract.RiskSignalListFilter) ([]c2cdomain.RiskSignal, int64, error) {
	query := s.db.Model(&c2cdomain.RiskSignal{})
	if filter.UserID != 0 {
		query = query.Where("user_id = ?", filter.UserID)
	}
	if strings.TrimSpace(filter.SignalType) != "" {
		query = query.Where("signal_type = ?", filter.SignalType)
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
	var rows []c2cdomain.RiskSignal
	if err := gormutil.ApplyPagination(query, filter.Page, filter.PageSize).
		Order("id desc").Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// ==================== 后台管理查询 ====================

func (s *Store) ListAdminListings(filter c2ccontract.AdminListingFilter) ([]c2cdomain.Listing, int64, error) {
	query := s.db.Model(&c2cdomain.Listing{})
	if filter.SellerUserID != 0 {
		query = query.Where("seller_user_id = ?", filter.SellerUserID)
	}
	if strings.TrimSpace(filter.Status) != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if strings.TrimSpace(filter.FiatCurrency) != "" {
		query = query.Where("fiat_currency = ?", strings.ToUpper(strings.TrimSpace(filter.FiatCurrency)))
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []c2cdomain.Listing
	if err := gormutil.ApplyPagination(query, filter.Page, filter.PageSize).
		Order("id desc").Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (s *Store) ListAdminTrades(filter c2ccontract.AdminTradeFilter) ([]c2cdomain.Trade, int64, error) {
	query := s.db.Model(&c2cdomain.Trade{})
	if strings.TrimSpace(filter.Status) != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.BuyerUserID != 0 {
		query = query.Where("buyer_user_id = ?", filter.BuyerUserID)
	}
	if filter.SellerUserID != 0 {
		query = query.Where("seller_user_id = ?", filter.SellerUserID)
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
	var rows []c2cdomain.Trade
	if err := gormutil.ApplyPagination(query, filter.Page, filter.PageSize).
		Order("id desc").Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// OverviewStats 统计 C2C 概览：活跃挂单数、进行中交易数、待仲裁申诉数、今日成交额。
func (s *Store) OverviewStats() (c2ccontract.OverviewStats, error) {
	out := c2ccontract.OverviewStats{TodayVolumeUSDT: decimal.Zero}
	s.db.Model(&c2cdomain.Listing{}).Where("status = ?", "active").Count(&out.ActiveListings)
	s.db.Model(&c2cdomain.Trade{}).
		Where("status IN ?", []string{"pending_payment", "paid", "disputed"}).
		Count(&out.ActiveTrades)
	s.db.Model(&c2cdomain.Dispute{}).Where("status = ?", "open").Count(&out.OpenDisputes)

	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	var sumResult struct {
		Sum string
	}
	if err := s.db.Model(&c2cdomain.Trade{}).
		Where("status = ? AND completed_at >= ?", "completed", start).
		Select("COALESCE(SUM(usdt_amount), 0) as sum").
		Scan(&sumResult).Error; err != nil {
		return out, err
	}
	if strings.TrimSpace(sumResult.Sum) != "" {
		if parsed, err := decimal.NewFromString(sumResult.Sum); err == nil {
			out.TodayVolumeUSDT = parsed.Round(2)
		}
	}
	return out, nil
}
