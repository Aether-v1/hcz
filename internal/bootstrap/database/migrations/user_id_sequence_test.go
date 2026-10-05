package migrations

import (
	"fmt"
	"testing"
	"time"

	userdomain "github.com/Aether-v1/hcz/internal/modules/identity/user/domain"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupUserIDSeqDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:user_id_seq_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&userdomain.User{}); err != nil {
		t.Fatalf("migrate users: %v", err)
	}
	return db
}

// TestEnsureUserIDSequenceStart_FreshInstall 全新空表：第一个用户 ID=1000，第二个=1001。
func TestEnsureUserIDSequenceStart_FreshInstall(t *testing.T) {
	db := setupUserIDSeqDB(t)

	if err := ensureUserIDSequenceStart(db); err != nil {
		t.Fatalf("ensureUserIDSequenceStart: %v", err)
	}

	first := userdomain.User{Email: "fresh1@example.com", PasswordHash: "x", Status: "active"}
	if err := db.Create(&first).Error; err != nil {
		t.Fatalf("create first user: %v", err)
	}
	if first.ID != 1000 {
		t.Fatalf("fresh install first user ID = %d, want 1000", first.ID)
	}

	second := userdomain.User{Email: "fresh2@example.com", PasswordHash: "x", Status: "active"}
	if err := db.Create(&second).Error; err != nil {
		t.Fatalf("create second user: %v", err)
	}
	if second.ID != 1001 {
		t.Fatalf("fresh install second user ID = %d, want 1001", second.ID)
	}
}

// TestEnsureUserIDSequenceStart_IdempotentOnEmpty 空表重复执行不报错、不改变下一个 ID。
func TestEnsureUserIDSequenceStart_IdempotentOnEmpty(t *testing.T) {
	db := setupUserIDSeqDB(t)

	if err := ensureUserIDSequenceStart(db); err != nil {
		t.Fatalf("first run: %v", err)
	}
	if err := ensureUserIDSequenceStart(db); err != nil {
		t.Fatalf("second run must be idempotent: %v", err)
	}

	u := userdomain.User{Email: "idem@example.com", PasswordHash: "x", Status: "active"}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	if u.ID != 1000 {
		t.Fatalf("idempotent runs should still yield first ID=1000, got %d", u.ID)
	}
}

// TestEnsureUserIDSequenceStart_DoesNotRegress 已有用户 ID=2350 时，
// sequence 不回退，下一个自动 ID >= 2351。
func TestEnsureUserIDSequenceStart_DoesNotRegress(t *testing.T) {
	db := setupUserIDSeqDB(t)

	// 模拟已有历史用户，显式指定 ID=2350。
	existing := userdomain.User{Email: "legacy@example.com", PasswordHash: "x", Status: "active"}
	existing.ID = 2350
	if err := db.Create(&existing).Error; err != nil {
		t.Fatalf("seed existing user with id=2350: %v", err)
	}

	if err := ensureUserIDSequenceStart(db); err != nil {
		t.Fatalf("ensureUserIDSequenceStart: %v", err)
	}

	next := userdomain.User{Email: "next@example.com", PasswordHash: "x", Status: "active"}
	if err := db.Create(&next).Error; err != nil {
		t.Fatalf("create next user: %v", err)
	}
	if next.ID < 2351 {
		t.Fatalf("next user ID = %d, want >= 2351 (must not regress below existing max)", next.ID)
	}

	// 幂等：再跑一次，下一个 ID 继续递增而非回退到 1000。
	if err := ensureUserIDSequenceStart(db); err != nil {
		t.Fatalf("second run: %v", err)
	}
	after := userdomain.User{Email: "after@example.com", PasswordHash: "x", Status: "active"}
	if err := db.Create(&after).Error; err != nil {
		t.Fatalf("create after second run: %v", err)
	}
	if after.ID <= next.ID {
		t.Fatalf("after ID = %d, want > %d (sequence must keep advancing)", after.ID, next.ID)
	}
}
