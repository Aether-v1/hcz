// Package integrationtest 提供工单系统的集成测试：SQLite 内存库 + gormstore + service。
package integrationtest

import (
	"context"
	"fmt"
	"testing"

	supportapp "github.com/Aether-v1/hcz/internal/modules/supportticket/application"
	supportcontract "github.com/Aether-v1/hcz/internal/modules/supportticket/contract"
	supportdomain "github.com/Aether-v1/hcz/internal/modules/supportticket/domain"
	supportgormstore "github.com/Aether-v1/hcz/internal/modules/supportticket/infrastructure/gormstore"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	glogger "gorm.io/gorm/logger"
)

var silentLogger = glogger.Default.LogMode(glogger.Silent)

// ---- mocks ----

type mockUserReader struct {
	emails map[uint]string
	names  map[uint]string
}

func (m *mockUserReader) GetByID(userID uint) (uint, string, string, error) {
	return userID, m.emails[userID], m.names[userID], nil
}

type mockAdminReader struct {
	usernames map[uint]string
}

func (m *mockAdminReader) GetAdminByID(adminID uint) (uint, string, error) {
	name, ok := m.usernames[adminID]
	if !ok {
		return adminID, "", fmt.Errorf("admin %d not found", adminID)
	}
	return adminID, name, nil
}

type mockNotifier struct {
	inputs []supportcontract.NotificationInput
}

func (m *mockNotifier) CreateNotification(_ context.Context, in supportcontract.NotificationInput) error {
	m.inputs = append(m.inputs, in)
	return nil
}

// ---- fixture ----

type fixture struct {
	db     *gorm.DB
	repo   *supportgormstore.Store
	svc    *supportapp.Service
	users  *mockUserReader
	admins *mockAdminReader
	notifier *mockNotifier
}

var dbCounter int

func newFixture(t *testing.T) *fixture {
	t.Helper()
	dbCounter++
	dsn := fmt.Sprintf("file:support%d?mode=memory&cache=shared", dbCounter)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: silentLogger})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&supportdomain.Category{},
		&supportdomain.Ticket{},
		&supportdomain.Message{},
		&supportdomain.Attachment{},
		&supportdomain.Audit{},
	); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	// biz 归属校验用到的业务表（只读原始 SQL）：建最小表结构。
	db.Exec("CREATE TABLE orders (id INTEGER PRIMARY KEY, user_id INTEGER, deleted_at DATETIME)")
	db.Exec("CREATE TABLE wallet_withdrawals (id INTEGER PRIMARY KEY, user_id INTEGER)")
	db.Exec("CREATE TABLE c2c_trades (id INTEGER PRIMARY KEY, buyer_user_id INTEGER, seller_user_id INTEGER)")
	db.Exec("CREATE TABLE wallet_recharge_orders (id INTEGER PRIMARY KEY, user_id INTEGER)")

	repo := supportgormstore.New(db)
	users := &mockUserReader{emails: map[uint]string{}, names: map[uint]string{}}
	admins := &mockAdminReader{usernames: map[uint]string{1: "admin1", 2: "admin2"}}
	notifier := &mockNotifier{}

	svc := supportapp.NewService(supportapp.Options{
		Repository: repo,
		UnitOfWork: repo,
		Users:      users,
		Admins:     admins,
		Notifier:   notifier,
	})
	return &fixture{db: db, repo: repo, svc: svc, users: users, admins: admins, notifier: notifier}
}

func (f *fixture) seedCategory(code, name, priority string, enabled bool) *supportdomain.Category {
	c := &supportdomain.Category{Code: code, Name: name, DefaultPriority: priority, Enabled: enabled}
	if err := f.repo.CreateCategory(c); err != nil {
		panic(err)
	}
	// GORM 在 Create 时会省略 false 零值（走 DB 默认 true），这里显式落库。
	if !enabled {
		f.db.Model(c).UpdateColumn("enabled", false)
		c.Enabled = false
	}
	return c
}
