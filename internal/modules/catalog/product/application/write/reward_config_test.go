package productwrite

import (
	"errors"
	"testing"

	productcontract "github.com/Aether-v1/hcz/internal/modules/catalog/product/contract"
	pointscontract "github.com/Aether-v1/hcz/internal/modules/points/contract"
)

func boolPtr(v bool) *bool { return &v }

// TestNormalizeRewardConfig 锁定商品固定积分奖励的唯一口径（P4 §10/§31）：
// 关闭必须清零，开启必须落在 (0, MaxPointsAmount]。
func TestNormalizeRewardConfig(t *testing.T) {
	cases := []struct {
		name        string
		enabled     *bool
		points      int64
		wantEnabled bool
		wantPoints  int64
		wantErr     error
	}{
		{"nil enabled keeps off", nil, 999, false, 0, nil},
		{"explicit off clears points", boolPtr(false), 999, false, 0, nil},
		{"zero points rejected", boolPtr(true), 0, false, 0, productcontract.ErrRewardPointsInvalid},
		{"negative points rejected", boolPtr(true), -5, false, 0, productcontract.ErrRewardPointsInvalid},
		{"over hard cap rejected", boolPtr(true), pointscontract.MaxPointsAmount + 1, false, 0, productcontract.ErrRewardPointsInvalid},
		{"typical typo magnitude rejected", boolPtr(true), 100000000000000, false, 0, productcontract.ErrRewardPointsInvalid},
		{"upper bound accepted", boolPtr(true), pointscontract.MaxPointsAmount, true, pointscontract.MaxPointsAmount, nil},
		{"valid points accepted", boolPtr(true), 50, true, 50, nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			enabled, points, err := normalizeRewardConfig(tc.enabled, tc.points)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
			if enabled != tc.wantEnabled {
				t.Errorf("enabled = %v, want %v", enabled, tc.wantEnabled)
			}
			if points != tc.wantPoints {
				t.Errorf("points = %d, want %d", points, tc.wantPoints)
			}
		})
	}
}
