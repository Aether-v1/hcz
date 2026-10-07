package migrations

import (
	"errors"
	"fmt"

	orderdomain "github.com/Aether-v1/hcz/internal/modules/order/domain"
	"github.com/Aether-v1/hcz/internal/platform/database/gormdb"
	"gorm.io/gorm"
)

const (
	orderIdempotencyUniqueIndex = "idx_orders_user_idempotency"
)

// ensureOrderIdempotencyUniqueIndex 为正式用户订单创建幂等部分唯一索引。
// 仅对 idempotency_key 非空的行生效，避免游客订单（user_id=0, key=”）和子订单（key=”）冲突。
// 这是 Order Create 幂等的数据库级安全边界：并发相同 (user_id, idempotency_key) 最多插入一行。
func ensureOrderIdempotencyUniqueIndex() error {
	if gormdb.DB == nil {
		return errors.New("database is not initialized")
	}
	migrator := gormdb.DB.Migrator()
	if migrator.HasIndex(&orderdomain.Order{}, orderIdempotencyUniqueIndex) {
		return nil
	}

	// 预检：是否存在重复的 (user_id, idempotency_key) 组合（仅非空 key）。
	// 历史数据理论上不会有（此前无幂等功能），但为安全起见先检查。
	type dupGroup struct {
		UserID         uint
		IdempotencyKey string
		Count          int64
	}
	var groups []dupGroup
	if err := gormdb.DB.Model(&orderdomain.Order{}).
		Select("user_id, idempotency_key, COUNT(*) AS count").
		Where("idempotency_key <> '' AND deleted_at IS NULL").
		Group("user_id, idempotency_key").
		Having("COUNT(*) > 1").
		Scan(&groups).Error; err != nil {
		return err
	}
	if len(groups) > 0 {
		sample := groups[0]
		return fmt.Errorf(
			"cannot create %s: found %d duplicate (user_id, idempotency_key) group(s); "+
				"first group user_id=%d idempotency_key=%q count=%d",
			orderIdempotencyUniqueIndex,
			len(groups),
			sample.UserID,
			sample.IdempotencyKey,
			sample.Count,
		)
	}

	// 使用原生 SQL 创建部分唯一索引，兼容 SQLite 与 PostgreSQL。
	// GORM AutoMigrate 不支持直接声明部分唯一索引，因此手动创建。
	err := gormdb.DB.Exec(fmt.Sprintf(
		"CREATE UNIQUE INDEX %s ON orders (user_id, idempotency_key) WHERE idempotency_key <> ''",
		orderIdempotencyUniqueIndex,
	)).Error
	if err == nil {
		return nil
	}
	// 容忍滚动部署竞态：另一实例已创建相同索引。
	if migrator.HasIndex(&orderdomain.Order{}, orderIdempotencyUniqueIndex) {
		return nil
	}
	return err
}

// 确保 gorm 引用被使用（避免未使用导入）。
var _ = gorm.ErrRecordNotFound
