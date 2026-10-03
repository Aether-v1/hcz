package rediscache

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"

	exchangerate "github.com/Aether-v1/hcz/internal/modules/exchangerate/contract"
)

type countingStore struct {
	state    exchangerate.State
	getCalls int
	saved    *exchangerate.State
}

func (m *countingStore) GetState() (exchangerate.State, error) {
	m.getCalls++
	return m.state, nil
}

func (m *countingStore) SaveState(s exchangerate.State) error {
	cp := s
	m.saved = &cp
	return nil
}

func baseState() exchangerate.State {
	return exchangerate.State{
		Currency:     "CNY",
		AutoEnabled:  true,
		AutoRate:     decimal.RequireFromString("7.18000000"),
		AutoFetchedAt: time.Now(),
		LastSuccessAt: time.Now(),
		ManualRate:   decimal.RequireFromString("7.2"),
	}
}

// Redis 不可用（单测环境）→ 必须回落 settings 真源，且绝不 1:1。
func TestGetState_RedisDown_FallsBackToSettings(t *testing.T) {
	inner := &countingStore{state: baseState()}
	c := New(inner)

	got, err := c.GetState()
	if err != nil {
		t.Fatalf("down path must not error: %v", err)
	}
	if inner.getCalls != 1 {
		t.Fatalf("settings must be read-through on redis down, getCalls=%d", inner.getCalls)
	}
	if !got.AutoRate.Equal(decimal.RequireFromString("7.18000000")) {
		t.Fatalf("must return real settings rate, got %v", got.AutoRate)
	}
}

// Redis 故障时绝不返回 1:1：拿到的必须是 settings 里的真实汇率。
func TestGetState_RedisDown_NoOneToOne(t *testing.T) {
	inner := &countingStore{state: baseState()}
	c := New(inner)
	got, _ := c.GetState()
	if got.AutoRate.Equal(decimal.NewFromInt(1)) {
		t.Fatal("redis down must never fall back to 1:1")
	}
}

// SaveState 必须先持久化真源（inner），缓存失败不影响真源。
func TestSaveState_PersistsSettingsFirst(t *testing.T) {
	inner := &countingStore{state: baseState()}
	c := New(inner)

	newState := baseState()
	newState.AutoRate = decimal.RequireFromString("7.19000000")
	if err := c.SaveState(newState); err != nil {
		t.Fatalf("save: %v", err)
	}
	if inner.saved == nil || !inner.saved.AutoRate.Equal(decimal.RequireFromString("7.19000000")) {
		t.Fatal("settings truth must be updated regardless of cache")
	}
}
