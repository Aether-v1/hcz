package container

import (
	"context"
	"time"

	exchangerateapp "github.com/Aether-v1/hcz/internal/modules/exchangerate/application"
	exchangeratedomain "github.com/Aether-v1/hcz/internal/modules/exchangerate/domain"
)

// exchangerateStaleDuration 是自动汇率允许的最大新鲜度；超过后自动视为失效并回退手动兜底。
// P4：将改为后台 max_auto_rate_age 配置（默认保持 10min）。
const exchangerateStaleDuration = 10 * time.Minute

// exchangerateResolverAdapter 把 exchangerate 用例适配为订单域需要的最小端口，
// 避免 order application 直接依赖具体汇率模块实现。
type exchangerateResolverAdapter struct {
	svc *exchangerateapp.Service
}

func (a exchangerateResolverAdapter) Resolve(ctx context.Context) (exchangeratedomain.Rate, error) {
	if a.svc == nil {
		return exchangeratedomain.Rate{}, exchangeratedomain.ErrRateUnavailable
	}
	return a.svc.Resolve(ctx)
}
