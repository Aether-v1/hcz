package application

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/Aether-v1/hcz/internal/constants"
	notificationcontract "github.com/Aether-v1/hcz/internal/modules/notification/contract"
	settingssecurity "github.com/Aether-v1/hcz/internal/modules/settings/schema/security"
	withdrawalcontract "github.com/Aether-v1/hcz/internal/modules/walletwithdrawal/contract"

	"github.com/shopspring/decimal"
)

// Options 装配提现用例所需依赖。
type Options struct {
	Repository withdrawalcontract.Repository
	UnitOfWork withdrawalcontract.UnitOfWork
	TOTP       withdrawalcontract.TOTPVerifier
	Config     withdrawalcontract.ConfigReader
	Notifier   withdrawalcontract.Notifier
	Users      withdrawalcontract.UserReader
}

// Service 提现用例入口。
type Service struct {
	repo     withdrawalcontract.Repository
	uow      withdrawalcontract.UnitOfWork
	totp     withdrawalcontract.TOTPVerifier
	config   withdrawalcontract.ConfigReader
	notifier withdrawalcontract.Notifier
	users    withdrawalcontract.UserReader
}

// NewService 创建提现 Service。
func NewService(options Options) *Service {
	return &Service{
		repo:     options.Repository,
		uow:      options.UnitOfWork,
		totp:     options.TOTP,
		config:   options.Config,
		notifier: options.Notifier,
		users:    options.Users,
	}
}

// loadConfig 读取并归一化提现配置；读取失败回退默认（disabled）。
func (s *Service) loadConfig() settingssecurity.WithdrawalConfig {
	if s.config == nil {
		return settingssecurity.DefaultWithdrawalConfig()
	}
	cfg, err := s.config.GetWithdrawalConfig()
	if err != nil {
		return settingssecurity.DefaultWithdrawalConfig()
	}
	return cfg
}

// parseDecimal 解析配置中的金额字符串为 decimal。
func parseConfigDecimal(raw string) decimal.Decimal {
	parsed, err := decimal.NewFromString(strings.TrimSpace(raw))
	if err != nil {
		return decimal.Zero
	}
	return parsed.Round(2)
}

// ---- TRC20 (Base58Check) 本地地址校验，不调用链上 API ----

const base58Alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

var base58Idx = func() [128]int {
	var idx [128]int
	for i := range idx {
		idx[i] = -1
	}
	for i, c := range base58Alphabet {
		idx[c] = i
	}
	return idx
}()

// IsValidTRC20Address 本地 Base58Check 校验：长度 34、首字符 T、checksum 正确。
func IsValidTRC20Address(address string) bool {
	address = strings.TrimSpace(address)
	if len(address) != 34 {
		return false
	}
	if !strings.HasPrefix(address, "T") {
		return false
	}
	for _, c := range address {
		if c > 127 || base58Idx[c] < 0 {
			return false
		}
	}
	decoded, err := base58Decode(address)
	if err != nil {
		return false
	}
	// decoded = 21 字节 payload + 4 字节 checksum = 25 字节
	if len(decoded) != 25 {
		return false
	}
	payload := decoded[:21]
	checksum := decoded[21:]
	first := sha256.Sum256(payload)
	second := sha256.Sum256(first[:])
	expected := second[:4]
	for i := 0; i < 4; i++ {
		if checksum[i] != expected[i] {
			return false
		}
	}
	return true
}

func base58Decode(input string) ([]byte, error) {
	zero := big.NewInt(0)
	base := big.NewInt(58)
	n := new(big.Int).SetInt64(0)
	for _, c := range input {
		if c > 127 || base58Idx[c] < 0 {
			return nil, errors.New("invalid base58 char")
		}
		n.Mul(n, base)
		n.Add(n, big.NewInt(int64(base58Idx[c])))
	}
	raw := n.Bytes()
	// 前导 '1' 映射为前导 0x00
	pad := 0
	for _, c := range input {
		if c == '1' {
			pad++
		} else {
			break
		}
	}
	result := make([]byte, 0, pad+len(raw))
	for i := 0; i < pad; i++ {
		result = append(result, 0x00)
	}
	result = append(result, raw...)
	// 确保 decoded 长度合理
	_ = zero
	return result, nil
}

// withdrawalReference 构造全局唯一 ledger / withdrawal reference。
func withdrawalReference(userID uint, idempotencyKey string) string {
	return fmt.Sprintf("wd:%d:%s", userID, strings.TrimSpace(idempotencyKey))
}

// refundReference 构造退款 ledger reference（幂等）。
func refundReference(withdrawalID uint) string {
	return fmt.Sprintf("wd_refund:%d", withdrawalID)
}

// isAddressBlacklisted 判断地址是否在黑名单。
func isAddressBlacklisted(cfg settingssecurity.WithdrawalConfig, address string) bool {
	addr := strings.TrimSpace(address)
	for _, blocked := range cfg.AddressBlacklist {
		if strings.EqualFold(strings.TrimSpace(blocked), addr) {
			return true
		}
	}
	return false
}

// notifySafely 事务提交后异步通知，失败不阻塞资金主流程。
func (s *Service) notifySafely(eventType string, bizID uint, data map[string]interface{}) {
	if s == nil || s.notifier == nil {
		return
	}
	defer func() { _ = recover() }()
	_ = s.notifier.Enqueue(notificationcontract.EnqueueInput{
		EventType: eventType,
		BizType:   constants.NotificationBizTypeWalletWithdrawal,
		BizID:     bizID,
		Force:     false,
		Data:      data,
	})
}

// dayBounds 返回当日（按 now 截断到 UTC 日界）。
func dayBounds(now time.Time) (time.Time, time.Time) {
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	end := start.Add(24 * time.Hour)
	return start, end
}
