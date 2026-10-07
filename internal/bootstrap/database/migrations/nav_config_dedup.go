package migrations

import (
	"errors"

	"github.com/Aether-v1/hcz/internal/constants"
	settingsstore "github.com/Aether-v1/hcz/internal/modules/settings/infrastructure/gormstore"
	"github.com/Aether-v1/hcz/internal/shared/jsonmap"
	"gorm.io/gorm"
)

// MigrateNavConfigDedup 是 P1-5 Navigation 单一数据源收口的一次性数据抢救迁移。
//
// 背景：历史上 Site Builder → NavFooter 会把导航配置嵌套写入
// settings.value_json（key='site_config'，Storage C），但前台
// public/handler.go 只读 settings.key='nav_config'（Storage A），
// 导致 Storage C 中的 nav_config 是死存储、前台不生效。
//
// 本迁移：
//  1. 读取 settings.key='site_config' 行的 value_json.nav_config；
//  2. 若该嵌套字段非空，且独立 settings.key='nav_config' 行不存在或为空，
//     则把嵌套内容抢救写入独立 nav_config 行（不覆盖已存在的活配置）；
//  3. 无论是否抢救，都从 site_config.value_json 中删除 nav_config 嵌套字段。
//
// 幂等：
//   - site_config 中已无 nav_config 嵌套字段 → 直接 no-op；
//   - nav_config 独立行已存在且非空 → 不覆盖，只清理嵌套字段；
//   - 重复执行不会重复写入、不会破坏 Storage A / Storage B（reseller overlay）。
func MigrateNavConfigDedup(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	if !db.Migrator().HasTable(&settingsstore.SettingRecord{}) {
		return nil
	}

	var siteCfg settingsstore.SettingRecord
	err := db.Where("key = ?", "site_config").First(&siteCfg).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}

	legacyRaw, hasLegacy := siteCfg.ValueJSON["nav_config"]
	if !hasLegacy {
		return nil
	}
	legacyMap, ok := legacyRaw.(map[string]interface{})
	if !ok || len(legacyMap) == 0 {
		// 嵌套字段存在但不是合法对象，直接清理。
		return removeNavConfigFromSiteConfig(db)
	}

	// 检查独立 nav_config 行（Storage A）是否已存在。
	var navRow settingsstore.SettingRecord
	navErr := db.Where("key = ?", constants.SettingKeyNavConfig).First(&navRow).Error
	switch {
	case errors.Is(navErr, gorm.ErrRecordNotFound):
		// 独立行不存在 → 用嵌套内容回填。
		navRow = settingsstore.SettingRecord{
			Key:       constants.SettingKeyNavConfig,
			ValueJSON: jsonmap.JSON(legacyMap),
		}
		if err := db.Create(&navRow).Error; err != nil {
			return err
		}
	case navErr != nil:
		return navErr
	case len(navRow.ValueJSON) > 0:
		// 独立行已有活配置 → 不覆盖，仅清理 site_config 中的嵌套死字段。
	default:
		// 独立行存在但为空 → 用嵌套内容回填覆盖。
		navRow.ValueJSON = jsonmap.JSON(legacyMap)
		if err := db.Save(&navRow).Error; err != nil {
			return err
		}
	}

	return removeNavConfigFromSiteConfig(db)
}

// removeNavConfigFromSiteConfig 重新读取最新 site_config，删除其 value_json.nav_config 字段。
// 重新读取避免与运行期间其他对 site_config 的写入发生覆盖。
func removeNavConfigFromSiteConfig(db *gorm.DB) error {
	var fresh settingsstore.SettingRecord
	if err := db.Where("key = ?", "site_config").First(&fresh).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	if _, stillThere := fresh.ValueJSON["nav_config"]; !stillThere {
		return nil
	}
	delete(fresh.ValueJSON, "nav_config")
	return db.Model(&settingsstore.SettingRecord{}).
		Where("key = ?", "site_config").
		Update("value_json", fresh.ValueJSON).Error
}
