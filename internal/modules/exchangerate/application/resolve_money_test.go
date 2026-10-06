package application

import (
	"context"
	"errors"
	"testing"
	"time"

	exchangerate "github.com/Aether-v1/hcz/internal/modules/exchangerate/contract"
	exchangeratedomain "github.com/Aether-v1/hcz/internal/modules/exchangerate/domain"
)

// TestAutoRateResolve: fresh AUTO is used directly.
func TestAutoRateResolve(t *testing.T) {
	store := &memStore{state: exchangerate.State{
		Currency: "CNY", AutoRate: mustDec("7.18"), AutoFetchedAt: time.Now(),
	}}
	svc := NewService(&fakeProvider{}, store, "CNY", time.Hour)
	r, err := svc.Resolve(context.Background())
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if r.Source != exchangeratedomain.SourceAuto {
		t.Fatalf("want AUTO, got %s", r.Source)
	}
	if !r.Rate.Equal(mustDec("7.18")) {
		t.Fatalf("want rate 7.18, got %s", r.Rate.String())
	}
}

// TestStaleAutoFallbackToManual: expired AUTO falls back to MANUAL.
func TestStaleAutoFallbackToManual(t *testing.T) {
	store := &memStore{state: exchangerate.State{
		Currency: "CNY",
		AutoRate: mustDec("7.18"), AutoFetchedAt: time.Now().Add(-48 * time.Hour), // stale
		ManualRate: mustDec("7.50"),
	}}
	svc := NewService(&fakeProvider{}, store, "CNY", time.Hour)
	r, err := svc.Resolve(context.Background())
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if r.Source != exchangeratedomain.SourceManual {
		t.Fatalf("want MANUAL, got %s", r.Source)
	}
	if !r.Rate.Equal(mustDec("7.50")) {
		t.Fatalf("want manual 7.50, got %s", r.Rate.String())
	}
}

// TestNoFallbackReject: neither AUTO nor MANUAL available => reject (fail-closed).
func TestNoFallbackReject(t *testing.T) {
	store := &memStore{state: exchangerate.State{
		Currency:   "CNY",
		AutoRate:   mustDec("7.18"),
		AutoFetchedAt: time.Now().Add(-48 * time.Hour), // stale, ignored
		// no manual
	}}
	svc := NewService(&fakeProvider{}, store, "CNY", time.Hour)
	if _, err := svc.Resolve(context.Background()); !errors.Is(err, exchangeratedomain.ErrRateUnavailable) {
		t.Fatalf("want ErrRateUnavailable, got %v", err)
	}
}

// TestManualTimestampCorrect: manual fallback must echo ManualRateUpdatedAt (the
// write time), NOT the resolve-now. P4 semantics: reading an old manual rate must
// not be reported as "just updated".
func TestManualTimestampCorrect(t *testing.T) {
	writeTime := time.Now().Add(-10 * time.Minute)
	store := &memStore{state: exchangerate.State{
		Currency:            "CNY",
		AutoRate:            mustDec("7.18"),
		AutoFetchedAt:       time.Now().Add(-48 * time.Hour),
		ManualRate:          mustDec("7.50"),
		ManualRateUpdatedAt: writeTime,
	}}
	svc := NewService(&fakeProvider{}, store, "CNY", time.Hour)
	r, err := svc.Resolve(context.Background())
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if r.Source != exchangeratedomain.SourceManual {
		t.Fatalf("want MANUAL, got %s", r.Source)
	}
	// Must echo the write time, NOT now.
	if !r.FetchedAt.Equal(writeTime) {
		t.Fatalf("manual FetchedAt = %v, want write-time %v (must not be resolve-now)", r.FetchedAt, writeTime)
	}
}
