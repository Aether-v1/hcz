//go:build integration
// +build integration

// application_pg_concurrency_test.go — Affiliate Application 在真实 PostgreSQL 上的并发验证。
//
// 运行（PowerShell）：
//
//	$env:TEST_POSTGRES_DSN="host=127.0.0.1 port=5432 user=postgres password=postgres dbname=hcz_test sslmode=disable TimeZone=UTC"
//	go test -tags integration -run TestPGApp -v -timeout 300s ./internal/modules/affiliate/integrationtest/
//
// 设计要点：
//   - 每个用例 newPGAppFixture 重建表并手动创建 partial unique index
//     （idx_affiliate_apps_user_pending ON user_id WHERE status='pending'），复刻生产 registry。
//   - 并发用 sync.WaitGroup + start 栅栏让 goroutine 同时开始。
//   - 行锁（SELECT ... FOR UPDATE）依赖 PG；SQLite 不支持，故本文件仅在 integration tag 下编译。
package integrationtest

import (
	"errors"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/Aether-v1/hcz/internal/constants"

	affiliateapp "github.com/Aether-v1/hcz/internal/modules/affiliate/application"
	affiliatecontract "github.com/Aether-v1/hcz/internal/modules/affiliate/contract"
	affiliatedomain "github.com/Aether-v1/hcz/internal/modules/affiliate/domain"
	affiliategormstore "github.com/Aether-v1/hcz/internal/modules/affiliate/infrastructure/gormstore"

	userdomain "github.com/Aether-v1/hcz/internal/modules/identity/user/domain"
	userstore "github.com/Aether-v1/hcz/internal/modules/identity/user/infrastructure/gormstore"
	admindomain "github.com/Aether-v1/hcz/internal/modules/identity/admin/domain"

	settingsapp "github.com/Aether-v1/hcz/internal/modules/settings/application"
	settingsintegration "github.com/Aether-v1/hcz/internal/modules/settings/schema/integration"
	"github.com/Aether-v1/hcz/internal/testkit/memorysettings"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	glogger "gorm.io/gorm/logger"
)

type pgAppFixture struct {
	db            *gorm.DB
	affiliateRepo *affiliategormstore.Store
	svc           *affiliateapp.Service
}

func newPGAppFixture(t *testing.T) *pgAppFixture {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("TEST_POSTGRES_DSN"))
	if dsn == "" {
		t.Skip("skip pg application concurrency test: TEST_POSTGRES_DSN is empty")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: glogger.Default.LogMode(glogger.Silent),
	})
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("sqlDB: %v", err)
	}
	sqlDB.SetMaxOpenConns(20)
	db.Exec("SET lock_timeout = '5s'")

	models := []interface{}{
		&admindomain.Admin{},
		&userdomain.User{},
		&affiliatedomain.Profile{},
		&affiliatedomain.Click{},
		&affiliatedomain.Commission{},
		&affiliatedomain.CommissionLedger{},
		&affiliatedomain.WithdrawRequest{},
		&affiliatedomain.Application{},
	}
	_ = db.Migrator().DropTable(models...)
	if err := db.AutoMigrate(models...); err != nil {
		t.Fatalf("migrate postgres: %v", err)
	}
	// 复刻生产 registry：partial unique index，保证同一用户至多一条 pending 申请。
	if err := db.Exec(`CREATE UNIQUE INDEX idx_aff_apps_user_pending ON affiliate_applications(user_id) WHERE status = 'pending'`).Error; err != nil {
		t.Fatalf("create partial unique index: %v", err)
	}

	settingRepo := memorysettings.New()
	settingSvc := settingsapp.NewService(settingRepo)
	if _, err := settingSvc.UpdateAffiliateSetting(settingsintegration.AffiliateSetting{
		Enabled: true, MaxLevel: 1, CommissionRate: 20,
	}); err != nil {
		t.Fatalf("init setting: %v", err)
	}

	affiliateRepo := affiliategormstore.New(db)
	svc := affiliateapp.NewService(affiliateRepo, userstore.New(db), nil, nil, settingSvc)

	// 预建 admin。
	for _, adminID := range []uint{1, 2, 3} {
		if err := db.Create(&admindomain.Admin{
			ID: adminID, Username: "pg-app-admin-" + itoa(int(adminID)), PasswordHash: "x",
		}).Error; err != nil {
			t.Fatalf("seed admin: %v", err)
		}
	}

	t.Cleanup(func() {
		_ = db.Migrator().DropTable(models...)
		_ = sqlDB.Close()
	})

	return &pgAppFixture{db: db, affiliateRepo: affiliateRepo, svc: svc}
}

// countPendingApps 统计某用户的 pending 申请数。
func (f *pgAppFixture) countPendingApps(t *testing.T, userID uint) int64 {
	t.Helper()
	var n int64
	if err := f.db.Model(&affiliatedomain.Application{}).
		Where("user_id = ? AND status = ?", userID, constants.AffiliateAppStatusPending).
		Count(&n).Error; err != nil {
		t.Fatalf("count pending: %v", err)
	}
	return n
}

func (f *pgAppFixture) countProfiles(t *testing.T, userID uint) int64 {
	t.Helper()
	var n int64
	if err := f.db.Model(&affiliatedomain.Profile{}).Where("user_id = ?", userID).Count(&n).Error; err != nil {
		t.Fatalf("count profiles: %v", err)
	}
	return n
}

func (f *pgAppFixture) appStatus(t *testing.T, appID uint) string {
	t.Helper()
	var app affiliatedomain.Application
	if err := f.db.First(&app, appID).Error; err != nil {
		t.Fatalf("load app %d: %v", appID, err)
	}
	return app.Status
}

// runConcurrently 启动 n 个 goroutine 同时执行 op，返回每个 goroutine 的 error。
func runConcurrently(n int, op func() error) []error {
	errs := make([]error, n)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			errs[i] = op()
		}(i)
	}
	close(start)
	wg.Wait()
	return errs
}

func countSuccess(errs []error) (ok int, blocked int) {
	for _, e := range errs {
		if e == nil {
			ok++
		} else {
			blocked++
		}
	}
	return
}

// 26. TestPGAppConcurrentApply_OnlyOnePending
// 两个 goroutine 同时对同一用户 apply → 最终只能有一条 pending（partial unique index 兜底）。
func TestPGAppConcurrentApply_OnlyOnePending(t *testing.T) {
	f := newPGAppFixture(t)
	u := createAffiliateTestUser(t, f.db, "pg-apply-u@hcz.test")

	errs := runConcurrently(2, func() error {
		_, err := f.svc.ApplyAffiliate(u.ID, "concurrent apply")
		return err
	})
	ok, blocked := countSuccess(errs)
	if ok != 1 {
		t.Fatalf("exactly 1 apply should succeed, got ok=%d errs=%v", ok, errs)
	}
	if blocked != 1 {
		t.Fatalf("the other apply must be blocked, got blocked=%d errs=%v", blocked, errs)
	}
	// 失败者必须是"已有 pending"类错误（而非别的 DB 错误）。
	for _, e := range errs {
		if e != nil && !errors.Is(e, affiliateapp.ErrApplicationPending) && !errors.Is(e, affiliateapp.ErrAlreadyActive) {
			t.Fatalf("blocked apply should be ErrApplicationPending, got %v", e)
		}
	}
	if n := f.countPendingApps(t, u.ID); n != 1 {
		t.Fatalf("want exactly 1 pending application, got %d", n)
	}
}

// 27. TestPGAppConcurrentApprove_OnlyOneProfile
// 两个管理员同时 Approve 同一申请 → 只有一个 profile、一个 affiliate code。
func TestPGAppConcurrentApprove_OnlyOneProfile(t *testing.T) {
	f := newPGAppFixture(t)
	u := createAffiliateTestUser(t, f.db, "pg-appr-u@hcz.test")
	app, err := f.svc.ApplyAffiliate(u.ID, "x")
	if err != nil {
		t.Fatalf("seed apply: %v", err)
	}

	errs := runConcurrently(2, func() error {
		_, e := f.svc.ApproveApplication(app.ID, 1)
		return e
	})
	ok, _ := countSuccess(errs)
	if ok != 1 {
		t.Fatalf("exactly 1 approve should succeed, got ok=%d errs=%v", ok, errs)
	}
	for _, e := range errs {
		if e != nil && !errors.Is(e, affiliateapp.ErrApplicationAlreadyReviewed) {
			t.Fatalf("blocked approve should be ErrApplicationAlreadyReviewed, got %v", e)
		}
	}
	if n := f.countProfiles(t, u.ID); n != 1 {
		t.Fatalf("want exactly 1 profile, got %d", n)
	}
	if st := f.appStatus(t, app.ID); st != constants.AffiliateAppStatusApproved {
		t.Fatalf("app status want approved, got %q", st)
	}
}

// 28. TestPGAppConcurrentApproveVsReject_OnlyOneWins
// 一个 goroutine Approve、一个 Reject 同时执行 → 只有一个状态成功；
// 禁止出现 application=rejected 但 profile=active 的撕裂。
func TestPGAppConcurrentApproveVsReject_OnlyOneWins(t *testing.T) {
	f := newPGAppFixture(t)
	u := createAffiliateTestUser(t, f.db, "pg-race-u@hcz.test")
	app, err := f.svc.ApplyAffiliate(u.ID, "x")
	if err != nil {
		t.Fatalf("seed apply: %v", err)
	}

	var approveErr, rejectErr error
	var wg sync.WaitGroup
	start := make(chan struct{})
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		_, approveErr = f.svc.ApproveApplication(app.ID, 1)
	}()
	go func() {
		defer wg.Done()
		<-start
		rejectErr = f.svc.RejectApplication(app.ID, 2, "race reject")
	}()
	close(start)
	wg.Wait()

	// 恰好一个成功、另一个被"已审核"拦截。
	approveOK := approveErr == nil
	rejectOK := rejectErr == nil
	if approveOK && rejectOK {
		t.Fatalf("both approve and reject succeeded (impossible): approveErr=%v rejectErr=%v", approveErr, rejectErr)
	}
	if !approveOK && !rejectOK {
		// 两者都失败时，失败者应为 ErrApplicationAlreadyReviewed。
		if !errors.Is(approveErr, affiliateapp.ErrApplicationAlreadyReviewed) && !errors.Is(rejectErr, affiliateapp.ErrApplicationAlreadyReviewed) {
			t.Fatalf("unexpected errors: approve=%v reject=%v", approveErr, rejectErr)
		}
	}

	st := f.appStatus(t, app.ID)
	profiles := f.countProfiles(t, u.ID)
	// 核心不变量：若申请最终为 rejected，则绝不能出现 active profile。
	if st == constants.AffiliateAppStatusRejected && profiles != 0 {
		t.Fatalf("INTEGRITY VIOLATION: application rejected but %d profile(s) exist", profiles)
	}
	if st == constants.AffiliateAppStatusApproved && profiles != 1 {
		t.Fatalf("approved but profile count=%d (want 1)", profiles)
	}
	if st != constants.AffiliateAppStatusApproved && st != constants.AffiliateAppStatusRejected {
		t.Fatalf("final app status want approved|rejected, got %q", st)
	}
}

// 29. TestPGAppConcurrentApprove_DuplicateClick
// 同一管理员快速两次 Approve（重复点击）→ 不产生重复 profile。
func TestPGAppConcurrentApprove_DuplicateClick(t *testing.T) {
	f := newPGAppFixture(t)
	u := createAffiliateTestUser(t, f.db, "pg-dup-u@hcz.test")
	app, err := f.svc.ApplyAffiliate(u.ID, "x")
	if err != nil {
		t.Fatalf("seed apply: %v", err)
	}

	errs := runConcurrently(5, func() error {
		_, e := f.svc.ApproveApplication(app.ID, 1) // 同一 admin_id 重复点击
		return e
	})
	ok, _ := countSuccess(errs)
	if ok != 1 {
		t.Fatalf("duplicate-click approve: exactly 1 should win, got ok=%d errs=%v", ok, errs)
	}
	if n := f.countProfiles(t, u.ID); n != 1 {
		t.Fatalf("duplicate-click must not create duplicate profile, got %d", n)
	}
	// 确保 affiliate code 全局唯一：该用户只有一个 profile。
	var codes []string
	f.db.Model(&affiliatedomain.Profile{}).Where("user_id = ?", u.ID).Pluck("affiliate_code", &codes)
	if len(codes) != 1 {
		t.Fatalf("want exactly 1 affiliate code, got %v", codes)
	}
}

// 保证编译期引用 contract 接口（避免未使用 import）。
var _ affiliatecontract.Store
