// Package gormstore 实现工单系统的 GORM 持久化。
//
// 红线：本包仅以只读原始 SQL 校验 biz 归属（orders / wallet_withdrawals /
// c2c_trades / wallet_recharge_orders），绝不 import 任何资金业务模块的 service。
package gormstore

import (
	"errors"

	supportcontract "github.com/Aether-v1/hcz/internal/modules/supportticket/contract"
	"gorm.io/gorm"
)

// Store 同时实现 Repository 与 UnitOfWork。
type Store struct {
	db *gorm.DB
}

var (
	_ supportcontract.Repository = (*Store)(nil)
	_ supportcontract.UnitOfWork = (*Store)(nil)
)

// New 创建工单 Store。
func New(db *gorm.DB) *Store {
	return &Store{db: db}
}

// Bind 返回绑定到新事务 tx 的 Store。
func (s *Store) Bind(tx *gorm.DB) *Store {
	if tx == nil {
		return s
	}
	return New(tx)
}

type transaction struct {
	store *Store
}

func (tx transaction) Tickets() supportcontract.Repository { return tx.store }

// UseTransaction 用已开启的 *gorm.DB 构造 Transaction。
func UseTransaction(tx *gorm.DB) supportcontract.Transaction {
	if tx == nil {
		return nil
	}
	return transaction{store: New(tx)}
}

// WithinTransaction 开启工单事务，回调内 Tickets() 共享同一 *gorm.DB。
func (s *Store) WithinTransaction(fn func(supportcontract.Transaction) error) error {
	if fn == nil {
		return nil
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		return fn(UseTransaction(tx))
	})
}

// firstErr 把 gorm.ErrRecordNotFound 归一为 nil。
func firstErr(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	return err
}
