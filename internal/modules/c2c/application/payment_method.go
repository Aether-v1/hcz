package application

import (
	"strings"
	"time"

	c2ccontract "github.com/Aether-v1/hcz/internal/modules/c2c/contract"
	c2cdomain "github.com/Aether-v1/hcz/internal/modules/c2c/domain"
)

// 允许的支付方式类型。
var allowedPaymentTypes = map[string]struct{}{
	"bank_card": {},
	"alipay":    {},
	"wechat":    {},
}

// ListPaymentMethods 返回用户全部支付方式（脱敏在 presenter 层完成）。
func (s *Service) ListPaymentMethods(userID uint) ([]c2cdomain.PaymentMethod, error) {
	if userID == 0 {
		return nil, c2ccontract.ErrPaymentMethodNotFound
	}
	return s.repo.ListPaymentMethodsByUserID(userID)
}

// CreatePaymentMethod 新增支付方式。
func (s *Service) CreatePaymentMethod(input c2ccontract.CreatePaymentMethodInput) (*c2cdomain.PaymentMethod, error) {
	if input.UserID == 0 {
		return nil, c2ccontract.ErrPaymentMethodNotFound
	}
	typ := strings.ToLower(strings.TrimSpace(input.Type))
	if _, ok := allowedPaymentTypes[typ]; !ok {
		return nil, c2ccontract.ErrInvalidPaymentType
	}
	if strings.TrimSpace(input.AccountIdentifier) == "" {
		return nil, c2ccontract.ErrPaymentMethodNotFound
	}
	now := time.Now()
	pm := &c2cdomain.PaymentMethod{
		UserID:            input.UserID,
		Type:              typ,
		AccountName:       strings.TrimSpace(input.AccountName),
		AccountIdentifier: strings.TrimSpace(input.AccountIdentifier),
		QRImage:           strings.TrimSpace(input.QRImage),
		Instructions:      strings.TrimSpace(input.Instructions),
		Enabled:           true,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := s.repo.CreatePaymentMethod(pm); err != nil {
		return nil, err
	}
	return pm, nil
}

// GetPaymentMethodByIDForUser 取支付方式并校验归属（IDOR 防护）。
func (s *Service) GetPaymentMethodByIDForUser(userID, id uint) (*c2cdomain.PaymentMethod, error) {
	if userID == 0 || id == 0 {
		return nil, c2ccontract.ErrPaymentMethodNotFound
	}
	pm, err := s.repo.GetPaymentMethodByID(id)
	if err != nil {
		return nil, err
	}
	if pm == nil || pm.UserID != userID {
		return nil, c2ccontract.ErrPaymentMethodNotFound
	}
	return pm, nil
}

// UpdatePaymentMethod 修改自己的支付方式。
func (s *Service) UpdatePaymentMethod(input c2ccontract.UpdatePaymentMethodInput) (*c2cdomain.PaymentMethod, error) {
	pm, err := s.GetPaymentMethodByIDForUser(input.UserID, input.ID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(input.AccountIdentifier) != "" {
		pm.AccountIdentifier = strings.TrimSpace(input.AccountIdentifier)
	}
	if strings.TrimSpace(input.AccountName) != "" {
		pm.AccountName = strings.TrimSpace(input.AccountName)
	}
	if strings.TrimSpace(input.QRImage) != "" {
		pm.QRImage = strings.TrimSpace(input.QRImage)
	}
	pm.Instructions = strings.TrimSpace(input.Instructions)
	pm.UpdatedAt = time.Now()
	if err := s.repo.UpdatePaymentMethod(pm); err != nil {
		return nil, err
	}
	return pm, nil
}

// DeletePaymentMethod 删除自己的支付方式。
func (s *Service) DeletePaymentMethod(userID, id uint) error {
	if _, err := s.GetPaymentMethodByIDForUser(userID, id); err != nil {
		return err
	}
	return s.repo.SoftDeletePaymentMethod(id)
}

// SetPaymentMethodEnabled 启用/停用自己的支付方式。
func (s *Service) SetPaymentMethodEnabled(userID, id uint, enabled bool) (*c2cdomain.PaymentMethod, error) {
	pm, err := s.GetPaymentMethodByIDForUser(userID, id)
	if err != nil {
		return nil, err
	}
	pm.Enabled = enabled
	pm.UpdatedAt = time.Now()
	if err := s.repo.UpdatePaymentMethod(pm); err != nil {
		return nil, err
	}
	return pm, nil
}
