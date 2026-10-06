package profitguard

import (
	"errors"
	"testing"

	settingsintegration "github.com/Aether-v1/hcz/internal/modules/settings/schema/integration"
	"github.com/shopspring/decimal"
)

func pgDec(s string) decimal.Decimal { d, _ := decimal.NewFromString(s); return d }

// base scenario: revenue 100 CNY, rate 10 (1 USDT = 10 CNY), usdt 10.
func baseInputs() Inputs {
	return Inputs{
		RevenueCNY:   pgDec("100"),
		UsdtTotal:    pgDec("10.00"),
		ExchangeRate: pgDec("10.0"),
		Affiliate: settingsintegration.NormalizeAffiliateSetting(settingsintegration.AffiliateSetting{
			Enabled: true,
			MaxLevel: 1,
			LevelRates: []settingsintegration.LevelRate{
				{Level: 1, Enabled: true, Rate: 10}, // L1 = 10%
			},
		}),
	}
}

func passCfg() Config {
	return Config{Enabled: true, MinimumProfitCNY: pgDec("10"), MinimumProfitRate: pgDec("0")}
}

// --- D. Cost ---

func TestNormalCostProfitGuard(t *testing.T) {
	in := baseInputs()
	in.Lines = []LineCost{{CostPrice: pgDec("50"), Quantity: 1}}
	if err := Validate(in, passCfg()); err != nil {
		t.Fatalf("normal cost order should pass: %v", err)
	}
}

func TestMissingCostReject(t *testing.T) {
	in := baseInputs()
	in.Lines = []LineCost{{CostPrice: pgDec("0"), Quantity: 1}} // formal product, cost not set
	cfg := passCfg()
	cfg.RequireCostPrice = true
	if err := Validate(in, cfg); !errors.Is(err, ErrProductCostNotConfigured) {
		t.Fatalf("want ErrProductCostNotConfigured, got %v", err)
	}
}

func TestZeroCostDigitalProduct(t *testing.T) {
	in := baseInputs()
	in.Lines = []LineCost{{CostPrice: pgDec("0"), Quantity: 1, ZeroCostAllowed: true}}
	cfg := passCfg()
	cfg.RequireCostPrice = true
	if err := Validate(in, cfg); err != nil {
		t.Fatalf("zero-cost digital entitlement must pass enforcement: %v", err)
	}
}

// --- E. Affiliate cost ---

func TestAffiliateL1Cost(t *testing.T) {
	in := baseInputs()
	// L1=10% on usdt 10 => 1.0 USDT => 10 CNY
	costUSDT := EstimateMaxAffiliateCostUSDT(in.UsdtTotal, in.Affiliate)
	if !costUSDT.Equal(pgDec("1.00")) {
		t.Fatalf("L1 commission = %s, want 1.00", costUSDT.String())
	}
}

func TestAffiliateMultiLevelCost(t *testing.T) {
	in := baseInputs()
	in.Affiliate = settingsintegration.NormalizeAffiliateSetting(settingsintegration.AffiliateSetting{
		Enabled: true, MaxLevel: 3,
		LevelRates: []settingsintegration.LevelRate{
			{Level: 1, Enabled: true, Rate: 10},
			{Level: 2, Enabled: true, Rate: 5},
			{Level: 3, Enabled: true, Rate: 2},
		},
	})
	sum := SumEnabledLevelRates(in.Affiliate)
	if !sum.Equal(pgDec("17")) {
		t.Fatalf("sum enabled rates = %s, want 17", sum.String())
	}
	// worst-case = 10 USDT × 17% = 1.70 USDT => 17 CNY
	costUSDT := EstimateMaxAffiliateCostUSDT(in.UsdtTotal, in.Affiliate)
	if !costUSDT.Equal(pgDec("1.70")) {
		t.Fatalf("multi-level commission = %s, want 1.70", costUSDT.String())
	}
}

func TestAffiliateMaximumApplicableCost(t *testing.T) {
	in := baseInputs()
	// MaxLevel=2 but L3 rate configured; only first 2 levels count.
	in.Affiliate = settingsintegration.NormalizeAffiliateSetting(settingsintegration.AffiliateSetting{
		Enabled: true, MaxLevel: 2,
		LevelRates: []settingsintegration.LevelRate{
			{Level: 1, Enabled: true, Rate: 10},
			{Level: 2, Enabled: true, Rate: 5},
			{Level: 3, Enabled: true, Rate: 50}, // beyond MaxLevel, ignored
		},
	})
	costUSDT := EstimateMaxAffiliateCostUSDT(in.UsdtTotal, in.Affiliate)
	// 10 × (10+5)/100 = 1.50
	if !costUSDT.Equal(pgDec("1.50")) {
		t.Fatalf("max applicable commission = %s, want 1.50 (L3 ignored)", costUSDT.String())
	}
}

func TestAffiliateMakesOrderNegative(t *testing.T) {
	in := baseInputs()
	// Very high affiliate rate eats the whole margin.
	in.Affiliate = settingsintegration.NormalizeAffiliateSetting(settingsintegration.AffiliateSetting{
		Enabled: true, MaxLevel: 1,
		LevelRates: []settingsintegration.LevelRate{
			{Level: 1, Enabled: true, Rate: 95},
		},
	})
	in.Lines = []LineCost{{CostPrice: pgDec("10"), Quantity: 1}} // net = 100-10-95 = -5
	cfg := passCfg()
	cfg.MinimumProfitCNY = pgDec("0")
	err := Validate(in, cfg)
	if !errors.Is(err, ErrUnprofitableOrder) {
		t.Fatalf("commission-eaten order must be rejected, got err=%v", err)
	}
}

// --- F. Profit Guard ---

func TestProfitGuardPositive(t *testing.T) {
	in := baseInputs()
	in.Lines = []LineCost{{CostPrice: pgDec("50"), Quantity: 1}}
	if err := Validate(in, passCfg()); err != nil {
		t.Fatalf("positive profit should pass: %v", err)
	}
}

func TestProfitGuardExactThreshold(t *testing.T) {
	in := baseInputs()
	// cost 50 + affiliate 10 = 60 => net = 40. required = 40 => pass (not less-than).
	in.Lines = []LineCost{{CostPrice: pgDec("50"), Quantity: 1}}
	cfg := passCfg()
	cfg.MinimumProfitCNY = pgDec("40")
	if err := Validate(in, cfg); err != nil {
		t.Fatalf("exactly-at-threshold must pass: %v", err)
	}
}

func TestProfitGuardBelowThreshold(t *testing.T) {
	in := baseInputs()
	in.Lines = []LineCost{{CostPrice: pgDec("50"), Quantity: 1}} // net 40
	cfg := passCfg()
	cfg.MinimumProfitCNY = pgDec("50") // net 40 < 50
	if err := Validate(in, cfg); !errors.Is(err, ErrUnprofitableOrder) {
		t.Fatalf("below threshold must reject, got %v", err)
	}
}

func TestProfitGuardNegative(t *testing.T) {
	in := baseInputs()
	in.Lines = []LineCost{{CostPrice: pgDec("200"), Quantity: 1}} // net = 100-200-10 <0
	cfg := passCfg()
	cfg.MinimumProfitCNY = pgDec("0")
	if err := Validate(in, cfg); !errors.Is(err, ErrUnprofitableOrder) {
		t.Fatalf("negative profit must reject, got %v", err)
	}
}

func TestProfitGuardFeeIncluded(t *testing.T) {
	in := baseInputs()
	in.Lines = []LineCost{{CostPrice: pgDec("50"), Quantity: 1}}
	in.FeeCostCNY = pgDec("5")
	cfg := passCfg()
	cfg.MinimumProfitCNY = pgDec("30") // net without fee=40, with fee=35 >=30 pass
	if err := Validate(in, cfg); err != nil {
		t.Fatalf("fee-included order should still pass at threshold 30: %v", err)
	}
	// raise required above 35 (net with fee) to prove fee actually counted.
	cfg.MinimumProfitCNY = pgDec("36")
	if err := Validate(in, cfg); !errors.Is(err, ErrUnprofitableOrder) {
		t.Fatalf("fee must have reduced net (35<36 reject), got err=%v", err)
	}
}

func TestProfitGuardNoDoubleFee(t *testing.T) {
	in := baseInputs()
	in.Lines = []LineCost{{CostPrice: pgDec("50"), Quantity: 1}}
	in.FeeCostCNY = pgDec("5")
	net, _ := Evaluate(in, passCfg())
	// net = 100 - 50 - 5(fee once) - 10(affiliate) = 35.
	if !net.Equal(pgDec("35")) {
		t.Fatalf("net must subtract fee exactly once = 35, got %s", net.String())
	}
}

func TestProfitGuardNoDoubleFXRisk(t *testing.T) {
	in := baseInputs()
	in.Lines = []LineCost{{CostPrice: pgDec("50"), Quantity: 1}}
	cfg := passCfg()
	cfg.FXBufferPercent = pgDec("0.5") // 0.5%
	// fxRisk = usdt10 × 0.5% × rate10 = 0.50 CNY, counted once.
	net, required := Evaluate(in, cfg)
	// net = 100 - 50 - 0 - 10 - 0.5 = 39.5
	if !net.Equal(pgDec("39.5")) {
		t.Fatalf("net = %s, want 39.5 (fx risk counted once)", net.String())
	}
	_ = required
}
