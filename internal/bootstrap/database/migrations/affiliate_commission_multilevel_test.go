package migrations

import (
	"fmt"
	"testing"
	"time"

	affiliatedomain "github.com/Aether-v1/hcz/internal/modules/affiliate/domain"
	userdomain "github.com/Aether-v1/hcz/internal/modules/identity/user/domain"
	orderdomain "github.com/Aether-v1/hcz/internal/modules/order/domain"
	settingsstore "github.com/Aether-v1/hcz/internal/modules/settings/infrastructure/gormstore"
	"github.com/Aether-v1/hcz/internal/platform/database/gormdb"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// setupMultilevelMigrationDB 搭建一个"升级前旧版本"的历史库：
//   - users / affiliate_profiles / orders / settings / affiliate_commissions 均已建表；
//   - affiliate_commissions 已含新列（AutoMigrate 产物）与新唯一索引；
//   - 历史行未回填（level=0, beneficiary=0, source=NULL）；
//   - 旧唯一索引 idx_affiliate_commission_unique 仍存在（待迁移删除）。
func setupMultilevelMigrationDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:ml_migrate_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&userdomain.User{},
		&affiliatedomain.Profile{},
		&affiliatedomain.Commission{},
		&orderdomain.Order{},
		&settingsstore.SettingRecord{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	// 旧版本残留的旧唯一索引。
	if err := db.Exec(`CREATE UNIQUE INDEX idx_affiliate_commission_unique ON affiliate_commissions(affiliate_profile_id, order_id, commission_type)`).Error; err != nil {
		t.Fatalf("create old unique index: %v", err)
	}

	prev := gormdb.DB
	gormdb.DB = db
	t.Cleanup(func() { gormdb.DB = prev })
	return db
}

func seedHistoryUsers(t *testing.T, db *gorm.DB, emails ...string) []userdomain.User {
	t.Helper()
	out := make([]userdomain.User, 0, len(emails))
	for _, e := range emails {
		u := userdomain.User{Email: e, PasswordHash: "x", Status: "active"}
		if err := db.Create(&u).Error; err != nil {
			t.Fatalf("seed user %s: %v", e, err)
		}
		out = append(out, u)
	}
	return out
}

func seedHistoryProfile(t *testing.T, db *gorm.DB, userID uint, code string) affiliatedomain.Profile {
	t.Helper()
	p := affiliatedomain.Profile{UserID: userID, AffiliateCode: code, Status: "active"}
	if err := db.Create(&p).Error; err != nil {
		t.Fatalf("seed profile: %v", err)
	}
	return p
}

func seedHistoryOrder(t *testing.T, db *gorm.DB, userID uint, orderNo string) orderdomain.Order {
	t.Helper()
	o := orderdomain.Order{OrderNo: orderNo, UserID: userID, Status: "completed"}
	if err := db.Create(&o).Error; err != nil {
		t.Fatalf("seed order %s: %v", orderNo, err)
	}
	return o
}

// seedUnbackfilledCommission 插入一条历史 commission：level/beneficiary 未回填。
func seedUnbackfilledCommission(t *testing.T, db *gorm.DB, profileID, orderID uint) {
	t.Helper()
	if err := db.Exec(`INSERT INTO affiliate_commissions
		(affiliate_profile_id, order_id, commission_type, beneficiary_user_id, source_user_id, level, base_amount, rate_percent, commission_amount, status, created_at, updated_at)
		VALUES (?, ?, 'order', 0, NULL, 0, 100, 5, 5, 'available', ?, ?)`,
		profileID, orderID, time.Now(), time.Now()).Error; err != nil {
		t.Fatalf("seed commission: %v", err)
	}
}

type commissionRow struct {
	ID                uint
	BeneficiaryUserID uint
	SourceUserID      *uint
	Level             int
}

func loadCommissionRows(t *testing.T, db *gorm.DB) []commissionRow {
	t.Helper()
	var rows []commissionRow
	if err := db.Raw(`SELECT id, beneficiary_user_id, source_user_id, level FROM affiliate_commissions ORDER BY id`).
		Scan(&rows).Error; err != nil {
		t.Fatalf("load commissions: %v", err)
	}
	return rows
}

func indexExists(t *testing.T, db *gorm.DB, name string) bool {
	t.Helper()
	var count int64
	db.Raw(`SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name=?`, name).Scan(&count)
	return count > 0
}

// 1. 历史数据回填正确：level=1，beneficiary/source 正确回填，查不到 source 保持 NULL；旧索引删除、新索引保留；幂等。
func TestMigrateAffiliateCommissionMultilevel_HappyPath(t *testing.T) {
	db := setupMultilevelMigrationDB(t)

	users := seedHistoryUsers(t, db, "h-u1@ml.test", "h-u2@ml.test", "h-buyer1@ml.test", "h-buyer2@ml.test")
	u1, u2, buyer1, buyer2 := users[0], users[1], users[2], users[3]
	p1 := seedHistoryProfile(t, db, u1.ID, "HPA001")
	p2 := seedHistoryProfile(t, db, u2.ID, "HPA002")
	o1 := seedHistoryOrder(t, db, buyer1.ID, "HORD0001")
	o2 := seedHistoryOrder(t, db, buyer2.ID, "HORD0002")

	seedUnbackfilledCommission(t, db, p1.ID, o1.ID) // -> beneficiary u1, source buyer1
	seedUnbackfilledCommission(t, db, p2.ID, o2.ID) // -> beneficiary u2, source buyer2
	seedUnbackfilledCommission(t, db, p1.ID, 8888)  // order 已删除 -> source NULL

	if err := migrateAffiliateCommissionMultilevel(); err != nil {
		t.Fatalf("migration: %v", err)
	}

	rows := loadCommissionRows(t, db)
	if len(rows) != 3 {
		t.Fatalf("expected 3 commissions preserved, got %d", len(rows))
	}
	first := rows[0]
	if first.Level != 1 {
		t.Fatalf("expected level=1, got %d", first.Level)
	}
	if first.BeneficiaryUserID != u1.ID {
		t.Fatalf("expected beneficiary backfilled to %d, got %d", u1.ID, first.BeneficiaryUserID)
	}
	if first.SourceUserID == nil || *first.SourceUserID != buyer1.ID {
		t.Fatalf("expected source backfilled to buyer1, got %v", first.SourceUserID)
	}
	second := rows[1]
	if second.BeneficiaryUserID != u2.ID || second.SourceUserID == nil || *second.SourceUserID != buyer2.ID {
		t.Fatalf("unexpected second row: %+v", second)
	}
	third := rows[2]
	if third.BeneficiaryUserID != u1.ID {
		t.Fatalf("expected third beneficiary backfilled to u1, got %d", third.BeneficiaryUserID)
	}
	if third.SourceUserID != nil {
		t.Fatalf("expected third source NULL (order deleted), got %v", *third.SourceUserID)
	}

	if indexExists(t, db, affiliateOldUniqueIndexName) {
		t.Fatalf("old unique index %s should be dropped", affiliateOldUniqueIndexName)
	}
	if !indexExists(t, db, affiliateNewUniqueIndexName) {
		t.Fatalf("new unique index %s should exist", affiliateNewUniqueIndexName)
	}

	// 幂等：再跑一次不报错、数据不变。
	if err := migrateAffiliateCommissionMultilevel(); err != nil {
		t.Fatalf("idempotent re-run: %v", err)
	}
	rows2 := loadCommissionRows(t, db)
	if len(rows2) != 3 {
		t.Fatalf("idempotent run changed row count: %d", len(rows2))
	}
	for i := range rows2 {
		if rows2[i].BeneficiaryUserID != rows[i].BeneficiaryUserID || rows2[i].Level != rows[i].Level {
			t.Fatalf("idempotent run changed row %d: %+v -> %+v", i, rows[i], rows2[i])
		}
	}
}

// 2. affiliate_profile 已被硬删除的 commission 无法回填 beneficiary，迁移报错中止。
func TestMigrateAffiliateCommissionMultilevel_OrphanProfileFails(t *testing.T) {
	db := setupMultilevelMigrationDB(t)

	users := seedHistoryUsers(t, db, "o-u1@ml.test", "o-buyer@ml.test", "o-buyer2@ml.test")
	p1 := seedHistoryProfile(t, db, users[0].ID, "OPA001")
	o1 := seedHistoryOrder(t, db, users[1].ID, "OORD0001")
	o2 := seedHistoryOrder(t, db, users[2].ID, "OORD0002")
	seedUnbackfilledCommission(t, db, p1.ID, o1.ID)
	// 一条指向不存在 profile 的历史 commission（不同 order 以避开新唯一索引）。
	seedUnbackfilledCommission(t, db, 9999, o2.ID)

	if err := migrateAffiliateCommissionMultilevel(); err == nil {
		t.Fatalf("expected migration to fail on orphan commission (beneficiary cannot be backfilled)")
	}
}

// 3. 回填后出现 (order, beneficiary, level) 重复组，迁移报错中止。
func TestMigrateAffiliateCommissionMultilevel_DuplicateDetected(t *testing.T) {
	db := setupMultilevelMigrationDB(t)

	users := seedHistoryUsers(t, db, "d-u1@ml.test", "d-buyer@ml.test")
	u1, buyer := users[0], users[1]
	p1 := seedHistoryProfile(t, db, u1.ID, "DPA001")
	o1 := seedHistoryOrder(t, db, buyer.ID, "DORD0001")

	// C1 已经是"回填后"形态：beneficiary=u1, level=1（异常历史数据）。
	if err := db.Exec(`INSERT INTO affiliate_commissions
		(affiliate_profile_id, order_id, commission_type, beneficiary_user_id, source_user_id, level, base_amount, rate_percent, commission_amount, status, created_at, updated_at)
		VALUES (7777, ?, 'order', ?, ?, 1, 100, 5, 5, 'available', ?, ?)`,
		o1.ID, u1.ID, buyer.ID, time.Now(), time.Now()).Error; err != nil {
		t.Fatalf("seed C1: %v", err)
	}
	// C2 未回填：profile=p1(user=u1), order=o1 -> 回填后与 C1 形成重复组。
	seedUnbackfilledCommission(t, db, p1.ID, o1.ID)

	if err := migrateAffiliateCommissionMultilevel(); err == nil {
		t.Fatalf("expected migration to detect duplicate (order,beneficiary,level) group")
	}
}
