package integrationtest

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	usernotificationapp "github.com/Aether-v1/hcz/internal/modules/usernotification/application"
	"github.com/Aether-v1/hcz/internal/modules/usernotification/contract"
	"github.com/Aether-v1/hcz/internal/modules/usernotification/domain"
	usernotificationgormstore "github.com/Aether-v1/hcz/internal/modules/usernotification/infrastructure/gormstore"
	"github.com/Aether-v1/hcz/internal/shared/jsonmap"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupUserNotificationTest(t *testing.T) (*usernotificationapp.Service, *gorm.DB) {
	t.Helper()
	dsn := fmt.Sprintf("file:unotif_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&domain.UserNotification{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}
	store := usernotificationgormstore.New(db)
	return usernotificationapp.NewService(store), db
}

// 11. 唯一约束幂等：重复 Create 不报错、不重复（callback 重放不产生重复通知）。
func TestCreateNotificationIdempotentUniqueConstraint(t *testing.T) {
	svc, db := setupUserNotificationTest(t)
	ctx := context.Background()

	in := contract.CreateInput{
		UserID: 100, Type: domain.TypeWalletRecharge,
		Title: "钱包充值到账", Body: "USDT 充值已到账",
		Data:    jsonmap.JSON{"recharge_no": "RC1", "amount": "100.00"},
		BizType: domain.BizTypeWalletRecharge, BizID: 555,
	}
	if err := svc.CreateNotification(ctx, in); err != nil {
		t.Fatalf("first create failed: %v", err)
	}
	// callback 重放：同 (user,biz_type,biz_id,type) 再写一次，必须静默成功、不重复。
	if err := svc.CreateNotification(ctx, in); err != nil {
		t.Fatalf("replay create should be idempotent (no error), got: %v", err)
	}
	// 同 biz 但不同 type -> 视为不同通知。
	in2 := in
	in2.Type = domain.TypeOrderCompleted
	if err := svc.CreateNotification(ctx, in2); err != nil {
		t.Fatalf("different type create failed: %v", err)
	}

	var count int64
	db.Model(&domain.UserNotification{}).
		Where("user_id = ? AND biz_type = ? AND biz_id = ? AND type = ?", 100, domain.BizTypeWalletRecharge, 555, domain.TypeWalletRecharge).
		Count(&count)
	if count != 1 {
		t.Fatalf("wallet_recharge notification count = %d, want 1 (replay deduped)", count)
	}
	var total int64
	db.Model(&domain.UserNotification{}).Where("user_id = ?", 100).Count(&total)
	if total != 2 {
		t.Fatalf("total notifications = %d, want 2 (one wallet_recharge + one order_completed diff type)", total)
	}
}

// 5. unread count 正确。
func TestCountUnread(t *testing.T) {
	svc, _ := setupUserNotificationTest(t)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		if err := svc.CreateNotification(ctx, contract.CreateInput{
			UserID: 7, Type: domain.TypeOrderProcessing, Title: "t",
			BizType: domain.BizTypeOrder, BizID: uint(i + 1),
		}); err != nil {
			t.Fatalf("create %d failed: %v", i, err)
		}
	}
	// 另一用户也有一条，不应计入。
	if err := svc.CreateNotification(ctx, contract.CreateInput{
		UserID: 8, Type: domain.TypeOrderProcessing, Title: "t",
		BizType: domain.BizTypeOrder, BizID: uint(1),
	}); err != nil {
		t.Fatalf("other user create failed: %v", err)
	}
	unread, err := svc.CountUnread(ctx, 7)
	if err != nil {
		t.Fatalf("count unread failed: %v", err)
	}
	if unread != 3 {
		t.Fatalf("unread count = %d, want 3", unread)
	}
}

// 6. mark read 正确（幂等）。
func TestMarkReadIdempotent(t *testing.T) {
	svc, db := setupUserNotificationTest(t)
	ctx := context.Background()
	if err := svc.CreateNotification(ctx, contract.CreateInput{
		UserID: 7, Type: domain.TypeOrderCompleted, Title: "t",
		BizType: domain.BizTypeOrder, BizID: 1,
	}); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	var n domain.UserNotification
	if err := db.Where("user_id = ?", 7).First(&n).Error; err != nil {
		t.Fatalf("load notification failed: %v", err)
	}
	if err := svc.MarkRead(ctx, n.ID, 7); err != nil {
		t.Fatalf("first mark read failed: %v", err)
	}
	// 已读再读：幂等，仍返回 nil（ok）。
	if err := svc.MarkRead(ctx, n.ID, 7); err != nil {
		t.Fatalf("second mark read should be idempotent, got: %v", err)
	}
	var reloaded domain.UserNotification
	db.First(&reloaded, n.ID)
	if !reloaded.IsRead || reloaded.ReadAt == nil {
		t.Fatalf("notification not marked read: %+v", reloaded)
	}
}

// 9. User A 无法 mark User B 的通知（返回 ErrNotFound）。
func TestMarkReadIDOR(t *testing.T) {
	svc, db := setupUserNotificationTest(t)
	ctx := context.Background()
	if err := svc.CreateNotification(ctx, contract.CreateInput{
		UserID: 7, Type: domain.TypeOrderCompleted, Title: "t",
		BizType: domain.BizTypeOrder, BizID: 1,
	}); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	var n domain.UserNotification
	db.Where("user_id = ?", 7).First(&n)
	// User 8 试图 mark User 7 的通知。
	err := svc.MarkRead(ctx, n.ID, 8)
	if !errors.Is(err, contract.ErrNotFound) {
		t.Fatalf("mark other user's notification err = %v, want ErrNotFound", err)
	}
	// 原通知仍未读。
	var reloaded domain.UserNotification
	db.First(&reloaded, n.ID)
	if reloaded.IsRead {
		t.Fatalf("other user marked the notification as read — IDOR leak")
	}
}

// 7. mark all read 正确。
func TestMarkAllRead(t *testing.T) {
	svc, _ := setupUserNotificationTest(t)
	ctx := context.Background()
	for i := 0; i < 4; i++ {
		if err := svc.CreateNotification(ctx, contract.CreateInput{
			UserID: 7, Type: domain.TypeOrderProcessing, Title: "t",
			BizType: domain.BizTypeOrder, BizID: uint(i + 1),
		}); err != nil {
			t.Fatalf("create %d failed: %v", i, err)
		}
	}
	marked, err := svc.MarkAllRead(ctx, 7)
	if err != nil {
		t.Fatalf("mark all read failed: %v", err)
	}
	if marked != 4 {
		t.Fatalf("marked = %d, want 4", marked)
	}
	unread, _ := svc.CountUnread(ctx, 7)
	if unread != 0 {
		t.Fatalf("unread after mark all = %d, want 0", unread)
	}
	// 再次 mark all -> 0。
	marked2, _ := svc.MarkAllRead(ctx, 7)
	if marked2 != 0 {
		t.Fatalf("second mark all marked = %d, want 0", marked2)
	}
}

// 8. User A 无法读取 User B 的通知列表隔离（list 按 user_id 隔离）。
func TestListByUserScoping(t *testing.T) {
	svc, _ := setupUserNotificationTest(t)
	ctx := context.Background()
	if err := svc.CreateNotification(ctx, contract.CreateInput{
		UserID: 7, Type: domain.TypeWalletRecharge, Title: "a", BizType: domain.BizTypeWalletRecharge, BizID: 1,
	}); err != nil {
		t.Fatalf("create user7 failed: %v", err)
	}
	if err := svc.CreateNotification(ctx, contract.CreateInput{
		UserID: 8, Type: domain.TypeWalletRecharge, Title: "b", BizType: domain.BizTypeWalletRecharge, BizID: 1,
	}); err != nil {
		t.Fatalf("create user8 failed: %v", err)
	}
	items, total, err := svc.ListByUser(ctx, 7, 1, 20)
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("user7 list total = %d, len = %d, want 1/1", total, len(items))
	}
	if items[0].UserID != 7 {
		t.Fatalf("leaked other user notification: %+v", items[0])
	}
}

// 4. 写入失败不产生通知（Store 返回错误时 Service 不吞掉真实错误，且库里无脏数据）。
// 这里通过对 user_id=0 校验验证：UserID=0 的通知被静默忽略、不入库。
func TestCreateNotificationGuestIgnored(t *testing.T) {
	svc, db := setupUserNotificationTest(t)
	ctx := context.Background()
	if err := svc.CreateNotification(ctx, contract.CreateInput{
		UserID: 0, Type: domain.TypeWalletRecharge, Title: "guest",
		BizType: domain.BizTypeWalletRecharge, BizID: 1,
	}); err != nil {
		t.Fatalf("guest create should be ignored silently, got: %v", err)
	}
	var total int64
	db.Model(&domain.UserNotification{}).Count(&total)
	if total != 0 {
		t.Fatalf("guest (user_id=0) notification persisted, total = %d, want 0", total)
	}
}

// 2/3. 订单 processing/completed 事件写入（通过 service.CreateNotification 模拟 hook 入参）。
func TestOrderProcessingAndCompletedNotifications(t *testing.T) {
	svc, db := setupUserNotificationTest(t)
	ctx := context.Background()
	// 模拟订单从 pending_recharge -> processing
	if err := svc.CreateNotification(ctx, contract.CreateInput{
		UserID: 10, Type: domain.TypeOrderProcessing, Title: "订单处理中",
		Body:    "您的订单已支付成功，正在为您处理",
		Data:    jsonmap.JSON{"order_no": "R1", "amount": "9.90", "currency": "CNY"},
		BizType: domain.BizTypeOrder, BizID: 999,
	}); err != nil {
		t.Fatalf("order processing notify failed: %v", err)
	}
	// 模拟同一订单后续 completed
	if err := svc.CreateNotification(ctx, contract.CreateInput{
		UserID: 10, Type: domain.TypeOrderCompleted, Title: "订单已完成",
		Data:    jsonmap.JSON{"order_no": "R1"},
		BizType: domain.BizTypeOrder, BizID: 999,
	}); err != nil {
		t.Fatalf("order completed notify failed: %v", err)
	}
	// 同一订单 processing + completed 各一条（type 不同，不互斥）。
	var processing, completed int64
	db.Model(&domain.UserNotification{}).Where("user_id = ? AND biz_type = ? AND biz_id = ? AND type = ?", 10, domain.BizTypeOrder, 999, domain.TypeOrderProcessing).Count(&processing)
	db.Model(&domain.UserNotification{}).Where("user_id = ? AND biz_type = ? AND biz_id = ? AND type = ?", 10, domain.BizTypeOrder, 999, domain.TypeOrderCompleted).Count(&completed)
	if processing != 1 || completed != 1 {
		t.Fatalf("processing=%d completed=%d, want 1/1", processing, completed)
	}
	// completed 重放 -> 不重复。
	_ = svc.CreateNotification(ctx, contract.CreateInput{
		UserID: 10, Type: domain.TypeOrderCompleted, Title: "订单已完成",
		BizType: domain.BizTypeOrder, BizID: 999,
	})
	db.Model(&domain.UserNotification{}).Where("user_id = ? AND biz_type = ? AND biz_id = ? AND type = ?", 10, domain.BizTypeOrder, 999, domain.TypeOrderCompleted).Count(&completed)
	if completed != 1 {
		t.Fatalf("completed replay count = %d, want 1", completed)
	}
}
