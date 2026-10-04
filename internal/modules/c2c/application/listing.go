package application

import (
	"strings"
	"time"

	c2ccontract "github.com/Aether-v1/hcz/internal/modules/c2c/contract"
	c2cdomain "github.com/Aether-v1/hcz/internal/modules/c2c/domain"
	userdomain "github.com/Aether-v1/hcz/internal/modules/identity/user/domain"
	"github.com/Aether-v1/hcz/internal/shared/money"

	"github.com/shopspring/decimal"
)

// listing 状态常量（挂单生命周期，区别于交易状态机）。
const (
	listingActive = "active"
	listingPaused = "paused"
	listingClosed = "closed"
)

// assertUserCanTrade 校验用户是否具备参与 C2C 的基础资格（存在/启用/未封禁/TOTP/全局开关）。
func (s *Service) assertUserCanTrade(userID uint) (*userdomain.User, error) {
	if userID == 0 {
		return nil, c2ccontract.ErrTradeNotFound
	}
	cfg := s.loadConfig()
	if !cfg.Enabled {
		return nil, c2ccontract.ErrC2CDisabled
	}
	if s.users == nil {
		return nil, c2ccontract.ErrUserInactive
	}
	u, err := s.users.GetByID(userID)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, c2ccontract.ErrUserInactive
	}
	if u.Status != "active" {
		return nil, c2ccontract.ErrUserInactive
	}
	if u.C2CBanned {
		return nil, c2ccontract.ErrUserBanned
	}
	if u.TOTPEnabledAt == nil {
		return nil, c2ccontract.ErrTOTPRequired
	}
	return u, nil
}

// CheckListingEligibility 发布挂单前的完整资格检查。
func (s *Service) CheckListingEligibility(userID uint) error {
	u, err := s.assertUserCanTrade(userID)
	if err != nil {
		return err
	}
	cfg := s.loadConfig()
	// 新用户冷却
	if cfg.NewUserCooldownHours > 0 && time.Since(u.CreatedAt) < time.Duration(cfg.NewUserCooldownHours)*time.Hour {
		return c2ccontract.ErrNewUserCooldown
	}
	// 至少一个启用的支付方式
	pms, err := s.repo.ListEnabledPaymentMethodsByUserID(userID)
	if err != nil {
		return err
	}
	if len(pms) == 0 {
		return c2ccontract.ErrNoPaymentMethod
	}
	// 当日已成交 USDT 限额
	if cfg.DailyTradeLimitUSDT > 0 {
		sum, err := s.repo.SumUserTradedUSDTSince(userID, dayStart(time.Now()))
		if err != nil {
			return err
		}
		if sum.GreaterThanOrEqual(decimal.NewFromFloat(cfg.DailyTradeLimitUSDT)) {
			return c2ccontract.ErrDailyLimitExceeded
		}
	}
	return nil
}

// CreateListing 发布挂单（卖家挂单卖出 USDT）。
func (s *Service) CreateListing(input c2ccontract.CreateListingInput) (*c2cdomain.Listing, error) {
	if err := s.CheckListingEligibility(input.UserID); err != nil {
		return nil, err
	}
	price := input.Price.Decimal.Round(2)
	total := input.TotalUSDT.Decimal.Round(2)
	minFiat := input.MinFiatAmount.Decimal.Round(2)
	maxFiat := input.MaxFiatAmount.Decimal.Round(2)
	if price.LessThanOrEqual(decimal.Zero) || total.LessThanOrEqual(decimal.Zero) {
		return nil, c2ccontract.ErrInvalidAmount
	}
	if minFiat.LessThanOrEqual(decimal.Zero) || maxFiat.LessThan(minFiat) {
		return nil, c2ccontract.ErrAmountOutOfRange
	}
	fiat := strings.ToUpper(strings.TrimSpace(input.FiatCurrency))
	if fiat == "" {
		fiat = "CNY"
	}

	now := time.Now()
	listing := &c2cdomain.Listing{
		ListingNo:     genNo("C2CL", now),
		SellerUserID:  input.UserID,
		FiatCurrency:  fiat,
		Price:         money.FromDecimal(price),
		MinFiatAmount: money.FromDecimal(minFiat),
		MaxFiatAmount: money.FromDecimal(maxFiat),
		TotalUSDT:     money.FromDecimal(total),
		AvailableUSDT: money.FromDecimal(total),
		Status:        listingActive,
		Terms:         strings.TrimSpace(input.Terms),
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	// 资格检查：钱包可用余额 >= 挂单总量（仅校验，不 Freeze；成交时才冻结）。
	err := s.uow.WithinTransaction(func(tx c2ccontract.Transaction) error {
		account, err := tx.Wallets().GetAccountByUserID(input.UserID)
		if err != nil {
			return err
		}
		if account == nil || account.AvailableBalance.Decimal.Round(2).LessThan(total) {
			return c2ccontract.ErrInsufficientBalance
		}
		return tx.C2C().CreateListing(listing)
	})
	if err != nil {
		return nil, err
	}
	return listing, nil
}

// ownedListing 取挂单并校验为本人所有。
func (s *Service) ownedListing(userID, id uint) (*c2cdomain.Listing, error) {
	if userID == 0 || id == 0 {
		return nil, c2ccontract.ErrListingNotFound
	}
	l, err := s.repo.GetListingByID(id)
	if err != nil {
		return nil, err
	}
	if l == nil || l.SellerUserID != userID {
		return nil, c2ccontract.ErrListingNotFound
	}
	return l, nil
}

// UpdateListing 修改自己的挂单（仅 active/paused 可改）。
func (s *Service) UpdateListing(input c2ccontract.UpdateListingInput) (*c2cdomain.Listing, error) {
	l, err := s.ownedListing(input.UserID, input.ID)
	if err != nil {
		return nil, err
	}
	if l.Status == listingClosed {
		return nil, c2ccontract.ErrListingNotActive
	}
	if price := input.Price.Decimal; !price.IsZero() {
		if price.Round(2).LessThanOrEqual(decimal.Zero) {
			return nil, c2ccontract.ErrInvalidAmount
		}
		l.Price = money.FromDecimal(price.Round(2))
	}
	minFiat := input.MinFiatAmount.Decimal.Round(2)
	maxFiat := input.MaxFiatAmount.Decimal.Round(2)
	if !minFiat.IsZero() {
		l.MinFiatAmount = money.FromDecimal(minFiat)
	}
	if !maxFiat.IsZero() {
		l.MaxFiatAmount = money.FromDecimal(maxFiat)
	}
	if l.MinFiatAmount.Decimal.Round(2).GreaterThan(l.MaxFiatAmount.Decimal.Round(2)) {
		return nil, c2ccontract.ErrAmountOutOfRange
	}
	l.Terms = strings.TrimSpace(input.Terms)
	l.UpdatedAt = time.Now()
	if err := s.repo.UpdateListing(l); err != nil {
		return nil, err
	}
	return l, nil
}

// PauseListing active -> paused（不影响已有 Trade）。
func (s *Service) PauseListing(userID, id uint) (*c2cdomain.Listing, error) {
	l, err := s.ownedListing(userID, id)
	if err != nil {
		return nil, err
	}
	if l.Status != listingActive {
		return nil, c2ccontract.ErrListingNotActive
	}
	l.Status = listingPaused
	l.UpdatedAt = time.Now()
	if err := s.repo.UpdateListing(l); err != nil {
		return nil, err
	}
	return l, nil
}

// ResumeListing paused -> active。
func (s *Service) ResumeListing(userID, id uint) (*c2cdomain.Listing, error) {
	l, err := s.ownedListing(userID, id)
	if err != nil {
		return nil, err
	}
	if l.Status != listingPaused {
		return nil, c2ccontract.ErrListingNotActive
	}
	l.Status = listingActive
	l.UpdatedAt = time.Now()
	if err := s.repo.UpdateListing(l); err != nil {
		return nil, err
	}
	return l, nil
}

// CloseListing active/paused -> closed（不影响已有 Trade）。
func (s *Service) CloseListing(userID, id uint) (*c2cdomain.Listing, error) {
	l, err := s.ownedListing(userID, id)
	if err != nil {
		return nil, err
	}
	if l.Status == listingClosed {
		return nil, c2ccontract.ErrListingNotActive
	}
	l.Status = listingClosed
	l.UpdatedAt = time.Now()
	if err := s.repo.UpdateListing(l); err != nil {
		return nil, err
	}
	return l, nil
}

// GetListingDetail 挂单详情（市场可见）。
func (s *Service) GetListingDetail(id uint) (*c2cdomain.Listing, error) {
	l, err := s.repo.GetListingByID(id)
	if err != nil {
		return nil, err
	}
	if l == nil {
		return nil, c2ccontract.ErrListingNotFound
	}
	return l, nil
}

// ListMarketListings 市场挂单（仅 active 且有可售余量，默认排除自己）。
func (s *Service) ListMarketListings(filter c2ccontract.ListingMarketFilter) ([]c2cdomain.Listing, int64, error) {
	return s.repo.ListMarketListings(filter)
}

// ListMyListings 我的挂单。
func (s *Service) ListMyListings(filter c2ccontract.ListingMyFilter) ([]c2cdomain.Listing, int64, error) {
	return s.repo.ListMyListings(filter)
}
