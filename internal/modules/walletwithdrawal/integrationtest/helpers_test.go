package integrationtest

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/Aether-v1/hcz/internal/constants"
	totpapplication "github.com/Aether-v1/hcz/internal/modules/identity/totp/application"
	userdomain "github.com/Aether-v1/hcz/internal/modules/identity/user/domain"
	notificationcontract "github.com/Aether-v1/hcz/internal/modules/notification/contract"
	settingssecurity "github.com/Aether-v1/hcz/internal/modules/settings/schema/security"
	walletdomain "github.com/Aether-v1/hcz/internal/modules/wallet/domain"
	walletgormstore "github.com/Aether-v1/hcz/internal/modules/wallet/infrastructure/gormstore"
	withdrawalapp "github.com/Aether-v1/hcz/internal/modules/walletwithdrawal/application"
	withdrawaldomain "github.com/Aether-v1/hcz/internal/modules/walletwithdrawal/domain"
	withdrawalgormstore "github.com/Aether-v1/hcz/internal/modules/walletwithdrawal/infrastructure/gormstore"
	"github.com/Aether-v1/hcz/internal/shared/money"

	"github.com/shopspring/decimal"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// ---- mocks ----

type mockTOTP struct {
	enabled bool
}

func (m *mockTOTP) VerifyChallengeCode(userID uint, code string) error {
	if !m.enabled {
		return totpapplication.ErrNotEnabled
	}
	if code == "123456" {
		return nil
	}
	return totpapplication.ErrCodeInvalid
}

type mockConfig struct {
	cfg settingssecurity.WithdrawalConfig
}

func (m *mockConfig) GetWithdrawalConfig() (settingssecurity.WithdrawalConfig, error) {
	return m.cfg, nil
}

type mockNotifier struct {
	events []string
}

func (m *mockNotifier) Enqueue(input notificationcontract.EnqueueInput) error {
	m.events = append(m.events, input.EventType)
	return nil
}

// mockUserReader 内存用户表，仅用于新用户冷却校验测试；未登记用户返回 (nil, nil)，与 gorm store 行为一致。
type mockUserReader struct {
	users map[uint]*userdomain.User
}

func (m *mockUserReader) GetByID(userID uint) (*userdomain.User, error) {
	if u, ok := m.users[userID]; ok {
		return u, nil
	}
	return nil, nil
}

// ---- test fixture ----

type fixture struct {
	db       *gorm.DB
	walletDB *walletgormstore.Store
	withDB   *withdrawalgormstore.Store
	svc      *withdrawalapp.Service
	totp     *mockTOTP
	config   *mockConfig
	notifier *mockNotifier
	users    *mockUserReader
}

var dbCounter int

func newFixture(t *testing.T) *fixture {
	t.Helper()
	dbCounter++
	dsn := fmt.Sprintf("file:withdrawal%d?mode=memory&cache=shared", dbCounter)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&walletdomain.Account{}, &walletdomain.Transaction{}, &withdrawaldomain.Withdrawal{}, &withdrawaldomain.Address{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	walletDB := walletgormstore.New(db)
	withDB := withdrawalgormstore.New(db, walletDB)
	totp := &mockTOTP{enabled: true}
	cfg := settingssecurity.WithdrawalConfig{
		Enabled:            true,
		Network:            "TRC20",
		Currency:           "USDT",
		MinAmount:          "10.00",
		MaxAmount:          "10000.00",
		DailyLimit:         "50000.00",
		DailyCountLimit:    100,
		FixedFee:           "1.00",
		PercentageFee:      "0.00",
		Require2FA:         true,
		FirstWithdrawalMax: "100000.00",
	}
	config := &mockConfig{cfg: cfg}
	notifier := &mockNotifier{}
	users := &mockUserReader{users: map[uint]*userdomain.User{}}
	svc := withdrawalapp.NewService(withdrawalapp.Options{
		Repository: withDB,
		UnitOfWork: withDB,
		TOTP:       totp,
		Config:     config,
		Notifier:   notifier,
		Users:      users,
	})
	return &fixture{
		db:       db,
		walletDB: walletDB,
		withDB:   withDB,
		svc:      svc,
		totp:     totp,
		config:   config,
		notifier: notifier,
		users:    users,
	}
}

// setUserCreatedAt 登记一个用户并指定其注册时间（用于新用户冷却测试）。
func (f *fixture) setUserCreatedAt(t *testing.T, userID uint, createdAt time.Time) {
	t.Helper()
	f.users.users[userID] = &userdomain.User{ID: userID, CreatedAt: createdAt}
}

// countWithdrawals 统计某用户的提现单数量（断言副作用）。
func (f *fixture) countWithdrawals(t *testing.T, userID uint) int64 {
	t.Helper()
	var n int64
	if err := f.db.Model(&withdrawaldomain.Withdrawal{}).Where("user_id = ?", userID).Count(&n).Error; err != nil {
		t.Fatalf("count withdrawals: %v", err)
	}
	return n
}

// countLedger 统计某用户的 ledger 记录数量（断言副作用）。
func (f *fixture) countLedger(t *testing.T, userID uint) int64 {
	t.Helper()
	var n int64
	if err := f.db.Model(&walletdomain.Transaction{}).Where("user_id = ?", userID).Count(&n).Error; err != nil {
		t.Fatalf("count ledger: %v", err)
	}
	return n
}

func (f *fixture) setBalance(t *testing.T, userID uint, amount string) {
	t.Helper()
	acc := &walletdomain.Account{
		UserID:           userID,
		AvailableBalance: money.FromDecimal(mustDec(amount)),
		FrozenBalance:    money.FromDecimal(decimal.Zero),
	}
	if err := f.walletDB.CreateAccount(acc); err != nil {
		t.Fatalf("create account: %v", err)
	}
}

func (f *fixture) getBalance(t *testing.T, userID uint) decimal.Decimal {
	t.Helper()
	acc, err := f.walletDB.GetAccountByUserID(userID)
	if err != nil {
		t.Fatalf("get account: %v", err)
	}
	if acc == nil {
		return decimal.Zero
	}
	return acc.AvailableBalance.Decimal
}

func mustDec(s string) decimal.Decimal {
	d, err := decimal.NewFromString(s)
	if err != nil {
		panic(err)
	}
	return d
}

// validTRC20Address 从一个已知 21 字节 payload 构造合法 TRC20 地址用于正向测试。
func validTRC20Address(payloadHex string) string {
	payload, _ := hex.DecodeString(payloadHex)
	checksum := doubleSHA256Checksum(payload)
	full := append(payload, checksum...)
	return base58Encode(full)
}

func doubleSHA256Checksum(payload []byte) []byte {
	first := sha256.Sum256(payload)
	second := sha256.Sum256(first[:])
	return second[:4]
}

const b58Alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

func base58Encode(input []byte) string {
	x := new(big.Int).SetBytes(input)
	base := big.NewInt(58)
	zero := big.NewInt(0)
	mod := new(big.Int)
	var result []byte
	for x.Cmp(zero) > 0 {
		x.DivMod(x, base, mod)
		result = append(result, b58Alphabet[mod.Int64()])
	}
	for _, b := range input {
		if b == 0 {
			result = append(result, '1')
		} else {
			break
		}
	}
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}
	return string(result)
}

var _ = constants.NotificationEventWithdrawalSubmitted
