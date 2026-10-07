// points_test.go — Points Core 账本集成测试（SQLite 内存库，无 build tag，短模式下跳过）。
//
// 覆盖：
//   - 无积分账户查询返回零值（不 404、不创建账户）
//   - 首次 credit 创建账户（唯一 user_id）
//   - AdminAdjust add / subtract、负余额语义
//   - Idempotency（同键重复执行只入账一次；同键不同参数 409）
//   - 首次创建并发（不产生重复账户）
//   - 并发 credit / concurrent debit（SQLite 串行化写，低并发可复现）
//   - 数据库不变量：points_accounts.balance == SUM(points_ledger.amount)
package integrationtest

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	pointsapp "github.com/Aether-v1/hcz/internal/modules/points/application"
	pointscontract "github.com/Aether-v1/hcz/internal/modules/points/contract"
	pointsdomain "github.com/Aether-v1/hcz/internal/modules/points/domain"
	pointsgormstore "github.com/Aether-v1/hcz/internal/modules/points/infrastructure/gormstore"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newPointsTestService(t *testing.T) (*pointsapp.Service, *gorm.DB) {
	t.Helper()
	// 文件型 SQLite + WAL + busy_timeout：并发写由 SQLite 写者串行化（busy 等待），
	// 可复现并发 credit/debit/首次创建语义；禁止 cache=shared（会触发 SQLITE_LOCKED deadlock）。
	dir := t.TempDir()
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)",
		filepath.Join(dir, "points.db"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&pointsdomain.Account{}, &pointsdomain.LedgerEntry{}); err != nil {
		t.Fatalf("migrate points schema: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("raw db: %v", err)
	}
	// SQLite 是单写者数据库：并发写必然 BUSY/LOCKED（与业务代码无关）。
	// 与 wallet/concurrency_test.go 先例一致，单连接串行化 DB 访问，
	// 验证应用层事务正确性（账户+流水同事务、首建唯一、幂等、余额精确）；
	// 真实并发行锁语义由 PG 集成测试（pg_concurrency_test.go）验证。
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() {
		// Windows 下文件型 SQLite 需显式关闭，否则 TempDir 清理会失败。
		_ = sqlDB.Close()
	})
	store := pointsgormstore.New(db)
	svc := pointsapp.NewService(pointsapp.Options{Repository: store, Transactions: store})
	return svc, db
}

// assertInvariant 校验数据库不变量：account.balance == SUM(ledger.amount)。
func assertInvariant(t *testing.T, db *gorm.DB, userID uint) {
	t.Helper()
	var account pointsdomain.Account
	if err := db.Where("user_id = ?", userID).First(&account).Error; err != nil {
		t.Fatalf("load account: %v", err)
	}
	var sum int64
	if err := db.Model(&pointsdomain.LedgerEntry{}).Where("user_id = ?", userID).Select("COALESCE(SUM(amount),0)").Scan(&sum).Error; err != nil {
		t.Fatalf("sum ledger: %v", err)
	}
	if account.Balance != sum {
		t.Fatalf("invariant violated: account.balance=%d SUM(ledger)=%d", account.Balance, sum)
	}
}

func TestGetAccountReturnsZeroValueWhenNeverCreated(t *testing.T) {
	if testing.Short() {
		t.Skip("skip in short mode")
	}
	svc, _ := newPointsTestService(t)
	account, err := svc.GetAccount(42)
	if err != nil {
		t.Fatalf("GetAccount: %v", err)
	}
	if account != nil {
		t.Fatalf("expected nil account for never-created user, got %+v", account)
	}
}

func TestAdminAdjustCreditCreatesAccountAndLedger(t *testing.T) {
	if testing.Short() {
		t.Skip("skip in short mode")
	}
	svc, db := newPointsTestService(t)
	account, entry, err := svc.AdminAdjust(pointscontract.AdjustInput{
		UserID: 1, OperatorAdminID: 9, Operation: "add", Amount: 100,
		Reason: "test credit", Reference: pointscontract.AdminAdjustReference("key-1"),
	})
	if err != nil {
		t.Fatalf("admin adjust: %v", err)
	}
	if account.Balance != 100 || account.TotalEarned != 100 || account.TotalSpent != 0 {
		t.Fatalf("unexpected account: %+v", account)
	}
	if entry.BalanceBefore != 0 || entry.BalanceAfter != 100 || entry.Amount != 100 {
		t.Fatalf("unexpected ledger: %+v", entry)
	}
	if entry.ActionType != pointscontract.ActionAdminAdd || entry.SourceType != pointscontract.SourceAdminAdjust {
		t.Fatalf("unexpected action/source: %s/%s", entry.ActionType, entry.SourceType)
	}
	if entry.OperatorType != pointscontract.OperatorAdmin || entry.OperatorID != 9 {
		t.Fatalf("unexpected operator: %s/%d", entry.OperatorType, entry.OperatorID)
	}
	assertInvariant(t, db, 1)
}

func TestAdminAdjustDebitCanProduceNegativeBalance(t *testing.T) {
	if testing.Short() {
		t.Skip("skip in short mode")
	}
	svc, db := newPointsTestService(t)
	if _, _, err := svc.AdminAdjust(pointscontract.AdjustInput{
		UserID: 2, OperatorAdminID: 9, Operation: "add", Amount: 20,
		Reason: "grant", Reference: pointscontract.AdminAdjustReference("key-a"),
	}); err != nil {
		t.Fatalf("grant: %v", err)
	}
	account, entry, err := svc.AdminAdjust(pointscontract.AdjustInput{
		UserID: 2, OperatorAdminID: 9, Operation: "subtract", Amount: 100,
		Reason: "penalty", Reference: pointscontract.AdminAdjustReference("key-b"),
	})
	if err != nil {
		t.Fatalf("debit: %v", err)
	}
	if account.Balance != -80 {
		t.Fatalf("expected balance -80 (admin may go negative), got %d", account.Balance)
	}
	if account.TotalEarned != 20 || account.TotalSpent != 0 {
		t.Fatalf("admin deduct must not pollute earned/spent: %+v", account)
	}
	if entry.Amount != -100 || entry.BalanceBefore != 20 || entry.BalanceAfter != -80 {
		t.Fatalf("unexpected ledger: %+v", entry)
	}
	assertInvariant(t, db, 2)
}

func TestAdminAdjustRejectsInvalidInputs(t *testing.T) {
	if testing.Short() {
		t.Skip("skip in short mode")
	}
	svc, _ := newPointsTestService(t)
	cases := []struct {
		name  string
		input pointscontract.AdjustInput
		want  error
	}{
		{"zero amount", pointscontract.AdjustInput{UserID: 1, OperatorAdminID: 9, Operation: "add", Amount: 0, Reason: "r", Reference: "ref:0"}, pointscontract.ErrInvalidAmount},
		{"negative amount", pointscontract.AdjustInput{UserID: 1, OperatorAdminID: 9, Operation: "add", Amount: -100, Reason: "r", Reference: "ref:-1"}, pointscontract.ErrInvalidAmount},
		{"bad operation", pointscontract.AdjustInput{UserID: 1, OperatorAdminID: 9, Operation: "double", Amount: 10, Reason: "r", Reference: "ref:badop"}, pointscontract.ErrInvalidOperation},
		{"missing reason", pointscontract.AdjustInput{UserID: 1, OperatorAdminID: 9, Operation: "add", Amount: 10, Reason: "  ", Reference: "ref:nr"}, pointscontract.ErrReasonRequired},
		{"missing reference", pointscontract.AdjustInput{UserID: 1, OperatorAdminID: 9, Operation: "add", Amount: 10, Reason: "r", Reference: " "}, pointscontract.ErrReferenceRequired},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := svc.AdminAdjust(tc.input); !errors.Is(err, tc.want) {
				t.Fatalf("want %v, got %v", tc.want, err)
			}
		})
	}
}

func TestAdminAdjustIdempotencySameKeyReplays(t *testing.T) {
	if testing.Short() {
		t.Skip("skip in short mode")
	}
	svc, db := newPointsTestService(t)
	input := pointscontract.AdjustInput{
		UserID: 3, OperatorAdminID: 9, Operation: "add", Amount: 50,
		Reason: "bonus", Reference: pointscontract.AdminAdjustReference("idem-1"),
	}
	first, _, err := svc.AdminAdjust(input)
	if err != nil {
		t.Fatalf("first adjust: %v", err)
	}
	second, entry, err := svc.AdminAdjust(input)
	if err != nil {
		t.Fatalf("second adjust: %v", err)
	}
	if first.Balance != 50 || second.Balance != 50 {
		t.Fatalf("balance must change only once: first=%d second=%d", first.Balance, second.Balance)
	}
	if entry == nil || entry.ID == 0 {
		t.Fatalf("idempotent replay must return original ledger")
	}
	assertInvariant(t, db, 3)
	var count int64
	if err := db.Model(&pointsdomain.LedgerEntry{}).Where("user_id = ?", 3).Count(&count).Error; err != nil {
		t.Fatalf("count ledger: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 ledger entry, got %d", count)
	}
}

func TestAdminAdjustIdempotencyConflictOnDifferentPayload(t *testing.T) {
	if testing.Short() {
		t.Skip("skip in short mode")
	}
	svc, _ := newPointsTestService(t)
	if _, _, err := svc.AdminAdjust(pointscontract.AdjustInput{
		UserID: 4, OperatorAdminID: 9, Operation: "add", Amount: 10,
		Reason: "a", Reference: pointscontract.AdminAdjustReference("idem-x"),
	}); err != nil {
		t.Fatalf("first: %v", err)
	}
	_, _, err := svc.AdminAdjust(pointscontract.AdjustInput{
		UserID: 4, OperatorAdminID: 9, Operation: "add", Amount: 999,
		Reason: "different", Reference: pointscontract.AdminAdjustReference("idem-x"),
	})
	if !errors.Is(err, pointscontract.ErrIdempotencyConflict) {
		t.Fatalf("want ErrIdempotencyConflict, got %v", err)
	}
}

func TestConcurrentFirstCreateCreatesSingleAccount(t *testing.T) {
	if testing.Short() {
		t.Skip("skip in short mode")
	}
	svc, db := newPointsTestService(t)
	const workers = 8
	var wg sync.WaitGroup
	errCh := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, _, err := svc.AdminAdjust(pointscontract.AdjustInput{
				UserID: 100, OperatorAdminID: 9, Operation: "add", Amount: 10,
				Reason:    fmt.Sprintf("first-create-%d", i),
				Reference: pointscontract.AdminAdjustReference(fmt.Sprintf("fc-%d", i)),
			})
			if err != nil {
				errCh <- err
			}
		}(i)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Errorf("concurrent first create: %v", err)
	}
	var accounts int64
	if err := db.Model(&pointsdomain.Account{}).Where("user_id = ?", 100).Count(&accounts).Error; err != nil {
		t.Fatalf("count accounts: %v", err)
	}
	if accounts != 1 {
		t.Fatalf("expected exactly 1 account, got %d", accounts)
	}
	assertInvariant(t, db, 100)
	var ledger int64
	if err := db.Model(&pointsdomain.LedgerEntry{}).Where("user_id = ?", 100).Count(&ledger).Error; err != nil {
		t.Fatalf("count ledger: %v", err)
	}
	if ledger != workers {
		t.Fatalf("expected %d ledger entries, got %d", workers, ledger)
	}
	account, err := svc.GetAccount(100)
	if err != nil || account == nil || account.Balance != int64(workers*10) {
		t.Fatalf("unexpected final balance: account=%+v err=%v", account, err)
	}
}

func TestConcurrentCreditNoLostUpdate(t *testing.T) {
	if testing.Short() {
		t.Skip("skip in short mode")
	}
	svc, db := newPointsTestService(t)
	const workers = 12
	var wg sync.WaitGroup
	errCh := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, _, err := svc.AdminAdjust(pointscontract.AdjustInput{
				UserID: 200, OperatorAdminID: 9, Operation: "add", Amount: 10,
				Reason:    fmt.Sprintf("credit-%d", i),
				Reference: pointscontract.AdminAdjustReference(fmt.Sprintf("cc-%d", i)),
			})
			if err != nil {
				errCh <- err
			}
		}(i)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Errorf("concurrent credit: %v", err)
	}
	account, err := svc.GetAccount(200)
	if err != nil || account == nil {
		t.Fatalf("get account: %+v err=%v", account, err)
	}
	if account.Balance != int64(workers*10) {
		t.Fatalf("lost update: want %d, got %d", workers*10, account.Balance)
	}
	assertInvariant(t, db, 200)
}

func TestConcurrentDebitAllowsNegative(t *testing.T) {
	if testing.Short() {
		t.Skip("skip in short mode")
	}
	svc, db := newPointsTestService(t)
	// 先给 1000 基数，再并发扣减 12×100 → -200（允许负）。
	if _, _, err := svc.AdminAdjust(pointscontract.AdjustInput{
		UserID: 300, OperatorAdminID: 9, Operation: "add", Amount: 1000,
		Reason: "base", Reference: pointscontract.AdminAdjustReference("base-300"),
	}); err != nil {
		t.Fatalf("base: %v", err)
	}
	const workers = 12
	var wg sync.WaitGroup
	errCh := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, _, err := svc.AdminAdjust(pointscontract.AdjustInput{
				UserID: 300, OperatorAdminID: 9, Operation: "subtract", Amount: 100,
				Reason:    fmt.Sprintf("debit-%d", i),
				Reference: pointscontract.AdminAdjustReference(fmt.Sprintf("cd-%d", i)),
			})
			if err != nil {
				errCh <- err
			}
		}(i)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Errorf("concurrent debit: %v", err)
	}
	account, err := svc.GetAccount(300)
	if err != nil || account == nil {
		t.Fatalf("get account: %+v err=%v", account, err)
	}
	if account.Balance != -200 {
		t.Fatalf("want -200 (1000 - 12*100), got %d", account.Balance)
	}
	assertInvariant(t, db, 300)
}

func TestListLedgerEntriesScopedByUser(t *testing.T) {
	if testing.Short() {
		t.Skip("skip in short mode")
	}
	svc, _ := newPointsTestService(t)
	for _, uid := range []uint{11, 12} {
		if _, _, err := svc.AdminAdjust(pointscontract.AdjustInput{
			UserID: uid, OperatorAdminID: 9, Operation: "add", Amount: 30,
			Reason: "grant", Reference: pointscontract.AdminAdjustReference(fmt.Sprintf("list-%d", uid)),
		}); err != nil {
			t.Fatalf("adjust user %d: %v", uid, err)
		}
	}
	entries, total, err := svc.ListLedgerEntries(pointscontract.LedgerListFilter{UserID: 11, Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 1 || len(entries) != 1 || entries[0].UserID != 11 {
		t.Fatalf("expected 1 entry for user 11 only, got total=%d len=%d entries=%+v", total, len(entries), entries)
	}
	if !strings.HasPrefix(entries[0].Reference, "admin_adjust:list-") {
		t.Fatalf("unexpected reference %q", entries[0].Reference)
	}
}
