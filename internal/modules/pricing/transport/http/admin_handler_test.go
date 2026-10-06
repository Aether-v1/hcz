package pricinghttp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	productdomain "github.com/Aether-v1/hcz/internal/modules/catalog/product/domain"
	exchangeratecontract "github.com/Aether-v1/hcz/internal/modules/exchangerate/contract"
	exchangeratedomain "github.com/Aether-v1/hcz/internal/modules/exchangerate/domain"
	settingsintegration "github.com/Aether-v1/hcz/internal/modules/settings/schema/integration"
	"github.com/Aether-v1/hcz/internal/shared/money"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

type fakeProductGetter struct{ product *productdomain.Product }

func (f *fakeProductGetter) GetAdminByID(string) (*productdomain.Product, error) {
	return f.product, nil
}

type fakeSettings struct{}

func (f *fakeSettings) GetProfitGuardSetting() (settingsintegration.ProfitGuardSetting, error) {
	return settingsintegration.ProfitGuardSetting{
		Enabled:                  true,
		RequireCostPrice:         true,
		RateSafetyBufferPercent:  1.0,
		MinimumProfitAmountCNY:   1.0,
		MinimumProfitRatePercent: 0,
	}, nil
}

func (f *fakeSettings) GetAffiliateSetting() (settingsintegration.AffiliateSetting, error) {
	return settingsintegration.DefaultAffiliateSetting(), nil
}

type fakeRates struct {
	state exchangeratecontract.State
	rate  exchangeratedomain.Rate
	err   error
}

func (f *fakeRates) Snapshot() (exchangeratecontract.State, exchangeratedomain.Rate, error) {
	return f.state, f.rate, f.err
}

func newTestProduct(price, cost string, exempt bool) *productdomain.Product {
	return &productdomain.Product{
		ID:             1,
		PriceAmount:    money.FromDecimal(mustDec(price)),
		CostPriceAmount: money.FromDecimal(mustDec(cost)),
		IsCostExempt:   exempt,
		TitleJSON:      map[string]interface{}{"zh-CN": "test"},
	}
}

func mustDec(s string) decimal.Decimal {
	d, err := decimal.NewFromString(s)
	if err != nil {
		panic(err)
	}
	return d
}

func doPreview(handler *AdminHandler, body string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/admin/pricing/preview", strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	handler.Preview(ctx)
	return w
}

func TestPreviewComputesProfitAndPasses(t *testing.T) {
	product := newTestProduct("100", "60", false)
	rates := &fakeRates{
		state: exchangeratecontract.State{Currency: "CNY", RateSafetyBufferPercent: 1.0},
		rate:  exchangeratedomain.Rate{Rate: mustDec("7.2"), Currency: "CNY", Source: exchangeratedomain.SourceAuto},
	}
	h := NewAdminHandler(&fakeProductGetter{product}, &fakeSettings{}, rates)

	w := doPreview(h, `{"product_id":1,"quantity":1}`)
	if w.Code != http.StatusOK {
		t.Fatalf("http=%d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		StatusCode int                      `json:"status_code"`
		Data       map[string]interface{}   `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	data := resp.Data
	if data["guard_result"] != "PASS" {
		t.Fatalf("expected PASS, got %v (%v)", data["guard_result"], data["guard_reason"])
	}
	if data["fx_available"] != true {
		t.Fatalf("expected fx available")
	}
	// effective_rate = 7.2 * (1 - 0.01) = 7.128
	if got := data["effective_rate"]; got != "7.128" {
		t.Fatalf("effective_rate=%v want 7.128", got)
	}
	if got := data["market_rate"]; got != "7.2" {
		t.Fatalf("market_rate=%v want 7.2", got)
	}
	// cost 60 < price 100, no cost missing.
	for _, wn := range data["warnings"].([]interface{}) {
		if wn == "COST_MISSING" {
			t.Fatalf("unexpected COST_MISSING warning: %v", data["warnings"])
		}
	}
}

func TestPreviewBlocksOnMissingCost(t *testing.T) {
	product := newTestProduct("100", "0", false)
	rates := &fakeRates{
		state: exchangeratecontract.State{Currency: "CNY", RateSafetyBufferPercent: 1.0},
		rate:  exchangeratedomain.Rate{Rate: mustDec("7.2"), Currency: "CNY", Source: exchangeratedomain.SourceAuto},
	}
	h := NewAdminHandler(&fakeProductGetter{product}, &fakeSettings{}, rates)

	w := doPreview(h, `{"product_id":1}`)
	var resp struct {
		Data map[string]interface{} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp.Data
	if data["guard_result"] != "BLOCKED" {
		t.Fatalf("expected BLOCKED on missing cost, got %v", data["guard_result"])
	}
	found := false
	for _, wn := range data["warnings"].([]interface{}) {
		if wn == "COST_MISSING" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected COST_MISSING warning, got %v", data["warnings"])
	}
}

func TestPreviewExemptSkipsCostGate(t *testing.T) {
	product := newTestProduct("100", "0", true) // 真零成本豁免
	rates := &fakeRates{
		state: exchangeratecontract.State{Currency: "CNY", RateSafetyBufferPercent: 1.0},
		rate:  exchangeratedomain.Rate{Rate: mustDec("7.2"), Currency: "CNY", Source: exchangeratedomain.SourceAuto},
	}
	h := NewAdminHandler(&fakeProductGetter{product}, &fakeSettings{}, rates)

	w := doPreview(h, `{"product_id":1}`)
	var resp struct {
		Data map[string]interface{} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp.Data
	// exempt + revenue 100 >= required 1 -> PASS despite cost=0
	if data["guard_result"] != "PASS" {
		t.Fatalf("exempt product should PASS cost gate, got %v (%v)", data["guard_result"], data["guard_reason"])
	}
}
