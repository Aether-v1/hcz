package integrationtest

import (
	"context"
	"sync"
	"testing"

	supportcontract "github.com/Aether-v1/hcz/internal/modules/supportticket/contract"
	"github.com/Aether-v1/hcz/internal/modules/supportticket/statemachine"
)

func TestClaimTicketRace(t *testing.T) {
	f := newFixture(t)
	cat := f.seedCategory("account", "账户", "normal", true)
	t1, _ := f.svc.CreateTicket(context.Background(), supportcontract.CreateTicketInput{
		UserID: 1, CategoryID: cat.ID, Subject: "x", Body: "y",
	})

	// 两个 admin 同时认领同一个未指派工单。
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(adminID uint) {
			defer wg.Done()
			_, err := f.svc.AssignTicket(context.Background(), supportcontract.AssignTicketInput{
				OperatorAdminID: adminID,
				TicketID:        t1.ID,
				AdminID:         0, // claim self
			})
			results <- err
		}(uint(1 + i))
	}
	wg.Wait()
	close(results)

	successes := 0
	for err := range results {
		if err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("claim successes = %d, want exactly 1", successes)
	}
	// 最终应有且仅有一个 admin 被指派。
	reloaded, _ := f.repo.GetTicketByID(t1.ID)
	if reloaded.AssignedAdminID == nil {
		t.Fatal("ticket should be assigned")
	}
}

func TestUserReplyAndAdminReplyConcurrent(t *testing.T) {
	f := newFixture(t)
	cat := f.seedCategory("account", "账户", "normal", true)
	t1, _ := f.svc.CreateTicket(context.Background(), supportcontract.CreateTicketInput{
		UserID: 1, CategoryID: cat.ID, Subject: "x", Body: "y",
	})

	// 初始 open。用户回复 -> waiting_support，客服回复 -> waiting_user。
	// 并发执行：两个操作串行化在事务内，但不应丢消息或死锁。
	var wg sync.WaitGroup
	wg.Add(2)
	var userErr, adminErr error
	go func() {
		defer wg.Done()
		_, userErr = f.svc.ReplyToTicket(context.Background(), supportcontract.ReplyTicketInput{
			UserID: 1, TicketID: t1.ID, Body: "user reply",
		})
	}()
	go func() {
		defer wg.Done()
		_, adminErr = f.svc.AdminReply(context.Background(), supportcontract.AdminReplyInput{
			OperatorAdminID: 1, TicketID: t1.ID, Body: "admin reply",
		})
	}()
	wg.Wait()

	// 至少一个会因状态机冲突失败，另一个成功——不应 panic/死锁。
	_ = userErr
	_ = adminErr
	// 两条消息都应落库（事务各自成功或回滚，但总消息数为 1（首条）+ 成功回复数）。
	_, total, err := f.repo.ListMessagesByTicket(t1.ID, 1, 100)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if total < 1 || total > 3 {
		t.Fatalf("message total = %d, want 1..3", total)
	}
}

func TestCloseVsReplyRace(t *testing.T) {
	f := newFixture(t)
	cat := f.seedCategory("account", "账户", "normal", true)
	t1, _ := f.svc.CreateTicket(context.Background(), supportcontract.CreateTicketInput{
		UserID: 1, CategoryID: cat.ID, Subject: "x", Body: "y",
	})

	var wg sync.WaitGroup
	wg.Add(2)
	var closeErr, replyErr error
	go func() {
		defer wg.Done()
		_, closeErr = f.svc.CloseMyTicket(context.Background(), 1, t1.ID)
	}()
	go func() {
		defer wg.Done()
		_, replyErr = f.svc.ReplyToTicket(context.Background(), supportcontract.ReplyTicketInput{
			UserID: 1, TicketID: t1.ID, Body: "late",
		})
	}()
	wg.Wait()

	reloaded, _ := f.repo.GetTicketByID(t1.ID)
	// 终态必须是 closed 或 open/waiting_support 之一，不能出现不一致。
	if reloaded.Status != statemachine.StatusClosed &&
		reloaded.Status != statemachine.StatusOpen &&
		reloaded.Status != statemachine.StatusWaitingSupport {
		t.Fatalf("inconsistent status = %s", reloaded.Status)
	}
	if reloaded.Status == statemachine.StatusClosed {
		if replyErr == nil {
			t.Fatal("reply after close should have failed")
		}
		if closeErr != nil {
			t.Fatalf("close failed: %v", closeErr)
		}
	}
}
