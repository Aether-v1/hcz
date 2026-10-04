package migrations

import (
	"errors"
	"fmt"
	"time"

	"github.com/Aether-v1/hcz/internal/logger"
	settingsstore "github.com/Aether-v1/hcz/internal/modules/settings/infrastructure/gormstore"
	walletdomain "github.com/Aether-v1/hcz/internal/modules/wallet/domain"
	"github.com/Aether-v1/hcz/internal/platform/database/gormdb"
	"github.com/Aether-v1/hcz/internal/shared/jsonmap"
	"gorm.io/gorm"
)

const walletDualBalanceMigrationKey = "migration/wallet_dual_balance_v1"

// walletAccountLegacySchema 仅用于迁移期间定位 wallet_accounts 表的旧 balance 列。
type walletAccountLegacySchema struct{}

func (walletAccountLegacySchema) TableName() string { return "wallet_accounts" }

// walletTransactionLegacySchema 仅用于迁移期间定位 wallet_transactions 表的旧 balance 列。
type walletTransactionLegacySchema struct{}

func (walletTransactionLegacySchema) TableName() string { return "wallet_transactions" }

// migrateWalletDualBalance 把 wallet_accounts 从单余额（balance）迁移为双余额
// （available_balance + frozen_balance），同时为 wallet_transactions 补齐可用/冻结快照列。
//
// 幂等：
//   - 用 setting key 跟踪完成状态；已完成直接返回。
//   - 列存在性先判断再补列。
//   - 回填只更新未回填的行，不覆盖已迁移数据。
//   - 不删除旧 balance 列（staged migration，待代码切换完成后再处理）。
func migrateWalletDualBalance() error {
	if gormdb.DB == nil {
		return errors.New("database is not initialized")
	}

	var marker settingsstore.SettingRecord
	if err := gormdb.DB.First(&marker, "key = ?", walletDualBalanceMigrationKey).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
	} else if migrationDone(marker.ValueJSON) {
		return nil
	}

	migrator := gormdb.DB.Migrator()

	// ============================================================
	// wallet_accounts: 防御性补列
	// ============================================================
	if !migrator.HasColumn(&walletdomain.Account{}, "available_balance") {
		if err := migrator.AddColumn(&walletdomain.Account{}, "available_balance"); err != nil {
			return fmt.Errorf("add wallet_accounts.available_balance: %w", err)
		}
	}
	if !migrator.HasColumn(&walletdomain.Account{}, "frozen_balance") {
		if err := migrator.AddColumn(&walletdomain.Account{}, "frozen_balance"); err != nil {
			return fmt.Errorf("add wallet_accounts.frozen_balance: %w", err)
		}
	}

	// ============================================================
	// wallet_transactions: 防御性补列
	// ============================================================
	ledgerNewCols := []string{"available_before", "available_after", "frozen_before", "frozen_after"}
	for _, col := range ledgerNewCols {
		if !migrator.HasColumn(&walletdomain.Transaction{}, col) {
			if err := migrator.AddColumn(&walletdomain.Transaction{}, col); err != nil {
				return fmt.Errorf("add wallet_transactions.%s: %w", col, err)
			}
		}
	}

	// ============================================================
	// 事务内回填
	// ============================================================
	hasOldBalanceCol := migrator.HasColumn(&walletAccountLegacySchema{}, "balance")
	hasOldLedgerCol := migrator.HasColumn(&walletTransactionLegacySchema{}, "balance_before")

	err := gormdb.DB.Transaction(func(tx *gorm.DB) error {
		// wallet_accounts: 把旧 balance 回填到 available_balance（仅当旧列存在时）
		// 幂等条件：available_balance 仍为 0 且旧 balance 非 0（只回填未迁移的行）
		if hasOldBalanceCol {
			if err := tx.Exec(`UPDATE wallet_accounts
				SET available_balance = balance
				WHERE available_balance = 0 AND balance <> 0`).Error; err != nil {
				return fmt.Errorf("backfill wallet_accounts.available_balance: %w", err)
			}
		}
		// wallet_accounts: 确保 frozen_balance 全为 0
		if err := tx.Exec(`UPDATE wallet_accounts
			SET frozen_balance = 0
			WHERE frozen_balance IS NULL OR frozen_balance <> 0`).Error; err != nil {
			return fmt.Errorf("reset wallet_accounts.frozen_balance: %w", err)
		}

		// wallet_transactions: 把旧 balance_before/after 回填到 available_before/after（仅当旧列存在时）
		// 幂等条件：4 个新列全为 0 且旧列非全零（只回填未迁移的行）
		if hasOldLedgerCol {
			if err := tx.Exec(`UPDATE wallet_transactions
				SET available_before = balance_before, available_after = balance_after
				WHERE available_before = 0 AND available_after = 0
				  AND frozen_before = 0 AND frozen_after = 0
				  AND (balance_before <> 0 OR balance_after <> 0)`).Error; err != nil {
				return fmt.Errorf("backfill wallet_transactions.available: %w", err)
			}
		}
		// wallet_transactions: 确保 frozen_before/after 全为 0
		if err := tx.Exec(`UPDATE wallet_transactions
			SET frozen_before = 0, frozen_after = 0
			WHERE frozen_before IS NULL OR frozen_after IS NULL
			   OR frozen_before <> 0 OR frozen_after <> 0`).Error; err != nil {
			return fmt.Errorf("reset wallet_transactions.frozen: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}

	// ============================================================
	// 校验
	// ============================================================

	// wallet_accounts: 非负校验 + 无 NULL
	var negAvailCount, negFrozenCount, nullCount int64
	if err := gormdb.DB.Model(&walletdomain.Account{}).
		Where("available_balance < 0").Count(&negAvailCount).Error; err != nil {
		return fmt.Errorf("validate available_balance >= 0: %w", err)
	}
	if negAvailCount > 0 {
		return fmt.Errorf("wallet_dual_balance: %d rows have negative available_balance", negAvailCount)
	}
	if err := gormdb.DB.Model(&walletdomain.Account{}).
		Where("frozen_balance < 0").Count(&negFrozenCount).Error; err != nil {
		return fmt.Errorf("validate frozen_balance >= 0: %w", err)
	}
	if negFrozenCount > 0 {
		return fmt.Errorf("wallet_dual_balance: %d rows have negative frozen_balance", negFrozenCount)
	}
	if err := gormdb.DB.Model(&walletdomain.Account{}).
		Where("available_balance IS NULL OR frozen_balance IS NULL").
		Count(&nullCount).Error; err != nil {
		return fmt.Errorf("validate no NULL in wallet_accounts: %w", err)
	}
	if nullCount > 0 {
		return fmt.Errorf("wallet_dual_balance: %d rows have NULL in available_balance/frozen_balance", nullCount)
	}

	// wallet_accounts: 总资产守恒（仅当旧 balance 列存在时校验）
	if hasOldBalanceCol {
		var totals struct {
			TotalBalance  float64
			TotalAvailFrz float64
		}
		if err := gormdb.DB.Raw(`SELECT
			COALESCE(SUM(balance), 0) AS total_balance,
			COALESCE(SUM(available_balance + frozen_balance), 0) AS total_avail_frz
			FROM wallet_accounts`).Scan(&totals).Error; err != nil {
			return fmt.Errorf("validate total balance conservation: %w", err)
		}
		if totals.TotalBalance != totals.TotalAvailFrz {
			return fmt.Errorf("wallet_dual_balance: total balance mismatch: SUM(balance)=%.2f vs SUM(available+frozen)=%.2f",
				totals.TotalBalance, totals.TotalAvailFrz)
		}
	}

	// wallet_transactions: balance_before = available_before + frozen_before（仅当旧列存在时）
	if hasOldLedgerCol {
		var ledgerMismatchCount int64
		if err := gormdb.DB.Raw(`SELECT COUNT(*) FROM wallet_transactions
			WHERE ABS(balance_before - (available_before + frozen_before)) > 0.001
			   OR ABS(balance_after - (available_after + frozen_after)) > 0.001`).
			Scan(&ledgerMismatchCount).Error; err != nil {
			return fmt.Errorf("validate ledger conservation: %w", err)
		}
		if ledgerMismatchCount > 0 {
			return fmt.Errorf("wallet_dual_balance: %d ledger rows fail balance = available + frozen conservation", ledgerMismatchCount)
		}
	}

	// ============================================================
	// 标记完成
	// ============================================================
	marker = settingsstore.SettingRecord{
		Key: walletDualBalanceMigrationKey,
		ValueJSON: jsonmap.JSON{
			"done":        true,
			"migrated_at": time.Now().UTC().Format(time.RFC3339),
		},
	}
	if err := gormdb.DB.Save(&marker).Error; err != nil {
		return err
	}
	logger.Infow("migrate_wallet_dual_balance_done")
	return nil
}
