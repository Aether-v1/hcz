package integrationtest

import (
	"context"
	"errors"
	"testing"
	"time"

	supportcontract "github.com/Aether-v1/hcz/internal/modules/supportticket/contract"
	supportdomain "github.com/Aether-v1/hcz/internal/modules/supportticket/domain"
	"github.com/Aether-v1/hcz/internal/modules/supportticket/statemachine"
)

func TestCreateTicketSuccess(t *testing.T) {
	f := newFixture(t)
	cat := f.seedCategory("account", "账户问题", "normal", true)

	ticket, err := f.svc.CreateTicket(context.Background(), supportcontract.CreateTicketInput{
		UserID:     100,
		CategoryID: cat.ID,
		Subject:    "无法登录",
		Body:       "我收不到验证码",
	})
	if err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	if ticket.Status != statemachine.StatusOpen {
		t.Fatalf("status = %s, want open", ticket.Status)
	}
	if ticket.Priority != "normal" {
		t.Fatalf("priority = %s, want normal", ticket.Priority)
	}
	if ticket.AdminUnreadCount != 1 {
		t.Fatalf("admin_unread = %d, want 1", ticket.AdminUnreadCount)
	}
	if ticket.UserUnreadCount != 0 {
		t.Fatalf("user_unread = %d, want 0", ticket.UserUnreadCount)
	}
	// 首条消息已创建
	msgs, total, err := f.repo.ListMessagesByTicket(ticket.ID, 1, 10)
	if err != nil || total != 1 || msgs[0].SenderType != "user" {
		t.Fatalf("first message wrong: total=%d err=%v", total, err)
	}
}

func TestCreateTicketDisabledCategory(t *testing.T) {
	f := newFixture(t)
	cat := f.seedCategory("bad", "已停用", "normal", false)
	_, err := f.svc.CreateTicket(context.Background(), supportcontract.CreateTicketInput{
		UserID: 1, CategoryID: cat.ID, Subject: "x", Body: "y",
	})
	if !errors.Is(err, supportcontract.ErrCategoryDisabled) {
		t.Fatalf("err = %v, want ErrCategoryDisabled", err)
	}
}

func TestCreateTicketUsesCategoryPriority(t *testing.T) {
	f := newFixture(t)
	cat := f.seedCategory("wallet", "钱包", "high", true)
	ticket, err := f.svc.CreateTicket(context.Background(), supportcontract.CreateTicketInput{
		UserID: 1, CategoryID: cat.ID, Subject: "x", Body: "y",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if ticket.Priority != "high" {
		t.Fatalf("priority = %s, want high", ticket.Priority)
	}
}

func TestUserIDOR(t *testing.T) {
	f := newFixture(t)
	cat := f.seedCategory("account", "账户", "normal", true)
	t1, _ := f.svc.CreateTicket(context.Background(), supportcontract.CreateTicketInput{
		UserID: 1, CategoryID: cat.ID, Subject: "x", Body: "y",
	})
	// user 2 cannot read user 1's ticket
	_, _, _, _, err := f.svc.GetMyTicket(2, t1.ID)
	if !errors.Is(err, supportcontract.ErrNotOwner) {
		t.Fatalf("err = %v, want ErrNotOwner", err)
	}
	// user 2 cannot reply
	_, err = f.svc.ReplyToTicket(context.Background(), supportcontract.ReplyTicketInput{
		UserID: 2, TicketID: t1.ID, Body: "hijack",
	})
	if !errors.Is(err, supportcontract.ErrNotOwner) {
		t.Fatalf("reply err = %v, want ErrNotOwner", err)
	}
}

func TestUserReplyUnreadCounts(t *testing.T) {
	f := newFixture(t)
	cat := f.seedCategory("account", "账户", "normal", true)
	t1, _ := f.svc.CreateTicket(context.Background(), supportcontract.CreateTicketInput{
		UserID: 1, CategoryID: cat.ID, Subject: "x", Body: "y",
	})
	// 初始 admin_unread=1, user_unread=0
	// user reply -> waiting_support, admin_unread+1 (=2), user_unread=0
	t1, err := f.svc.ReplyToTicket(context.Background(), supportcontract.ReplyTicketInput{
		UserID: 1, TicketID: t1.ID, Body: "follow up",
	})
	if err != nil {
		t.Fatalf("reply: %v", err)
	}
	if t1.Status != statemachine.StatusWaitingSupport {
		t.Fatalf("status = %s, want waiting_support", t1.Status)
	}
	// reload to check atomic counters
	reloaded, _ := f.repo.GetTicketByID(t1.ID)
	if reloaded.AdminUnreadCount != 2 {
		t.Fatalf("admin_unread = %d, want 2", reloaded.AdminUnreadCount)
	}
	if reloaded.UserUnreadCount != 0 {
		t.Fatalf("user_unread = %d, want 0", reloaded.UserUnreadCount)
	}
}

func TestAdminReplyUnreadAndNotification(t *testing.T) {
	f := newFixture(t)
	cat := f.seedCategory("account", "账户", "normal", true)
	t1, _ := f.svc.CreateTicket(context.Background(), supportcontract.CreateTicketInput{
		UserID: 1, CategoryID: cat.ID, Subject: "x", Body: "y",
	})
	// admin reply -> waiting_user, user_unread+1, admin_unread=0
	t1, err := f.svc.AdminReply(context.Background(), supportcontract.AdminReplyInput{
		OperatorAdminID: 1, TicketID: t1.ID, Body: "已处理",
	})
	if err != nil {
		t.Fatalf("admin reply: %v", err)
	}
	if t1.Status != statemachine.StatusWaitingUser {
		t.Fatalf("status = %s, want waiting_user", t1.Status)
	}
	reloaded, _ := f.repo.GetTicketByID(t1.ID)
	if reloaded.UserUnreadCount != 1 {
		t.Fatalf("user_unread = %d, want 1", reloaded.UserUnreadCount)
	}
	if reloaded.AdminUnreadCount != 0 {
		t.Fatalf("admin_unread = %d, want 0", reloaded.AdminUnreadCount)
	}
	// notification created
	if len(f.notifier.inputs) != 1 {
		t.Fatalf("notifications = %d, want 1", len(f.notifier.inputs))
	}
	if f.notifier.inputs[0].Type != "ticket_admin_replied" {
		t.Fatalf("notif type = %s", f.notifier.inputs[0].Type)
	}
}

func TestReadDetailResetsUnread(t *testing.T) {
	f := newFixture(t)
	cat := f.seedCategory("account", "账户", "normal", true)
	t1, _ := f.svc.CreateTicket(context.Background(), supportcontract.CreateTicketInput{
		UserID: 1, CategoryID: cat.ID, Subject: "x", Body: "y",
	})
	// admin reply makes user_unread=1
	f.svc.AdminReply(context.Background(), supportcontract.AdminReplyInput{OperatorAdminID: 1, TicketID: t1.ID, Body: "hi"})
	// user opens detail -> user_unread reset
	_, err := f.svc.GetMyTicketView(context.Background(), 1, t1.ID)
	if err != nil {
		t.Fatalf("get detail: %v", err)
	}
	reloaded, _ := f.repo.GetTicketByID(t1.ID)
	if reloaded.UserUnreadCount != 0 {
		t.Fatalf("user_unread after read = %d, want 0", reloaded.UserUnreadCount)
	}
}

func TestResolveThenReopenWithinWindow(t *testing.T) {
	f := newFixture(t)
	cat := f.seedCategory("account", "账户", "normal", true)
	t1, _ := f.svc.CreateTicket(context.Background(), supportcontract.CreateTicketInput{
		UserID: 1, CategoryID: cat.ID, Subject: "x", Body: "y",
	})
	t1, err := f.svc.ResolveTicket(context.Background(), supportcontract.ResolveTicketInput{OperatorAdminID: 1, TicketID: t1.ID})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if t1.Status != statemachine.StatusResolved || t1.ResolvedAt == nil {
		t.Fatalf("after resolve: status=%s resolved_at=%v", t1.Status, t1.ResolvedAt)
	}
	// reopen within 7 days
	t1, err = f.svc.ReopenMyTicket(context.Background(), 1, t1.ID)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if t1.Status != statemachine.StatusWaitingSupport {
		t.Fatalf("after reopen: status=%s", t1.Status)
	}
	if t1.ResolvedAt != nil {
		t.Fatalf("resolved_at should be cleared")
	}
}

func TestReopenAfterExpired(t *testing.T) {
	f := newFixture(t)
	cat := f.seedCategory("account", "账户", "normal", true)
	t1, _ := f.svc.CreateTicket(context.Background(), supportcontract.CreateTicketInput{
		UserID: 1, CategoryID: cat.ID, Subject: "x", Body: "y",
	})
	f.svc.ResolveTicket(context.Background(), supportcontract.ResolveTicketInput{OperatorAdminID: 1, TicketID: t1.ID})
	// 手动把 resolved_at 改成 8 天前
	old := time.Now().Add(-8 * 24 * time.Hour)
	f.db.Model(&supportdomain.Ticket{}).Where("id = ?", t1.ID).Update("resolved_at", old)
	_, err := f.svc.ReopenMyTicket(context.Background(), 1, t1.ID)
	if !errors.Is(err, supportcontract.ErrReopenExpired) {
		t.Fatalf("err = %v, want ErrReopenExpired", err)
	}
}

func TestClosedIsTerminal(t *testing.T) {
	f := newFixture(t)
	cat := f.seedCategory("account", "账户", "normal", true)
	t1, _ := f.svc.CreateTicket(context.Background(), supportcontract.CreateTicketInput{
		UserID: 1, CategoryID: cat.ID, Subject: "x", Body: "y",
	})
	// user closes
	t1, err := f.svc.CloseMyTicket(context.Background(), 1, t1.ID)
	if err != nil {
		t.Fatalf("close: %v", err)
	}
	if t1.Status != statemachine.StatusClosed {
		t.Fatalf("status = %s, want closed", t1.Status)
	}
	// cannot reply
	_, err = f.svc.ReplyToTicket(context.Background(), supportcontract.ReplyTicketInput{UserID: 1, TicketID: t1.ID, Body: "again"})
	if !errors.Is(err, supportcontract.ErrTicketClosed) {
		t.Fatalf("reply after close err = %v, want ErrTicketClosed", err)
	}
}

func TestAdminResolveNotifies(t *testing.T) {
	f := newFixture(t)
	cat := f.seedCategory("account", "账户", "normal", true)
	t1, _ := f.svc.CreateTicket(context.Background(), supportcontract.CreateTicketInput{
		UserID: 1, CategoryID: cat.ID, Subject: "x", Body: "y",
	})
	_, err := f.svc.ResolveTicket(context.Background(), supportcontract.ResolveTicketInput{OperatorAdminID: 1, TicketID: t1.ID})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	found := false
	for _, in := range f.notifier.inputs {
		if in.Type == "ticket_resolved" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected ticket_resolved notification, got %v", f.notifier.inputs)
	}
}

func TestBizOwnershipRejectsForeign(t *testing.T) {
	f := newFixture(t)
	cat := f.seedCategory("withdrawal", "提现", "high", true)
	// 没有 orders 表，校验应失败（owned=false）
	_, err := f.svc.CreateTicket(context.Background(), supportcontract.CreateTicketInput{
		UserID: 1, CategoryID: cat.ID, Subject: "x", Body: "y",
		BizType: "order", BizID: 999,
	})
	if !errors.Is(err, supportcontract.ErrInvalidBizOwnership) {
		t.Fatalf("err = %v, want ErrInvalidBizOwnership", err)
	}
}
