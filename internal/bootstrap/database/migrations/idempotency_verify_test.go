package migrations

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/Aether-v1/hcz/internal/platform/database/gormdb"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// TestIdempotencyVerifyOnTempFile 验证 AutoMigrate 在临时文件 SQLite 上可重复执行且幂等。
// 步骤：
//  1. 创建临时 SQLite 文件
//  2. 第一次执行完整 AutoMigrate
//  3. 记录 schema（表名列表 + 每表列数）
//  4. 第二次执行相同 AutoMigrate
//  5. 确认无错误、表结构不变
func TestIdempotencyVerifyOnTempFile(t *testing.T) {
	// 1. 创建临时 SQLite 文件
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "hcz_idempotency_verify.db")

	dsn := fmt.Sprintf("%s?_pragma=foreign_keys(1)", dbPath)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open temp sqlite failed: %v", err)
	}

	// 保存并替换全局 gormdb.DB（AutoMigrate() 内部使用 gormdb.DB）
	prevDB := gormdb.DB
	t.Cleanup(func() {
		gormdb.DB = prevDB
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	gormdb.DB = db

	// 2. 第一次执行完整 AutoMigrate
	if err := AutoMigrate(); err != nil {
		t.Fatalf("first AutoMigrate failed: %v", err)
	}
	t.Log("first AutoMigrate: OK")

	// 3. 记录 schema（表名列表 + 每表列数）
	schemaBefore, err := dumpSchema(db)
	if err != nil {
		t.Fatalf("dump schema after first migrate failed: %v", err)
	}
	t.Logf("schema after first migrate: %d tables", len(schemaBefore))
	for table, cols := range schemaBefore {
		t.Logf("  - %s (%d columns)", table, cols)
	}

	// 4. 第二次执行相同 AutoMigrate
	if err := AutoMigrate(); err != nil {
		t.Fatalf("second AutoMigrate (idempotency) failed: %v", err)
	}
	t.Log("second AutoMigrate: OK")

	// 5. 确认表结构不变
	schemaAfter, err := dumpSchema(db)
	if err != nil {
		t.Fatalf("dump schema after second migrate failed: %v", err)
	}

	if len(schemaBefore) != len(schemaAfter) {
		t.Fatalf("table count changed: before=%d after=%d", len(schemaBefore), len(schemaAfter))
	}
	for table, colsBefore := range schemaBefore {
		colsAfter, ok := schemaAfter[table]
		if !ok {
			t.Errorf("table %s missing after second migrate", table)
			continue
		}
		if colsBefore != colsAfter {
			t.Errorf("table %s column count changed: before=%d after=%d", table, colsBefore, colsAfter)
		}
	}

	// 确认临时文件确实存在
	if _, err := os.Stat(dbPath); err != nil {
		t.Errorf("temp sqlite file not found: %v", err)
	}

	t.Logf("IDEMPOTENCY VERIFICATION PASSED: %d tables, second run produced no schema change", len(schemaBefore))
}

// dumpSchema 返回表名 -> 列数 的映射（仅用户表，排除 sqlite 内部表）。
func dumpSchema(db *gorm.DB) (map[string]int, error) {
	var tables []string
	if err := db.Raw(
		"SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name",
	).Scan(&tables).Error; err != nil {
		return nil, err
	}
	result := make(map[string]int, len(tables))
	for _, table := range tables {
		var cols []struct {
			Name string `gorm:"column:name"`
		}
		if err := db.Raw(fmt.Sprintf("PRAGMA table_info(%s)", table)).Scan(&cols).Error; err != nil {
			return nil, fmt.Errorf("pragma table_info %s: %w", table, err)
		}
		result[table] = len(cols)
	}
	return result, nil
}
