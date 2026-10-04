package migrations

import (
	"errors"
	"fmt"

	userdomain "github.com/Aether-v1/hcz/internal/modules/identity/user/domain"

	"gorm.io/gorm"
)

// inviteCodeUniqueIndex 是 invite_code 的全局唯一索引名。
// 注意：不在 User 结构上打 uniqueIndex tag，因为历史行 invite_code 默认为空串，
// 直接建唯一索引会因多行空串冲突而失败。必须先 backfill 再建索引。
const inviteCodeUniqueIndex = "uni_users_invite_code"

// BackfillInviteCodes 为 invite_code 为空的历史用户批量生成唯一邀请码。
// 幂等：只处理 invite_code 为空（” 或 NULL）的行；已有邀请码的用户不动。
// 历史用户 inviter_id 保持 NULL，不根据旧订单/佣金反推历史邀请关系。
// 完成后创建 invite_code 唯一索引。
func BackfillInviteCodes(db *gorm.DB) error {
	if db == nil {
		return errors.New("db is nil")
	}
	// 列可能不存在于极旧的库；AutoMigrate 已先跑过，这里防御性检查。
	if !db.Migrator().HasColumn(&userdomain.User{}, "invite_code") {
		return nil
	}

	type pendingRow struct {
		ID uint
	}
	var pending []pendingRow
	if err := db.Model(&userdomain.User{}).
		Select("id").
		Where("invite_code IS NULL OR invite_code = ''").
		Find(&pending).Error; err != nil {
		return fmt.Errorf("scan empty invite_code users: %w", err)
	}

	// 逐个生成唯一邀请码并更新。历史数据量通常不大，逐条处理足够且安全。
	for _, row := range pending {
		code, err := uniqueInviteCode(db)
		if err != nil {
			return fmt.Errorf("generate invite code for user %d: %w", row.ID, err)
		}
		if err := db.Model(&userdomain.User{}).
			Where("id = ? AND (invite_code IS NULL OR invite_code = '')", row.ID).
			Update("invite_code", code).Error; err != nil {
			return fmt.Errorf("backfill invite_code for user %d: %w", row.ID, err)
		}
	}

	// backfill 完成后，建立唯一索引（仅约束未删除行，与其他 users 索引口径一致）。
	if !db.Migrator().HasIndex(&userdomain.User{}, inviteCodeUniqueIndex) {
		// 用原生 SQL 创建部分唯一索引，兼容 SQLite 与 PostgreSQL。
		createSQL := fmt.Sprintf(
			"CREATE UNIQUE INDEX IF NOT EXISTS %s ON users (invite_code) WHERE deleted_at IS NULL",
			inviteCodeUniqueIndex,
		)
		if err := db.Exec(createSQL).Error; err != nil {
			return fmt.Errorf("create unique index %s: %w", inviteCodeUniqueIndex, err)
		}
	}
	return nil
}

// uniqueInviteCode 生成一个在当前 users 表中未被占用的邀请码，碰撞则重试。
func uniqueInviteCode(db *gorm.DB) (string, error) {
	const maxAttempts = 16
	for i := 0; i < maxAttempts; i++ {
		code, err := userdomain.GenerateInviteCode()
		if err != nil {
			return "", err
		}
		var count int64
		if err := db.Model(&userdomain.User{}).Where("invite_code = ?", code).Count(&count).Error; err != nil {
			return "", err
		}
		if count == 0 {
			return code, nil
		}
	}
	return "", errors.New("failed to allocate unique invite code after retries")
}
