package application

import (
	withdrawalcontract "github.com/Aether-v1/hcz/internal/modules/walletwithdrawal/contract"
	withdrawaldomain "github.com/Aether-v1/hcz/internal/modules/walletwithdrawal/domain"
)

// ListUserWithdrawals 用户分页查询自己的提现单（IDOR fail-closed：user_id 强制过滤）。
func (s *Service) ListUserWithdrawals(userID uint, page, pageSize int, status string) ([]withdrawaldomain.Withdrawal, int64, error) {
	if userID == 0 {
		return nil, 0, withdrawalcontract.ErrWithdrawalNotFound
	}
	return s.repo.ListWithdrawals(withdrawalcontract.WithdrawalListFilter{
		Page:     page,
		PageSize: pageSize,
		UserID:   userID,
		Status:   status,
	})
}

// GetUserWithdrawalDetail 用户查询提现单详情，仅本人（IDOR fail-closed）。
func (s *Service) GetUserWithdrawalDetail(userID, id uint) (*withdrawaldomain.Withdrawal, error) {
	if userID == 0 || id == 0 {
		return nil, withdrawalcontract.ErrWithdrawalNotFound
	}
	w, err := s.repo.GetWithdrawalByID(id)
	if err != nil {
		return nil, err
	}
	if w == nil || w.UserID != userID {
		return nil, withdrawalcontract.ErrWithdrawalNotFound
	}
	return w, nil
}

// GetAdminWithdrawalDetail 后台查询提现单详情。
func (s *Service) GetAdminWithdrawalDetail(id uint) (*withdrawaldomain.Withdrawal, error) {
	if id == 0 {
		return nil, withdrawalcontract.ErrWithdrawalNotFound
	}
	w, err := s.repo.GetWithdrawalByID(id)
	if err != nil {
		return nil, err
	}
	if w == nil {
		return nil, withdrawalcontract.ErrWithdrawalNotFound
	}
	return w, nil
}

// ListAdminWithdrawals 后台分页查询提现单。
func (s *Service) ListAdminWithdrawals(filter withdrawalcontract.AdminWithdrawalListFilter) ([]withdrawaldomain.Withdrawal, int64, error) {
	return s.repo.ListAdminWithdrawals(filter)
}
