package ordermachine

import (
	"testing"

	"github.com/Aether-v1/hcz/internal/constants"
)

func TestNormalize_HistoryMapping(t *testing.T) {
	cases := map[string]View{
		"pending_payment":               {constants.OrderStatusPendingRecharge, constants.OrderRefundStatusNone},
		"paid":                          {constants.OrderStatusPendingRecharge, constants.OrderRefundStatusNone},
		"fulfilling":                    {constants.OrderStatusProcessing, constants.OrderRefundStatusNone},
		"partially_delivered":           {constants.OrderStatusProcessing, constants.OrderRefundStatusNone},
		"delivered":                     {constants.OrderStatusCompleted, constants.OrderRefundStatusNone},
		"completed":                     {constants.OrderStatusCompleted, constants.OrderRefundStatusNone},
		"canceled":                      {constants.OrderStatusCanceled, constants.OrderRefundStatusFull},
		"partially_refunded":            {constants.OrderStatusCompleted, constants.OrderRefundStatusPartial},
		"refunded":                      {constants.OrderStatusCompleted, constants.OrderRefundStatusFull},
		"pending_recharge":              {constants.OrderStatusPendingRecharge, constants.OrderRefundStatusNone},
		"processing":                    {constants.OrderStatusProcessing, constants.OrderRefundStatusNone},
		"failed":                        {constants.OrderStatusFailed, constants.OrderRefundStatusFull},
	}
	for old, want := range cases {
		got := Normalize(old)
		if got != want {
			t.Errorf("Normalize(%q) = %+v, want %+v", old, got, want)
		}
	}
}

func TestCanTransition_Allowed(t *testing.T) {
	allowed := [][2]string{
		{constants.OrderStatusPendingRecharge, constants.OrderStatusProcessing},
		{constants.OrderStatusPendingRecharge, constants.OrderStatusFailed},
		{constants.OrderStatusPendingRecharge, constants.OrderStatusCanceled},
		{constants.OrderStatusProcessing, constants.OrderStatusCompleted},
		{constants.OrderStatusProcessing, constants.OrderStatusFailed},
	}
	for _, c := range allowed {
		if !CanTransition(c[0], c[1]) {
			t.Errorf("expected allowed %s->%s", c[0], c[1])
		}
	}
}

func TestCanTransition_Forbidden(t *testing.T) {
	forbidden := [][2]string{
		{constants.OrderStatusCompleted, constants.OrderStatusProcessing},
		{constants.OrderStatusCompleted, constants.OrderStatusPendingRecharge},
		{constants.OrderStatusFailed, constants.OrderStatusProcessing},
		{constants.OrderStatusFailed, constants.OrderStatusCompleted},
		{constants.OrderStatusCanceled, constants.OrderStatusProcessing},
		{constants.OrderStatusCanceled, constants.OrderStatusCompleted},
		{constants.OrderStatusProcessing, constants.OrderStatusPendingRecharge},
	}
	for _, c := range forbidden {
		if CanTransition(c[0], c[1]) {
			t.Errorf("expected forbidden %s->%s", c[0], c[1])
		}
	}
}

func TestUserOnlyCancelFromPendingRecharge(t *testing.T) {
	if !AllowedByUser(constants.OrderStatusPendingRecharge, constants.OrderStatusCanceled) {
		t.Error("user must be able to cancel pending_recharge")
	}
	bad := [][2]string{
		{constants.OrderStatusProcessing, constants.OrderStatusCanceled},
		{constants.OrderStatusCompleted, constants.OrderStatusCanceled},
		{constants.OrderStatusFailed, constants.OrderStatusCanceled},
	}
	for _, c := range bad {
		if AllowedByUser(c[0], c[1]) {
			t.Errorf("user cancel must be rejected from %s", c[0])
		}
	}
}

func TestAutoRefundOnFailedCanceled(t *testing.T) {
	if !RequiresAutoRefund(constants.OrderStatusPendingRecharge, constants.OrderStatusFailed) {
		t.Error("pending_recharge->failed must auto refund")
	}
	if !RequiresAutoRefund(constants.OrderStatusProcessing, constants.OrderStatusFailed) {
		t.Error("processing->failed must auto refund")
	}
	if !RequiresAutoRefund(constants.OrderStatusPendingRecharge, constants.OrderStatusCanceled) {
		t.Error("pending_recharge->canceled must auto refund")
	}
	if RequiresAutoRefund(constants.OrderStatusProcessing, constants.OrderStatusCompleted) {
		t.Error("completed must not auto refund")
	}
}

func TestTerminal(t *testing.T) {
	for _, s := range []string{constants.OrderStatusCompleted, constants.OrderStatusFailed, constants.OrderStatusCanceled} {
		if !IsTerminal(s) {
			t.Errorf("%s must be terminal", s)
		}
	}
	for _, s := range []string{constants.OrderStatusPendingRecharge, constants.OrderStatusProcessing} {
		if IsTerminal(s) {
			t.Errorf("%s must not be terminal", s)
		}
	}
}
