package application

import (
	"context"
	"strings"
	"time"

	exchangerate "github.com/Aether-v1/hcz/internal/modules/exchangerate/contract"
	exchangeratedomain "github.com/Aether-v1/hcz/internal/modules/exchangerate/domain"
	"github.com/shopspring/decimal"
)

// Service 是全局汇率用例：负责刷新（周期任务/手动）与下单时解析。
//
// 解析优先级（fail-closed）：
//  1. 自动汇率有效且未过期 → 用 AUTO
//  2. 自动不可用/过期 → 若设置了手动兜底 → 用 MANUAL
//  3. 都不可用 → 返回 ErrRateUnavailable，拒绝下单
type Service struct {
	provider   exchangerate.Provider
	store      exchangerate.Store
	currency   string
	staleness  time.Duration
	now        func() time.Time
}

// NewService 构造。currency 为站点计价币种；staleness 为自动汇率允许的最大新鲜度。
func NewService(provider exchangerate.Provider, store exchangerate.Store, currency string, staleness time.Duration) *Service {
	if staleness <= 0 {
		staleness = 24 * time.Hour
	}
	return &Service{
		provider:  provider,
		store:     store,
		currency:  currency,
		staleness: staleness,
		now:       time.Now,
	}
}

// Resolve 实现 exchangerate.Resolver。
func (s *Service) Resolve(ctx context.Context) (exchangeratedomain.Rate, error) {
	if s == nil || s.store == nil {
		return exchangeratedomain.Rate{}, exchangeratedomain.ErrRateUnavailable
	}
	state, err := s.store.GetState()
	if err != nil {
		return exchangeratedomain.Rate{}, exchangeratedomain.ErrRateUnavailable
	}
	now := s.now()

	// 1. 自动汇率有效且未过期
	if state.AutoRate.GreaterThan(decimal.Zero) && !state.AutoFetchedAt.IsZero() &&
		now.Sub(state.AutoFetchedAt) <= s.staleness {
		return exchangeratedomain.Rate{
			Rate: state.AutoRate, Currency: state.Currency,
			Source: exchangeratedomain.SourceAuto, FetchedAt: state.AutoFetchedAt,
		}, nil
	}
	// 2. 手动兜底
	if state.ManualRate.GreaterThan(decimal.Zero) {
		return exchangeratedomain.Rate{
			Rate: state.ManualRate, Currency: state.Currency,
			Source: exchangeratedomain.SourceManual, FetchedAt: now,
		}, nil
	}
	// 3. fail-closed
	return exchangeratedomain.Rate{}, exchangeratedomain.ErrRateUnavailable
}

// Refresh 由周期任务/手动触发，从 provider 拉取最新自动汇率并持久化。
// provider 失败时只记录错误，不抛 1:1 兜底；保留上一次成功自动汇率供 Resolve 使用。
func (s *Service) Refresh(ctx context.Context) error {
	if s == nil {
		return exchangeratedomain.ErrRateUnavailable
	}
	state, err := s.store.GetState()
	if err != nil {
		return err
	}
	if state.Currency == "" {
		state.Currency = s.currency
	}
	// 自动汇率被 Admin 关闭时，不拉源；不覆盖已有合法自动率。
	if !state.AutoEnabled {
		return nil
	}

	if s.provider == nil {
		state.LastError = "no_provider"
		_ = s.store.SaveState(state)
		return exchangeratedomain.ErrRateUnavailable
	}
	rate, fetchedAt, err := s.provider.Fetch(ctx, state.Currency, state.APIKey)
	if err != nil {
		// 不把 key 写进错误
		state.LastError = "provider_fetch_failed"
		_ = s.store.SaveState(state)
		return err
	}
	if rate.LessThanOrEqual(decimal.Zero) {
		state.LastError = "non_positive_rate"
		_ = s.store.SaveState(state)
		return exchangeratedomain.ErrRateUnavailable
	}
	state.AutoRate = rate
	state.AutoFetchedAt = fetchedAt
	state.LastSuccessAt = fetchedAt
	state.LastError = ""
	return s.store.SaveState(state)
}

// SetManual 设置/清除手动兜底汇率（rate<=0 表示清除）。
func (s *Service) SetManual(rate decimal.Decimal) error {
	if s == nil || s.store == nil {
		return exchangeratedomain.ErrRateUnavailable
	}
	state, err := s.store.GetState()
	if err != nil {
		return err
	}
	if state.Currency == "" {
		state.Currency = s.currency
	}
	state.ManualRate = rate
	return s.store.SaveState(state)
}

// UpdateConfig 由 Admin 更新配置。apiKey 传非空才更新；空串保留现有 key（避免误清空）。
func (s *Service) UpdateConfig(apiKey string, autoEnabled bool, refreshIntervalMin int) error {
	if s == nil || s.store == nil {
		return exchangeratedomain.ErrRateUnavailable
	}
	state, err := s.store.GetState()
	if err != nil {
		return err
	}
	if state.Currency == "" {
		state.Currency = s.currency
	}
	state.Provider = "coingecko"
	if key := strings.TrimSpace(apiKey); key != "" {
		state.APIKey = key
	}
	state.AutoEnabled = autoEnabled
	if refreshIntervalMin > 0 {
		state.RefreshIntervalMin = refreshIntervalMin
	}
	return s.store.SaveState(state)
}

// Snapshot 返回当前状态（供 Admin 展示）。
func (s *Service) Snapshot() (exchangerate.State, exchangeratedomain.Rate, error) {
	if s == nil || s.store == nil {
		return exchangerate.State{}, exchangeratedomain.Rate{}, exchangeratedomain.ErrRateUnavailable
	}
	state, err := s.store.GetState()
	if err != nil {
		return state, exchangeratedomain.Rate{}, err
	}
	effective, resolveErr := s.Resolve(context.Background())
	return state, effective, resolveErr
}
