package application

import (
	"math"
	"strings"
	"time"

	pointscontract "github.com/Aether-v1/hcz/internal/modules/points/contract"
	pointsdomain "github.com/Aether-v1/hcz/internal/modules/points/domain"
)

// actionMeta 定义每个积分动作的记账语义。
// 所有积分变更都通过 action 元数据驱动余额/统计更新，
// 禁止以后为"系统冲正""商城返还"等复制两套扣分逻辑。
type actionMeta struct {
	allowNegative bool // 是否允许余额为负（系统/Admin 冲正允许；用户消费禁止）
	trackEarned   bool // amount 为正时计入 total_earned
	trackSpent    bool // amount 为负时计入 total_spent（绝对值）
}

// actionMetas 是 action_type → 记账语义的唯一权威表。
// 八种动作在 P0–P4 全部已上线并产生流水（P0 Admin 调整/补偿、P1 订单奖励与冲正、
// P2 签到、P3 兑换扣减与返还）；新增 action 必须同时在此表登记，否则 applyMutation 拒绝入账。
var actionMetas = map[string]actionMeta{
	pointscontract.ActionAdminAdd:          {allowNegative: false, trackEarned: true, trackSpent: false},
	pointscontract.ActionAdminDeduct:       {allowNegative: true, trackEarned: false, trackSpent: false},
	pointscontract.ActionAdminCompensation: {allowNegative: false, trackEarned: true, trackSpent: false},

	pointscontract.ActionOrderReward:         {allowNegative: false, trackEarned: true, trackSpent: false},
	pointscontract.ActionOrderRewardReversal: {allowNegative: true, trackEarned: false, trackSpent: false},
	pointscontract.ActionCheckinReward:       {allowNegative: false, trackEarned: true, trackSpent: false},
	pointscontract.ActionRedeem:              {allowNegative: false, trackEarned: false, trackSpent: true},
	pointscontract.ActionRedeemRefund:        {allowNegative: false, trackEarned: true, trackSpent: false},
}

// mutationInput 是核心记账输入的完整描述。
type mutationInput struct {
	userID          uint
	actionType      string
	sourceType      string
	sourceID        uint
	amount          int64 // 有符号：正=入账，负=出账
	reference       string
	reason          string
	operatorType    string
	operatorID      uint
	orderID         *uint
	checkinDate     *time.Time
	exchangeOrderID *uint
}

// applyMutation 是积分变更的唯一核心入口：
//
//	在调用方事务内：
//	SELECT points_accounts FOR UPDATE（不存在则安全创建）
//	读取 balance_before
//	校验 mutation policy（负余额 / 溢出）
//	计算 balance_after，按 action 语义更新 total_earned / total_spent
//	UPDATE points_accounts
//	INSERT points_ledger
//
// Account 更新与 Ledger 写入必须同事务（由调用方保证事务边界）。
func (s *Service) applyMutation(tx pointscontract.Transaction, input mutationInput) (*pointsdomain.Account, *pointsdomain.LedgerEntry, error) {
	if tx == nil {
		return nil, nil, pointscontract.ErrTransactionRequired
	}
	if input.userID == 0 {
		return nil, nil, pointscontract.ErrAccountNotFound
	}
	meta, ok := actionMetas[input.actionType]
	if !ok {
		return nil, nil, pointscontract.ErrInvalidOperation
	}
	if input.amount == 0 {
		return nil, nil, pointscontract.ErrInvalidAmount
	}
	if strings.TrimSpace(input.reference) == "" {
		return nil, nil, pointscontract.ErrReferenceRequired
	}
	if err := validateReference(strings.TrimSpace(input.reference)); err != nil {
		return nil, nil, err
	}
	if err := validateReason(cleanReason(input.reason, "")); err != nil {
		return nil, nil, err
	}

	repo := tx.Points()
	now := time.Now()
	account, err := ensureAccountForUpdate(repo, input.userID, now)
	if err != nil {
		return nil, nil, err
	}

	before := account.Balance
	after := before + input.amount

	// BIGINT 溢出防御（int64 有符号加减）。
	if input.amount > 0 && after < before {
		return nil, nil, pointscontract.ErrInvalidAmount
	}
	if input.amount < 0 && after > before {
		return nil, nil, pointscontract.ErrInvalidAmount
	}

	// 负余额 policy：仅允许 allowNegative 的动作产生负余额。
	if after < 0 && !meta.allowNegative {
		return nil, nil, pointscontract.ErrNegativeNotAllowed
	}

	account.Balance = after
	if meta.trackEarned && input.amount > 0 {
		// earned 语义：amount 恒为正（获得类动作）。
		// 累计字段独立溢出保护：balance 不溢出 ≠ total_earned 不溢出。
		if account.TotalEarned > math.MaxInt64-input.amount {
			return nil, nil, pointscontract.ErrInvalidAmount
		}
		account.TotalEarned += input.amount
	}
	if meta.trackSpent && input.amount < 0 {
		// spent 语义：amount 恒为负（消费类动作），累计其绝对值。
		delta := -input.amount
		if account.TotalSpent > math.MaxInt64-delta {
			return nil, nil, pointscontract.ErrInvalidAmount
		}
		account.TotalSpent += delta
	}
	account.UpdatedAt = now

	if err := repo.UpdateAccount(account); err != nil {
		return nil, nil, err
	}

	entry := &pointsdomain.LedgerEntry{
		UserID:          input.userID,
		ActionType:      input.actionType,
		SourceType:      input.sourceType,
		SourceID:        input.sourceID,
		Amount:          input.amount,
		BalanceBefore:   before,
		BalanceAfter:    after,
		Reference:       strings.TrimSpace(input.reference),
		Reason:          cleanReason(input.reason, ""),
		OperatorType:    input.operatorType,
		OperatorID:      input.operatorID,
		OrderID:         input.orderID,
		CheckinDate:     input.checkinDate,
		ExchangeOrderID: input.exchangeOrderID,
		CreatedAt:       now,
	}
	if err := repo.CreateLedgerEntry(entry); err != nil {
		return nil, nil, err
	}
	return account, entry, nil
}

// ensureAccountForUpdate 在事务内行锁读取账户；不存在则安全创建。
//
// 并发首次创建：两个事务同时发现账户不存在时，先 Create 者成功、
// 后 Create 者撞 user_id 唯一索引 → 立即重新行锁读取已创建账户返回。
// 禁止"唯一约束失败直接报业务错误"。
func ensureAccountForUpdate(repository pointscontract.Repository, userID uint, now time.Time) (*pointsdomain.Account, error) {
	if repository == nil {
		return nil, pointscontract.ErrTransactionRequired
	}
	account, err := repository.GetAccountByUserIDForUpdate(userID)
	if err != nil {
		return nil, err
	}
	if account != nil {
		return account, nil
	}
	account = &pointsdomain.Account{
		UserID:    userID,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := repository.CreateAccount(account); err != nil {
		// 并发下另一事务已创建：重查并返回已存在账户。
		created, queryErr := repository.GetAccountByUserIDForUpdate(userID)
		if queryErr == nil && created != nil {
			return created, nil
		}
		return nil, pointscontract.ErrAccountCreateFailed
	}
	return account, nil
}

// isDuplicateKeyError 识别唯一索引冲突（SQLite / PostgreSQL / MySQL 驱动统一口径）。
func isDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "unique constraint") ||
		strings.Contains(lower, "duplicate key") ||
		strings.Contains(lower, "duplicate entry") ||
		strings.Contains(lower, "unique index")
}
