package application

import (
	"sort"
	"strings"
	"time"

	"github.com/Aether-v1/hcz/internal/constants"
	walletcontract "github.com/Aether-v1/hcz/internal/modules/wallet/contract"
	walletdomain "github.com/Aether-v1/hcz/internal/modules/wallet/domain"
	"github.com/Aether-v1/hcz/internal/shared/money"

	"github.com/shopspring/decimal"
)

// freezeCurrency 是 C2C freeze 原语使用的账本币种。
const freezeCurrency = "USDT"

// LockAccountsByUserIDOrder 在事务内按 user_id 升序获取多个账户的行锁，防止死锁。
// 即使 source > target 也必须按升序锁，不能按业务角色顺序。
// 返回 map[userID]*Account。不存在的账户不在 map 中（调用方负责创建）。
//
// 这是双账户加锁的唯一入口：所有需要锁多个账户的写路径都必须经过这里，
// 禁止各调用方自己决定锁顺序，否则 A→B 与 B→A 并发时会形成循环等待。
func LockAccountsByUserIDOrder(repo walletcontract.Repository, userIDs []uint) (map[uint]*walletdomain.Account, error) {
	if repo == nil {
		return nil, walletcontract.ErrTransactionRequired
	}
	// 1. 去重并过滤 0
	seen := make(map[uint]struct{}, len(userIDs))
	ordered := make([]uint, 0, len(userIDs))
	for _, id := range userIDs {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ordered = append(ordered, id)
	}
	// 2. 升序排序 —— 锁顺序死协议，禁止按业务角色排序
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
	// 3. 依次调用 ForUpdate 获取行锁
	result := make(map[uint]*walletdomain.Account, len(ordered))
	for _, id := range ordered {
		acc, err := repo.GetAccountByUserIDForUpdate(id)
		if err != nil {
			return nil, err
		}
		if acc != nil {
			result[id] = acc
		}
	}
	return result, nil
}

// lockedAccountOrCreate 在已持有的升序锁顺序内取账户；不存在则创建。
// 创建发生在所有现存账户已按升序加锁之后，不会破坏全局锁顺序协议。
func lockedAccountOrCreate(repo walletcontract.Repository, locked map[uint]*walletdomain.Account, userID uint, now time.Time) (*walletdomain.Account, error) {
	if acc, ok := locked[userID]; ok && acc != nil {
		return acc, nil
	}
	acc := &walletdomain.Account{
		UserID:           userID,
		AvailableBalance: money.FromDecimal(decimal.Zero),
		FrozenBalance:    money.FromDecimal(decimal.Zero),
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := repo.CreateAccount(acc); err != nil {
		// 并发下唯一索引冲突，退化为查询已存在的行
		created, queryErr := repo.GetAccountByUserIDForUpdate(userID)
		if queryErr == nil && created != nil {
			return created, nil
		}
		return nil, walletcontract.ErrAccountCreateFailed
	}
	return acc, nil
}

// Freeze 同一账户事务内: available -= amount, frozen += amount。
// 要求: available >= amount, amount > 0, total(available+frozen) 不变。
// 在调用方事务内执行，行锁账户，reference 幂等。
func (s *Service) Freeze(tx walletcontract.Transaction, input walletcontract.FreezeInput) (*walletdomain.Account, *walletdomain.Transaction, error) {
	if tx == nil {
		return nil, nil, walletcontract.ErrTransactionRequired
	}
	if input.UserID == 0 {
		return nil, nil, walletcontract.ErrAccountNotFound
	}
	amount := input.Amount.Decimal.Round(2)
	if amount.LessThanOrEqual(decimal.Zero) {
		return nil, nil, walletcontract.ErrInvalidAmount
	}
	reference := strings.TrimSpace(input.Reference)
	if reference == "" {
		return nil, nil, walletcontract.ErrTransactionCreateFailed
	}
	repo := tx.Wallets()
	if repo == nil {
		return nil, nil, walletcontract.ErrTransactionRequired
	}

	// 幂等预检：同一 reference 已写过则直接返回既有结果，不重复冻结。
	existing, err := repo.GetTransactionByReference(reference)
	if err != nil {
		return nil, nil, err
	}
	if existing != nil {
		account, accountErr := repo.GetAccountByUserID(input.UserID)
		if accountErr != nil {
			return nil, nil, accountErr
		}
		if account == nil {
			account, accountErr = ensureAccountForUpdate(repo, input.UserID, time.Now())
			if accountErr != nil {
				return nil, nil, accountErr
			}
		}
		return account, existing, nil
	}

	now := time.Now()
	account, err := ensureAccountForUpdate(repo, input.UserID, now)
	if err != nil {
		return nil, nil, err
	}
	beforeAvailable := account.AvailableBalance.Decimal.Round(2)
	if beforeAvailable.LessThan(amount) {
		return nil, nil, walletcontract.ErrInsufficientBalance
	}
	beforeFrozen := account.FrozenBalance.Decimal.Round(2)
	afterAvailable := beforeAvailable.Sub(amount).Round(2)
	afterFrozen := beforeFrozen.Add(amount).Round(2)

	account.AvailableBalance = money.FromDecimal(afterAvailable)
	account.FrozenBalance = money.FromDecimal(afterFrozen)
	account.UpdatedAt = now
	if err := repo.UpdateAccount(account); err != nil {
		return nil, nil, walletcontract.ErrAccountUpdateFailed
	}

	transaction := &walletdomain.Transaction{
		UserID:          input.UserID,
		Type:            constants.WalletTxnTypeC2CFreeze,
		Direction:       constants.WalletTxnDirectionOut,
		Amount:          money.FromDecimal(amount),
		AvailableBefore: money.FromDecimal(beforeAvailable),
		AvailableAfter:  money.FromDecimal(afterAvailable),
		FrozenBefore:    money.FromDecimal(beforeFrozen),
		FrozenAfter:     money.FromDecimal(afterFrozen),
		BalanceBefore:   money.FromDecimal(beforeAvailable.Add(beforeFrozen).Round(2)),
		BalanceAfter:    money.FromDecimal(afterAvailable.Add(afterFrozen).Round(2)),
		Currency:        freezeCurrency,
		Reference:       reference,
		Remark:          cleanRemark(input.Remark, "C2C资金冻结"),
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := repo.CreateTransaction(transaction); err != nil {
		return nil, nil, walletcontract.ErrTransactionCreateFailed
	}
	return account, transaction, nil
}

// Unfreeze: frozen -= amount, available += amount。
// 要求: frozen >= amount, total 不变。
func (s *Service) Unfreeze(tx walletcontract.Transaction, input walletcontract.UnfreezeInput) (*walletdomain.Account, *walletdomain.Transaction, error) {
	if tx == nil {
		return nil, nil, walletcontract.ErrTransactionRequired
	}
	if input.UserID == 0 {
		return nil, nil, walletcontract.ErrAccountNotFound
	}
	amount := input.Amount.Decimal.Round(2)
	if amount.LessThanOrEqual(decimal.Zero) {
		return nil, nil, walletcontract.ErrInvalidAmount
	}
	reference := strings.TrimSpace(input.Reference)
	if reference == "" {
		return nil, nil, walletcontract.ErrTransactionCreateFailed
	}
	repo := tx.Wallets()
	if repo == nil {
		return nil, nil, walletcontract.ErrTransactionRequired
	}

	existing, err := repo.GetTransactionByReference(reference)
	if err != nil {
		return nil, nil, err
	}
	if existing != nil {
		account, accountErr := repo.GetAccountByUserID(input.UserID)
		if accountErr != nil {
			return nil, nil, accountErr
		}
		if account == nil {
			account, accountErr = ensureAccountForUpdate(repo, input.UserID, time.Now())
			if accountErr != nil {
				return nil, nil, accountErr
			}
		}
		return account, existing, nil
	}

	now := time.Now()
	account, err := ensureAccountForUpdate(repo, input.UserID, now)
	if err != nil {
		return nil, nil, err
	}
	beforeAvailable := account.AvailableBalance.Decimal.Round(2)
	beforeFrozen := account.FrozenBalance.Decimal.Round(2)
	if beforeFrozen.LessThan(amount) {
		return nil, nil, walletcontract.ErrInsufficientFrozen
	}
	afterFrozen := beforeFrozen.Sub(amount).Round(2)
	afterAvailable := beforeAvailable.Add(amount).Round(2)

	account.AvailableBalance = money.FromDecimal(afterAvailable)
	account.FrozenBalance = money.FromDecimal(afterFrozen)
	account.UpdatedAt = now
	if err := repo.UpdateAccount(account); err != nil {
		return nil, nil, walletcontract.ErrAccountUpdateFailed
	}

	transaction := &walletdomain.Transaction{
		UserID:          input.UserID,
		Type:            constants.WalletTxnTypeC2CUnfreeze,
		Direction:       constants.WalletTxnDirectionIn,
		Amount:          money.FromDecimal(amount),
		AvailableBefore: money.FromDecimal(beforeAvailable),
		AvailableAfter:  money.FromDecimal(afterAvailable),
		FrozenBefore:    money.FromDecimal(beforeFrozen),
		FrozenAfter:     money.FromDecimal(afterFrozen),
		BalanceBefore:   money.FromDecimal(beforeAvailable.Add(beforeFrozen).Round(2)),
		BalanceAfter:    money.FromDecimal(afterAvailable.Add(afterFrozen).Round(2)),
		Currency:        freezeCurrency,
		Reference:       reference,
		Remark:          cleanRemark(input.Remark, "C2C资金解冻"),
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := repo.CreateTransaction(transaction); err != nil {
		return nil, nil, walletcontract.ErrTransactionCreateFailed
	}
	return account, transaction, nil
}

// SettleFrozen 将 source frozen 转给 target available。
// source: frozen -= amount; target: available += amount。
// 同一事务，双账户按 user_id 升序锁（用 LockAccountsByUserIDOrder）。
// source == target 拒绝（ErrSameAccount）。
func (s *Service) SettleFrozen(tx walletcontract.Transaction, input walletcontract.SettleInput) error {
	if tx == nil {
		return walletcontract.ErrTransactionRequired
	}
	if input.SourceUserID == 0 || input.TargetUserID == 0 {
		return walletcontract.ErrAccountNotFound
	}
	if input.SourceUserID == input.TargetUserID {
		return walletcontract.ErrSameAccount
	}
	amount := input.Amount.Decimal.Round(2)
	if amount.LessThanOrEqual(decimal.Zero) {
		return walletcontract.ErrInvalidAmount
	}
	sourceRef := strings.TrimSpace(input.SourceReference)
	targetRef := strings.TrimSpace(input.TargetReference)
	if sourceRef == "" || targetRef == "" {
		return walletcontract.ErrTransactionCreateFailed
	}
	repo := tx.Wallets()
	if repo == nil {
		return walletcontract.ErrTransactionRequired
	}

	// 幂等预检：两侧 reference 的存在性必须一致，否则视为之前部分失败。
	sourceExisting, err := repo.GetTransactionByReference(sourceRef)
	if err != nil {
		return err
	}
	targetExisting, err := repo.GetTransactionByReference(targetRef)
	if err != nil {
		return err
	}
	switch {
	case sourceExisting != nil && targetExisting != nil:
		return nil // 两侧都已写，幂等成功
	case sourceExisting != nil || targetExisting != nil:
		// 一侧已写另一侧未写：历史部分失败，拒绝继续以免破坏两阶段一致性
		return walletcontract.ErrTransactionCreateFailed
	}

	// 按 user_id 升序锁两账户（死锁防护的唯一入口）。
	locked, err := LockAccountsByUserIDOrder(repo, []uint{input.SourceUserID, input.TargetUserID})
	if err != nil {
		return err
	}

	// 加锁后二次幂等检查：行锁已把同一账户对的并发 settle 串行化，
	// 此时若另一并发者先提交了同一对 reference，直接返回幂等成功而非撞唯一索引。
	sourceExisting, err = repo.GetTransactionByReference(sourceRef)
	if err != nil {
		return err
	}
	targetExisting, err = repo.GetTransactionByReference(targetRef)
	if err != nil {
		return err
	}
	switch {
	case sourceExisting != nil && targetExisting != nil:
		return nil
	case sourceExisting != nil || targetExisting != nil:
		return walletcontract.ErrTransactionCreateFailed
	}

	now := time.Now()
	source, err := lockedAccountOrCreate(repo, locked, input.SourceUserID, now)
	if err != nil {
		return err
	}
	target, err := lockedAccountOrCreate(repo, locked, input.TargetUserID, now)
	if err != nil {
		return err
	}

	sourceAvailableBefore := source.AvailableBalance.Decimal.Round(2)
	sourceFrozenBefore := source.FrozenBalance.Decimal.Round(2)
	if sourceFrozenBefore.LessThan(amount) {
		return walletcontract.ErrInsufficientFrozen
	}
	sourceFrozenAfter := sourceFrozenBefore.Sub(amount).Round(2)

	targetAvailableBefore := target.AvailableBalance.Decimal.Round(2)
	targetFrozenBefore := target.FrozenBalance.Decimal.Round(2)
	targetAvailableAfter := targetAvailableBefore.Add(amount).Round(2)

	// source: frozen 减少，available 不变
	source.AvailableBalance = money.FromDecimal(sourceAvailableBefore)
	source.FrozenBalance = money.FromDecimal(sourceFrozenAfter)
	source.UpdatedAt = now
	if err := repo.UpdateAccount(source); err != nil {
		return walletcontract.ErrAccountUpdateFailed
	}
	// target: available 增加，frozen 不变
	target.AvailableBalance = money.FromDecimal(targetAvailableAfter)
	target.FrozenBalance = money.FromDecimal(targetFrozenBefore)
	target.UpdatedAt = now
	if err := repo.UpdateAccount(target); err != nil {
		return walletcontract.ErrAccountUpdateFailed
	}

	// source 侧 ledger: c2c_settle, out（available 不变，frozen 减少）
	sourceTxn := &walletdomain.Transaction{
		UserID:          input.SourceUserID,
		Type:            constants.WalletTxnTypeC2CSettle,
		Direction:       constants.WalletTxnDirectionOut,
		Amount:          money.FromDecimal(amount),
		AvailableBefore: money.FromDecimal(sourceAvailableBefore),
		AvailableAfter:  money.FromDecimal(sourceAvailableBefore),
		FrozenBefore:    money.FromDecimal(sourceFrozenBefore),
		FrozenAfter:     money.FromDecimal(sourceFrozenAfter),
		BalanceBefore:   money.FromDecimal(sourceAvailableBefore.Add(sourceFrozenBefore).Round(2)),
		BalanceAfter:    money.FromDecimal(sourceAvailableBefore.Add(sourceFrozenAfter).Round(2)),
		Currency:        freezeCurrency,
		Reference:       sourceRef,
		Remark:          cleanRemark(input.Remark, "C2C结算划出"),
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := repo.CreateTransaction(sourceTxn); err != nil {
		return walletcontract.ErrTransactionCreateFailed
	}

	// target 侧 ledger: c2c_receive, in（frozen 不变，available 增加）
	targetTxn := &walletdomain.Transaction{
		UserID:          input.TargetUserID,
		Type:            constants.WalletTxnTypeC2CReceive,
		Direction:       constants.WalletTxnDirectionIn,
		Amount:          money.FromDecimal(amount),
		AvailableBefore: money.FromDecimal(targetAvailableBefore),
		AvailableAfter:  money.FromDecimal(targetAvailableAfter),
		FrozenBefore:    money.FromDecimal(targetFrozenBefore),
		FrozenAfter:     money.FromDecimal(targetFrozenBefore),
		BalanceBefore:   money.FromDecimal(targetAvailableBefore.Add(targetFrozenBefore).Round(2)),
		BalanceAfter:    money.FromDecimal(targetAvailableAfter.Add(targetFrozenBefore).Round(2)),
		Currency:        freezeCurrency,
		Reference:       targetRef,
		Remark:          cleanRemark(input.Remark, "C2C结算划入"),
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := repo.CreateTransaction(targetTxn); err != nil {
		return walletcontract.ErrTransactionCreateFailed
	}
	return nil
}
