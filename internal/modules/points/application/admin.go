package application

import (
	"strings"

	"github.com/Aether-v1/hcz/internal/logger"
	pointscontract "github.com/Aether-v1/hcz/internal/modules/points/contract"
	pointsdomain "github.com/Aether-v1/hcz/internal/modules/points/domain"
)

// adminMutationInput 描述一次 Admin 侧积分写入。
// Amount 为有符号入账金额（正=加，负=扣）；Magnitude 是客户端提交的无符号幅度（>0），
// 两者分开保存：Admin 接口禁止用负数 Amount 表达扣减，负数幅度必须在入账前就被拒绝。
type adminMutationInput struct {
	userID          uint
	operatorAdminID uint
	actionType      string
	sourceType      string
	amount          int64 // 有符号
	magnitude       int64 // 无符号幅度（>0，用于业务上限校验）
	reason          string
	reference       string
	orderID         *uint
}

// AdminAdjust 管理员增减用户积分。
//
// 语义：
//   - Operation 为 add / subtract，Amount 恒为正；扣除由 subtract 表达，
//     禁止用负数 Amount 表达扣减（避免双重负数错误）；
//   - Admin 扣减允许余额为负（审计结论：系统内部 / Admin 允许负，用户消费禁止）；
//   - Reason 必填；Reference（Idempotency-Key 派生）必填；
//   - 单笔金额受 MaxPointsAmount 业务上限约束；
//   - 幂等：事务外先按 reference 查重，命中且参数一致 → 返回原流水（HTTP 200 不重复入账），
//     命中但参数不一致 → ErrIdempotencyConflict（HTTP 409）；
//     并发最终防线是 points_ledger.reference 唯一索引（isDuplicateKeyError 兜底）。
//   - 积分流水 append-only：本用例只新增流水，永不修改/删除历史记录。
func (s *Service) AdminAdjust(input pointscontract.AdjustInput) (*pointsdomain.Account, *pointsdomain.LedgerEntry, error) {
	op := strings.ToLower(strings.TrimSpace(input.Operation))
	if op != "add" && op != "subtract" {
		return nil, nil, pointscontract.ErrInvalidOperation
	}
	amount := input.Amount
	actionType := pointscontract.ActionAdminAdd
	if op == "subtract" {
		amount = -input.Amount
		actionType = pointscontract.ActionAdminDeduct
	}
	return s.adminMutation(adminMutationInput{
		userID:          input.UserID,
		operatorAdminID: input.OperatorAdminID,
		actionType:      actionType,
		sourceType:      pointscontract.SourceAdminAdjust,
		amount:          amount,
		magnitude:       input.Amount,
		reason:          input.Reason,
		reference:       input.Reference,
	})
}

// AdminCompensate 管理员人工补偿积分（P4）。
//
// 决策（P4 报告固化）：
//   - 历史异常需要人工补发订单奖励时，一律使用独立 action ADMIN_COMPENSATION，
//     绝不伪造 ORDER_REWARD，也不重放订单完成生命周期（重放会连带触发佣金/通知等副作用）；
//   - 仍走积分核心 applyMutation（同一套行锁、溢出与幂等语义），不建第二套记账逻辑；
//   - 不做自动修复引擎：补偿永远是"某位管理员的一次显式决定"。
func (s *Service) AdminCompensate(input pointscontract.CompensateInput) (*pointsdomain.Account, *pointsdomain.LedgerEntry, error) {
	var orderID *uint
	if input.OrderID != 0 {
		orderID = &input.OrderID
	}
	return s.adminMutation(adminMutationInput{
		userID:          input.UserID,
		operatorAdminID: input.OperatorAdminID,
		actionType:      pointscontract.ActionAdminCompensation,
		sourceType:      pointscontract.SourceAdminCompensation,
		amount:          input.Amount,
		magnitude:       input.Amount,
		reason:          input.Reason,
		reference:       input.Reference,
		orderID:         orderID,
	})
}

// adminMutation 是 Admin 侧积分写入的唯一共用流程：
// 校验 → reference 幂等预查 → 事务内 applyMutation → 唯一索引冲突回查。
func (s *Service) adminMutation(input adminMutationInput) (*pointsdomain.Account, *pointsdomain.LedgerEntry, error) {
	if input.userID == 0 || input.operatorAdminID == 0 {
		return nil, nil, pointscontract.ErrAccountNotFound
	}
	if err := validateAmountBounds(input.magnitude); err != nil {
		return nil, nil, err
	}
	reason := strings.TrimSpace(input.reason)
	if reason == "" {
		return nil, nil, pointscontract.ErrReasonRequired
	}
	ref := strings.TrimSpace(input.reference)
	if ref == "" {
		return nil, nil, pointscontract.ErrReferenceRequired
	}
	if err := validateReason(reason); err != nil {
		return nil, nil, err
	}
	if err := validateReference(ref); err != nil {
		return nil, nil, err
	}

	// 幂等预查（事务外）。
	existing, err := s.repository.GetLedgerEntryByReference(ref)
	if err != nil {
		return nil, nil, err
	}
	if existing != nil {
		return s.replayIdempotent(existing, input.userID, input.actionType, input.amount)
	}

	var account *pointsdomain.Account
	var entry *pointsdomain.LedgerEntry
	err = s.transactions.WithinTransaction(func(tx pointscontract.Transaction) error {
		acc, e, txErr := s.applyMutation(tx, mutationInput{
			userID:       input.userID,
			actionType:   input.actionType,
			sourceType:   input.sourceType,
			sourceID:     input.userID,
			amount:       input.amount,
			reference:    ref,
			reason:       cleanReason(input.reason, "管理员操作"),
			operatorType: pointscontract.OperatorAdmin,
			operatorID:   input.operatorAdminID,
			orderID:      input.orderID,
		})
		if txErr != nil {
			return txErr
		}
		account, entry = acc, e
		return nil
	})
	if err != nil {
		// 并发下同 reference 的另一事务已提交：唯一索引兜底 → 幂等返回。
		if isDuplicateKeyError(err) {
			existing, queryErr := s.repository.GetLedgerEntryByReference(ref)
			if queryErr != nil {
				return nil, nil, queryErr
			}
			if existing != nil {
				return s.replayIdempotent(existing, input.userID, input.actionType, input.amount)
			}
		}
		logger.Warnw("points_admin_mutation_failed",
			"user_id", input.userID,
			"admin_id", input.operatorAdminID,
			"action_type", input.actionType,
			"amount", input.amount,
			"reference", ref,
			"error", err.Error())
		return nil, nil, err
	}

	// 后台资产写审计：操作者、对象、前后余额、幂等键（不含任何敏感载荷）。
	logger.Infow("points_admin_mutation",
		"admin_id", input.operatorAdminID,
		"user_id", input.userID,
		"action_type", input.actionType,
		"amount", input.amount,
		"balance_before", entry.BalanceBefore,
		"balance_after", entry.BalanceAfter,
		"reason", cleanReason(input.reason, ""),
		"reference", ref)
	return account, entry, nil
}

// replayIdempotent 处理 reference 命中：参数一致回放原流水，不一致返回幂等冲突。
func (s *Service) replayIdempotent(existing *pointsdomain.LedgerEntry, userID uint, actionType string, amount int64) (*pointsdomain.Account, *pointsdomain.LedgerEntry, error) {
	if existing.UserID == userID && existing.ActionType == actionType && existing.Amount == amount {
		acct, acctErr := s.repository.GetAccountByUserID(userID)
		if acctErr != nil {
			return nil, nil, acctErr
		}
		return acct, existing, nil
	}
	return nil, nil, pointscontract.ErrIdempotencyConflict
}
