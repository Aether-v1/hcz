package wallethttp

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	walletdomain "github.com/Aether-v1/hcz/internal/modules/wallet/domain"
	userauthapp "github.com/Aether-v1/hcz/internal/modules/identity/userauth/application"
	"github.com/Aether-v1/hcz/internal/shared/money"

	"github.com/shopspring/decimal"

	"github.com/gin-gonic/gin"
)

// --- test doubles ---

type fakeChannelWallets struct {
	byUser map[uint]*walletdomain.Account
	getErr error
}

func (f fakeChannelWallets) GetAccount(userID uint) (*walletdomain.Account, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	a, ok := f.byUser[userID]
	if !ok {
		return nil, errors.New("no such account")
	}
	return a, nil
}
func (fakeChannelWallets) ListTransactions(uint, int, int) ([]walletdomain.Transaction, int64, error) {
	return nil, 0, nil
}
func (fakeChannelWallets) ListUserRechargeOrders(uint, int, int, string, string) ([]walletdomain.RechargeOrder, int64, error) {
	return nil, 0, nil
}
func (fakeChannelWallets) StatsUserRechargeOrders(uint, string) (map[string]int64, error) {
	return nil, nil
}
func (fakeChannelWallets) GetRechargeOrderByRechargeNo(uint, string) (*walletdomain.RechargeOrder, error) {
	return nil, nil
}
func (fakeChannelWallets) GetRechargeOrderByPaymentIDAndUser(uint, uint) (*walletdomain.RechargeOrder, error) {
	return nil, nil
}

type fakeProvisioner struct {
	userID       uint
	err          error
	calledWith   string
}

func (p *fakeProvisioner) ProvisionUserID(channelUserID string) (uint, error) {
	p.calledWith = channelUserID
	if p.err != nil {
		return 0, p.err
	}
	return p.userID, nil
}

func amt(s string) money.Amount { return money.FromDecimal(decimal.RequireFromString(s)) }

// decodeChannelData parses the {status_code,msg,data} envelope and returns data map.
func decodeChannelData(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var env struct {
		StatusCode int                    `json:"status_code"`
		Data       map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode envelope: %v body=%s", err, rec.Body.String())
	}
	if env.StatusCode != 0 {
		t.Fatalf("expected status_code 0, got %d body=%s", env.StatusCode, rec.Body.String())
	}
	return env.Data
}

func getWallet(t *testing.T, prov *fakeProvisioner, wallets WalletService, query string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/channel/wallet?"+query, nil)
	h := &ChannelHandler{wallets: wallets, users: prov}
	h.GetWallet(c)
	return rec
}

// --- contract tests ---

func TestChannelWallet_NormalWithFrozen(t *testing.T) {
	wallets := fakeChannelWallets{byUser: map[uint]*walletdomain.Account{
		7: {UserID: 7, AvailableBalance: amt("80"), FrozenBalance: amt("20")},
	}}
	prov := &fakeProvisioner{userID: 7}
	rec := getWallet(t, prov, wallets, "channel_user_id=tg-7")

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200 got %d body=%s", rec.Code, rec.Body.String())
	}
	data := decodeChannelData(t, rec)
	assertEq(t, data, "available_balance", "80.00")
	assertEq(t, data, "frozen_balance", "20.00")
	assertEq(t, data, "total_balance", "100.00")
	assertEq(t, data, "currency", "USDT")
	// legacy fields must be gone.
	if _, ok := data["balance"]; ok {
		t.Fatal("legacy 'balance' field must be removed")
	}
	if cny, ok := data["currency"]; ok && cny == "CNY" {
		t.Fatal("currency must not be CNY")
	}
	if prov.calledWith != "tg-7" {
		t.Fatalf("provisioner got channel id %q, want tg-7", prov.calledWith)
	}
}

func TestChannelWallet_ZeroBalance(t *testing.T) {
	wallets := fakeChannelWallets{byUser: map[uint]*walletdomain.Account{
		9: {UserID: 9, AvailableBalance: amt("0"), FrozenBalance: amt("0")},
	}}
	rec := getWallet(t, &fakeProvisioner{userID: 9}, wallets, "channel_user_id=tg-9")
	data := decodeChannelData(t, rec)
	assertEq(t, data, "available_balance", "0.00")
	assertEq(t, data, "frozen_balance", "0.00")
	assertEq(t, data, "total_balance", "0.00")
	assertEq(t, data, "currency", "USDT")
}

func TestChannelWallet_NoFrozen(t *testing.T) {
	wallets := fakeChannelWallets{byUser: map[uint]*walletdomain.Account{
		3: {UserID: 3, AvailableBalance: amt("55.5"), FrozenBalance: amt("0")},
	}}
	rec := getWallet(t, &fakeProvisioner{userID: 3}, wallets, "channel_user_id=tg-3")
	data := decodeChannelData(t, rec)
	assertEq(t, data, "available_balance", "55.50")
	assertEq(t, data, "frozen_balance", "0.00")
	assertEq(t, data, "total_balance", "55.50")
}

func TestChannelWallet_MissingChannelUserID(t *testing.T) {
	rec := getWallet(t, &fakeProvisioner{}, fakeChannelWallets{}, "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 got %d", rec.Code)
	}
}

func TestChannelWallet_UnboundUser(t *testing.T) {
	prov := &fakeProvisioner{err: userauthapp.ErrNotFound}
	rec := getWallet(t, prov, fakeChannelWallets{}, "channel_user_id=ghost")
	// channelIdentityError maps ErrNotFound -> 404
	if rec.Code != http.StatusNotFound {
		t.Fatalf("want 404 for unbound, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestChannelWallet_AccountStoreError(t *testing.T) {
	wallets := fakeChannelWallets{getErr: errors.New("db down")}
	rec := getWallet(t, &fakeProvisioner{userID: 5}, wallets, "channel_user_id=tg-5")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("want 500 got %d", rec.Code)
	}
}

// IDOR: the resolved HCZ user id comes only from channel_user_id -> provisioner.
// A spoofed user_id query param must be ignored, and distinct channel ids map
// to distinct users so Telegram A can never read Telegram B's wallet.
func TestChannelWallet_IDORUsesChannelIdentityOnly(t *testing.T) {
	wallets := fakeChannelWallets{byUser: map[uint]*walletdomain.Account{
		11: {UserID: 11, AvailableBalance: amt("1"), FrozenBalance: amt("0")},
		22: {UserID: 22, AvailableBalance: amt("999"), FrozenBalance: amt("0")},
	}}
	// Telegram A (channel id "a") deterministically maps to user 11.
	provA := &fakeProvisioner{userID: 11}
	recA := getWallet(t, provA, wallets, "channel_user_id=a&user_id=22") // spoofed user_id must be ignored
	if provA.calledWith != "a" {
		t.Fatalf("provisioner must key off channel_user_id, got %q", provA.calledWith)
	}
	dataA := decodeChannelData(t, recA)
	assertEq(t, dataA, "available_balance", "1.00") // A sees its own wallet, not 999.

	// Telegram B maps to a different user.
	provB := &fakeProvisioner{userID: 22}
	recB := getWallet(t, provB, wallets, "channel_user_id=b")
	dataB := decodeChannelData(t, recB)
	assertEq(t, dataB, "available_balance", "999.00")
}

func assertEq(t *testing.T, data map[string]any, key, want string) {
	t.Helper()
	got, ok := data[key]
	if !ok {
		t.Fatalf("missing field %q in %v", key, data)
	}
	if gotS, _ := got.(string); gotS != want {
		t.Fatalf("field %q = %v, want %q", key, got, want)
	}
}
