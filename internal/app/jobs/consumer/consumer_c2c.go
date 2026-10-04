package consumer

import (
	"context"
	"encoding/json"

	"github.com/Aether-v1/hcz/internal/logger"
	"github.com/Aether-v1/hcz/internal/queue"

	"github.com/hibiken/asynq"
)

// handleC2CTradeExpire 处理 C2C 交易支付超时：待付款单解冻卖家并恢复挂单余量。
// 幂等：已付款/完成/取消/争议的交易直接跳过，绝不动资金。
func (c *Consumer) handleC2CTradeExpire(_ context.Context, task *asynq.Task) error {
	if c == nil || task == nil {
		logger.Debugw("worker_c2c_trade_expire_skip_nil", "consumer_nil", c == nil, "task_nil", task == nil)
		return nil
	}
	var payload queue.C2CTradeExpirePayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		logger.Warnw("worker_c2c_trade_expire_unmarshal_failed", "error", err)
		return err
	}
	if payload.TradeID == 0 {
		logger.Debugw("worker_c2c_trade_expire_skip_invalid_payload", "trade_id", payload.TradeID)
		return nil
	}
	if c.C2CService == nil {
		logger.Warnw("worker_c2c_trade_expire_skip_service_nil", "trade_id", payload.TradeID)
		return nil
	}
	if err := c.C2CService.ExpireTrade(payload.TradeID); err != nil {
		logger.Warnw("worker_c2c_trade_expire_failed", "trade_id", payload.TradeID, "error", err)
		return err
	}
	logger.Debugw("worker_c2c_trade_expire_done", "trade_id", payload.TradeID)
	return nil
}
