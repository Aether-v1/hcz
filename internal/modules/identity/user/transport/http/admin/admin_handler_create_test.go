package adminuserhttp

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	auditlogapp "github.com/Aether-v1/hcz/internal/modules/auditlog/application"
	userdomain "github.com/Aether-v1/hcz/internal/modules/identity/user/domain"
	walletdomain "github.com/Aether-v1/hcz/internal/modules/wallet/domain"
	"github.com/Aether-v1/hcz/internal/platform/http/response"
	"github.com/Aether-v1/hcz/internal/shared/money"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

// --- fakes ---

type fakeCreateUserStore struct {
	users     map[uint]*userdomain.User
	byEmail   map[string]*userdomain.User
	byInvite  map[string]*userdomain.User
	nextID    uint
	createErr error
}

func newFakeCreateUserStore() *fakeCreateUserStore {
	return &fakeCreateUserStore{
		users:    map[uint]*userdomain.User{},
		byEmail:  map[string]*userdomain.User{},
		byInvite: map[string]*userdomain.User{},
		nextID:   1000,
	}
}

func (s *fakeCreateUserStore) List(UserListFilter) ([]userdomain.User, int64, error) {
	return nil, 0, nil
}
func (s *fakeCreateUserStore) GetByID(id uint) (*userdomain.User, error) { return s.users[id], nil }
func (s *fakeCreateUserStore) GetByEmail(email string) (*userdomain.User, error) {
	return s.byEmail[email], nil
}
func (s *fakeCreateUserStore) GetByInviteCode(code string) (*userdomain.User, error) {
	return s.byInvite[code], nil
}
func (s *fakeCreateUserStore) Create(u *userdomain.User) error {
	if s.createErr != nil {
		return s.createErr
	}
	u.ID = s.nextID
	s.nextID++
	s.users[u.ID] = u
	s.byEmail[u.Email] = u
	if u.InviteCode != "" {
		s.byInvite[u.InviteCode] = u
	}
	return nil
}
func (s *fakeCreateUserStore) Update(*userdomain.User) error      { return nil }
func (s *fakeCreateUserStore) BatchUpdateStatus([]uint, string) error { return nil }

type fakeCreateEmailNormalizer struct{}

func (fakeCreateEmailNormalizer) NormalizeEmail(email string) (string, error) {
	if email == "" || !strings.Contains(email, "@") {
		return "", errors.New("invalid email")
	}
	return email, nil
}

type fakeCreateWallet struct {
	createdUserIDs []uint
}

func (w *fakeCreateWallet) GetBalancesByUserIDs([]uint) (map[uint]money.Amount, error) {
	return map[uint]money.Amount{}, nil
}
func (w *fakeCreateWallet) GetAccount(userID uint) (*walletdomain.Account, error) {
	w.createdUserIDs = append(w.createdUserIDs, userID)
	return &walletdomain.Account{
		UserID:           userID,
		AvailableBalance: money.FromDecimal(decimal.Zero),
	}, nil
}

type fakePasswordValidator struct {
	weakErr error
}

func (f *fakePasswordValidator) ValidatePassword(string) error {
	return f.weakErr
}

type fakeAuditRecorder struct {
	records []auditlogapp.AuthzRecord
}

func (r *fakeAuditRecorder) Record(input auditlogapp.AuthzRecord) error {
	r.records = append(r.records, input)
	return nil
}

type fakeMemberLevelAssigner struct {
	assignedUserIDs []uint
}

func (a *fakeMemberLevelAssigner) AssignDefaultLevel(userID uint) error {
	a.assignedUserIDs = append(a.assignedUserIDs, userID)
	return nil
}

// weakPasswordErr 模拟 passwordpolicy.violation（带 Key/Args，用于走 i18n 分支）。
type weakPasswordErr struct{}

func (weakPasswordErr) Error() string         { return "error.password_min_length" }
func (weakPasswordErr) Key() string           { return "error.password_min_length" }
func (weakPasswordErr) Args() []interface{}   { return []interface{}{8} }

func setupCreateRouter(h *AdminHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("admin_id", uint(7))
		c.Set("username", "root")
		c.Set("request_id", "req-123")
		c.Next()
	})
	RegisterAdminRoutes(r.Group("/admin"), h)
	return r
}

func doCreate(t *testing.T, r *gin.Engine, body string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/users", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

func TestCreateAdminUser_Success(t *testing.T) {
	store := newFakeCreateUserStore()
	wallets := &fakeCreateWallet{}
	audit := &fakeAuditRecorder{}
	member := &fakeMemberLevelAssigner{}
	h := &AdminHandler{
		users:        store,
		emails:       fakeCreateEmailNormalizer{},
		wallets:      wallets,
		passwords:    &fakePasswordValidator{},
		audit:        audit,
		memberLevels: member,
	}
	r := setupCreateRouter(h)

	w := doCreate(t, r, `{"email":"New.User@Example.com","password":"StrongPass123!","nickname":"Newbie"}`)

	var resp struct {
		StatusCode int `json:"status_code"`
		Data       struct {
			ID          uint   `json:"id"`
			Email       string `json:"email"`
			DisplayName string `json:"display_name"`
			Status      string `json:"status"`
			InviteCode  string `json:"invite_code"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v body=%s", err, w.Body.String())
	}
	if resp.StatusCode != response.CodeOK {
		t.Fatalf("status_code=%d body=%s", resp.StatusCode, w.Body.String())
	}
	if resp.Data.ID != 1000 {
		t.Fatalf("created user ID = %d, want 1000", resp.Data.ID)
	}
	if resp.Data.Email != "New.User@Example.com" {
		t.Fatalf("email = %q", resp.Data.Email)
	}
	if resp.Data.DisplayName != "Newbie" {
		t.Fatalf("display_name = %q, want Newbie", resp.Data.DisplayName)
	}
	if resp.Data.Status != "active" {
		t.Fatalf("status = %q, want active", resp.Data.Status)
	}
	if resp.Data.InviteCode == "" {
		t.Fatal("invite_code must be generated")
	}

	// 钱包已为新用户创建
	if len(wallets.createdUserIDs) != 1 || wallets.createdUserIDs[0] != 1000 {
		t.Fatalf("wallet created for user IDs = %v, want [1000]", wallets.createdUserIDs)
	}
	// 默认会员等级已分配（未显式指定）
	if len(member.assignedUserIDs) != 1 || member.assignedUserIDs[0] != 1000 {
		t.Fatalf("default member level assigned to = %v, want [1000]", member.assignedUserIDs)
	}
	// 审计日志已写入
	if len(audit.records) != 1 {
		t.Fatalf("audit records = %d, want 1", len(audit.records))
	}
	rec := audit.records[0]
	if rec.Action != "admin.user.create" {
		t.Fatalf("audit action = %q", rec.Action)
	}
	if rec.Object != "user:1000" {
		t.Fatalf("audit object = %q, want user:1000", rec.Object)
	}
	if rec.OperatorAdminID != 7 {
		t.Fatalf("audit operator = %d, want 7", rec.OperatorAdminID)
	}
}

func TestCreateAdminUser_DuplicateEmail(t *testing.T) {
	store := newFakeCreateUserStore()
	// 预置一个同邮箱用户
	_ = store.Create(&userdomain.User{Email: "dup@example.com", PasswordHash: "x"})

	h := &AdminHandler{
		users:     store,
		emails:    fakeCreateEmailNormalizer{},
		wallets:   &fakeCreateWallet{},
		passwords: &fakePasswordValidator{},
		audit:     &fakeAuditRecorder{},
	}
	r := setupCreateRouter(h)

	w := doCreate(t, r, `{"email":"dup@example.com","password":"StrongPass123!"}`)

	var resp struct {
		StatusCode int    `json:"status_code"`
		Msg        string `json:"msg"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.StatusCode != response.CodeConflict {
		t.Fatalf("status_code=%d, want %d (409); body=%s", resp.StatusCode, response.CodeConflict, w.Body.String())
	}
	if resp.Msg != "error.email_already_exists" {
		t.Fatalf("msg=%q, want error.email_already_exists", resp.Msg)
	}
}

func TestCreateAdminUser_WeakPassword(t *testing.T) {
	store := newFakeCreateUserStore()
	h := &AdminHandler{
		users:     store,
		emails:    fakeCreateEmailNormalizer{},
		wallets:   &fakeCreateWallet{},
		passwords: &fakePasswordValidator{weakErr: weakPasswordErr{}},
		audit:     &fakeAuditRecorder{},
	}
	r := setupCreateRouter(h)

	w := doCreate(t, r, `{"email":"weak@example.com","password":"123"}`)

	var resp struct {
		StatusCode int    `json:"status_code"`
		Msg        string `json:"msg"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.StatusCode != response.CodeBadRequest {
		t.Fatalf("status_code=%d, want %d (400); body=%s", resp.StatusCode, response.CodeBadRequest, w.Body.String())
	}
	// 弱密码不应落库
	if len(store.users) != 0 {
		t.Fatalf("weak password must not create user, got %d users", len(store.users))
	}
}

func TestCreateAdminUser_InvalidEmail(t *testing.T) {
	store := newFakeCreateUserStore()
	h := &AdminHandler{
		users:     store,
		emails:    fakeCreateEmailNormalizer{},
		wallets:   &fakeCreateWallet{},
		passwords: &fakePasswordValidator{},
		audit:     &fakeAuditRecorder{},
	}
	r := setupCreateRouter(h)

	w := doCreate(t, r, `{"email":"not-an-email","password":"StrongPass123!"}`)

	var resp struct {
		StatusCode int `json:"status_code"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.StatusCode != response.CodeBadRequest {
		t.Fatalf("status_code=%d, want 400 for invalid email; body=%s", resp.StatusCode, w.Body.String())
	}
}
