package application

import (
	"time"

	"github.com/Aether-v1/hcz/internal/logger"
	pointscontract "github.com/Aether-v1/hcz/internal/modules/points/contract"
)

// GetStats 返回后台积分运营看板的基础指标。
//
// 口径（P4 决策）：全部直接聚合自事实表
//
//	points_ledger（入账/出账/订单奖励、余额总量、负余额账户数）
//	user_checkins（今日签到人数）
//	points_exchange_orders（今日兑换单数、待处理/处理中数量）
//
// 不新增"实时累计总表"作为第二事实源；"今日"统一使用签到域的
// Asia/Shanghai 业务日边界（由装配层注入 BusinessDayRange，积分不自建第二套时区口径）。
func (s *Service) GetStats(now time.Time) (*pointscontract.StatsResult, error) {
	if s.stats.Ledger == nil || s.stats.Checkins == nil || s.stats.Exchanges == nil || s.stats.BusinessDayRange == nil {
		return nil, pointscontract.ErrStatsSourceRequired
	}

	from, to := s.stats.BusinessDayRange(now)

	agg, err := s.stats.Ledger.AggregateLedgerRange(from, to)
	if err != nil {
		logger.Warnw("points_stats_ledger_failed", "error", err.Error())
		return nil, err
	}
	totalBalance, negativeAccounts, err := s.stats.Ledger.SumBalances()
	if err != nil {
		logger.Warnw("points_stats_balance_failed", "error", err.Error())
		return nil, err
	}
	checkinUsers, err := s.stats.Checkins.CountCheckinUsers(from, to)
	if err != nil {
		logger.Warnw("points_stats_checkin_failed", "error", err.Error())
		return nil, err
	}
	exchangesToday, err := s.stats.Exchanges.CountExchangeOrdersCreated(from, to)
	if err != nil {
		logger.Warnw("points_stats_exchange_count_failed", "error", err.Error())
		return nil, err
	}
	pending, err := s.stats.Exchanges.CountPendingExchangeOrders()
	if err != nil {
		logger.Warnw("points_stats_exchange_pending_failed", "error", err.Error())
		return nil, err
	}
	processing, err := s.stats.Exchanges.CountProcessingExchangeOrders()
	if err != nil {
		logger.Warnw("points_stats_exchange_processing_failed", "error", err.Error())
		return nil, err
	}

	return &pointscontract.StatsResult{
		TotalBalance:         totalBalance,
		TodayGranted:         agg.Granted,
		TodaySpent:           agg.Spent,
		TodayOrderReward:     agg.OrderReward,
		TodayCheckinUsers:    checkinUsers,
		TodayExchanges:       exchangesToday,
		PendingExchanges:     pending,
		ProcessingExchanges:  processing,
		NegativeBalanceUsers: negativeAccounts,
		TodayStart:           from.Format(time.RFC3339),
	}, nil
}
