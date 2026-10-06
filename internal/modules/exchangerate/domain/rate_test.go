package exchangerate

import (
	"errors"
	"testing"

	"github.com/shopspring/decimal"
)

func mustRateDec(s string) decimal.Decimal {
	d, _ := decimal.NewFromString(s)
	return d
}

// TestRateInvalidReject: rate<=0 must be rejected on every conversion path.
func TestRateInvalidReject(t *testing.T) {
	r := Rate{Rate: mustRateDec("0"), Currency: "CNY", Source: SourceAuto}
	if _, err := r.ToUSDT(mustRateDec("100")); !errors.Is(err, ErrRateUnavailable) {
		t.Fatalf("ToUSDT zero rate: want ErrRateUnavailable, got %v", err)
	}
	if _, err := r.EffectiveRate(mustRateDec("0.5")); !errors.Is(err, ErrRateUnavailable) {
		t.Fatalf("EffectiveRate zero rate: want ErrRateUnavailable, got %v", err)
	}
	neg := Rate{Rate: mustRateDec("-7"), Currency: "CNY", Source: SourceAuto}
	if _, err := neg.ToUSDT(mustRateDec("100")); !errors.Is(err, ErrRateUnavailable) {
		t.Fatalf("ToUSDT negative rate: want ErrRateUnavailable, got %v", err)
	}
}

// TestRateSafetyBuffer: effective_rate = market_rate × (1 - buffer/100),
// and the resulting USDT receivable is larger (ceil 2dp).
func TestRateSafetyBuffer(t *testing.T) {
	r := Rate{Rate: mustRateDec("7.0"), Currency: "CNY", Source: SourceAuto}

	// effective_rate = 7.0 × (1 - 0.5/100) = 6.965
	eff, err := r.EffectiveRate(mustRateDec("0.5"))
	if err != nil {
		t.Fatalf("EffectiveRate: %v", err)
	}
	if !eff.Equal(mustRateDec("6.965")) {
		t.Fatalf("effective_rate = %s, want 6.965", eff.String())
	}

	// No buffer baseline: 100 / 7.0 = 14.29 (round 2dp)
	plain, err := r.ToUSDT(mustRateDec("100"))
	if err != nil {
		t.Fatalf("ToUSDT: %v", err)
	}
	// Buffered: ceil(100 / 6.965, 2dp) = ceil(14.3575...) = 14.36
	buffered, err := r.ToUSDTWithBuffer(mustRateDec("100"), mustRateDec("0.5"))
	if err != nil {
		t.Fatalf("ToUSDTWithBuffer: %v", err)
	}
	if buffered.LessThan(plain) {
		t.Fatalf("buffered usdt %s must be >= plain %s (buffer collects more)", buffered.String(), plain.String())
	}
	if !buffered.Equal(mustRateDec("14.36")) {
		t.Fatalf("buffered usdt = %s, want 14.36", buffered.String())
	}

	// Non-positive buffer falls back to market rate, no discount.
	effZero, _ := r.EffectiveRate(mustRateDec("0"))
	if !effZero.Equal(mustRateDec("7.0")) {
		t.Fatalf("zero buffer effective rate = %s, want 7.0", effZero.String())
	}
}
