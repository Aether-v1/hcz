package money

import (
	"testing"

	"github.com/shopspring/decimal"
)

func mustSettlementDec(t *testing.T, s string) decimal.Decimal {
	t.Helper()
	d, err := decimal.NewFromString(s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return d
}

// TestSettlementCeil2dp locks the forward-receivable settlement rounding:
// CEIL to 2dp so the platform never under-collects USDT.
func TestSettlementCeil2dp(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"13.880000", "13.88"},
		{"13.880001", "13.89"},
		{"13.889", "13.89"},
		{"13.890001", "13.90"},
	}
	for _, c := range cases {
		got := CeilTo2(mustSettlementDec(t, c.in))
		if !got.Equal(mustSettlementDec(t, c.want)) {
			t.Fatalf("CeilTo2(%s) = %s, want %s", c.in, got.String(), c.want)
		}
	}
}

// TestUIDisplayRoundHalfUp locks the UI display rounding: ROUND_HALF_UP 2dp,
// which must stay independent from settlement CEIL.
func TestUIDisplayRoundHalfUp(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"13.880000", "13.88"},
		{"13.880001", "13.88"}, // UI does NOT ceil the tail
		{"13.889", "13.89"},
		{"13.890001", "13.89"}, // UI does NOT ceil the tail
		{"1.235", "1.24"},      // half-up: .5 rounds up
		{"1.234", "1.23"},
	}
	for _, c := range cases {
		got := RoundHalfUpTo2(mustSettlementDec(t, c.in))
		if !got.Equal(mustSettlementDec(t, c.want)) {
			t.Fatalf("RoundHalfUpTo2(%s) = %s, want %s", c.in, got.String(), c.want)
		}
	}
}
