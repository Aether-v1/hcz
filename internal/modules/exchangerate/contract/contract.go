package exchangerate

import (
	"context"
	"time"

	exchangeratedomain "github.com/Aether-v1/hcz/internal/modules/exchangerate/domain"
	"github.com/shopspring/decimal"
)

// Provider 是外部汇率源端口（如 CoinGecko）。业务服务只依赖此端口，不直接依赖具体源。
type Provider interface {
	// Name 返回源标识（coingecko 等）。
	Name() string
	// Fetch 返回 1 USDT 折合多少 siteCurrency。apiKey 为可选的源认证 key。
	Fetch(ctx context.Context, siteCurrency string, apiKey string) (rate decimal.Decimal, fetchedAt time.Time, err error)
}

// State 是持久化到 settings KV 的汇率状态与配置。
type State struct {
	Currency      string
	AutoRate      decimal.Decimal
	AutoFetchedAt time.Time
	ManualRate    decimal.Decimal // <=0 表示未设置手动兜底
	// ManualRateUpdatedAt 是 Admin 上次写入手动兜底汇率的时间。
	// P4 修复：Resolve 的 MANUAL 分支必须回显这个时间，禁止填 now。
	ManualRateUpdatedAt time.Time
	LastSuccessAt       time.Time
	LastError           string

	// 配置（Admin 后台管理，不写死源码/环境变量）
	Provider           string // 第一版固定 coingecko
	APIKey             string // CoinGecko key，只存后端，前端只返回脱敏
	AutoEnabled        bool
	RefreshIntervalMin int // 刷新间隔（分钟）

	// P4: Profit Guard FX 安全配置。
	// RateSafetyBufferPercent = 汇率安全缓冲（%），effective_rate = market × (1 - buffer/100)。
	RateSafetyBufferPercent float64
	// MaxAutoRateAgeMinutes = AUTO 汇率允许的最大新鲜度（分钟），超过则视为 stale 并回退 MANUAL。
	MaxAutoRateAgeMinutes int
}

// Store 持久化汇率状态（settings KV 的最小端口）。
type Store interface {
	GetState() (State, error)
	SaveState(State) error
}

// Resolver 是订单侧依赖的最小端口：下单时拿一个有效汇率。
type Resolver interface {
	// Resolve 返回当前有效汇率；无有效汇率返回 domain.ErrRateUnavailable（fail-closed）。
	Resolve(ctx context.Context) (exchangeratedomain.Rate, error)
}
