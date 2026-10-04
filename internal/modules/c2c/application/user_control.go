package application

import (
	c2ccontract "github.com/Aether-v1/hcz/internal/modules/c2c/contract"
)

// GetUserC2CStatus 查询用户 C2C 状态。
func (s *Service) GetUserC2CStatus(userID uint) (*c2ccontract.UserC2CStatusView, error) {
	if s.userAdmin == nil {
		return nil, c2ccontract.ErrUserInactive
	}
	u, err := s.userAdmin.GetByID(userID)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, c2ccontract.ErrUserInactive
	}
	return &c2ccontract.UserC2CStatusView{
		UserID:    u.ID,
		C2CBanned: u.C2CBanned,
		Status:    u.Status,
	}, nil
}

// AdminDisableUserC2C 禁用用户 C2C：置 users.c2c_banned=true。
// 被禁用用户不能发布挂单/创建新 Trade（CheckListingEligibility / CreateTrade 已校验 C2CBanned），
// 但已有 Trade 仍可正常 mark-paid/confirm/cancel/dispute。
func (s *Service) AdminDisableUserC2C(adminID, userID uint, reason string) (*c2ccontract.UserC2CStatusView, error) {
	if adminID == 0 || userID == 0 {
		return nil, c2ccontract.ErrUserInactive
	}
	return s.setUserC2CBanned(userID, true)
}

// AdminEnableUserC2C 启用用户 C2C：置 users.c2c_banned=false。
func (s *Service) AdminEnableUserC2C(adminID, userID uint) (*c2ccontract.UserC2CStatusView, error) {
	if adminID == 0 || userID == 0 {
		return nil, c2ccontract.ErrUserInactive
	}
	return s.setUserC2CBanned(userID, false)
}

func (s *Service) setUserC2CBanned(userID uint, banned bool) (*c2ccontract.UserC2CStatusView, error) {
	if s.userAdmin == nil {
		return nil, c2ccontract.ErrUserInactive
	}
	u, err := s.userAdmin.GetByID(userID)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, c2ccontract.ErrUserInactive
	}
	if u.C2CBanned == banned {
		return &c2ccontract.UserC2CStatusView{UserID: u.ID, C2CBanned: u.C2CBanned, Status: u.Status}, nil
	}
	u.C2CBanned = banned
	if err := s.userAdmin.Update(u); err != nil {
		return nil, err
	}
	return &c2ccontract.UserC2CStatusView{UserID: u.ID, C2CBanned: u.C2CBanned, Status: u.Status}, nil
}
