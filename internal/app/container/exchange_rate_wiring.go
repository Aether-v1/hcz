package container

import (
	"context"
	"time"

	exchangerateapp "github.com/Aether-v1/hcz/internal/modules/exchangerate/application"
	exchangeratedomain "github.com/Aether-v1/hcz/internal/modules/exchangerate/domain"
	"github.com/shopspring/decimal"
)

// exchangerateStaleDuration 是自动汇率允许的最大新鲜度；超过后自动视为失效并回退手动兜底。
const exchangerateStaleDuration = 10 * time.Minute

// exchangerateResolverAdapter 把 exchangerate 用例适配为订单域需要的最小端口，
// 避免 order application 直接依赖具体汇率模块类型。
type exchangerateResolverAdapter struct {
	svc *exchangerateapp.Service
}

func (a exchangerateResolverAdapter) Resolve(ctx context.Context) (decimal.Decimal, string, time.Time, error) {
	if a.svc == nil {
		return decimal.Zero, "", time.Time{}, exchangeratedomain.ErrRateUnavailable
	}
	rate, err := a.svc.Resolve(ctx)
	if err != nil {
		return decimal.Zero, "", time.Time{}, err
	}
	return rate.Rate, rate.Source, rate.FetchedAt, nil
}
