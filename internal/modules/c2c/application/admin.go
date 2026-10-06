package application

import (
	c2ccontract "github.com/Aether-v1/hcz/internal/modules/c2c/contract"
	c2cdomain "github.com/Aether-v1/hcz/internal/modules/c2c/domain"
)

// Overview 返回 C2C 概览统计。
func (s *Service) Overview() (c2ccontract.OverviewStats, error) {
	return s.repo.OverviewStats()
}

// ListAdminListings 管理员挂单列表。
func (s *Service) ListAdminListings(filter c2ccontract.AdminListingFilter) ([]c2cdomain.Listing, int64, error) {
	return s.repo.ListAdminListings(filter)
}

// GetListingByIDAdmin 管理员查看挂单详情。
func (s *Service) GetListingByIDAdmin(id uint) (*c2cdomain.Listing, error) {
	return s.repo.GetListingByID(id)
}

// AdminCloseListing 管理员强制关闭挂单（不校验卖家归属）。
// 新模型：复用 closeListingWithinTx —— 行锁 listing、检查 active trade、Unfreeze 剩余 frozen、status=closed。
// 不能只改 status，否则卖家冻结资金会泄漏。
func (s *Service) AdminCloseListing(adminID, id uint) (*c2cdomain.Listing, error) {
	if id == 0 {
		return nil, c2ccontract.ErrListingNotFound
	}
	// 预检查存在性与当前状态（事务内会再行锁校验一次）。
	l, err := s.repo.GetListingByID(id)
	if err != nil {
		return nil, err
	}
	if l == nil {
		return nil, c2ccontract.ErrListingNotFound
	}
	if l.Status == listingClosed {
		return l, nil
	}
	var out *c2cdomain.Listing
	err = s.uow.WithinTransaction(func(tx c2ccontract.Transaction) error {
		closed, cerr := s.closeListingWithinTx(tx, id)
		if cerr != nil {
			return cerr
		}
		out = closed
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ListAdminTrades 管理员交易列表。
func (s *Service) ListAdminTrades(filter c2ccontract.AdminTradeFilter) ([]c2cdomain.Trade, int64, error) {
	return s.repo.ListAdminTrades(filter)
}

// GetTradeByIDAdmin 管理员查看交易详情（含支付方式快照）。
func (s *Service) GetTradeByIDAdmin(id uint) (*c2cdomain.Trade, error) {
	t, err := s.repo.GetTradeByID(id)
	if err != nil {
		return nil, err
	}
	if t == nil {
		return nil, c2ccontract.ErrTradeNotFound
	}
	return t, nil
}

// ListDisputes 管理员申诉列表。
func (s *Service) ListDisputes(filter c2ccontract.DisputeListFilter) ([]c2cdomain.Dispute, int64, error) {
	return s.repo.ListDisputes(filter)
}

// GetDisputeByID 管理员申诉详情。
func (s *Service) GetDisputeByID(id uint) (*c2cdomain.Dispute, error) {
	d, err := s.repo.GetDisputeByID(id)
	if err != nil {
		return nil, err
	}
	if d == nil {
		return nil, c2ccontract.ErrDisputeNotFound
	}
	return d, nil
}
