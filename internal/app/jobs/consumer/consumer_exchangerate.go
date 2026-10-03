package consumer

import (
	"context"

	"github.com/Aether-v1/hcz/internal/logger"
	"github.com/Aether-v1/hcz/internal/queue"

	"github.com/hibiken/asynq"
)

// handleExchangeRateRefresh 周期刷新全局汇率（P0-2）。失败仅记录，不抛 1:1 兜底。
func (c *Consumer) handleExchangeRateRefresh(ctx context.Context, t *asynq.Task) error {
	if c == nil || c.ExchangeRateService == nil {
		return nil
	}
	if err := c.ExchangeRateService.Refresh(ctx); err != nil {
		logger.Warnw("exchange_rate_refresh_failed", "error", err)
		return nil // 不重试堆积；下一个周期再拉
	}
	logger.Infow("exchange_rate_refresh_ok")
	return nil
}

// RegisterExchangeRateRefresh 注册 mux 路由。
func (c *Consumer) RegisterExchangeRateRefresh(mux *asynq.ServeMux) {
	if mux == nil {
		return
	}
	mux.HandleFunc(queue.TaskExchangeRateRefresh, withPanicRecovery(queue.TaskExchangeRateRefresh, c.handleExchangeRateRefresh))
}
