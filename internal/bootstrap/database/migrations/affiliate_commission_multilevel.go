package migrations

import (
	"errors"
	"fmt"
	"time"

	"github.com/Aether-v1/hcz/internal/logger"
	settingsstore "github.com/Aether-v1/hcz/internal/modules/settings/infrastructure/gormstore"
	"github.com/Aether-v1/hcz/internal/platform/database/gormdb"
	"github.com/Aether-v1/hcz/internal/shared/jsonmap"
	"gorm.io/gorm"
)

const (
	affiliateCommissionMultilevelMigrationKey = "migration/affiliate_commission_multilevel_v1"
	affiliateOldUniqueIndexName               = "idx_affiliate_commission_unique"
	affiliateNewUniqueIndexName               = "idx_affiliate_commission_multilevel_unique"
)

// affiliateCommissionMultilevelSchema 仅用于迁移期间定位 affiliate_commissions 表的索引/列。
type affiliateCommissionMultilevelSchema struct{}

func (affiliateCommissionMultilevelSchema) TableName() string {
	return "affiliate_commissions"
}

// migrateAffiliateCommissionMultilevel 把 affiliate_commissions 从单层（affiliate_profile_id,order_id,commission_type）
// 唯一索引迁移到多级别（order_id,beneficiary_user_id,level,commission_type）唯一索引。
//
// 幂等：
//   - 用 setting key 跟踪完成状态；已完成直接返回。
//   - 列存在性、索引存在性均先判断再操作。
//   - 不删除任何历史 commission；历史数据 level 一律回填为 1，不推算 L2~L10。
func migrateAffiliateCommissionMultilevel() error {
	if gormdb.DB == nil {
		return errors.New("database is not initialized")
	}

	var marker settingsstore.SettingRecord
	if err := gormdb.DB.First(&marker, "key = ?", affiliateCommissionMultilevelMigrationKey).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
	} else if migrationDone(marker.ValueJSON) {
		return nil
	}

	migrator := gormdb.DB.Migrator()

	// 1. 防御性补列（AutoMigrate 通常已完成，这里保证直连/旧库也能跑）。
	if !migrator.HasColumn(&affiliateCommissionMultilevelSchema{}, "beneficiary_user_id") {
		if err := migrator.AddColumn(&affiliateCommissionMultilevelSchema{}, "beneficiary_user_id"); err != nil {
			return fmt.Errorf("add beneficiary_user_id: %w", err)
		}
	}
	if !migrator.HasColumn(&affiliateCommissionMultilevelSchema{}, "source_user_id") {
		if err := migrator.AddColumn(&affiliateCommissionMultilevelSchema{}, "source_user_id"); err != nil {
			return fmt.Errorf("add source_user_id: %w", err)
		}
	}
	if !migrator.HasColumn(&affiliateCommissionMultilevelSchema{}, "level") {
		if err := migrator.AddColumn(&affiliateCommissionMultilevelSchema{}, "level"); err != nil {
			return fmt.Errorf("add level: %w", err)
		}
	}

	// 2. 回填历史数据（只更新未回填的行），全部在事务中执行。
	err := gormdb.DB.Transaction(func(tx *gorm.DB) error {
		// level = 0 -> 1（历史数据默认 1 级，不推算 L2~L10）
		if err := tx.Exec(`UPDATE affiliate_commissions SET level = 1 WHERE level IS NULL OR level = 0`).Error; err != nil {
			return fmt.Errorf("backfill level: %w", err)
		}
		// beneficiary_user_id = 0 -> affiliate_profiles.user_id
		if err := tx.Exec(`UPDATE affiliate_commissions SET beneficiary_user_id = (
			SELECT user_id FROM affiliate_profiles WHERE affiliate_profiles.id = affiliate_commissions.affiliate_profile_id LIMIT 1
		) WHERE beneficiary_user_id = 0`).Error; err != nil {
			return fmt.Errorf("backfill beneficiary_user_id: %w", err)
		}
		// source_user_id = NULL -> orders.user_id（查不到保持 NULL）
		if err := tx.Exec(`UPDATE affiliate_commissions SET source_user_id = (
			SELECT user_id FROM orders WHERE orders.id = affiliate_commissions.order_id LIMIT 1
		) WHERE source_user_id IS NULL`).Error; err != nil {
			return fmt.Errorf("backfill source_user_id: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}

	// 3. 异常检测：回填失败的 beneficiary_user_id=0 记录。
	var zeroBeneficiaryCount int64
	if err := gormdb.DB.Model(&affiliateCommissionMultilevelSchema{}).
		Where("beneficiary_user_id = 0 AND deleted_at IS NULL").
		Count(&zeroBeneficiaryCount).Error; err != nil {
		return err
	}
	if zeroBeneficiaryCount > 0 {
		return fmt.Errorf(
			"affiliate_commission_multilevel: %d commission rows still have beneficiary_user_id=0 after backfill; "+
				"inspect: SELECT id, affiliate_profile_id, order_id FROM affiliate_commissions WHERE beneficiary_user_id=0 AND deleted_at IS NULL",
			zeroBeneficiaryCount,
		)
	}

	// 4. 异常检测：新唯一键上的重复组（order_id, beneficiary_user_id, level, commission_type）。
	type duplicateGroup struct {
		OrderID           uint
		BeneficiaryUserID uint
		Level             int
		CommissionType    string
		Count             int64
	}
	var groups []duplicateGroup
	if err := gormdb.DB.Model(&affiliateCommissionMultilevelSchema{}).
		Select("order_id, beneficiary_user_id, level, commission_type, COUNT(*) AS count").
		Where("deleted_at IS NULL").
		Group("order_id, beneficiary_user_id, level, commission_type").
		Having("COUNT(*) > 1").
		Scan(&groups).Error; err != nil {
		return err
	}
	if len(groups) > 0 {
		sample := groups[0]
		return fmt.Errorf(
			"affiliate_commission_multilevel: found %d duplicate group(s) on (order_id,beneficiary_user_id,level,commission_type); "+
				"first sample order_id=%d beneficiary_user_id=%d level=%d commission_type=%q count=%d; "+
				"inspect with SELECT id FROM affiliate_commissions WHERE order_id=%d AND beneficiary_user_id=%d AND level=%d AND commission_type=%q AND deleted_at IS NULL ORDER BY id",
			len(groups),
			sample.OrderID, sample.BeneficiaryUserID, sample.Level, sample.CommissionType, sample.Count,
			sample.OrderID, sample.BeneficiaryUserID, sample.Level, sample.CommissionType,
		)
	}

	// 5. 删除旧唯一索引（如果存在）。
	if migrator.HasIndex(&affiliateCommissionMultilevelSchema{}, affiliateOldUniqueIndexName) {
		if err := migrator.DropIndex(&affiliateCommissionMultilevelSchema{}, affiliateOldUniqueIndexName); err != nil {
			return fmt.Errorf("drop old unique index %s: %w", affiliateOldUniqueIndexName, err)
		}
	}

	// 6. 确保新唯一索引存在（AutoMigrate 通常已创建，这里兜底）。
	if !migrator.HasIndex(&affiliateCommissionMultilevelSchema{}, affiliateNewUniqueIndexName) {
		if err := migrator.CreateIndex(&affiliateCommissionMultilevelSchema{}, affiliateNewUniqueIndexName); err != nil {
			return fmt.Errorf("create new unique index %s: %w", affiliateNewUniqueIndexName, err)
		}
	}

	// 7. 标记完成。
	marker = settingsstore.SettingRecord{
		Key: affiliateCommissionMultilevelMigrationKey,
		ValueJSON: jsonmap.JSON{
			"done":        true,
			"migrated_at": time.Now().UTC().Format(time.RFC3339),
		},
	}
	if err := gormdb.DB.Save(&marker).Error; err != nil {
		return err
	}
	logger.Infow("migrate_affiliate_commission_multilevel_done")
	return nil
}
