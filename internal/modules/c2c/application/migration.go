package application

import (
	"time"

	c2ccontract "github.com/Aether-v1/hcz/internal/modules/c2c/contract"
	c2cdomain "github.com/Aether-v1/hcz/internal/modules/c2c/domain"
	walletcontract "github.com/Aether-v1/hcz/internal/modules/wallet/contract"
	"github.com/Aether-v1/hcz/internal/shared/money"

	"github.com/shopspring/decimal"
)

// listingStatusSuspended 迁移时卖家 available 不足，挂单被挂起（不再可成交）。
const listingStatusSuspended = "suspended"

// MigrationStats 历史迁移统计。
type MigrationStats struct {
	Scanned   int64 // 扫描的 active/paused 挂单数
	Frozen    int64 // 成功冻结（或命中既有 reference 幂等返回）的挂单数
	Suspended int64 // 卖家 available 不足，被标记 suspended 的挂单数
	Skipped   int64 // AvailableUSDT <= 0，无需冻结的挂单数
	Failed    int64 // 其他失败数（事务回滚）
}

// MigrateListingsToListingFreeze 把旧模型（CreateTrade 时 Freeze）迁移到新模型（CreateListing 时 Freeze）。
//
// 对每个 status IN ('active','paused') 的 listing：
//   - remaining = listing.AvailableUSDT；若 <= 0 直接记 Skipped（无需冻结）。
//   - 否则在单条事务内：锁 seller wallet → 检查 available >= remaining →
//     Freeze(remaining, ref="c2c_freeze:listing:{listingNo}")。
//   - 若 available < remaining（或无钱包账户）：将 listing.status 设为 "suspended"，
//     不硬扣成负数。
//
// 幂等：wallet.Freeze 内部按 reference 幂等，重复迁移不会双冻结。
//
// 注意：旧模型下 active trade 金额已通过 trade-level freeze 冻结，迁移只冻结
// listing.AvailableUSDT（= original - settled - committed），不重复冻结 active trade 金额。
//
// 迁移前后对账：sum(available) + sum(frozen) 不变（仅 available→frozen 内部转移；
// suspended 的 listing 资金未动）。
func (s *Service) MigrateListingsToListingFreeze() (MigrationStats, error) {
	var stats MigrationStats

	listings, err := s.repo.ListListingsByStatus([]string{listingActive, listingPaused})
	if err != nil {
		return stats, err
	}
	stats.Scanned = int64(len(listings))

	for i := range listings {
		l := listings[i]
		ref := "c2c_freeze:listing:" + l.ListingNo

		remaining := l.AvailableUSDT.Decimal.Round(2)
		if remaining.LessThanOrEqual(decimal.Zero) {
			stats.Skipped++
			continue
		}

		// 在单条事务内完成：锁 seller wallet → 校验 available → Freeze 或 suspend。
		err := s.uow.WithinTransaction(func(tx c2ccontract.Transaction) error {
			acc, err := tx.Wallets().GetAccountByUserIDForUpdate(l.SellerUserID)
			if err != nil {
				return err
			}
			if acc == nil {
				return s.suspendListing(tx, &l)
			}
			avail := acc.AvailableBalance.Decimal.Round(2)
			if avail.LessThan(remaining) {
				return s.suspendListing(tx, &l)
			}
			// Freeze 内部再次行锁 + available>=amount 校验 + reference 幂等。
			if _, _, err := s.wallet.Freeze(tx, walletcontract.FreezeInput{
				UserID:    l.SellerUserID,
				Amount:    money.FromDecimal(remaining),
				Reference: ref,
				Remark:    "C2C挂单冻结(迁移)",
			}); err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			stats.Failed++
			continue
		}
		// 通过重新读 listing status 区分：suspended 或 frozen。
		reloaded, rerr := s.repo.GetListingByID(l.ID)
		if rerr != nil || reloaded == nil {
			stats.Failed++
			continue
		}
		if reloaded.Status == listingStatusSuspended {
			stats.Suspended++
		} else {
			stats.Frozen++
		}
	}
	return stats, nil
}

// suspendListing 在事务内把 listing 状态改为 suspended（不硬扣资金）。
func (s *Service) suspendListing(tx c2ccontract.Transaction, l *c2cdomain.Listing) error {
	now := time.Now()
	l.Status = listingStatusSuspended
	l.UpdatedAt = now
	return tx.C2C().UpdateListing(l)
}
