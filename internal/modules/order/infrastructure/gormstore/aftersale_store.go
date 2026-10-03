package gormstore

import (
	"errors"

	orderdomain "github.com/Aether-v1/hcz/internal/modules/order/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// P1 after-sale：与订单共享同一 gorm 连接/事务，不独立开事务。
func (r *Store) CreateAfterSaleTicket(t *orderdomain.AfterSaleTicket) error {
	return r.db.Create(t).Error
}

func (r *Store) GetAfterSaleTicketByOrderID(orderID uint) (*orderdomain.AfterSaleTicket, error) {
	var t orderdomain.AfterSaleTicket
	if err := r.db.Where("order_id = ?", orderID).Order("id DESC").First(&t).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &t, nil
}

// LockAfterSalePendingByOrderIDForUpdate 事务内 SELECT ... FOR UPDATE。
func (r *Store) LockAfterSalePendingByOrderIDForUpdate(orderID uint) (*orderdomain.AfterSaleTicket, error) {
	var t orderdomain.AfterSaleTicket
	if err := r.db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("order_id = ? AND status = ?", orderID, orderdomain.AfterSaleStatusPending).
		First(&t).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &t, nil
}

func (r *Store) UpdateAfterSaleTicket(id uint, updates map[string]interface{}) error {
	return r.db.Model(&orderdomain.AfterSaleTicket{}).Where("id = ?", id).Updates(updates).Error
}
