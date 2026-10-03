package settingsstore

import (
	"time"

	exchangerate "github.com/Aether-v1/hcz/internal/modules/exchangerate/contract"
	"github.com/Aether-v1/hcz/internal/shared/jsonmap"
	"github.com/shopspring/decimal"
)

const key = "global_exchange_rate"

// KV 是 settings 存储的最小端口，避免 exchangerate 直接依赖 settings 内部实现。
type KV interface {
	GetByKey(key string) (jsonmap.JSON, bool, error)
	Upsert(key string, value jsonmap.JSON) (jsonmap.JSON, error)
}

// Store 把汇率状态持久化到 settings KV。
type Store struct {
	kv KV
}

func New(kv KV) *Store { return &Store{kv: kv} }

func (s *Store) GetState() (exchangerate.State, error) {
	var state exchangerate.State
	if s == nil || s.kv == nil {
		return state, nil
	}
	raw, ok, err := s.kv.GetByKey(key)
	if err != nil {
		return state, err
	}
	if !ok {
		return state, nil
	}
	state.Currency = str(raw["currency"])
	state.AutoRate = dec(raw["auto_rate"])
	state.ManualRate = dec(raw["manual_rate"])
	state.LastError = str(raw["last_error"])
	state.Provider = str(raw["provider"])
	state.APIKey = str(raw["api_key"])
	state.AutoEnabled = parseBool(raw["auto_enabled"])
	state.RefreshIntervalMin = parseInt(raw["refresh_interval_min"])
	state.AutoFetchedAt = ts(raw["auto_fetched_at"])
	state.LastSuccessAt = ts(raw["last_success_at"])
	return state, nil
}

func (s *Store) SaveState(state exchangerate.State) error {
	if s == nil || s.kv == nil {
		return nil
	}
	value := jsonmap.JSON{
		"currency":            state.Currency,
		"auto_rate":           state.AutoRate.String(),
		"manual_rate":         state.ManualRate.String(),
		"last_error":          state.LastError,
		"provider":             state.Provider,
		"api_key":             state.APIKey,
		"auto_enabled":        state.AutoEnabled,
		"refresh_interval_min": state.RefreshIntervalMin,
		"auto_fetched_at":     state.AutoFetchedAt.Unix(),
		"last_success_at":     state.LastSuccessAt.Unix(),
	}
	_, err := s.kv.Upsert(key, value)
	return err
}

func str(v interface{}) string {
	s, _ := v.(string)
	return s
}

func dec(v interface{}) decimal.Decimal {
	switch t := v.(type) {
	case string:
		if d, err := decimal.NewFromString(t); err == nil {
			return d
		}
	case float64:
		return decimal.NewFromFloat(t)
	}
	return decimal.Zero
}

func ts(v interface{}) time.Time {
	if n, ok := v.(float64); ok && n > 0 {
		return time.Unix(int64(n), 0)
	}
	return time.Time{}
}

func parseBool(v interface{}) bool {
	b, _ := v.(bool)
	return b
}

func parseInt(v interface{}) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	}
	return 0
}
