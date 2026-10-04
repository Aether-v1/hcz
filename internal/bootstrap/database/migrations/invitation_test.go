package migrations

import (
	"fmt"
	"testing"
	"time"

	userdomain "github.com/Aether-v1/hcz/internal/modules/identity/user/domain"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupInvitationBackfillDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:invite_backfill_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&userdomain.User{}); err != nil {
		t.Fatalf("migrate users: %v", err)
	}
	return db
}

// 12. 历史用户 backfill 生成 invite_code，且幂等；inviter_id 保持 null
func TestBackfillInviteCodes(t *testing.T) {
	db := setupInvitationBackfillDB(t)

	// 模拟历史用户：invite_code 为空，inviter_id 为 null
	history := []userdomain.User{
		{Email: "h1@example.com", PasswordHash: "x", Status: "active"},
		{Email: "h2@example.com", PasswordHash: "x", Status: "active"},
		{Email: "h3@example.com", PasswordHash: "x", Status: "active"},
	}
	for i := range history {
		if err := db.Create(&history[i]).Error; err != nil {
			t.Fatalf("seed history user: %v", err)
		}
	}
	// 一个已有邀请码的用户，backfill 不应改动
	already := userdomain.User{Email: "already@example.com", PasswordHash: "x", Status: "active", InviteCode: "KEEPME12"}
	if err := db.Create(&already).Error; err != nil {
		t.Fatalf("seed already user: %v", err)
	}

	if err := BackfillInviteCodes(db); err != nil {
		t.Fatalf("backfill: %v", err)
	}

	// 校验：历史用户都拿到了非空唯一邀请码；inviter_id 保持 null；已有码用户不变
	codes := map[string]bool{}
	for _, id := range []uint{history[0].ID, history[1].ID, history[2].ID} {
		var u userdomain.User
		if err := db.First(&u, id).Error; err != nil {
			t.Fatalf("reload user %d: %v", id, err)
		}
		if u.InviteCode == "" {
			t.Fatalf("history user %d should have invite_code after backfill", id)
		}
		if codes[u.InviteCode] {
			t.Fatalf("duplicate invite_code after backfill: %s", u.InviteCode)
		}
		codes[u.InviteCode] = true
		if u.InviterID != nil {
			t.Fatalf("history user %d inviter_id must stay null, got %v", id, u.InviterID)
		}
	}
	var kept userdomain.User
	if err := db.First(&kept, already.ID).Error; err != nil {
		t.Fatalf("reload already: %v", err)
	}
	if kept.InviteCode != "KEEPME12" {
		t.Fatalf("already-coded user must not be rewritten: %q", kept.InviteCode)
	}

	// 唯一索引已创建
	if !db.Migrator().HasIndex(&userdomain.User{}, inviteCodeUniqueIndex) {
		t.Fatalf("unique index %s should exist", inviteCodeUniqueIndex)
	}

	// 幂等：再跑一次不应报错、不应改动已有码
	if err := BackfillInviteCodes(db); err != nil {
		t.Fatalf("second backfill run: %v", err)
	}
	var after userdomain.User
	db.First(&after, history[0].ID)
	if !codes[after.InviteCode] {
		t.Fatalf("idempotent backfill changed existing code: %q", after.InviteCode)
	}
}

// 13. invite_code 唯一性约束：唯一索引建成后插入重复码应失败
func TestInviteCodeUniqueIndexEnforced(t *testing.T) {
	db := setupInvitationBackfillDB(t)
	u1 := userdomain.User{Email: "u1@example.com", PasswordHash: "x", Status: "active", InviteCode: "FIRSTCODE"}
	if err := db.Create(&u1).Error; err != nil {
		t.Fatalf("create u1: %v", err)
	}
	if err := BackfillInviteCodes(db); err != nil {
		t.Fatalf("backfill: %v", err)
	}
	// 用相同 invite_code 插入第二行，应因唯一索引失败
	u2 := userdomain.User{Email: "u2@example.com", PasswordHash: "x", Status: "active", InviteCode: "FIRSTCODE"}
	err := db.Create(&u2).Error
	if err == nil {
		t.Fatalf("expected unique constraint violation on duplicate invite_code")
	}
}
