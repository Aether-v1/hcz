package settingspoints

import (
	"testing"
)

func TestDefaultCheckinConfig(t *testing.T) {
	cfg := DefaultCheckinConfig()
	if !cfg.Enabled {
		t.Fatal("default must be enabled")
	}
	if len(cfg.Rewards) != 7 {
		t.Fatalf("default rewards must have 7 entries, got %d", len(cfg.Rewards))
	}
	for i, want := range []int64{1, 2, 3, 4, 5, 6, 10} {
		if cfg.Rewards[i] != want {
			t.Fatalf("default rewards[%d]=%d want %d", i, cfg.Rewards[i], want)
		}
	}
}

func TestNormalizeCheckinConfigFallsBackToDefaultOnCorruption(t *testing.T) {
	defaults := DefaultCheckinConfig()

	cases := []CheckinConfig{
		{Rewards: nil},
		{Rewards: []int64{1, 2}},                       // 长度不对
		{Rewards: []int64{1, -2, 3, 4, 5, 6, 10}},      // 负数
		{Rewards: []int64{1, 2, 3, 4, 5, 6, 999999999999999999}}, // 超上限
	}
	for i, cfg := range cases {
		got := NormalizeCheckinConfig(cfg)
		if got.Enabled != defaults.Enabled || len(got.Rewards) != 7 {
			t.Fatalf("case %d: corruption must fall back to default, got %+v", i, got)
		}
		if got.Rewards[0] != 1 || got.Rewards[6] != 10 {
			t.Fatalf("case %d: default rewards not restored: %+v", i, got.Rewards)
		}
	}
}

func TestValidateCheckinConfigRejectsInvalid(t *testing.T) {
	valid := CheckinConfig{Enabled: true, Rewards: []int64{1, 2, 3, 4, 5, 6, 10}}
	if err := ValidateCheckinConfig(valid); err != nil {
		t.Fatalf("valid config must pass: %v", err)
	}
	zeroReward := CheckinConfig{Enabled: true, Rewards: []int64{1, 2, 0, 4, 5, 6, 10}}
	if err := ValidateCheckinConfig(zeroReward); err != nil {
		t.Fatalf("zero reward day is legal: %v", err)
	}
	for _, invalid := range []CheckinConfig{
		{Enabled: true, Rewards: []int64{1, 2, 3}},
		{Enabled: true, Rewards: []int64{1, -1, 3, 4, 5, 6, 10}},
		{Enabled: true, Rewards: []int64{1, 2, 3, 4, 5, 6, 1000001}},
		{Enabled: true, Rewards: []int64{1, 2, 3, 4, 5, 6, 1000000000000000000}},
	} {
		if err := ValidateCheckinConfig(invalid); err == nil {
			t.Fatalf("invalid config must be rejected: %+v", invalid)
		}
	}
}

func TestDecodeCheckinConfigRoundTrip(t *testing.T) {
	defaults := DefaultCheckinConfig()
	cfg := CheckinConfig{Enabled: true, Rewards: []int64{1, 2, 3, 0, 5, 6, 10}}
	encoded := EncodeCheckinConfig(cfg)
	decoded := DecodeCheckinConfig(encoded, defaults)
	if len(decoded.Rewards) != 7 || decoded.Rewards[3] != 0 {
		t.Fatalf("round trip failed: %+v", decoded)
	}
	// 损坏 JSON 回退默认
	corrupt := DecodeCheckinConfig(map[string]interface{}{"enabled": true, "rewards": "not-an-array"}, defaults)
	if len(corrupt.Rewards) != 7 || corrupt.Rewards[6] != 10 {
		t.Fatalf("corrupt raw must fall back to default: %+v", corrupt)
	}
}
