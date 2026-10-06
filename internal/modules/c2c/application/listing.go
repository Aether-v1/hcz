package application

import (
	"errors"
	"strings"
	"time"

	"github.com/Aether-v1/hcz/internal/logger"
	c2ccontract "github.com/Aether-v1/hcz/internal/modules/c2c/contract"
	c2cdomain "github.com/Aether-v1/hcz/internal/modules/c2c/domain"
	userdomain "github.com/Aether-v1/hcz/internal/modules/identity/user/domain"
	walletcontract "github.com/Aether-v1/hcz/internal/modules/wallet/contract"
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
// 新模型：挂单创建时即在事务内 Freeze 卖家 USDT（reference c2c_freeze:listing:{listingNo}），
// 之后 CreateTrade 只占用 listing 可售额度，不再重复 Freeze。
// Freeze 内部已做行锁 + available>=amount 校验，无需额外只读余额校验。
// 任一步失败整体回滚，不允许残留冻结或残留 listing。
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
	// 预生成 listingNo，Freeze 与 Listing INSERT 共用同一单号。
	listingNo := genNo("C2CL", now)
	listing := &c2cdomain.Listing{
		ListingNo:     listingNo,
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

	err := s.uow.WithinTransaction(func(tx c2ccontract.Transaction) error {
		// 1. Freeze 卖家 USDT（内部行锁 seller wallet + available>=amount 校验 + reference 幂等）。
		if _, _, err := s.wallet.Freeze(tx, walletcontract.FreezeInput{
			UserID:    input.UserID,
			Amount:    money.FromDecimal(total),
			Reference: "c2c_freeze:listing:" + listingNo,
			Remark:    "C2C挂单冻结",
		}); err != nil {
			// 把 wallet 层 ErrInsufficientBalance 映射为 C2C 领域错误，便于 handler 统一处理。
			if errors.Is(err, walletcontract.ErrInsufficientBalance) {
				return c2ccontract.ErrInsufficientBalance
			}
			// METRICS_PENDING: c2c_listing_freeze_failed
			logger.Errorw("c2c_listing_freeze_failed",
				"user_id", input.UserID,
				"listing_no", listingNo,
				"amount", total.String(),
				"error", err.Error(),
			)
			return err
		}
		// 2. 创建 Listing。
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

// CloseListing active/paused -> closed。
// 新模型：行锁 listing → 检查 active trade → Unfreeze 剩余 frozen → status=closed。
// 整个过程在事务内完成；已 closed 的 listing 直接返回（幂等）。
func (s *Service) CloseListing(userID, id uint) (*c2cdomain.Listing, error) {
	l, err := s.ownedListing(userID, id)
	if err != nil {
		return nil, err
	}
	if l.Status == listingClosed {
		return l, nil
	}
	var out *c2cdomain.Listing
	err = s.uow.WithinTransaction(func(tx c2ccontract.Transaction) error {
		closed, err := s.closeListingWithinTx(tx, id)
		if err != nil {
			return err
		}
		out = closed
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// closeListingWithinTx 在已开启的事务内关闭挂单：
//  1. 行锁 listing（GetListingByIDForUpdate）
//  2. 已 closed → 直接返回（幂等）
//  3. CountActiveTradesByListingID > 0 → ErrListingNotClosable
//  4. remaining = listing.AvailableUSDT；> 0 时 Unfreeze 回卖家 available
//  5. listing.status = closed
//
// 锁顺序：listing row → seller wallet account（由 Unfreeze 内部加锁）。
func (s *Service) closeListingWithinTx(tx c2ccontract.Transaction, listingID uint) (*c2cdomain.Listing, error) {
	l, err := tx.C2C().GetListingByIDForUpdate(listingID)
	if err != nil {
		return nil, err
	}
	if l == nil {
		return nil, c2ccontract.ErrListingNotFound
	}
	if l.Status == listingClosed {
		return l, nil
	}
	// 有未完结交易不允许关闭。
	active, err := tx.C2C().CountActiveTradesByListingID(l.ID)
	if err != nil {
		return nil, err
	}
	if active > 0 {
		return nil, c2ccontract.ErrListingNotClosable
	}
	// 无 active trade 时 committed=0，AvailableUSDT = frozen remaining。
	remaining := l.AvailableUSDT.Decimal.Round(2)
	if remaining.GreaterThan(decimal.Zero) {
		if _, _, err := s.wallet.Unfreeze(tx, walletcontract.UnfreezeInput{
			UserID:    l.SellerUserID,
			Amount:    money.FromDecimal(remaining),
			Reference: "c2c_unfreeze:listing:" + l.ListingNo,
			Remark:    "C2C挂单关闭解冻",
		}); err != nil {
			return nil, err
		}
	}
	l.Status = listingClosed
	l.UpdatedAt = time.Now()
	if err := tx.C2C().UpdateListing(l); err != nil {
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
