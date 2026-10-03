package application

import (
	"context"
	"errors"
	"testing"
	"time"

	exchangerate "github.com/Aether-v1/hcz/internal/modules/exchangerate/contract"
	exchangeratedomain "github.com/Aether-v1/hcz/internal/modules/exchangerate/domain"
	"github.com/shopspring/decimal"
)

type memStore struct {
	state exchangerate.State
}

func (m *memStore) GetState() (exchangerate.State, error) { return m.state, nil }
func (m *memStore) SaveState(s exchangerate.State) error {
	m.state = s
	return nil
}

type fakeProvider struct {
	rate decimal.Decimal
	err  error
}

func (f *fakeProvider) Name() string { return "fake" }
func (f *fakeProvider) Fetch(ctx context.Context, c string, key string) (decimal.Decimal, time.Time, error) {
	if f.err != nil {
		return decimal.Zero, time.Time{}, f.err
	}
	return f.rate, time.Now(), nil
}

func mustDec(s string) decimal.Decimal { d, _ := decimal.NewFromString(s); return d }

func TestResolveUsesFreshAuto(t *testing.T) {
	store := &memStore{state: exchangerate.State{
		Currency: "CNY", AutoRate: mustDec("7.18"), AutoFetchedAt: time.Now(),
	}}
	svc := NewService(&fakeProvider{rate: mustDec("7.18")}, store, "CNY", time.Hour)
	r, err := svc.Resolve(context.Background())
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if r.Source != exchangeratedomain.SourceAuto {
		t.Fatalf("source want AUTO got %s", r.Source)
	}
	usdt, _ := r.ToUSDT(mustDec("100.00"))
	if !usdt.Equal(mustDec("13.93")) {
		t.Fatalf("100 CNY / 7.18 = 13.93 USDT, got %s", usdt)
	}
}

func TestResolveFallsBackToManualWhenAutoStale(t *testing.T) {
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
		t.Fatalf("source want MANUAL got %s", r.Source)
	}
}

func TestResolveFailClosedWhenNothingAvailable(t *testing.T) {
	store := &memStore{state: exchangerate.State{Currency: "CNY"}}
	svc := NewService(&fakeProvider{}, store, "CNY", time.Hour)
	if _, err := svc.Resolve(context.Background()); !errors.Is(err, exchangeratedomain.ErrRateUnavailable) {
		t.Fatalf("want ErrRateUnavailable got %v", err)
	}
}

func TestRefreshRecordsProviderErrorWithout1to1(t *testing.T) {
	store := &memStore{state: exchangerate.State{Currency: "CNY", AutoEnabled: true}}
	svc := NewService(&fakeProvider{err: errors.New("network down")}, store, "CNY", time.Hour)
	if err := svc.Refresh(context.Background()); err == nil {
		t.Fatal("refresh should surface provider error")
	}
	// 失败后仍无有效汇率 → fail-closed
	if _, err := svc.Resolve(context.Background()); !errors.Is(err, exchangeratedomain.ErrRateUnavailable) {
		t.Fatalf("want fail-closed got %v", err)
	}
	if store.state.LastError == "" {
		t.Fatal("last_error should be recorded")
	}
}

func TestUSDTCurrencyNoConversionNeeded(t *testing.T) {
	store := &memStore{state: exchangerate.State{
		Currency: "USDT", AutoRate: mustDec("1"), AutoFetchedAt: time.Now(),
	}}
	svc := NewService(&fakeProvider{}, store, "USDT", time.Hour)
	r, err := svc.Resolve(context.Background())
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	usdt, _ := r.ToUSDT(mustDec("50.00"))
	if !usdt.Equal(mustDec("50.00")) {
		t.Fatalf("USDT site should be 1:1, got %s", usdt)
	}
}
