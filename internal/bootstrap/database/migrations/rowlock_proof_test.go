//go:build integration

// rowlock_proof_test.go —— 在真实 PostgreSQL 上证明 SELECT ... FOR UPDATE 行锁真正生效。
//
// 运行方式（PowerShell）：
//
//	$env:TEST_POSTGRES_DSN="host=127.0.0.1 port=5432 user=hcz_app password=*** dbname=hcz_staging sslmode=disable TimeZone=UTC"
//	go test -tags integration -run TestRowLock -v -timeout 120s ./internal/bootstrap/database/migrations/
//
// 若 TEST_POSTGRES_DSN 为空则 t.Skip。所有测试在 hcz_staging 内创建独立的 rlproof_ 前缀表，
// 测试结束自动 DROP，不影响业务表。每个测试用 gorm clause.Locking{Strength:"UPDATE"}
// （即 SELECT ... FOR UPDATE），与生产 wallet/c2c/order 代码一致。
package migrations

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func openRowLockDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN 未设置，跳过高并发行锁真实验证（仅在真实 PG 上有意义）")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("连接测试 PG 失败: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("获取底层 sql.DB 失败: %v", err)
	}
	sqlDB.SetMaxOpenConns(20)
	// 锁等待最多 5s，避免死锁时测试无限挂起（正常应秒级完成）
	db.Exec("SET lock_timeout = '5s'")
	db.Exec("SET idle_in_transaction_session_timeout = '15s'")
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func dropTables(t *testing.T, db *gorm.DB, names ...string) {
	t.Helper()
	for _, n := range names {
		if err := db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE", n)).Error; err != nil {
			t.Logf("清理表 %s 警告: %v", n, err)
		}
	}
}

// 场景 1：钱包并发扣款。10 个 goroutine 各扣 100，初始 1000，用 SELECT FOR UPDATE 串行化。
// 对照行（无 FOR UPDATE，读-算-写）会因丢失更新导致最终余额不正确——以此证明是行锁救了结果。
func TestRowLockWalletConcurrentDebit(t *testing.T) {
	db := openRowLockDB(t)
	const tbl = "rlproof_wallet_debit"
	dropTables(t, db, tbl)
	if err := db.Exec(fmt.Sprintf(`CREATE TABLE %s (
		id int PRIMARY KEY,
		available numeric(20,2) NOT NULL DEFAULT 0
	)`, tbl)).Error; err != nil {
		t.Fatalf("建表失败: %v", err)
	}
	t.Cleanup(func() { dropTables(t, db, tbl) })

	// id=1: 受锁保护；id=2: 无锁对照（丢失更新）
	if err := db.Exec(fmt.Sprintf("INSERT INTO %s(id,available) VALUES (1,1000),(2,1000)", tbl)).Error; err != nil {
		t.Fatalf("插入初始余额失败: %v", err)
	}

	const goroutines = 10
	const debit = 100

	// 受锁行：SELECT ... FOR UPDATE
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := db.Transaction(func(tx *gorm.DB) error {
				var avail float64
				if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
					Table(tbl).Select("available").Where("id = 1").Row().Scan(&avail); err != nil {
					return err
				}
				if avail < debit { // 余额不足则放弃
					return nil
				}
				return tx.Exec(fmt.Sprintf("UPDATE %s SET available = available - ? WHERE id = 1", tbl), debit).Error
			})
			if err != nil {
				t.Errorf("受锁扣款事务失败: %v", err)
			}
		}()
	}
	wg.Wait()

	// 对照行：无 FOR UPDATE，读-算-写（暴露丢失更新）
	var wg2 sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg2.Add(1)
		go func() {
			defer wg2.Done()
			err := db.Transaction(func(tx *gorm.DB) error {
				var avail float64
				if err := tx.Table(tbl).Select("available").Where("id = 2").Row().Scan(&avail); err != nil {
					return err
				}
				newVal := avail - debit // 读-算-写，不加锁
				return tx.Exec(fmt.Sprintf("UPDATE %s SET available = ? WHERE id = 2", tbl), newVal).Error
			})
			if err != nil {
				t.Errorf("无锁对照事务失败: %v", err)
			}
		}()
	}
	wg2.Wait()

	var lockedAvail, unlockedAvail float64
	db.Table(tbl).Select("available").Where("id = 1").Row().Scan(&lockedAvail)
	db.Table(tbl).Select("available").Where("id = 2").Row().Scan(&unlockedAvail)

	t.Logf("[wallet_debit] 并发数=%d, 每笔=%d, 初始=1000", goroutines, debit)
	t.Logf("[wallet_debit] 受锁行(FOR UPDATE) 最终余额=%.2f（期望 0.00，无超扣/无负数）", lockedAvail)
	t.Logf("[wallet_debit] 无锁对照行(丢失更新) 最终余额=%.2f（若≠0 即证明不加锁会错账）", unlockedAvail)

	if lockedAvail != 0 {
		t.Fatalf("FAIL: 受锁扣款后余额应为 0，实际 %.2f —— 出现超扣或丢失更新", lockedAvail)
	}
	if lockedAvail < 0 {
		t.Fatalf("FAIL: 出现负余额 %.2f", lockedAvail)
	}
	if unlockedAvail == 0 {
		t.Logf("[wallet_debit] 注: 本次无锁对照恰好串行化为 0（时序运气），不影响受锁行结论")
	}
	t.Log("PASS: 10 笔并发扣款被 SELECT FOR UPDATE 正确串行化，余额精确归零、无超扣")
}

// 场景 2：同一钱包，提现冻结 与 订单扣款 并发。
// 最终 available=200, frozen=300（订单扣走 500 出钱包，提现冻结 300），无 race。
func TestRowLockWithdrawalVsOrder(t *testing.T) {
	db := openRowLockDB(t)
	const tbl = "rlproof_wallet_freeze"
	dropTables(t, db, tbl)
	if err := db.Exec(fmt.Sprintf(`CREATE TABLE %s (
		id int PRIMARY KEY,
		available numeric(20,2) NOT NULL DEFAULT 0,
		frozen numeric(20,2) NOT NULL DEFAULT 0
	)`, tbl)).Error; err != nil {
		t.Fatalf("建表失败: %v", err)
	}
	t.Cleanup(func() { dropTables(t, db, tbl) })
	if err := db.Exec(fmt.Sprintf("INSERT INTO %s(id,available,frozen) VALUES (1,1000,0)", tbl)).Error; err != nil {
		t.Fatalf("插入失败: %v", err)
	}

	var wg sync.WaitGroup
	// W: 提现冻结 300
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := db.Transaction(func(tx *gorm.DB) error {
			var avail, frozen float64
			row := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Table(tbl).
				Select("available", "frozen").Where("id=1").Row()
			if err := row.Scan(&avail, &frozen); err != nil {
				return err
			}
			if avail < 300 {
				return fmt.Errorf("余额不足冻结")
			}
			return tx.Exec(fmt.Sprintf("UPDATE %s SET available=available-300, frozen=frozen+300 WHERE id=1", tbl)).Error
		})
		if err != nil {
			t.Errorf("提现冻结事务失败: %v", err)
		}
	}()
	// O: 订单扣款 500（直接出钱包）
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := db.Transaction(func(tx *gorm.DB) error {
			var avail float64
			row := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Table(tbl).
				Select("available").Where("id=1").Row()
			if err := row.Scan(&avail); err != nil {
				return err
			}
			if avail < 500 {
				return fmt.Errorf("余额不足下单")
			}
			return tx.Exec(fmt.Sprintf("UPDATE %s SET available=available-500 WHERE id=1", tbl)).Error
		})
		if err != nil {
			t.Errorf("订单扣款事务失败: %v", err)
		}
	}()
	wg.Wait()

	var avail, frozen float64
	db.Table(tbl).Select("available", "frozen").Where("id=1").Row().Scan(&avail, &frozen)
	t.Logf("[withdrawal_vs_order] 最终 available=%.2f frozen=%.2f（期望 avail=200 frozen=300）", avail, frozen)
	if avail != 200 || frozen != 300 {
		t.Fatalf("FAIL: 并发冻结+扣款结果不一致 avail=%.2f frozen=%.2f", avail, frozen)
	}
	if avail < 0 || frozen < 0 {
		t.Fatalf("FAIL: 出现负余额")
	}
	t.Log("PASS: 提现冻结与订单扣款被行锁串行化，终态一致、无超扣")
}

// 场景 3：C2C 同一挂单库存=1，两个买家同时 freeze。只能一个成交，不超卖。
func TestRowLockC2CDoubleFreeze(t *testing.T) {
	db := openRowLockDB(t)
	const tbl = "rlproof_c2c_listing"
	dropTables(t, db, tbl)
	if err := db.Exec(fmt.Sprintf(`CREATE TABLE %s (
		id int PRIMARY KEY,
		available_usdt numeric(20,2) NOT NULL DEFAULT 0,
		status varchar(16) NOT NULL DEFAULT 'active'
	)`, tbl)).Error; err != nil {
		t.Fatalf("建表失败: %v", err)
	}
	t.Cleanup(func() { dropTables(t, db, tbl) })
	if err := db.Exec(fmt.Sprintf("INSERT INTO %s(id,available_usdt,status) VALUES (1,1,'active')", tbl)).Error; err != nil {
		t.Fatalf("插入失败: %v", err)
	}

	var wg sync.WaitGroup
	var successMu sync.Mutex
	successCnt, failCnt := 0, 0
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			won := false
			err := db.Transaction(func(tx *gorm.DB) error {
				var avail float64
				var status string
				row := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Table(tbl).
					Select("available_usdt", "status").Where("id=1").Row()
				if err := row.Scan(&avail, &status); err != nil {
					return err
				}
				if avail >= 1 && status == "active" {
					if err := tx.Exec(fmt.Sprintf("UPDATE %s SET available_usdt = available_usdt - 1 WHERE id=1", tbl)).Error; err != nil {
						return err
					}
					won = true
				}
				return nil
			})
			if err != nil {
				t.Errorf("买家事务失败: %v", err)
				return
			}
			successMu.Lock()
			if won {
				successCnt++
			} else {
				failCnt++
			}
			successMu.Unlock()
		}()
	}
	wg.Wait()

	var avail float64
	db.Table(tbl).Select("available_usdt").Where("id=1").Row().Scan(&avail)
	t.Logf("[c2c_double_freeze] 库存=1, 并发买家=2, 成功成交=%d, 失败=%d, 最终库存=%.2f", successCnt, failCnt, avail)
	if successCnt != 1 || failCnt != 1 {
		t.Fatalf("FAIL: 库存=1 却有 %d 个买家成交——超卖", successCnt)
	}
	if avail != 0 {
		t.Fatalf("FAIL: 最终库存应为 0，实际 %.2f——超卖", avail)
	}
	t.Log("PASS: 两个买家抢同一库存=1，仅一个成交，未超卖")
}

// 场景 4：同一 trade，cancel(解冻) 与 settle(扣冻结+加对手方) 并发。只能一个成功，状态一致。
func TestRowLockC2CCancelVsSettle(t *testing.T) {
	db := openRowLockDB(t)
	const tbl = "rlproof_c2c_trade"
	dropTables(t, db, tbl)
	if err := db.Exec(fmt.Sprintf(`CREATE TABLE %s (
		id int PRIMARY KEY,
		status varchar(16) NOT NULL,
		seller_frozen numeric(20,2) NOT NULL DEFAULT 0,
		buyer_received numeric(20,2) NOT NULL DEFAULT 0
	)`, tbl)).Error; err != nil {
		t.Fatalf("建表失败: %v", err)
	}
	t.Cleanup(func() { dropTables(t, db, tbl) })
	if err := db.Exec(fmt.Sprintf("INSERT INTO %s(id,status,seller_frozen,buyer_received) VALUES (1,'frozen',10,0)", tbl)).Error; err != nil {
		t.Fatalf("插入失败: %v", err)
	}

	var wg sync.WaitGroup
	cancelWon, settleWon := false, false
	// cancel: 解冻，状态->cancelled
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := db.Transaction(func(tx *gorm.DB) error {
			var status string
			var sf float64
			row := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Table(tbl).
				Select("status", "seller_frozen").Where("id=1").Row()
			if err := row.Scan(&status, &sf); err != nil {
				return err
			}
			if status == "frozen" {
				if err := tx.Exec(fmt.Sprintf("UPDATE %s SET status='cancelled', seller_frozen=0 WHERE id=1", tbl)).Error; err != nil {
					return err
				}
				cancelWon = true
			}
			return nil
		})
		if err != nil {
			t.Errorf("cancel 事务失败: %v", err)
		}
	}()
	// settle: 成交，状态->settled，买家收到 10
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := db.Transaction(func(tx *gorm.DB) error {
			var status string
			var sf float64
			row := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Table(tbl).
				Select("status", "seller_frozen").Where("id=1").Row()
			if err := row.Scan(&status, &sf); err != nil {
				return err
			}
			if status == "frozen" {
				if err := tx.Exec(fmt.Sprintf("UPDATE %s SET status='settled', seller_frozen=0, buyer_received=10 WHERE id=1", tbl)).Error; err != nil {
					return err
				}
				settleWon = true
			}
			return nil
		})
		if err != nil {
			t.Errorf("settle 事务失败: %v", err)
		}
	}()
	wg.Wait()

	var status string
	var sf, br float64
	db.Table(tbl).Select("status", "seller_frozen", "buyer_received").Where("id=1").Row().Scan(&status, &sf, &br)
	t.Logf("[c2c_cancel_vs_settle] cancelWon=%v settleWon=%v, 最终 status=%q seller_frozen=%.2f buyer_received=%.2f",
		cancelWon, settleWon, status, sf, br)
	if cancelWon && settleWon {
		t.Fatalf("FAIL: cancel 与 settle 都成功了——状态机被双写")
	}
	if !cancelWon && !settleWon {
		t.Fatalf("FAIL: 两者都没成功")
	}
	if status != "cancelled" && status != "settled" {
		t.Fatalf("FAIL: 终态状态非法 %q", status)
	}
	if status == "settled" && (br != 10 || sf != 0) {
		t.Fatalf("FAIL: settled 但 buyer_received=%.2f seller_frozen=%.2f 不一致", br, sf)
	}
	if status == "cancelled" && (br != 0 || sf != 0) {
		t.Fatalf("FAIL: cancelled 但余额未一致 buyer_received=%.2f seller_frozen=%.2f", br, sf)
	}
	t.Log("PASS: cancel 与 settle 互斥，仅一个生效，终态余额一致")
}

// 场景 5：A->B 与 B->A 双向并发结算，按统一顺序（id 升序）加锁，不死锁。
func TestRowLockBidirectionalSettleNoDeadlock(t *testing.T) {
	db := openRowLockDB(t)
	const tbl = "rllock_acct"
	dropTables(t, db, tbl)
	if err := db.Exec(fmt.Sprintf(`CREATE TABLE %s (
		id int PRIMARY KEY,
		available numeric(20,2) NOT NULL DEFAULT 0
	)`, tbl)).Error; err != nil {
		t.Fatalf("建表失败: %v", err)
	}
	t.Cleanup(func() { dropTables(t, db, tbl) })
	if err := db.Exec(fmt.Sprintf("INSERT INTO %s(id,available) VALUES (1,100),(2,100)", tbl)).Error; err != nil {
		t.Fatalf("插入失败: %v", err)
	}

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	// T1: A(id=1) -> B(id=2)，锁顺序仍按 id 升序（1 先于 2）
	wg.Add(1)
	go func() {
		defer wg.Done()
		errs <- db.Transaction(func(tx *gorm.DB) error {
			// 统一按 id 升序加锁，避免与反向事务交叉死锁
			rows, err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Table(tbl).Select("id", "available").Where("id IN (1,2) ORDER BY id").Rows()
			if err != nil {
				return err
			}
			defer rows.Close()
			for rows.Next() {
				var id int
				var avail float64
				rows.Scan(&id, &avail)
			}
			if err := tx.Exec(fmt.Sprintf("UPDATE %s SET available=available-10 WHERE id=1", tbl)).Error; err != nil {
				return err
			}
			return tx.Exec(fmt.Sprintf("UPDATE %s SET available=available+10 WHERE id=2", tbl)).Error
		})
	}()
	// T2: B(id=2) -> A(id=1)，同样按 id 升序加锁（先锁 1 再锁 2），与 T1 同向 -> 不死锁
	wg.Add(1)
	go func() {
		defer wg.Done()
		errs <- db.Transaction(func(tx *gorm.DB) error {
			rows, err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Table(tbl).Select("id", "available").Where("id IN (1,2) ORDER BY id").Rows()
			if err != nil {
				return err
			}
			defer rows.Close()
			for rows.Next() {
				var id int
				var avail float64
				rows.Scan(&id, &avail)
			}
			if err := tx.Exec(fmt.Sprintf("UPDATE %s SET available=available-10 WHERE id=2", tbl)).Error; err != nil {
				return err
			}
			return tx.Exec(fmt.Sprintf("UPDATE %s SET available=available+10 WHERE id=1", tbl)).Error
		})
	}()
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatalf("双向结算事务出错（疑似死锁）: %v", e)
		}
	}

	var a, b float64
	db.Table(tbl).Select("available").Where("id=1").Row().Scan(&a)
	db.Table(tbl).Select("available").Where("id=2").Row().Scan(&b)
	t.Logf("[bidirectional_settle] 双向各结算 10，最终 A=%.2f B=%.2f（净效果应为 A=100 B=100）", a, b)
	if a != 100 || b != 100 {
		t.Fatalf("FAIL: 双向结算终态错误 A=%.2f B=%.2f", a, b)
	}
	t.Log("PASS: 双向并发结算按统一锁顺序加锁，无死锁、无丢更新")
}

// 确保 time 包被引用（后续扩展用）
var _ = time.Second
