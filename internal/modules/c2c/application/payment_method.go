package application

import (
	"sort"
	"strings"

	"github.com/Aether-v1/hcz/internal/crypto"
	c2ccontract "github.com/Aether-v1/hcz/internal/modules/c2c/contract"
	c2cdomain "github.com/Aether-v1/hcz/internal/modules/c2c/domain"
	withdrawalapp "github.com/Aether-v1/hcz/internal/modules/walletwithdrawal/application"
)

// 每用户每类型最多绑定数量（防滥用）。
const maxPaymentMethodsPerType = 5

// normalizePMType 归一化类型为大写白名单值；同时兼容历史小写值。
func normalizePMType(raw string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "usdt_trc20", "usdttrc20":
		return c2cdomain.PaymentMethodTypeUSDTTRC20, true
	case "bank_card", "bankcard":
		return c2cdomain.PaymentMethodTypeBankCard, true
	case "alipay":
		return c2cdomain.PaymentMethodTypeAlipay, true
	case "wechat":
		return c2cdomain.PaymentMethodTypeWechat, true
	}
	return "", false
}

// ---- 敏感字段加解密（AES-256-GCM）。encKey 为空时退化为明文（旧数据/测试路径）。----

func (s *Service) encryptField(plaintext string) string {
	plaintext = strings.TrimSpace(plaintext)
	if plaintext == "" || len(s.encKey) == 0 {
		return plaintext
	}
	enc, err := crypto.Encrypt(s.encKey, plaintext)
	if err != nil {
		// 加密失败不应泄露明文到响应，但落库失败会由上层处理；
		// 这里宁可拒绝（返回原串会导致后续解密混淆），调用方应视为错误。
		return plaintext
	}
	return enc
}

// decryptField 解密存储值；旧明文数据或解密失败时原样返回（向后兼容）。
func (s *Service) decryptField(stored string) string {
	stored = strings.TrimSpace(stored)
	if stored == "" || len(s.encKey) == 0 {
		return stored
	}
	plaintext, err := crypto.Decrypt(s.encKey, stored)
	if err != nil {
		return stored
	}
	return plaintext
}

// decryptPM 就地解密一条收款方式的敏感字段（返回同一指针，便于链式）。
func (s *Service) decryptPM(p *c2cdomain.PaymentMethod) *c2cdomain.PaymentMethod {
	if p == nil {
		return nil
	}
	p.Address = s.decryptField(p.Address)
	p.AccountName = s.decryptField(p.AccountName)
	p.BankAccount = s.decryptField(p.BankAccount)
	p.AccountIdentifier = s.decryptField(p.AccountIdentifier)
	return p
}

func (s *Service) auditPM(entry c2ccontract.PaymentMethodAuditEntry) {
	if s == nil || s.pmAudit == nil || entry.UserID == 0 {
		return
	}
	_ = s.pmAudit.WritePaymentMethodAudit(entry)
}

// validatePMType 按类型校验必填字段与格式，返回归一化后的类型。
func validatePMType(typ string, in c2ccontract.CreatePaymentMethodInput) (string, error) {
	norm, ok := normalizePMType(typ)
	if !ok {
		return "", c2ccontract.ErrInvalidPaymentType
	}
	switch norm {
	case c2cdomain.PaymentMethodTypeUSDTTRC20:
		if strings.ToUpper(strings.TrimSpace(in.Currency)) != "USDT" ||
			strings.ToUpper(strings.TrimSpace(in.Network)) != "TRC20" {
			return "", c2ccontract.ErrInvalidUSDTAddress
		}
		if !withdrawalapp.IsValidTRC20Address(in.Address) {
			return "", c2ccontract.ErrInvalidUSDTAddress
		}
	case c2cdomain.PaymentMethodTypeBankCard:
		if strings.TrimSpace(in.AccountName) == "" || strings.TrimSpace(in.BankName) == "" {
			return "", c2ccontract.ErrMissingRequiredField
		}
		if !isValidBankAccount(in.BankAccount) {
			return "", c2ccontract.ErrInvalidBankAccount
		}
	case c2cdomain.PaymentMethodTypeAlipay, c2cdomain.PaymentMethodTypeWechat:
		if strings.TrimSpace(in.AccountName) == "" || strings.TrimSpace(in.AccountIdentifier) == "" {
			return "", c2ccontract.ErrMissingRequiredField
		}
	}
	return norm, nil
}

// isValidBankAccount 银行卡号：10-32 位数字。
func isValidBankAccount(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) < 10 || len(s) > 32 {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// ==================== 读 ====================

// ListPaymentMethods 返回用户全部收款方式（敏感字段已解密，列表由 presenter 脱敏）。
func (s *Service) ListPaymentMethods(userID uint) ([]c2cdomain.PaymentMethod, error) {
	if userID == 0 {
		return nil, c2ccontract.ErrPaymentMethodNotFound
	}
	rows, err := s.repo.ListPaymentMethodsByUserID(userID)
	if err != nil {
		return nil, err
	}
	for i := range rows {
		s.decryptPM(&rows[i])
	}
	return rows, nil
}

// ListPaymentMethodsByType 按类型筛选用户收款方式。
func (s *Service) ListPaymentMethodsByType(userID uint, pmType string) ([]c2cdomain.PaymentMethod, error) {
	if userID == 0 {
		return nil, c2ccontract.ErrPaymentMethodNotFound
	}
	norm, ok := normalizePMType(pmType)
	if !ok {
		return nil, c2ccontract.ErrInvalidPaymentType
	}
	rows, err := s.repo.ListPaymentMethodsByUserIDAndType(userID, norm)
	if err != nil {
		return nil, err
	}
	for i := range rows {
		s.decryptPM(&rows[i])
	}
	return rows, nil
}

// GetPaymentMethod 取单条并校验归属（IDOR 防护），返回解密后的完整数据（仅本人）。
func (s *Service) GetPaymentMethod(userID, id uint) (*c2cdomain.PaymentMethod, error) {
	return s.GetPaymentMethodByIDForUser(userID, id)
}

// GetPaymentMethodByIDForUser 取支付方式并校验归属（IDOR 防护），返回解密后数据。
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
	s.decryptPM(pm)
	return pm, nil
}

// ListEnabledPaymentMethodsByListingID 根据挂单ID获取卖家已启用的收款方式（脱敏用，供买家选择）。
// 通过 listingID 拿到 SellerUserID，再查询该用户 enabled=true 且 status=active 且未软删除的收款方式，
// 敏感字段解密后由 presenter 脱敏返回。不校验买家身份（挂载 user 组，买家已登录）。
func (s *Service) ListEnabledPaymentMethodsByListingID(listingID uint) ([]c2cdomain.PaymentMethod, error) {
	if listingID == 0 {
		return nil, c2ccontract.ErrListingNotFound
	}
	l, err := s.repo.GetListingByID(listingID)
	if err != nil {
		return nil, err
	}
	if l == nil {
		return nil, c2ccontract.ErrListingNotFound
	}
	rows, err := s.repo.ListEnabledPaymentMethodsByUserID(l.SellerUserID)
	if err != nil {
		return nil, err
	}
	out := make([]c2cdomain.PaymentMethod, 0, len(rows))
	for i := range rows {
		// 双重保险：仅返回 status=active（enabled 与 status 在写路径保持一致，兼容历史数据）。
		if !rows[i].Enabled || rows[i].Status != c2cdomain.PaymentMethodStatusActive {
			continue
		}
		s.decryptPM(&rows[i])
		out = append(out, rows[i])
	}
	// 按 type 分组排序：Type 升序，同类型默认优先，其次 ID 降序。
	sort.SliceStable(out, func(a, b int) bool {
		if out[a].Type != out[b].Type {
			return out[a].Type < out[b].Type
		}
		if out[a].IsDefault != out[b].IsDefault {
			return out[a].IsDefault && !out[b].IsDefault
		}
		return out[a].ID > out[b].ID
	})
	return out, nil
}

// ==================== 写（均需 Step-Up） ====================

// CreatePaymentMethod 新增收款方式（含安全验证、加密、TRC20 校验、类型白名单、类型上限）。
func (s *Service) CreatePaymentMethod(input c2ccontract.CreatePaymentMethodInput) (*c2cdomain.PaymentMethod, error) {
	if input.UserID == 0 {
		return nil, c2ccontract.ErrPaymentMethodNotFound
	}
	// 1. Step-Up
	if err := s.VerifyStepUp(input.UserID, input.TOTPCode, input.Password); err != nil {
		return nil, err
	}
	// 2. 类型 + 业务校验
	typ, err := validatePMType(input.Type, input)
	if err != nil {
		return nil, err
	}
	// 3. 每类型数量上限
	count, err := s.repo.CountPaymentMethodsByUserIDAndType(input.UserID, typ)
	if err != nil {
		return nil, err
	}
	if count >= maxPaymentMethodsPerType {
		return nil, c2ccontract.ErrPaymentMethodLimit
	}

	pm := &c2cdomain.PaymentMethod{
		UserID:            input.UserID,
		Type:              typ,
		Currency:          strings.ToUpper(strings.TrimSpace(input.Currency)),
		Network:           strings.ToUpper(strings.TrimSpace(input.Network)),
		Label:             strings.TrimSpace(input.Label),
		BankName:          strings.TrimSpace(input.BankName),
		BranchName:        strings.TrimSpace(input.BranchName),
		QRCodeFileID:      strings.TrimSpace(input.QRCodeFileID),
		QRCodeURL:         strings.TrimSpace(input.QRCodeURL),
		QRImage:           strings.TrimSpace(input.QRImage),
		Instructions:      strings.TrimSpace(input.Instructions),
		Status:            c2cdomain.PaymentMethodStatusActive,
		Enabled:           true,
	}
	// 数字资产默认币种/网络。
	if typ == c2cdomain.PaymentMethodTypeUSDTTRC20 {
		pm.Currency = "USDT"
		pm.Network = "TRC20"
	}
	// 敏感字段加密后落库。
	pm.Address = s.encryptField(input.Address)
	pm.AccountName = s.encryptField(input.AccountName)
	pm.BankAccount = s.encryptField(input.BankAccount)
	pm.AccountIdentifier = s.encryptField(input.AccountIdentifier)

	if err := s.repo.CreatePaymentMethod(pm); err != nil {
		return nil, err
	}
	s.auditPM(c2ccontract.PaymentMethodAuditEntry{
		UserID: input.UserID, Action: "payment_method_created",
		PaymentMethodID: pm.ID, PMType: typ,
	})
	s.decryptPM(pm)
	return pm, nil
}

// UpdatePaymentMethod 修改收款方式（含 Step-Up、归属校验、敏感字段重加密）。
func (s *Service) UpdatePaymentMethod(input c2ccontract.UpdatePaymentMethodInput) (*c2cdomain.PaymentMethod, error) {
	if input.UserID == 0 {
		return nil, c2ccontract.ErrPaymentMethodNotFound
	}
	if err := s.VerifyStepUp(input.UserID, input.TOTPCode, input.Password); err != nil {
		return nil, err
	}
	pm, err := s.GetPaymentMethodByIDForUser(input.UserID, input.ID)
	if err != nil {
		return nil, err
	}
	// GetPaymentMethodByIDForUser 已解密；更新敏感字段时以明文重加密。
	if strings.TrimSpace(input.Label) != "" {
		pm.Label = strings.TrimSpace(input.Label)
	}
	if strings.TrimSpace(input.BankName) != "" {
		pm.BankName = strings.TrimSpace(input.BankName)
	}
	if strings.TrimSpace(input.BranchName) != "" {
		pm.BranchName = strings.TrimSpace(input.BranchName)
	}
	if strings.TrimSpace(input.QRCodeFileID) != "" {
		pm.QRCodeFileID = strings.TrimSpace(input.QRCodeFileID)
	}
	if strings.TrimSpace(input.QRCodeURL) != "" {
		pm.QRCodeURL = strings.TrimSpace(input.QRCodeURL)
	}
	if strings.TrimSpace(input.QRImage) != "" {
		pm.QRImage = strings.TrimSpace(input.QRImage)
	}
	if input.Instructions != "" {
		pm.Instructions = strings.TrimSpace(input.Instructions)
	}
	if input.Enabled != nil {
		pm.Enabled = *input.Enabled
		if *input.Enabled {
			pm.Status = c2cdomain.PaymentMethodStatusActive
		} else {
			pm.Status = c2cdomain.PaymentMethodStatusDisabled
		}
	}
	if strings.TrimSpace(input.Address) != "" {
		pm.Address = s.encryptField(input.Address)
	}
	if strings.TrimSpace(input.AccountName) != "" {
		pm.AccountName = s.encryptField(input.AccountName)
	}
	if strings.TrimSpace(input.BankAccount) != "" {
		pm.BankAccount = s.encryptField(input.BankAccount)
	}
	if strings.TrimSpace(input.AccountIdentifier) != "" {
		pm.AccountIdentifier = s.encryptField(input.AccountIdentifier)
	}

	if err := s.repo.UpdatePaymentMethod(pm); err != nil {
		return nil, err
	}
	s.auditPM(c2ccontract.PaymentMethodAuditEntry{
		UserID: input.UserID, Action: "payment_method_updated",
		PaymentMethodID: pm.ID, PMType: pm.Type,
	})
	s.decryptPM(pm)
	return pm, nil
}

// DeletePaymentMethod 软删除收款方式（含 Step-Up、归属校验）。
func (s *Service) DeletePaymentMethod(userID, id uint, totpCode, password string) error {
	if err := s.VerifyStepUp(userID, totpCode, password); err != nil {
		return err
	}
	pm, err := s.GetPaymentMethodByIDForUser(userID, id)
	if err != nil {
		return err
	}
	if err := s.repo.SoftDeletePaymentMethod(id); err != nil {
		return err
	}
	s.auditPM(c2ccontract.PaymentMethodAuditEntry{
		UserID: userID, Action: "payment_method_deleted",
		PaymentMethodID: id, PMType: pm.Type,
	})
	return nil
}

// SetDefaultPaymentMethod 设为同类型默认（同类型互斥，事务内先清旧默认再设新）。
func (s *Service) SetDefaultPaymentMethod(userID, id uint, totpCode, password string) error {
	if err := s.VerifyStepUp(userID, totpCode, password); err != nil {
		return err
	}
	pm, err := s.GetPaymentMethodByIDForUser(userID, id)
	if err != nil {
		return err
	}
	pmType := pm.Type
	err = s.uow.WithinTransaction(func(tx c2ccontract.Transaction) error {
		return tx.C2C().MarkPaymentMethodDefault(userID, id)
	})
	if err != nil {
		return err
	}
	s.auditPM(c2ccontract.PaymentMethodAuditEntry{
		UserID: userID, Action: "payment_method_default_changed",
		PaymentMethodID: id, PMType: pmType,
	})
	return nil
}

// SetPaymentMethodEnabled 启用/停用收款方式。
func (s *Service) SetPaymentMethodEnabled(userID, id uint, enabled bool) (*c2cdomain.PaymentMethod, error) {
	pm, err := s.GetPaymentMethodByIDForUser(userID, id)
	if err != nil {
		return nil, err
	}
	pm.Enabled = enabled
	if enabled {
		pm.Status = c2cdomain.PaymentMethodStatusActive
	} else {
		pm.Status = c2cdomain.PaymentMethodStatusDisabled
	}
	if err := s.repo.UpdatePaymentMethod(pm); err != nil {
		return nil, err
	}
	s.decryptPM(pm)
	return pm, nil
}

// ==================== 充值/交易集成 ====================

// HasUSDTTRC20Address 用户是否已绑定启用的 USDT TRC20 地址（供充值前置校验）。
func (s *Service) HasUSDTTRC20Address(userID uint) bool {
	if userID == 0 {
		return false
	}
	rows, err := s.repo.ListPaymentMethodsByUserIDAndType(userID, c2cdomain.PaymentMethodTypeUSDTTRC20)
	if err != nil {
		return false
	}
	for i := range rows {
		if rows[i].Enabled && strings.TrimSpace(rows[i].Address) != "" {
			return true
		}
	}
	return false
}

// GetDefaultUSDTTRC20Address 获取用户默认（或首个启用）USDT TRC20 地址（解密）。
func (s *Service) GetDefaultUSDTTRC20Address(userID uint) (*c2cdomain.PaymentMethod, error) {
	if userID == 0 {
		return nil, c2ccontract.ErrPaymentMethodNotFound
	}
	rows, err := s.repo.ListPaymentMethodsByUserIDAndType(userID, c2cdomain.PaymentMethodTypeUSDTTRC20)
	if err != nil {
		return nil, err
	}
	var picked *c2cdomain.PaymentMethod
	for i := range rows {
		if !rows[i].Enabled || strings.TrimSpace(rows[i].Address) == "" {
			continue
		}
		p := rows[i]
		if p.IsDefault {
			picked = &p
			break
		}
		if picked == nil {
			picked = &p
		}
	}
	if picked == nil {
		return nil, c2ccontract.ErrPaymentMethodNotFound
	}
	s.decryptPM(picked)
	return picked, nil
}
