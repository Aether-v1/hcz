package migrations

// affiliate_application_migration_test.go — affiliate_applications 表与回填迁移测试（SQLite）。
//
// 覆盖：
//   30. fresh DB → AutoMigrate → affiliate_applications / affiliate_commission_ledgers / affiliate_profiles 表存在。
//   31. affiliate_commission_ledgers 表存在（P1 修复验证）。
//   32. 预存 active profile → migrateGrandfatheredAffiliateApplications → 生成 approved application。
//   33. 预存 disabled profile → 不生成 application。
//   34. migration 幂等：跑两次不产生重复 application。
//
// 说明：partial unique index（idx_affiliate_apps_user_pending）仅在 PostgreSQL 创建，
// SQLite 不支持 partial index；其正确性由 application_pg_concurrency_test.go（integration tag）验证。

import (
	"fmt"
	"testing"
	"time"

	"github.com/Aether-v1/hcz/internal/constants"

	affiliatedomain "github.com/Aether-v1/hcz/internal/modules/affiliate/domain"
	userdomain "github.com/Aether-v1/hcz/internal/modules/identity/user/domain"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupAppMigrationDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:app_migrate_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	return db
}

// sqliteTableExists 判断 SQLite 中某表是否存在。
func sqliteTableExists(t *testing.T, db *gorm.DB, name string) bool {
	t.Helper()
	var count int64
	if err := db.Raw(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, name).Scan(&count).Error; err != nil {
		t.Fatalf("check table %s: %v", name, err)
	}
	return count > 0
}

func seedMigUser(t *testing.T, db *gorm.DB, email string) userdomain.User {
	t.Helper()
	u := userdomain.User{Email: email, PasswordHash: "x", Status: "active"}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return u
}

func seedMigProfile(t *testing.T, db *gorm.DB, userID uint, code, status string) affiliatedomain.Profile {
	t.Helper()
	p := affiliatedomain.Profile{UserID: userID, AffiliateCode: code, Status: status}
	if err := db.Create(&p).Error; err != nil {
		t.Fatalf("seed profile: %v", err)
	}
	return p
}

// 30. TestFreshDBAutoMigrate_CreatesAllTables
func TestFreshDBAutoMigrate_CreatesAllTables(t *testing.T) {
	db := setupAppMigrationDB(t)
	if err := db.AutoMigrate(
		&userdomain.User{},
		&affiliatedomain.Profile{},
		&affiliatedomain.CommissionLedger{},
		&affiliatedomain.Application{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	for _, tbl := range []string{"users", "affiliate_profiles", "affiliate_commission_ledgers", "affiliate_applications"} {
		if !sqliteTableExists(t, db, tbl) {
			t.Fatalf("expected table %s to exist after AutoMigrate", tbl)
		}
	}
}

// 31. TestCommissionLedgerTableExists —— affiliate_commission_ledgers 表已创建（P1 修复验证）。
func TestCommissionLedgerTableExists(t *testing.T) {
	db := setupAppMigrationDB(t)
	if err := db.AutoMigrate(&affiliatedomain.CommissionLedger{}); err != nil {
		t.Fatalf("auto migrate ledger: %v", err)
	}
	if !sqliteTableExists(t, db, "affiliate_commission_ledgers") {
		t.Fatal("affiliate_commission_ledgers table must exist (P1)")
	}
}

// 32. TestGrandfatherMigration_ActiveProfileGetsApprovedApp
func TestGrandfatherMigration_ActiveProfileGetsApprovedApp(t *testing.T) {
	db := setupAppMigrationDB(t)
	if err := db.AutoMigrate(&userdomain.User{}, &affiliatedomain.Profile{}, &affiliatedomain.Application{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	u := seedMigUser(t, db, "gf-active@hcz.test")
	seedMigProfile(t, db, u.ID, "GFPA0001", constants.AffiliateProfileStatusActive)

	if err := migrateGrandfatheredAffiliateApplications(db); err != nil {
		t.Fatalf("migration: %v", err)
	}

	var apps []affiliatedomain.Application
	if err := db.Where("user_id = ?", u.ID).Find(&apps).Error; err != nil {
		t.Fatalf("load apps: %v", err)
	}
	if len(apps) != 1 {
		t.Fatalf("want 1 grandfathered approved app for active profile, got %d", len(apps))
	}
	if apps[0].Status != constants.AffiliateAppStatusApproved {
		t.Fatalf("grandfathered app status want approved, got %q", apps[0].Status)
	}
}

// 33. TestGrandfatherMigration_DisabledProfileNoApp
func TestGrandfatherMigration_DisabledProfileNoApp(t *testing.T) {
	db := setupAppMigrationDB(t)
	if err := db.AutoMigrate(&userdomain.User{}, &affiliatedomain.Profile{}, &affiliatedomain.Application{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	u := seedMigUser(t, db, "gf-disabled@hcz.test")
	seedMigProfile(t, db, u.ID, "GFPD0001", constants.AffiliateProfileStatusDisabled)

	if err := migrateGrandfatheredAffiliateApplications(db); err != nil {
		t.Fatalf("migration: %v", err)
	}

	var count int64
	db.Model(&affiliatedomain.Application{}).Where("user_id = ?", u.ID).Count(&count)
	if count != 0 {
		t.Fatalf("disabled profile must NOT get a grandfathered app, got %d", count)
	}
}

// 34. TestGrandfatherMigration_Idempotent
func TestGrandfatherMigration_Idempotent(t *testing.T) {
	db := setupAppMigrationDB(t)
	if err := db.AutoMigrate(&userdomain.User{}, &affiliatedomain.Profile{}, &affiliatedomain.Application{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	u1 := seedMigUser(t, db, "gf-idem1@hcz.test")
	u2 := seedMigUser(t, db, "gf-idem2@hcz.test")
	seedMigProfile(t, db, u1.ID, "GFPI001", constants.AffiliateProfileStatusActive)
	seedMigProfile(t, db, u2.ID, "GFPI002", constants.AffiliateProfileStatusActive)

	if err := migrateGrandfatheredAffiliateApplications(db); err != nil {
		t.Fatalf("first migration: %v", err)
	}
	// 第二次运行必须幂等，不产生重复 app。
	if err := migrateGrandfatheredAffiliateApplications(db); err != nil {
		t.Fatalf("second migration: %v", err)
	}

	var count int64
	db.Model(&affiliatedomain.Application{}).Count(&count)
	if count != 2 {
		t.Fatalf("idempotent migration should keep 2 apps, got %d", count)
	}
}
