// Package pricinghttp 提供管理员专用的「定价预览 / Profit Guard 核算明细」只读接口。
//
// 该接口只服务 admin，返回成本价、Affiliate 成本、内部 Buffer、Fee Cost 等敏感字段，
// 严禁复用到 user public DTO。核算口径与下单链路一致：调用 profitguard domain 的 Evaluate，
// 输入来自 product（成本/豁免）、settings（ProfitGuard/Affiliate）、exchangerate（有效汇率）。
package pricinghttp

import (
	"errors"
	"strconv"
	"time"

	productcontract "github.com/Aether-v1/hcz/internal/modules/catalog/product/contract"
	productdomain "github.com/Aether-v1/hcz/internal/modules/catalog/product/domain"
	exchangeratecontract "github.com/Aether-v1/hcz/internal/modules/exchangerate/contract"
	exchangeratedomain "github.com/Aether-v1/hcz/internal/modules/exchangerate/domain"
	settingsintegration "github.com/Aether-v1/hcz/internal/modules/settings/schema/integration"
	"github.com/Aether-v1/hcz/internal/platform/http/ginutil"
	"github.com/Aether-v1/hcz/internal/platform/http/response"
	profitguard "github.com/Aether-v1/hcz/internal/modules/profitguard/domain"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

// ProductGetter 按 ID 读取商品（admin 口径，含成本字段）。
type ProductGetter interface {
	GetAdminByID(id string) (*productdomain.Product, error)
}

// SettingsService 提供 Profit Guard 与 Affiliate 配置。
type SettingsService interface {
	GetProfitGuardSetting() (settingsintegration.ProfitGuardSetting, error)
	GetAffiliateSetting() (settingsintegration.AffiliateSetting, error)
}

// RateSnapshotter 提供当前汇率状态与已解析的有效市场汇率。
type RateSnapshotter interface {
	Snapshot() (exchangeratecontract.State, exchangeratedomain.Rate, error)
}

// AdminHandler 处理 /admin/pricing/preview。
type AdminHandler struct {
	products ProductGetter
	settings SettingsService
	rates    RateSnapshotter
}

func NewAdminHandler(products ProductGetter, settings SettingsService, rates RateSnapshotter) *AdminHandler {
	if products == nil || settings == nil || rates == nil {
		panic("pricing admin handler: nil dependency")
	}
	return &AdminHandler{products: products, settings: settings, rates: rates}
}

type previewRequest struct {
	ProductID uint `json:"product_id" binding:"required"`
	Quantity  int  `json:"quantity"`
}

// Preview 返回单个商品在当前配置下的 Profit Guard 核算明细（只读，不拒单）。
func (h *AdminHandler) Preview(c *gin.Context) {
	var req previewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	quantity := req.Quantity
	if quantity <= 0 {
		quantity = 1
	}

	product, err := h.products.GetAdminByID(strconv.FormatUint(uint64(req.ProductID), 10))
	if err != nil {
		if errors.Is(err, productcontract.ErrNotFound) {
			ginutil.RespondError(c, response.CodeNotFound, "error.product_not_found", nil)
			return
		}
		ginutil.RespondError(c, response.CodeInternal, "error.product_fetch_failed", err)
		return
	}

	pg, err := h.settings.GetProfitGuardSetting()
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.settings_fetch_failed", err)
		return
	}
	affiliate, err := h.settings.GetAffiliateSetting()
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.settings_fetch_failed", err)
		return
	}

	state, marketRate, snapErr := h.rates.Snapshot()

	qty := decimal.NewFromInt(int64(quantity))
	revenueCNY := product.PriceAmount.Decimal.Mul(qty).Round(2)
	costCNY := product.CostPriceAmount.Decimal.Mul(qty).Round(2)
	bufferPct := decimal.NewFromFloat(pg.RateSafetyBufferPercent)

	lines := []profitguard.OrderLine{
		{
			CostPriceCNY: product.CostPriceAmount.Decimal,
			Quantity:     quantity,
			CostExempt:   product.IsCostExempt,
		},
	}

	warnings := []string{}
	resp := gin.H{
		"product_id":                  product.ID,
		"product_title":               product.TitleJSON,
		"quantity":                     quantity,
		"sale_price_cny":               revenueCNY.String(),
		"cost_price_cny":               costCNY.String(),
		"is_cost_exempt":               product.IsCostExempt,
		"rate_safety_buffer_percent":   pg.RateSafetyBufferPercent,
		"max_auto_rate_age_minutes":    state.MaxAutoRateAgeMinutes,
		"manual_fallback_rate":         state.ManualRate.String(),
		"manual_rate_updated_at":       fmtTime(state.ManualRateUpdatedAt),
	}

	// 成本缺失（未配置成本且未豁免）。
	if !product.IsCostExempt && product.CostPriceAmount.Decimal.LessThanOrEqual(decimal.Zero) {
		warnings = append(warnings, "COST_MISSING")
	}
	// 成本 >= 售价（倒挂）。
	if product.CostPriceAmount.Decimal.IsPositive() && product.CostPriceAmount.Decimal.GreaterThanOrEqual(product.PriceAmount.Decimal) {
		warnings = append(warnings, "UNPROFITABLE_PRICE")
	}

	if snapErr != nil || marketRate.Rate.LessThanOrEqual(decimal.Zero) {
		warnings = append(warnings, "FX_UNAVAILABLE")
		resp["fx_available"] = false
		resp["market_rate"] = ""
		resp["effective_rate"] = ""
		resp["effective_source"] = ""
		resp["theoretical_usdt"] = ""
		resp["final_usdt"] = ""
		resp["max_affiliate_cost_cny"] = "0"
		resp["fee_cost_cny"] = "0"
		resp["expected_profit_cny"] = ""
		resp["required_profit_cny"] = decimal.NewFromFloat(pg.MinimumProfitAmountCNY).String()
		resp["guard_result"] = "BLOCKED"
		resp["guard_reason"] = "exchange_rate_unavailable"
		resp["warnings"] = warnings
		response.Success(c, resp)
		return
	}

	effectiveRate, err := marketRate.EffectiveRate(bufferPct)
	if err != nil {
		effectiveRate = marketRate.Rate
	}
	theoreticalUSDT := revenueCNY.Div(marketRate.Rate).Round(2)
	finalUSDT := revenueCNY.Div(effectiveRate).RoundCeil(2)

	in := profitguard.Input{
		RevenueCNY:        revenueCNY,
		Lines:             lines,
		MarketRate:        marketRate.Rate,
		BufferPct:         bufferPct,
		AffiliateEnabled:  affiliate.Enabled,
		AffiliateSetting:  affiliate,
		FeeCostCNY:        decimal.Zero, // KNOWN_BOUNDARY：订单级手续费归因 V1 未接入，恒为 0
		MinProfitAmountCNY: decimal.NewFromFloat(pg.MinimumProfitAmountCNY),
		MinProfitRatePct:   decimal.NewFromFloat(pg.MinimumProfitRatePercent),
	}
	result := profitguard.Evaluate(in, finalUSDT)
	guardErr := result.Guard(pg.RequireCostPrice, lines)

	guardResult := "PASS"
	guardReason := ""
	if guardErr != nil {
		guardResult = "BLOCKED"
		guardReason = guardErr.Error()
		if errors.Is(guardErr, profitguard.ErrProductCostNotConfigured) {
			warnings = append(warnings, "COST_ENFORCED_MISSING")
		}
		if errors.Is(guardErr, profitguard.ErrUnprofitableOrder) {
			warnings = append(warnings, "PROFIT_BELOW_REQUIRED")
		}
	}

	// AUTO 汇率陈旧且无 MANUAL 兜底提示。
	if marketRate.Source == exchangeratedomain.SourceAuto && !state.AutoFetchedAt.IsZero() {
		maxAge := state.MaxAutoRateAgeMinutes
		if maxAge > 0 && int(time.Since(state.AutoFetchedAt).Minutes()) > maxAge {
			warnings = append(warnings, "RATE_STALE")
		}
	}

	resp["fx_available"] = true
	resp["market_rate"] = marketRate.Rate.String()
	resp["effective_rate"] = effectiveRate.String()
	resp["effective_source"] = marketRate.Source
	resp["theoretical_usdt"] = theoreticalUSDT.String()
	resp["final_usdt"] = finalUSDT.String()
	resp["max_affiliate_cost_cny"] = result.AffiliateCostCNY.String()
	resp["fee_cost_cny"] = result.FeeCostCNY.String()
	resp["expected_profit_cny"] = result.ExpectedProfitCNY.String()
	resp["required_profit_cny"] = result.RequiredProfitCNY.String()
	resp["guard_result"] = guardResult
	resp["guard_reason"] = guardReason
	resp["warnings"] = warnings
	response.Success(c, resp)
}

func fmtTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}
