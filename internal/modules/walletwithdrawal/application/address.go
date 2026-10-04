package application

import (
	"strings"

	withdrawalcontract "github.com/Aether-v1/hcz/internal/modules/walletwithdrawal/contract"
	withdrawaldomain "github.com/Aether-v1/hcz/internal/modules/walletwithdrawal/domain"
)

// ListAddresses 用户列出自己的提现地址。
func (s *Service) ListAddresses(userID uint) ([]withdrawaldomain.Address, error) {
	if userID == 0 {
		return nil, withdrawalcontract.ErrWithdrawalNotFound
	}
	return s.repo.ListAddressesByUserID(userID)
}

// CreateAddress 用户新增提现地址。
func (s *Service) CreateAddress(input withdrawalcontract.CreateAddressInput) (*withdrawaldomain.Address, error) {
	if input.UserID == 0 {
		return nil, withdrawalcontract.ErrWithdrawalNotFound
	}
	network := strings.ToUpper(strings.TrimSpace(input.Network))
	address := strings.TrimSpace(input.Address)
	if network == "" || address == "" {
		return nil, withdrawalcontract.ErrInvalidAddress
	}
	if network == "TRC20" && !IsValidTRC20Address(address) {
		return nil, withdrawalcontract.ErrInvalidAddress
	}
	// 重复地址校验
	existing, err := s.repo.GetAddressByUserIDNetworkAddress(input.UserID, network, address)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, withdrawalcontract.ErrAddressDuplicate
	}
	a := &withdrawaldomain.Address{
		UserID:  input.UserID,
		Network: network,
		Address: address,
		Label:   strings.TrimSpace(input.Label),
	}
	if err := s.repo.CreateAddress(a); err != nil {
		return nil, err
	}
	return a, nil
}

// DeleteAddress 用户删除提现地址（仅本人）。
func (s *Service) DeleteAddress(userID, id uint) error {
	if userID == 0 || id == 0 {
		return withdrawalcontract.ErrAddressNotFound
	}
	a, err := s.repo.GetAddressByID(id)
	if err != nil {
		return err
	}
	if a == nil || a.UserID != userID {
		return withdrawalcontract.ErrAddressNotFound
	}
	return s.repo.DeleteAddress(id)
}

// SetDefaultAddress 用户设置默认地址（仅本人）。
func (s *Service) SetDefaultAddress(userID, id uint) (*withdrawaldomain.Address, error) {
	if userID == 0 || id == 0 {
		return nil, withdrawalcontract.ErrAddressNotFound
	}
	a, err := s.repo.GetAddressByID(id)
	if err != nil {
		return nil, err
	}
	if a == nil || a.UserID != userID {
		return nil, withdrawalcontract.ErrAddressNotFound
	}
	if err := s.repo.ClearDefaultAddress(userID); err != nil {
		return nil, err
	}
	a.IsDefault = true
	if err := s.repo.UpdateAddress(a); err != nil {
		return nil, err
	}
	return a, nil
}
