package application

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	orderdomain "github.com/Aether-v1/hcz/internal/modules/order/domain"

	productdomain "github.com/Aether-v1/hcz/internal/modules/catalog/product/domain"

	affiliatecontract "github.com/Aether-v1/hcz/internal/modules/affiliate/contract"
	affiliatedomain "github.com/Aether-v1/hcz/internal/modules/affiliate/domain"
	settingsintegration "github.com/Aether-v1/hcz/internal/modules/settings/schema/integration"
	usernotificationcontract "github.com/Aether-v1/hcz/internal/modules/usernotification/contract"
	usernotificationdomain "github.com/Aether-v1/hcz/internal/modules/usernotification/domain"

	"github.com/Aether-v1/hcz/internal/constants"
	"github.com/Aether-v1/hcz/internal/logger"
	"github.com/Aether-v1/hcz/internal/shared/jsonmap"
	"github.com/Aether-v1/hcz/internal/shared/money"

	"github.com/shopspring/decimal"
)

// HandleOrderPaid 已退休：Phase 4 起佣金在订单进入 completed 时生成（见 HandleOrderCompleted）。
// 保留方法签名仅为兼容旧调用方，新逻辑直接返回 nil，不再在 paid 阶段生成佣金。
func (s *Service) HandleOrderPaid(orderID uint) error {
	return nil
}

// HandleOrderCompleted 订单进入 completed 时，沿 users.inviter_id 向上递归最多 maxLevel 级生成佣金。
// 真源 = users.inviter_id；中间用户无 affiliate profile（或未激活）时跳过该层但层级不压缩。
// 幂等：同一 (order_id, beneficiary_user_id, level, commission_type='order') 仅生成一次。
func (s *Service) HandleOrderCompleted(orderID uint) error {
	if orderID == 0 || s.repo == nil || s.orderRepo == nil || s.userRepo == nil {
		return nil
	}
	setting, err := s.settings.GetAffiliateSetting()
	if err != nil {
		return err
	}
	if !setting.Enabled {
		return nil
	}

	order, err := s.orderRepo.GetByID(orderID)
	if err != nil {
		return err
	}
	if order == nil {
		return nil
	}
	// 佣金基数固定为 USDT 钱包实付额，禁止使用 total_amount 或汇率换算。
	if order.WalletPaidAmount.Decimal.LessThanOrEqual(decimal.Zero) {
		return nil
	}
	if order.UserID == 0 {
		return nil
	}

	maxLevel := setting.MaxLevel
	if maxLevel < 1 {
		maxLevel = 1
	}
	if maxLevel > affiliateMaxLevel {
		maxLevel = affiliateMaxLevel
	}

	// 向上构建邀请链：chain[i] 对应第 i+1 层的收益人用户ID。
	currentUserID := order.UserID
	visited := map[uint]bool{order.UserID: true}
	chain := make([]uint, 0, maxLevel)
	for level := 1; level <= maxLevel; level++ {
		current, err := s.userRepo.GetByID(currentUserID)
		if err != nil {
			return err
		}
		if current == nil {
			break
		}
		if current.InviterID == nil {
			break
		}
		inviterID := *current.InviterID
		if inviterID == 0 || visited[inviterID] {
			if inviterID != 0 {
				logger.Warnw("affiliate_invite_chain_cycle",
					"user_id", currentUserID,
					"inviter_id", inviterID,
					"order_id", order.ID,
				)
			}
			break
		}
		visited[inviterID] = true
		chain = append(chain, inviterID)
		currentUserID = inviterID
	}
	if len(chain) == 0 {
		return nil
	}

	// 批量资格检查：一次取出链上所有用户的 affiliate profile。
	profiles, err := s.repo.GetProfilesByUserIDs(chain)
	if err != nil {
		return err
	}
	profileByUser := make(map[uint]affiliatedomain.Profile, len(profiles))
	for _, p := range profiles {
		profileByUser[p.UserID] = p
	}

	baseAmount := order.WalletPaidAmount.Decimal.Round(2)
	now := time.Now()
	commissions := make([]*affiliatedomain.Commission, 0, len(chain))
	for idx, beneficiaryUserID := range chain {
		level := idx + 1
		profile, ok := profileByUser[beneficiaryUserID]
		if !ok {
			continue // 无 profile：跳过该层，但 chain 已包含上层，继续下一层
		}
		if strings.TrimSpace(profile.Status) != constants.AffiliateProfileStatusActive {
			continue
		}
		rate := resolveLevelRate(setting, level)
		if rate.LessThanOrEqual(decimal.Zero) {
			continue
		}
		commissionAmount := baseAmount.Mul(rate).Div(decimal.NewFromInt(100)).Round(2)
		if commissionAmount.LessThan(decimal.NewFromFloat(0.01)) {
			continue
		}
		if beneficiaryUserID == order.UserID {
			continue // 兜底自购检查（chain 构建已保证）
		}

		// 幂等检查：已存在则跳过。
		existing, err := s.repo.GetCommissionByOrderBeneficiaryLevel(order.ID, beneficiaryUserID, level)
		if err != nil {
			return err
		}
		if existing != nil {
			continue
		}

		status := constants.AffiliateCommissionStatusPendingConfirm
		var confirmAt *time.Time
		var availableAt *time.Time
		if setting.ConfirmDays <= 0 {
			status = constants.AffiliateCommissionStatusAvailable
			availableAt = &now
		} else {
			t := now.Add(time.Duration(setting.ConfirmDays) * 24 * time.Hour)
			confirmAt = &t
		}

		sourceUserID := order.UserID
		commissions = append(commissions, &affiliatedomain.Commission{
			AffiliateProfileID: profile.ID,
			OrderID:            order.ID,
			CommissionType:     constants.AffiliateCommissionTypeOrder,
			BeneficiaryUserID:  beneficiaryUserID,
			SourceUserID:       &sourceUserID,
			Level:              level,
			BaseAmount:         money.FromDecimal(baseAmount),
			RatePercent:        money.FromDecimal(rate),
			CommissionAmount:   money.FromDecimal(commissionAmount),
			Status:             status,
			ConfirmAt:          confirmAt,
			AvailableAt:        availableAt,
		})
	}
	if len(commissions) == 0 {
		return nil
	}

	// 事务内批量创建佣金 + CREDIT ledger，任一层失败整体回滚。
	err = s.repo.WithinTransaction(func(tx affiliatecontract.Store) error {
		if err := tx.BatchCreateCommissions(commissions); err != nil {
			return err
		}
		// 佣金创建成功后，为每条佣金创建 CREDIT ledger（append-only）。
		for _, c := range commissions {
			ref := fmt.Sprintf("affiliate_credit:order:%d:comm:%d", c.OrderID, c.ID)
			if _, err := s.appendLedger(tx, LedgerEntry{
				CommissionID:       c.ID,
				AffiliateProfileID: c.AffiliateProfileID,
				BeneficiaryUserID:  c.BeneficiaryUserID,
				OrderID:            c.OrderID,
				Type:               constants.AffiliateLedgerTypeCredit,
				Amount:             c.CommissionAmount.Decimal,
				Reference:          ref,
				Remark:             fmt.Sprintf("Order completed, level %d commission", c.Level),
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		// 并发兜底：幂等检查与插入之间的竞争导致唯一冲突，静默跳过。
		if isDuplicateKeyError(err) {
			logger.Warnw("affiliate_handle_order_completed_duplicate",
				"order_id", order.ID,
				"error", err,
			)
			return nil
		}
		return err
	}

	// ConfirmDays<=0 时佣金直接 available，立即发送到账通知（pending_confirm 阶段不发）。
	for _, c := range commissions {
		if c.Status == constants.AffiliateCommissionStatusAvailable {
			s.notifyCommissionConfirmed(c)
		}
	}
	return nil
}

// resolveLevelRate 返回指定层级的费率（百分比）。
// LevelRates 由 NormalizeAffiliateSetting 补全到 10 项；legacy CommissionRate 已在 L1 fallback 中归一。
func resolveLevelRate(setting settingsintegration.AffiliateSetting, level int) decimal.Decimal {
	if level < 1 || level > affiliateMaxLevel {
		return decimal.Zero
	}
	if level > len(setting.LevelRates) {
		return decimal.Zero
	}
	item := setting.LevelRates[level-1]
	if !item.Enabled {
		return decimal.Zero
	}
	rate := decimal.NewFromFloat(item.Rate).Round(2)
	if rate.LessThanOrEqual(decimal.Zero) {
		return decimal.Zero
	}
	return rate
}

// notifyCommissionConfirmed 佣金转为 available 后写用户站内通知（尽力而为+幂等）。
// 唯一约束 (user_id,biz_type=commission,biz_id=commission.ID,type=commission_confirmed) 拦截重复。
func (s *Service) notifyCommissionConfirmed(c *affiliatedomain.Commission) {
	if s == nil || s.userNotifier == nil || c == nil || c.BeneficiaryUserID == 0 {
		return
	}
	data := jsonmap.JSON{
		"commission_id":     c.ID,
		"order_id":          c.OrderID,
		"level":             c.Level,
		"commission_amount": c.CommissionAmount.String(),
		"base_amount":       c.BaseAmount.String(),
		"rate_percent":      c.RatePercent.String(),
	}
	if err := s.userNotifier.CreateNotification(context.Background(), usernotificationcontract.CreateInput{
		UserID:  c.BeneficiaryUserID,
		Type:    usernotificationdomain.TypeCommissionConfirmed,
		Title:   "佣金到账",
		Body:    "您的推广佣金已转入可提现余额",
		Data:    data,
		BizType: usernotificationdomain.BizTypeCommission,
		BizID:   c.ID,
	}); err != nil {
		logger.Warnw("usernotification_commission_confirmed_failed",
			"commission_id", c.ID,
			"order_id", c.OrderID,
			"beneficiary_user_id", c.BeneficiaryUserID,
			"error", err,
		)
	}
}

// isDuplicateKeyError 跨驱动识别唯一约束冲突（SQLite/Postgres/MySQL）。
func isDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	lower := strings.ToLower(msg)
	return strings.Contains(lower, "unique constraint") ||
		strings.Contains(lower, "duplicate key") ||
		strings.Contains(lower, "duplicate entry") ||
		strings.Contains(lower, "unique index")
}

// ConfirmDueCommissions 将到期佣金转可提现，并对每条转换成功的佣金发送到账通知。
func (s *Service) ConfirmDueCommissions(now time.Time) error {
	if s.repo == nil {
		return nil
	}
	rows, err := s.repo.MarkPendingCommissionsAvailable(now, now)
	if err != nil {
		return err
	}
	for i := range rows {
		s.notifyCommissionConfirmed(&rows[i])
	}
	return nil
}

// HandleOrderCanceled 处理订单取消/退款后的佣金逆向（ledger 版本）。
// 核心原则：不再 UPDATE commission amount，改为创建 REVERSAL ledger。
// 已绑定提现的佣金不再 SKIP（修复 P1-1）：
//   - 提现未 PAID：创建 REVERSAL 减少净余额，pay 时检查余额是否足够。
//   - 提现已 PAID：创建 REVERSAL 后净余额为负，同时创建 DEBT 记录债务。
func (s *Service) HandleOrderCanceled(orderID uint, reason string) error {
	if orderID == 0 || s.repo == nil {
		return nil
	}
	rows, err := s.repo.ListCommissionsByOrder(orderID, []string{
		constants.AffiliateCommissionStatusPendingConfirm,
		constants.AffiliateCommissionStatusAvailable,
		constants.AffiliateCommissionStatusWithdrawn, // 已出金佣金取消时必须冲正并产生 DEBT
	})
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}

	now := time.Now()
	reasonText := strings.TrimSpace(reason)
	if reasonText == "" {
		reasonText = "order_canceled"
	}

	return s.repo.WithinTransaction(func(tx affiliatecontract.Store) error {
		for i := range rows {
			item := rows[i]
			// 全额冲正：冲正金额 = 原始佣金金额（因为 commission amount 创建后不可变）
			reversalAmount := item.CommissionAmount.Decimal.Round(2)
			if reversalAmount.LessThanOrEqual(decimal.Zero) {
				continue
			}

			// 判断是否已出金：withdrawn 状态且关联提现已 PAID。
			// 已出金只创建 DEBT，未出金只创建 REVERSAL（避免双重扣减）。
			isSettled := false
			if item.Status == constants.AffiliateCommissionStatusWithdrawn && item.WithdrawRequestID != nil {
				withdrawReq, err := tx.GetWithdrawByID(*item.WithdrawRequestID)
				if err != nil {
					return err
				}
				if withdrawReq != nil && withdrawReq.Status == constants.AffiliateWithdrawStatusPaid {
					isSettled = true
				}
			}

			if isSettled {
				// 已出金：创建 DEBT
				debtRef := fmt.Sprintf("affiliate_debt:o%d:c%d:cancel", orderID, item.ID)
				if _, err := s.appendLedger(tx, LedgerEntry{
					CommissionID:       item.ID,
					AffiliateProfileID: item.AffiliateProfileID,
					BeneficiaryUserID:  item.BeneficiaryUserID,
					OrderID:            item.OrderID,
					Type:               constants.AffiliateLedgerTypeDebt,
					Amount:             reversalAmount.Neg(),
					Reference:          debtRef,
					WithdrawRequestID:  item.WithdrawRequestID,
					Remark:             reasonText + " (canceled after payout, debt owed)",
				}); err != nil {
					// METRICS_PENDING: affiliate_reversal_failed
					logger.Errorw("affiliate_reversal_failed",
						"order_id", orderID, "commission_id", item.ID,
						"type", "debt_cancel", "error", err.Error(),
					)
					return err
				}
				// METRICS_PENDING: affiliate_debt_created
				logger.Infow("affiliate_debt_created",
					"order_id", orderID, "commission_id", item.ID,
					"amount", reversalAmount.Neg().String(), "ref", debtRef,
				)
			} else {
				// 未出金：创建 REVERSAL
				ref := fmt.Sprintf("affiliate_reversal:o%d:c%d:cancel", orderID, item.ID)
				if _, err := s.appendLedger(tx, LedgerEntry{
					CommissionID:       item.ID,
					AffiliateProfileID: item.AffiliateProfileID,
					BeneficiaryUserID:  item.BeneficiaryUserID,
					OrderID:            item.OrderID,
					Type:               constants.AffiliateLedgerTypeReversal,
					Amount:             reversalAmount.Neg(),
					Reference:          ref,
					WithdrawRequestID:  item.WithdrawRequestID,
					Remark:             reasonText,
				}); err != nil {
					// METRICS_PENDING: affiliate_reversal_failed
					logger.Errorw("affiliate_reversal_failed",
						"order_id", orderID, "commission_id", item.ID,
						"type", "reversal_cancel", "error", err.Error(),
					)
					return err
				}
			}

			// 佣金净余额清零后，将状态改为 rejected。
			// withdrawn 状态（已出金）的佣金不改变状态，保持 withdrawn 用于审计追溯。
			if item.Status != constants.AffiliateCommissionStatusWithdrawn {
				item.Status = constants.AffiliateCommissionStatusRejected
				item.InvalidReason = reasonText
				item.ConfirmAt = nil
				item.AvailableAt = nil
			}
			item.UpdatedAt = now
			if err := tx.UpdateCommission(&item); err != nil {
				return err
			}
		}
		return nil
	})
}

// HandleOrderRefunded 使用调用方提供的事务 Store 处理退款后的佣金回滚（ledger 版本）。
// 多级别兼容：按 commission 逐条处理，天然覆盖 L1~L10；每级独立按比例扣减。
// 核心改造：不再 UPDATE commission amount，改为创建 REVERSAL ledger（append-only）。
func (s *Service) HandleOrderRefunded(
	repoTx affiliatecontract.Store,
	order *orderdomain.Order,
	refundDelta decimal.Decimal,
	refundedBefore decimal.Decimal,
	reason string,
) error {
	if repoTx == nil || order == nil || order.ID == 0 {
		return nil
	}
	delta := refundDelta.Round(2)
	if delta.LessThanOrEqual(decimal.Zero) {
		return nil
	}
	// 退款比例分母统一用 USDT 实付额。
	totalAmount := order.TotalAmount.Decimal.Round(2)
	if order.WalletPaidAmount.Decimal.GreaterThan(decimal.Zero) {
		totalAmount = order.WalletPaidAmount.Decimal.Round(2)
	}
	if totalAmount.LessThanOrEqual(decimal.Zero) {
		return nil
	}
	before := refundedBefore.Round(2)
	if before.LessThan(decimal.Zero) {
		before = decimal.Zero
	}
	if before.GreaterThan(totalAmount) {
		before = totalAmount
	}
	remaining := totalAmount.Sub(before).Round(2)
	if remaining.LessThanOrEqual(decimal.Zero) {
		// 全量退款兜底。
		return s.rejectActiveCommissionsOnFullRefund(repoTx, order.ID, reason)
	}
	if delta.GreaterThan(remaining) {
		delta = remaining
	}
	// delta 已经吃掉全部剩余时同样走兜底清零。
	if delta.GreaterThanOrEqual(remaining) {
		return s.rejectActiveCommissionsOnFullRefund(repoTx, order.ID, reason)
	}

	rows, err := repoTx.ListCommissionsByOrderForUpdate(order.ID, []string{
		constants.AffiliateCommissionStatusPendingConfirm,
		constants.AffiliateCommissionStatusAvailable,
		constants.AffiliateCommissionStatusWithdrawn, // 已出金佣金退款时必须冲正并产生 DEBT
	})
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}

	now := time.Now()
	reasonText := strings.TrimSpace(reason)
	if reasonText == "" {
		reasonText = "order_refunded"
	}
	for i := range rows {
		item := rows[i]

		// 按"本次退款金额 / 当前剩余未退款金额"比例计算应冲正金额。
		// 注意：commission.CommissionAmount 是原始创建时的金额（不可变），
		// 不是当前净余额。当前净余额需要从 ledger SUM 计算。
		originalCommission := item.CommissionAmount.Decimal.Round(2)
		if originalCommission.LessThanOrEqual(decimal.Zero) {
			continue
		}

		// 计算当前该 commission 的净余额（SUM ledger 按 commission_id 过滤）
		// 简化：直接按比例从原始金额计算冲正额。
		// 因为是多次退款累积，每次都基于 remaining（剩余未退款金额）计算比例。
		deduct := originalCommission.Mul(delta).Div(remaining).Round(2)
		if deduct.LessThanOrEqual(decimal.Zero) {
			continue
		}

		// 判断该佣金是否已出金（withdrawn 状态且关联提现已 PAID）。
		// 已出金佣金退款时只创建 DEBT（资金已离开系统，需追回），不创建 REVERSAL（避免与 SETTLE 双重扣减）。
		isSettled := false
		if item.Status == constants.AffiliateCommissionStatusWithdrawn && item.WithdrawRequestID != nil {
			withdrawReq, err := repoTx.GetWithdrawByID(*item.WithdrawRequestID)
			if err != nil {
				return err
			}
			if withdrawReq != nil && withdrawReq.Status == constants.AffiliateWithdrawStatusPaid {
				isSettled = true
			}
		}

		if isSettled {
			// 已出金：创建 DEBT 记录债务（负金额）
			debtRef := fmt.Sprintf("affiliate_debt:o%d:c%d:b%s", order.ID, item.ID, before.StringFixed(2))
			if _, err := s.appendLedger(repoTx, LedgerEntry{
				CommissionID:       item.ID,
				AffiliateProfileID: item.AffiliateProfileID,
				BeneficiaryUserID:  item.BeneficiaryUserID,
				OrderID:            item.OrderID,
				Type:               constants.AffiliateLedgerTypeDebt,
				Amount:             deduct.Neg(),
				Reference:          debtRef,
				WithdrawRequestID:  item.WithdrawRequestID,
				Remark:             fmt.Sprintf("%s (partial refund after payout, debt %.2f)", reasonText, deduct.InexactFloat64()),
			}); err != nil {
				// METRICS_PENDING: affiliate_reversal_failed
				logger.Errorw("affiliate_reversal_failed",
					"order_id", order.ID, "commission_id", item.ID,
					"type", "debt_partial_refund", "error", err.Error(),
				)
				return err
			}
			// METRICS_PENDING: affiliate_debt_created
			logger.Infow("affiliate_debt_created",
				"order_id", order.ID, "commission_id", item.ID,
				"amount", deduct.Neg().String(), "ref", debtRef,
			)
		} else {
			// 未出金：创建 REVERSAL 冲正（负金额）
			ref := fmt.Sprintf("affiliate_reversal:o%d:c%d:b%s", order.ID, item.ID, before.StringFixed(2))
			if _, err := s.appendLedger(repoTx, LedgerEntry{
				CommissionID:       item.ID,
				AffiliateProfileID: item.AffiliateProfileID,
				BeneficiaryUserID:  item.BeneficiaryUserID,
				OrderID:            item.OrderID,
				Type:               constants.AffiliateLedgerTypeReversal,
				Amount:             deduct.Neg(),
				Reference:          ref,
				WithdrawRequestID:  item.WithdrawRequestID,
				Remark:             fmt.Sprintf("%s (partial refund %.2f%%)", reasonText, delta.Div(remaining).Mul(decimal.NewFromInt(100)).InexactFloat64()),
			}); err != nil {
				// METRICS_PENDING: affiliate_reversal_failed
				logger.Errorw("affiliate_reversal_failed",
					"order_id", order.ID, "commission_id", item.ID,
					"type", "reversal_partial_refund", "error", err.Error(),
				)
				return err
			}
		}

		// 计算该 commission 的当前净余额，如果为 0 则状态改为 rejected。
		// 注意：withdrawn 状态（已出金）的佣金不改变状态，保持 withdrawn 用于审计追溯。
		currentNet := originalCommission.Sub(deduct).Round(2)
		if currentNet.LessThanOrEqual(decimal.Zero) && item.Status != constants.AffiliateCommissionStatusWithdrawn {
			item.Status = constants.AffiliateCommissionStatusRejected
			item.InvalidReason = reasonText
			item.ConfirmAt = nil
			item.AvailableAt = nil
		}
		// 注意：不再修改 CommissionAmount 和 BaseAmount（保持原始值不变）
		item.UpdatedAt = now
		if err := repoTx.UpdateCommission(&item); err != nil {
			return err
		}
	}
	return nil
}

// rejectActiveCommissionsOnFullRefund 全量退款兜底：创建全额 REVERSAL ledger。
// 注意：反转金额 = 当前该 commission 的净余额（SUM ledger），不是原始 commission amount。
// 因为之前可能已经有部分退款，需要只冲正剩余部分。
func (s *Service) rejectActiveCommissionsOnFullRefund(repoTx affiliatecontract.Store, orderID uint, reason string) error {
	rows, err := repoTx.ListCommissionsByOrderForUpdate(orderID, []string{
		constants.AffiliateCommissionStatusPendingConfirm,
		constants.AffiliateCommissionStatusAvailable,
		constants.AffiliateCommissionStatusWithdrawn, // 已出金佣金全额退款时必须冲正并产生 DEBT
	})
	if err != nil {
		return err
	}
	now := time.Now()
	reasonText := strings.TrimSpace(reason)
	if reasonText == "" {
		reasonText = "order_refunded"
	}
	for i := range rows {
		item := rows[i]

		// 计算当前该 commission 的净余额（从 ledger SUM）
		ledgers, err := repoTx.ListLedgersByCommission(item.ID)
		if err != nil {
			return err
		}
		currentNet := decimal.Zero
		for _, l := range ledgers {
			currentNet = currentNet.Add(l.Amount.Decimal).Round(2)
		}
		if currentNet.LessThanOrEqual(decimal.Zero) {
			continue // 已经净余额为 0，不需要再冲正
		}

		// 判断是否已出金：withdrawn 状态且关联提现已 PAID。
		// 已出金佣金只创建 DEBT（资金已离开系统），未出金只创建 REVERSAL。
		isSettled := false
		if item.Status == constants.AffiliateCommissionStatusWithdrawn && item.WithdrawRequestID != nil {
			withdrawReq, err := repoTx.GetWithdrawByID(*item.WithdrawRequestID)
			if err != nil {
				return err
			}
			if withdrawReq != nil && withdrawReq.Status == constants.AffiliateWithdrawStatusPaid {
				isSettled = true
			}
		}

		if isSettled {
			// 已出金：创建 DEBT 记录剩余债务
			debtRef := fmt.Sprintf("affiliate_debt:o%d:c%d:full", orderID, item.ID)
			if _, err := s.appendLedger(repoTx, LedgerEntry{
				CommissionID:       item.ID,
				AffiliateProfileID: item.AffiliateProfileID,
				BeneficiaryUserID:  item.BeneficiaryUserID,
				OrderID:            item.OrderID,
				Type:               constants.AffiliateLedgerTypeDebt,
				Amount:             currentNet.Neg(),
				Reference:          debtRef,
				WithdrawRequestID:  item.WithdrawRequestID,
				Remark:             reasonText + " (full refund after payout, debt for remaining balance)",
			}); err != nil {
				// METRICS_PENDING: affiliate_reversal_failed
				logger.Errorw("affiliate_reversal_failed",
					"order_id", orderID, "commission_id", item.ID,
					"type", "debt_full_refund", "error", err.Error(),
				)
				return err
			}
			// METRICS_PENDING: affiliate_debt_created
			logger.Infow("affiliate_debt_created",
				"order_id", orderID, "commission_id", item.ID,
				"amount", currentNet.Neg().String(), "ref", debtRef,
			)
		} else {
			// 未出金：创建 REVERSAL 冲正剩余净余额
			ref := fmt.Sprintf("affiliate_reversal:o%d:c%d:full", orderID, item.ID)
			if _, err := s.appendLedger(repoTx, LedgerEntry{
				CommissionID:       item.ID,
				AffiliateProfileID: item.AffiliateProfileID,
				BeneficiaryUserID:  item.BeneficiaryUserID,
				OrderID:            item.OrderID,
				Type:               constants.AffiliateLedgerTypeReversal,
				Amount:             currentNet.Neg(),
				Reference:          ref,
				WithdrawRequestID:  item.WithdrawRequestID,
				Remark:             reasonText + " (full refund, remaining balance)",
			}); err != nil {
				// METRICS_PENDING: affiliate_reversal_failed
				logger.Errorw("affiliate_reversal_failed",
					"order_id", orderID, "commission_id", item.ID,
					"type", "reversal_full_refund", "error", err.Error(),
				)
				return err
			}
		}

		// withdrawn 状态（已出金）的佣金不改变状态，保持 withdrawn 用于审计追溯。
		if item.Status != constants.AffiliateCommissionStatusWithdrawn {
			item.Status = constants.AffiliateCommissionStatusRejected
			item.InvalidReason = reasonText
			item.ConfirmAt = nil
			item.AvailableAt = nil
		}
		item.UpdatedAt = now
		// 不再修改 CommissionAmount 和 BaseAmount（保持原始值不变）
		if err := repoTx.UpdateCommission(&item); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) resolveAffiliateProfileForOrder(order *orderdomain.Order) (*affiliatedomain.Profile, error) {
	if order == nil || s.repo == nil {
		return nil, nil
	}
	if order.AffiliateProfileID != nil && *order.AffiliateProfileID > 0 {
		return s.repo.GetProfileByID(*order.AffiliateProfileID)
	}
	if strings.TrimSpace(order.AffiliateCode) != "" {
		return s.repo.GetProfileByCode(order.AffiliateCode)
	}
	return nil, nil
}

func (s *Service) calculateCommissionBaseAmount(order *orderdomain.Order) (decimal.Decimal, error) {
	if order == nil || s.productRepo == nil {
		return decimal.Zero, nil
	}
	productIDs := collectAffiliateProductIDs(order)
	if len(productIDs) == 0 {
		return decimal.Zero, nil
	}
	products, err := s.productRepo.ListByIDs(productIDs)
	if err != nil {
		return decimal.Zero, err
	}
	productMap := make(map[uint]productdomain.Product, len(products))
	for _, product := range products {
		productMap[product.ID] = product
	}

	targetOrders := order.Children
	if len(targetOrders) == 0 {
		targetOrders = []orderdomain.Order{*order}
	}

	total := decimal.Zero
	for _, current := range targetOrders {
		for _, item := range current.Items {
			product, ok := productMap[item.ProductID]
			if !ok || !product.IsAffiliateEnabled {
				continue
			}
			payable := item.TotalPrice.Decimal.Sub(item.CouponDiscount.Decimal).Round(2)
			if payable.LessThan(decimal.Zero) {
				payable = decimal.Zero
			}
			total = total.Add(payable).Round(2)
		}
	}
	// P0-2: 钱包是 USDT。订单有 Global Rate 快照时，把 Site Currency 返利基数一次性换算为 USDT，
	// 后续 commission_amount 即 USDT，返利入账/展示均为 USDT。旧单无快照保持 legacy。
	if order.ExchangeRate.Valid && order.ExchangeRate.Decimal.GreaterThan(decimal.Zero) {
		total = total.Div(order.ExchangeRate.Decimal).Round(2)
	}
	return total, nil
}

func collectAffiliateProductIDs(order *orderdomain.Order) []uint {
	if order == nil {
		return nil
	}
	ids := make([]uint, 0)
	seen := make(map[uint]struct{})
	appendItem := func(item orderdomain.OrderItem) {
		if item.ProductID == 0 {
			return
		}
		if _, ok := seen[item.ProductID]; ok {
			return
		}
		seen[item.ProductID] = struct{}{}
		ids = append(ids, item.ProductID)
	}
	for _, item := range order.Items {
		appendItem(item)
	}
	for _, child := range order.Children {
		for _, item := range child.Items {
			appendItem(item)
		}
	}
	return ids
}

func buildSplitCommissionType(sourceID uint) string {
	suffix := strconv.FormatInt(time.Now().UnixNano()%1000000, 10)
	base := affiliateSplitTypePrefix + strconv.FormatUint(uint64(sourceID), 36)
	result := base + suffix
	if len(result) > 20 {
		return result[:20]
	}
	return result
}
