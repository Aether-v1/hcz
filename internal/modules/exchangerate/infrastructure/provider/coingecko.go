package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

const defaultCoinGeckoBase = "https://api.coingecko.com/api/v3"

// CoinGecko 是 Global Rate 的第一个外部汇率源实现。只由后端调用，前端浏览器不直连。
// 查询 tether(USDT) 相对站点计价币种的价格，返回 1 USDT = R SiteCurrency。
type CoinGecko struct {
	baseURL string
	client  *http.Client
}

// NewCoinGecko 构造。baseURL 传空用官方默认；测试可注入 mock 地址。
func NewCoinGecko(baseURL string) *CoinGecko {
	if baseURL == "" {
		baseURL = defaultCoinGeckoBase
	}
	return &CoinGecko{
		baseURL: baseURL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *CoinGecko) Name() string { return "coingecko" }

func (c *CoinGecko) Fetch(ctx context.Context, siteCurrency string, apiKey string) (decimal.Decimal, time.Time, error) {
	if siteCurrency == "" {
		return decimal.Zero, time.Time{}, fmt.Errorf("empty site currency")
	}
	// 当站点本身就是 USDT，无需换算，汇率固定为 1。
	if siteCurrency == "USDT" {
		return decimal.NewFromInt(1), time.Now(), nil
	}

	url := fmt.Sprintf("%s/simple/price?ids=tether&vs_currencies=%s", c.baseURL, siteCurrency)
	if key := strings.TrimSpace(apiKey); key != "" {
		url += "&x_cg_demo_api_key=" + key
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return decimal.Zero, time.Time{}, err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return decimal.Zero, time.Time{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return decimal.Zero, time.Time{}, fmt.Errorf("coingecko status %d", resp.StatusCode)
	}
	var parsed struct {
		Tether map[string]decimal.Decimal `json:"tether"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return decimal.Zero, time.Time{}, err
	}
	rate, ok := parsed.Tether[siteCurrency]
	if !ok {
		return decimal.Zero, time.Time{}, fmt.Errorf("coingecko missing %s", siteCurrency)
	}
	if rate.LessThanOrEqual(decimal.Zero) {
		return decimal.Zero, time.Time{}, fmt.Errorf("coingecko non-positive rate")
	}
	return rate, time.Now(), nil
}
