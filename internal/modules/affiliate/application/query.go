package application

import (
	"math"
	"strings"

	affiliatecontract "github.com/Aether-v1/hcz/internal/modules/affiliate/contract"
	affiliatedomain "github.com/Aether-v1/hcz/internal/modules/affiliate/domain"

	"github.com/Aether-v1/hcz/internal/constants"
	"github.com/Aether-v1/hcz/internal/shared/money"

	"github.com/shopspring/decimal"
)

// GetUserDashboard 获取用户返利中心数据。
// Opened = profile 存在（含懒创建的 commission anchor）；不再等价于申请已通过。
// ApplicationStatus / TransferEnabled 由 affiliate_applications 真源决定。
func (s *Service) GetUserDashboard(userID uint) (Dashboard, error) {
	zero := money.FromDecimal(decimal.Zero)
	dashboard := Dashboard{
		Opened:                   false,
		ApplicationStatus:        constants.AffiliateAppStatusNotApplied,
		TransferEnabled:          false,
		PendingCommission:        zero,
		AvailableCommission:      zero,
		WithdrawnCommission:      zero,
		AvailableTransferBalance: zero,
		DebtAmount:               zero,
		TransferredAmount:        zero,
	}
	if userID == 0 || s.repo == nil {
		return dashboard, nil
	}

	// 申请状态真源：affiliate_applications
	latestApp, err := s.repo.GetLatestApplicationByUserID(userID)
	if err != nil {
		return dashboard, err
	}
	if latestApp != nil {
		dashboard.ApplicationStatus = strings.TrimSpace(latestApp.Status)
	}

	profile, err := s.repo.GetProfileByUserID(userID)
	if err != nil {
		return dashboard, err
	}
	if profile == nil {
		return dashboard, nil
	}

	dashboard.Opened = true
	dashboard.TransferEnabled = strings.TrimSpace(profile.Status) == constants.AffiliateProfileStatusActive &&
		dashboard.ApplicationStatus == constants.AffiliateAppStatusApproved

	stats, err := s.buildProfileStats(profile.ID)
	if err != nil {
		return dashboard, err
	}
	dashboard.AffiliateCode = profile.AffiliateCode
	dashboard.PromotionPath = "/?aff=" + profile.AffiliateCode
	dashboard.ClickCount = stats.ClickCount
	dashboard.ValidOrderCount = stats.ValidOrderCount
	dashboard.ConversionRate = stats.ConversionRate
	dashboard.PendingCommission = stats.PendingCommission
	dashboard.AvailableCommission = stats.AvailableCommission
	dashboard.WithdrawnCommission = stats.WithdrawnCommission
	dashboard.AvailableTransferBalance = stats.AvailableTransferBalance
	dashboard.DebtAmount = stats.DebtAmount
	dashboard.TransferredAmount = stats.TransferredAmount
	return dashboard, nil
}

// ListUserCommissions 查询用户佣金记录
func (s *Service) ListUserCommissions(userID uint, page, pageSize int, status string) ([]affiliatedomain.Commission, int64, error) {
	if userID == 0 || s.repo == nil {
		return []affiliatedomain.Commission{}, 0, nil
	}
	profile, err := s.repo.GetProfileByUserID(userID)
	if err != nil {
		return nil, 0, err
	}
	if profile == nil {
		return []affiliatedomain.Commission{}, 0, nil
	}
	return s.repo.ListCommissions(affiliatecontract.CommissionListFilter{
		Page:               page,
		PageSize:           pageSize,
		AffiliateProfileID: profile.ID,
		Status:             strings.TrimSpace(status),
	})
}

// ListUserWithdraws 查询用户提现记录
func (s *Service) ListUserWithdraws(userID uint, page, pageSize int, status string) ([]affiliatedomain.WithdrawRequest, int64, error) {
	if userID == 0 || s.repo == nil {
		return []affiliatedomain.WithdrawRequest{}, 0, nil
	}
	profile, err := s.repo.GetProfileByUserID(userID)
	if err != nil {
		return nil, 0, err
	}
	if profile == nil {
		return []affiliatedomain.WithdrawRequest{}, 0, nil
	}
	return s.repo.ListWithdraws(affiliatecontract.WithdrawListFilter{
		Page:               page,
		PageSize:           pageSize,
		AffiliateProfileID: profile.ID,
		Status:             strings.TrimSpace(status),
	})
}

// ListAdminUsers 后台查询推广用户列表
func (s *Service) ListAdminUsers(filter AdminProfileListFilter) ([]AdminUserItem, int64, error) {
	if s.repo == nil {
		return []AdminUserItem{}, 0, nil
	}
	rows, total, err := s.repo.ListProfiles(affiliatecontract.ProfileListFilter{
		Page:     filter.Page,
		PageSize: filter.PageSize,
		UserID:   filter.UserID,
		Status:   strings.TrimSpace(filter.Status),
		Code:     strings.TrimSpace(filter.Code),
		Keyword:  strings.TrimSpace(filter.Keyword),
	})
	if err != nil {
		return nil, 0, err
	}
	profileIDs := make([]uint, 0, len(rows))
	for _, row := range rows {
		if row.ID == 0 {
			continue
		}
		profileIDs = append(profileIDs, row.ID)
	}
	statsMap, err := s.repo.GetProfileStatsBatch(profileIDs)
	if err != nil {
		return nil, 0, err
	}
	result := make([]AdminUserItem, 0, len(rows))
	for _, row := range rows {
		agg := statsMap[row.ID]
		stats := Stats{
			ClickCount:          agg.ClickCount,
			ValidOrderCount:     agg.ValidOrderCount,
			ConversionRate:      calcAffiliateConversion(agg.ValidOrderCount, agg.ClickCount),
			PendingCommission:   money.FromDecimal(agg.PendingCommission.Round(2)),
			AvailableCommission: money.FromDecimal(agg.AvailableCommission.Round(2)),
			WithdrawnCommission: money.FromDecimal(agg.WithdrawnCommission.Round(2)),
		}
		result = append(result, AdminUserItem{
			Profile: row,
			Stats:   stats,
		})
	}
	return result, total, nil
}

// ListAdminCommissions 后台查询佣金记录
func (s *Service) ListAdminCommissions(filter AdminCommissionListFilter) ([]affiliatedomain.Commission, int64, error) {
	if s.repo == nil {
		return []affiliatedomain.Commission{}, 0, nil
	}
	return s.repo.ListCommissions(affiliatecontract.CommissionListFilter{
		Page:               filter.Page,
		PageSize:           filter.PageSize,
		AffiliateProfileID: filter.AffiliateProfileID,
		OrderNo:            strings.TrimSpace(filter.OrderNo),
		Status:             strings.TrimSpace(filter.Status),
		Keyword:            strings.TrimSpace(filter.Keyword),
		Level:              filter.Level,
	})
}

// ListAdminWithdraws 后台查询提现申请
func (s *Service) ListAdminWithdraws(filter AdminWithdrawListFilter) ([]affiliatedomain.WithdrawRequest, int64, error) {
	if s.repo == nil {
		return []affiliatedomain.WithdrawRequest{}, 0, nil
	}
	return s.repo.ListWithdraws(affiliatecontract.WithdrawListFilter{
		Page:               filter.Page,
		PageSize:           filter.PageSize,
		AffiliateProfileID: filter.AffiliateProfileID,
		Status:             strings.TrimSpace(filter.Status),
		Keyword:            strings.TrimSpace(filter.Keyword),
	})
}

func (s *Service) buildProfileStats(profileID uint) (Stats, error) {
	zero := money.FromDecimal(decimal.Zero)
	stats := Stats{
		PendingCommission:        zero,
		AvailableCommission:      zero,
		WithdrawnCommission:      zero,
		AvailableTransferBalance: zero,
		DebtAmount:               zero,
		TransferredAmount:        zero,
	}
	if profileID == 0 || s.repo == nil {
		return stats, nil
	}
	clickCount, err := s.repo.CountClicksByProfile(profileID)
	if err != nil {
		return stats, err
	}
	validOrders, err := s.repo.CountValidOrdersByProfile(profileID)
	if err != nil {
		return stats, err
	}
	pendingAmount, err := s.repo.SumCommissionByProfile(profileID, []string{
		constants.AffiliateCommissionStatusPendingConfirm,
	}, false)
	if err != nil {
		return stats, err
	}

	// ---- 基于 ledger 的余额计算（划转时代）----
	// 可划转余额 = totalEarned - settled(历史提现) - transferred(划转)
	availableTransfer, err := s.computeAvailableTransferBalance(s.repo, profileID)
	if err != nil {
		return stats, err
	}
	// 历史已出金 = ABS(SUM(withdraw_settle))
	settled, err := s.getSettledAmount(s.repo, profileID)
	if err != nil {
		return stats, err
	}
	// 累计划转 = ABS(SUM(transfer_to_wallet))
	transferSum, err := s.sumLedgerByProfile(s.repo, profileID, []string{
		constants.AffiliateLedgerTypeTransferToWallet,
	})
	if err != nil {
		return stats, err
	}
	transferred := transferSum.Abs().Round(2)
	// 累计已出金（兼容旧字段）= 历史提现 + 累计划转
	withdrawnTotal := settled.Add(transferred).Round(2)

	// 欠款 = MAX(0, -netBalance)，netBalance = SUM(all ledger)
	netBalance, err := s.getNetLedgerBalance(s.repo, profileID)
	if err != nil {
		return stats, err
	}
	debt := decimal.Zero
	if netBalance.LessThan(decimal.Zero) {
		debt = netBalance.Neg().Round(2)
	}

	stats.ClickCount = clickCount
	stats.ValidOrderCount = validOrders
	stats.ConversionRate = calcAffiliateConversion(validOrders, clickCount)
	stats.PendingCommission = money.FromDecimal(pendingAmount)
	stats.AvailableCommission = money.FromDecimal(availableTransfer)
	stats.WithdrawnCommission = money.FromDecimal(withdrawnTotal)
	stats.AvailableTransferBalance = money.FromDecimal(availableTransfer)
	stats.DebtAmount = money.FromDecimal(debt)
	stats.TransferredAmount = money.FromDecimal(transferred)
	return stats, nil
}

func calcAffiliateConversion(validOrders, clicks int64) float64 {
	if clicks <= 0 || validOrders <= 0 {
		return 0
	}
	value := (float64(validOrders) / float64(clicks)) * 100
	return math.Round(value*100) / 100
}
