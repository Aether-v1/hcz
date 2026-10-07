package integrationtest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Aether-v1/hcz/internal/app/container"
	"github.com/Aether-v1/hcz/internal/app/httpserver"
	"github.com/Aether-v1/hcz/internal/bootstrap/database/migrations"
	"github.com/Aether-v1/hcz/internal/config"
	admindomain "github.com/Aether-v1/hcz/internal/modules/identity/admin/domain"
	userdomain "github.com/Aether-v1/hcz/internal/modules/identity/user/domain"
	"github.com/Aether-v1/hcz/internal/platform/database/gormdb"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// 本文件是 Points 系统的真实 HTTP 冒烟（P4 §55/§56）：
// 走完整 composition root（container.NewContainer → httpserver.SetupRouter → httptest.Server），
// 覆盖用户端 Earn/Check-in/Mall 与后台端 Adjust/Compensate/Settings/Stats/Order 处置，
// 并逐条验证 IDOR、RBAC 越权与无效令牌拒绝。service 层测试不能替代这里。

const smokeSecret = "points-smoke-secret-key-0001"

type smokeApp struct {
	server       *httptest.Server
	dependencies *container.Container
}

type smokeResponse struct {
	httpStatus int
	statusCode int
	msg        string
	data       json.RawMessage
	raw        []byte
}

// object 将 data 解析为 JSON 对象（非对象返回 nil）。
func (r smokeResponse) object() map[string]interface{} {
	var object map[string]interface{}
	if err := json.Unmarshal(r.data, &object); err != nil {
		return nil
	}
	return object
}

// array 将 data 解析为 JSON 数组（分页列表接口的 data 即数组本身）。
func (r smokeResponse) array() []interface{} {
	var items []interface{}
	if err := json.Unmarshal(r.data, &items); err != nil {
		return nil
	}
	return items
}

// nested 取 data 下的子对象。
func (r smokeResponse) nested(key string) map[string]interface{} {
	object := r.object()
	if object == nil {
		return nil
	}
	child, _ := object[key].(map[string]interface{})
	return child
}

func mapInt64(object map[string]interface{}, key string) int64 {
	if object == nil {
		return 0
	}
	switch value := object[key].(type) {
	case float64:
		return int64(value)
	case int64:
		return value
	case json.Number:
		parsed, _ := value.Int64()
		return parsed
	}
	return 0
}

func newPointsSmokeApp(t *testing.T) *smokeApp {
	t.Helper()
	gin.SetMode(gin.ReleaseMode)

	// 日志目录必须落在进程级临时目录：zap/lumberjack 持有 app.log 句柄，
	// Windows 下 t.TempDir() 的自动清理会因文件占用而失败。
	logDir, err := os.MkdirTemp("", "hcz-points-smoke-logs")
	if err != nil {
		t.Fatalf("create log dir: %v", err)
	}

	dbPath := filepath.Join(t.TempDir(), "points_http_smoke.db")
	if err := gormdb.InitDB("sqlite", dbPath, gormdb.DBPoolConfig{
		MaxOpenConns:           4,
		MaxIdleConns:           4,
		ConnMaxLifetimeSeconds: 60,
		ConnMaxIdleTimeSeconds: 60,
	}, "release"); err != nil {
		t.Fatalf("init sqlite failed: %v", err)
	}
	if err := migrations.AutoMigrate(); err != nil {
		t.Fatalf("automigrate failed: %v", err)
	}

	cfg := &config.Config{}
	cfg.App.SecretKey = smokeSecret
	cfg.App.TOTPIssuer = "HCZSmoke"
	cfg.Server.Mode = "release"
	cfg.Server.Host = "127.0.0.1"
	cfg.Server.Port = "0"
	cfg.JWT.SecretKey = smokeSecret
	cfg.JWT.ExpireHours = 1
	cfg.UserJWT.SecretKey = smokeSecret
	cfg.UserJWT.ExpireHours = 1
	cfg.Database.Driver = "sqlite"
	cfg.Database.DSN = dbPath
	cfg.Log.Dir = logDir
	cfg.Upload.MaxSize = 5 << 20

	dependencies, err := container.NewContainer(cfg)
	if err != nil {
		t.Fatalf("container build failed: %v", err)
	}
	server := httptest.NewServer(httpserver.SetupRouter(cfg, dependencies))
	t.Cleanup(func() {
		if sqlDB, err := gormdb.DB.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	t.Cleanup(server.Close)
	return &smokeApp{server: server, dependencies: dependencies}
}

// mintAccessToken 用与中间件一致的 HS256 密钥签发真实访问令牌。
func mintAccessToken(t *testing.T, claims jwt.MapClaims) string {
	t.Helper()
	now := time.Now()
	claims["iat"] = now.Unix()
	claims["exp"] = now.Add(time.Hour).Unix()
	claims["typ"] = "access"
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(smokeSecret))
	if err != nil {
		t.Fatalf("sign token failed: %v", err)
	}
	return signed
}

func (a *smokeApp) do(t *testing.T, method, path, token string, body interface{}, headers map[string]string) smokeResponse {
	t.Helper()
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(payload)
	}
	request, err := http.NewRequest(method, a.server.URL+path, reader)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}

	response, err := a.server.Client().Do(request)
	if err != nil {
		t.Fatalf("%s %s transport error: %v", method, path, err)
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	result := smokeResponse{httpStatus: response.StatusCode, raw: raw}
	var envelope struct {
		StatusCode int             `json:"status_code"`
		Msg        string          `json:"msg"`
		Data       json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err == nil {
		result.statusCode = envelope.StatusCode
		result.msg = envelope.Msg
		result.data = envelope.Data
	}
	return result
}

func (a *smokeApp) mustSuccess(t *testing.T, method, path, token string, body interface{}, headers map[string]string) smokeResponse {
	t.Helper()
	response := a.do(t, method, path, token, body, headers)
	if response.httpStatus != http.StatusOK || response.statusCode != 0 {
		t.Fatalf("%s %s failed: http=%d status_code=%d msg=%q body=%s",
			method, path, response.httpStatus, response.statusCode, response.msg, response.raw)
	}
	return response
}

// seedUser 直接落库一个活跃用户（跳过注册副作用，聚焦积分链路）。
// invite_code 带全局唯一索引且默认空串，因此必须为每个用户分配互不相同的短码。
func seedUser(t *testing.T, email, inviteCode string) uint {
	t.Helper()
	user := userdomain.User{Email: email, PasswordHash: "not-used", Status: "active", InviteCode: inviteCode}
	if err := gormdb.DB.Create(&user).Error; err != nil {
		t.Fatalf("seed user %s: %v", email, err)
	}
	return user.ID
}

// seedAdmin 落库一个非超管管理员并绑定角色，返回其 ID 与访问令牌。
func seedAdmin(t *testing.T, app *smokeApp, username string, roles ...string) (uint, string) {
	t.Helper()
	admin := admindomain.Admin{Username: username, PasswordHash: "not-used"}
	if err := gormdb.DB.Create(&admin).Error; err != nil {
		t.Fatalf("seed admin %s: %v", username, err)
	}
	if len(roles) > 0 {
		if err := app.dependencies.AuthzService.SetAdminRoles(admin.ID, roles); err != nil {
			t.Fatalf("bind roles for %s: %v", username, err)
		}
	}
	token := mintAccessToken(t, jwt.MapClaims{
		"admin_id": admin.ID, "username": admin.Username, "token_version": admin.TokenVersion,
	})
	return admin.ID, token
}

func userToken(t *testing.T, userID uint, email string) string {
	t.Helper()
	return mintAccessToken(t, jwt.MapClaims{"user_id": userID, "email": email, "token_version": uint64(0)})
}

func idem(key string) map[string]string { return map[string]string{"Idempotency-Key": key} }

// TestPointsHTTPSmoke 跑通用户端 + 后台端全链路，并逐条验证安全边界。
func TestPointsHTTPSmoke(t *testing.T) {
	app := newPointsSmokeApp(t)

	userAEmail := "user-a@smoke.invalid"
	userBEmail := "user-b@smoke.invalid"
	userA := seedUser(t, userAEmail, "SMOKEUSERA")
	userB := seedUser(t, userBEmail, "SMOKEUSERB")
	tokenA := userToken(t, userA, userAEmail)
	tokenB := userToken(t, userB, userBEmail)

	_, financeToken := seedAdmin(t, app, "finance-smoke", "finance")
	_, auditorToken := seedAdmin(t, app, "auditor-smoke", "readonly_auditor")
	_, opsToken := seedAdmin(t, app, "ops-smoke", "operations")
	_, systemToken := seedAdmin(t, app, "system-smoke", "system_admin")
	_, rolelessToken := seedAdmin(t, app, "roleless-smoke")

	const productPrice = int64(500)
	adjustPath := fmt.Sprintf("/api/v1/admin/users/%d/points/adjust", userA)

	t.Run("用户零起点可读账户与流水", func(t *testing.T) {
		account := app.mustSuccess(t, http.MethodGet, "/api/v1/points/account", tokenA, nil, nil)
		if got := mapInt64(account.object(), "balance"); got != 0 {
			t.Fatalf("fresh balance want 0 got %d", got)
		}
		if entries := app.mustSuccess(t, http.MethodGet, "/api/v1/points/ledger", tokenA, nil, nil).array(); len(entries) != 0 {
			t.Fatalf("fresh ledger want 0 entries got %d", len(entries))
		}
	})

	t.Run("Admin 调整积分并幂等回放", func(t *testing.T) {
		body := map[string]interface{}{"amount": 1000, "operation": "add", "reason": "冒烟充值"}
		first := app.mustSuccess(t, http.MethodPost, adjustPath, financeToken, body, idem("smoke-adjust-1"))
		if got := mapInt64(first.nested("account"), "balance"); got != 1000 {
			t.Fatalf("balance after adjust want 1000 got %d", got)
		}
		replay := app.mustSuccess(t, http.MethodPost, adjustPath, financeToken, body, idem("smoke-adjust-1"))
		if got := mapInt64(replay.nested("account"), "balance"); got != 1000 {
			t.Fatalf("idempotent replay must not double credit, got %d", got)
		}
		if mapInt64(first.nested("ledger"), "id") != mapInt64(replay.nested("ledger"), "id") {
			t.Fatalf("idempotent replay must return the original ledger entry")
		}
		conflict := app.do(t, http.MethodPost, adjustPath, financeToken,
			map[string]interface{}{"amount": 2000, "operation": "add", "reason": "同键不同额"}, idem("smoke-adjust-1"))
		if conflict.statusCode != http.StatusConflict {
			t.Fatalf("same key with different payload want 409 got %d body=%s", conflict.statusCode, conflict.raw)
		}
	})

	t.Run("幂等键缺失与超长被拒绝", func(t *testing.T) {
		missing := app.do(t, http.MethodPost, adjustPath, financeToken,
			map[string]interface{}{"amount": 10, "operation": "add", "reason": "无键"}, nil)
		if missing.statusCode != http.StatusBadRequest {
			t.Fatalf("missing idempotency key want 400 got %d body=%s", missing.statusCode, missing.raw)
		}
		tooLong := app.do(t, http.MethodPost, adjustPath, financeToken,
			map[string]interface{}{"amount": 10, "operation": "add", "reason": "超长键"}, idem(strings.Repeat("k", 65)))
		if tooLong.statusCode != http.StatusBadRequest {
			t.Fatalf("overlong idempotency key want 400 got %d body=%s", tooLong.statusCode, tooLong.raw)
		}
		negative := app.do(t, http.MethodPost, adjustPath, financeToken,
			map[string]interface{}{"amount": -100, "operation": "add", "reason": "负数金额"}, idem("smoke-negative-amount"))
		if negative.statusCode != http.StatusBadRequest {
			t.Fatalf("negative amount want 400 got %d body=%s", negative.statusCode, negative.raw)
		}
		overCap := app.do(t, http.MethodPost, adjustPath, financeToken,
			map[string]interface{}{"amount": 100000000000000, "operation": "add", "reason": "误输入天文数字"}, idem("smoke-overcap"))
		if overCap.statusCode != http.StatusBadRequest {
			t.Fatalf("over-cap adjust want 400 got %d body=%s", overCap.statusCode, overCap.raw)
		}
	})

	t.Run("人工补偿使用独立 action 且不改变订单状态", func(t *testing.T) {
		compensated := app.mustSuccess(t, http.MethodPost,
			fmt.Sprintf("/api/v1/admin/users/%d/points/compensate", userA), financeToken,
			map[string]interface{}{"amount": 300, "reason": "历史订单漏发补发"}, idem("smoke-comp-1"))
		if got := mapInt64(compensated.nested("account"), "balance"); got != 1300 {
			t.Fatalf("balance after compensation want 1300 got %d", got)
		}
		ledger := app.mustSuccess(t, http.MethodGet,
			fmt.Sprintf("/api/v1/admin/users/%d/points/ledger?action_type=ADMIN_COMPENSATION", userA), financeToken, nil, nil)
		if entries := ledger.array(); len(entries) != 1 {
			t.Fatalf("compensation ledger want exactly 1 entry got %d body=%s", len(entries), ledger.raw)
		}
		noFakeReward := app.mustSuccess(t, http.MethodGet,
			fmt.Sprintf("/api/v1/admin/users/%d/points/ledger?action_type=ORDER_REWARD", userA), financeToken, nil, nil)
		if entries := noFakeReward.array(); len(entries) != 0 {
			t.Fatalf("compensation must not fabricate ORDER_REWARD, got %d entries", len(entries))
		}
	})

	t.Run("签到配置读取与更新受 RBAC 约束", func(t *testing.T) {
		current := app.mustSuccess(t, http.MethodGet, "/api/v1/admin/settings/checkin", systemToken, nil, nil)
		if current.object()["rewards"] == nil {
			t.Fatalf("checkin config payload unexpected: %s", current.raw)
		}
		app.mustSuccess(t, http.MethodPut, "/api/v1/admin/settings/checkin", systemToken,
			map[string]interface{}{"enabled": true, "rewards": []int64{1, 2, 3, 4, 5, 6, 10}}, nil)
		denied := app.do(t, http.MethodPut, "/api/v1/admin/settings/checkin", financeToken,
			map[string]interface{}{"enabled": true, "rewards": []int64{1, 1, 1, 1, 1, 1, 1}}, nil)
		if denied.statusCode != http.StatusForbidden {
			t.Fatalf("finance updating checkin config want 403 got %d body=%s", denied.statusCode, denied.raw)
		}
		overCap := app.do(t, http.MethodPut, "/api/v1/admin/settings/checkin", systemToken,
			map[string]interface{}{"enabled": true, "rewards": []int64{100000000000000, 2, 3, 4, 5, 6, 10}}, nil)
		if overCap.statusCode != http.StatusBadRequest {
			t.Fatalf("over-cap daily reward want 400 got %d body=%s", overCap.statusCode, overCap.raw)
		}
	})

	t.Run("用户签到入账且同日重复不二次发放", func(t *testing.T) {
		app.mustSuccess(t, http.MethodGet, "/api/v1/checkin/status", tokenA, nil, nil)
		first := app.mustSuccess(t, http.MethodPost, "/api/v1/checkin", tokenA, nil, nil)
		if got := mapInt64(first.object(), "points_awarded"); got != 1 {
			t.Fatalf("day-1 reward want 1 got %d body=%s", got, first.raw)
		}
		duplicate := app.mustSuccess(t, http.MethodPost, "/api/v1/checkin", tokenA, nil, nil)
		if already, _ := duplicate.object()["already_checked_in"].(bool); !already {
			t.Fatalf("same-day repeat must report already_checked_in, body=%s", duplicate.raw)
		}
		account := app.mustSuccess(t, http.MethodGet, "/api/v1/points/account", tokenA, nil, nil)
		if got := mapInt64(account.object(), "balance"); got != 1301 {
			t.Fatalf("balance after single checkin want 1301 got %d", got)
		}
		history := app.mustSuccess(t, http.MethodGet, "/api/v1/checkin/history", tokenA, nil, nil)
		if dates, ok := history.object()["checked_dates"].([]interface{}); !ok || len(dates) != 1 {
			t.Fatalf("checkin history want 1 checked date, body=%s", history.raw)
		}
	})

	t.Run("后台只读签到历史且无任何写入口", func(t *testing.T) {
		read := app.mustSuccess(t, http.MethodGet, fmt.Sprintf("/api/v1/admin/users/%d/checkins", userA), auditorToken, nil, nil)
		if read.object()["user_id"] == nil {
			t.Fatalf("admin checkin history payload unexpected: %s", read.raw)
		}
		for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
			res := app.do(t, method, fmt.Sprintf("/api/v1/admin/users/%d/checkins", userA), systemToken, map[string]interface{}{}, nil)
			if res.httpStatus != http.StatusNotFound {
				t.Fatalf("%s checkins must not be routed, got http=%d body=%s", method, res.httpStatus, res.raw)
			}
		}
	})

	var productID uint
	t.Run("运营创建积分商品并挡住越界价格", func(t *testing.T) {
		created := app.mustSuccess(t, http.MethodPost, "/api/v1/admin/points/products", opsToken, map[string]interface{}{
			"name": "冒烟月度会员", "points_price": productPrice, "stock": 3, "enabled": true,
			"fulfillment_type": "MANUAL", "per_user_limit": 2, "reason": "冒烟创建",
		}, nil)
		productID = uint(mapInt64(created.object(), "id"))
		if productID == 0 {
			t.Fatalf("product id missing: %s", created.raw)
		}
		overCap := app.do(t, http.MethodPost, "/api/v1/admin/points/products", opsToken, map[string]interface{}{
			"name": "越界价格", "points_price": 100000000000000, "stock": 1, "enabled": true,
			"fulfillment_type": "MANUAL", "reason": "误输入",
		}, nil)
		if overCap.statusCode != http.StatusBadRequest {
			t.Fatalf("over-cap product price want 400 got %d body=%s", overCap.statusCode, overCap.raw)
		}
	})

	var orderA uint
	t.Run("用户浏览商城并兑换扣分", func(t *testing.T) {
		list := app.mustSuccess(t, http.MethodGet, "/api/v1/points/products", tokenA, nil, nil)
		items := list.array()
		if len(items) == 0 {
			t.Fatalf("mall product list empty: %s", list.raw)
		}
		created := app.mustSuccess(t, http.MethodPost, "/api/v1/points/exchange-orders", tokenA,
			map[string]interface{}{"product_id": productID}, idem("smoke-exchange-1"))
		orderA = uint(mapInt64(created.nested("order"), "id"))
		if orderA == 0 {
			t.Fatalf("exchange payload missing order: %s", created.raw)
		}
		if got := mapInt64(created.object(), "current_balance"); got != 801 {
			t.Fatalf("balance after redeem want 801 got %d", got)
		}
		replay := app.mustSuccess(t, http.MethodPost, "/api/v1/points/exchange-orders", tokenA,
			map[string]interface{}{"product_id": productID}, idem("smoke-exchange-1"))
		if uint(mapInt64(replay.nested("order"), "id")) != orderA {
			t.Fatalf("idempotent replay must return the same order: %s", replay.raw)
		}
		if got := mapInt64(replay.object(), "current_balance"); got != 801 {
			t.Fatalf("idempotent replay must not deduct twice, got %d", got)
		}
	})

	t.Run("兑换订单详情严格 owner scoped", func(t *testing.T) {
		if got := app.mustSuccess(t, http.MethodGet, fmt.Sprintf("/api/v1/points/exchange-orders/%d", orderA), tokenA, nil, nil); got.object() == nil {
			t.Fatalf("owner detail empty: %s", got.raw)
		}
		intruder := app.do(t, http.MethodGet, fmt.Sprintf("/api/v1/points/exchange-orders/%d", orderA), tokenB, nil, nil)
		if intruder.statusCode == 0 {
			t.Fatalf("user B must not read user A exchange order: %s", intruder.raw)
		}
		cancelIntruder := app.do(t, http.MethodPost, fmt.Sprintf("/api/v1/points/exchange-orders/%d/cancel", orderA), tokenB,
			map[string]interface{}{"reason": "越权取消"}, nil)
		if cancelIntruder.statusCode == 0 {
			t.Fatalf("user B must not cancel user A order: %s", cancelIntruder.raw)
		}
		anonymous := app.do(t, http.MethodGet, fmt.Sprintf("/api/v1/points/exchange-orders/%d", orderA), "", nil, nil)
		if anonymous.statusCode != http.StatusUnauthorized {
			t.Fatalf("missing token want 401 got %d body=%s", anonymous.statusCode, anonymous.raw)
		}
		broken := app.do(t, http.MethodGet, "/api/v1/points/account", "not-a-jwt", nil, nil)
		if broken.statusCode != http.StatusUnauthorized {
			t.Fatalf("invalid token want 401 got %d body=%s", broken.statusCode, broken.raw)
		}
	})

	t.Run("履约状态机推进到终态且不自动撤销", func(t *testing.T) {
		app.mustSuccess(t, http.MethodPost, fmt.Sprintf("/api/v1/admin/points/exchange-orders/%d/process", orderA), opsToken,
			map[string]interface{}{"reason": "开始处理"}, nil)
		app.mustSuccess(t, http.MethodPost, fmt.Sprintf("/api/v1/admin/points/exchange-orders/%d/complete", orderA), opsToken,
			map[string]interface{}{"reason": "已发放权益"}, nil)
		detail := app.mustSuccess(t, http.MethodGet, fmt.Sprintf("/api/v1/admin/points/exchange-orders/%d", orderA), opsToken, nil, nil)
		order := detail.nested("order")
		if order == nil {
			order = detail.object()
		}
		if status, _ := order["status"].(string); status != "COMPLETED" {
			t.Fatalf("exchange order must reach COMPLETED, got %v body=%s", order["status"], detail.raw)
		}
		rollback := app.do(t, http.MethodPost, fmt.Sprintf("/api/v1/admin/points/exchange-orders/%d/cancel", orderA), financeToken,
			map[string]interface{}{"reason": "试图撤销已完成订单"}, nil)
		if rollback.statusCode == 0 {
			t.Fatalf("COMPLETED must be terminal, cancel succeeded: %s", rollback.raw)
		}
		balance := app.mustSuccess(t, http.MethodGet, "/api/v1/points/account", tokenA, nil, nil)
		if got := mapInt64(balance.object(), "balance"); got != 801 {
			t.Fatalf("COMPLETED must not auto-refund, balance got %d want 801", got)
		}
	})

	t.Run("失败处置返还积分与库存恰好一次", func(t *testing.T) {
		app.mustSuccess(t, http.MethodPost, fmt.Sprintf("/api/v1/admin/users/%d/points/adjust", userB), financeToken,
			map[string]interface{}{"amount": productPrice, "operation": "add", "reason": "为 B 准备兑换余额"}, idem("smoke-adjust-b"))
		before := app.mustSuccess(t, http.MethodGet, "/api/v1/points/account", tokenB, nil, nil)
		if got := mapInt64(before.object(), "balance"); got != productPrice {
			t.Fatalf("user B seeded balance want %d got %d", productPrice, got)
		}
		created := app.mustSuccess(t, http.MethodPost, "/api/v1/points/exchange-orders", tokenB,
			map[string]interface{}{"product_id": productID}, idem("smoke-exchange-b1"))
		orderB := uint(mapInt64(created.nested("order"), "id"))
		if orderB == 0 {
			t.Fatalf("user B exchange failed: %s", created.raw)
		}
		app.mustSuccess(t, http.MethodPost, fmt.Sprintf("/api/v1/admin/points/exchange-orders/%d/fail", orderB), financeToken,
			map[string]interface{}{"reason": "权益无法发放"}, nil)
		// 同状态重放：幂等 no-op（绝不二次返还/二次恢复库存）。
		replayFail := app.mustSuccess(t, http.MethodPost, fmt.Sprintf("/api/v1/admin/points/exchange-orders/%d/fail", orderB), financeToken,
			map[string]interface{}{"reason": "重复失败"}, nil)
		if status, _ := replayFail.object()["status"].(string); status != "FAILED" {
			t.Fatalf("replayed fail must keep FAILED, got %s", status)
		}
		// 终态之间禁止互相转移：FAILED → CANCELLED 必须拒绝。
		crossTerminal := app.do(t, http.MethodPost, fmt.Sprintf("/api/v1/admin/points/exchange-orders/%d/cancel", orderB), financeToken,
			map[string]interface{}{"reason": "试图改为取消"}, nil)
		if crossTerminal.statusCode == 0 {
			t.Fatalf("FAILED order must not transition to CANCELLED: %s", crossTerminal.raw)
		}
		account := app.mustSuccess(t, http.MethodGet, "/api/v1/points/account", tokenB, nil, nil)
		if got := mapInt64(account.object(), "balance"); got != productPrice {
			t.Fatalf("user B balance want %d after full refund got %d", productPrice, got)
		}
		ledger := app.mustSuccess(t, http.MethodGet,
			fmt.Sprintf("/api/v1/admin/users/%d/points/ledger?action_type=REDEEM_REFUND", userB), financeToken, nil, nil)
		if entries := ledger.array(); len(entries) != 1 {
			t.Fatalf("REDEEM_REFUND want exactly 1 entry got %d body=%s", len(entries), ledger.raw)
		}
		// 库存恰好恢复一次：初始 3 → A 兑换 2 → B 兑换 1 → B 失败恢复 2。
		products := app.mustSuccess(t, http.MethodGet, "/api/v1/admin/points/products", auditorToken, nil, nil).array()
		for _, item := range products {
			product, ok := item.(map[string]interface{})
			if !ok || uint(mapInt64(product, "id")) != productID {
				continue
			}
			if got := mapInt64(product, "stock"); got != 2 {
				t.Fatalf("stock must be restored exactly once, want 2 got %d", got)
			}
		}
	})

	t.Run("RBAC 越权一律拒绝", func(t *testing.T) {
		cases := []struct {
			name   string
			token  string
			method string
			path   string
			body   interface{}
		}{
			{"auditor cannot adjust", auditorToken, http.MethodPost, adjustPath, map[string]interface{}{"amount": 1, "operation": "add", "reason": "r"}},
			{"auditor cannot compensate", auditorToken, http.MethodPost, fmt.Sprintf("/api/v1/admin/users/%d/points/compensate", userA), map[string]interface{}{"amount": 1, "reason": "r"}},
			{"auditor cannot fail orders", auditorToken, http.MethodPost, fmt.Sprintf("/api/v1/admin/points/exchange-orders/%d/fail", orderA), map[string]interface{}{"reason": "r"}},
			{"roleless admin denied", rolelessToken, http.MethodGet, "/api/v1/admin/points/stats", nil},
			{"operations cannot refund", opsToken, http.MethodPost, fmt.Sprintf("/api/v1/admin/points/exchange-orders/%d/cancel", orderA), map[string]interface{}{"reason": "r"}},
			{"finance cannot edit products", financeToken, http.MethodPost, "/api/v1/admin/points/products", map[string]interface{}{"name": "x", "points_price": 1, "stock": 1, "fulfillment_type": "MANUAL", "reason": "r"}},
			{"user token cannot reach admin", tokenA, http.MethodGet, "/api/v1/admin/points/stats", nil},
		}
		for _, tc := range cases {
			res := app.do(t, tc.method, tc.path, tc.token, tc.body, idem("smoke-rbac"))
			if res.statusCode == 0 {
				t.Fatalf("%s unexpectedly allowed: %s", tc.name, res.raw)
			}
			if res.statusCode != http.StatusForbidden && res.statusCode != http.StatusUnauthorized && res.statusCode != http.StatusNotFound {
				t.Fatalf("%s want 401/403/404 got %d body=%s", tc.name, res.statusCode, res.raw)
			}
		}
	})

	t.Run("运营查询面可用且参数不受信于客户端", func(t *testing.T) {
		if accounts := app.mustSuccess(t, http.MethodGet, "/api/v1/admin/points/accounts", financeToken, nil, nil).array(); len(accounts) == 0 {
			t.Fatalf("account list empty after activity")
		}
		app.mustSuccess(t, http.MethodGet, "/api/v1/admin/points/accounts?negative_only=true", financeToken, nil, nil)
		stats := app.mustSuccess(t, http.MethodGet, "/api/v1/admin/points/stats", auditorToken, nil, nil)
		if stats.object() == nil {
			t.Fatalf("stats payload empty: %s", stats.raw)
		}
		// 排序固定服务端（created_at DESC），客户端注入 sort 字段必须被忽略而非拼进 SQL。
		injected := app.mustSuccess(t, http.MethodGet,
			fmt.Sprintf("/api/v1/admin/users/%d/points/ledger?sort=amount%%3BDROP%%20TABLE%%20users", userA), auditorToken, nil, nil)
		if len(injected.array()) == 0 {
			t.Fatalf("ledger with injected sort must still return rows: %s", injected.raw)
		}
		var userRows int64
		if err := gormdb.DB.Table("users").Count(&userRows).Error; err != nil {
			t.Fatalf("users table must survive: %v", err)
		}
		badID := app.do(t, http.MethodGet, "/api/v1/admin/users/-1/points", auditorToken, nil, nil)
		if badID.statusCode == 0 {
			t.Fatalf("negative :id must be rejected: %s", badID.raw)
		}
		textID := app.do(t, http.MethodGet, "/api/v1/admin/users/abc/points", auditorToken, nil, nil)
		if textID.statusCode == 0 {
			t.Fatalf("non-numeric :id must be rejected: %s", textID.raw)
		}
		missingUser := app.do(t, http.MethodGet, "/api/v1/admin/users/99999999/points", auditorToken, nil, nil)
		if missingUser.statusCode != http.StatusNotFound {
			t.Fatalf("unknown user want 404 got %d body=%s", missingUser.statusCode, missingUser.raw)
		}
		badStatus := app.do(t, http.MethodGet, "/api/v1/admin/points/exchange-orders?status=NOT_A_STATUS", auditorToken, nil, nil)
		if badStatus.statusCode != http.StatusBadRequest {
			t.Fatalf("invalid status filter want 400 got %d body=%s", badStatus.statusCode, badStatus.raw)
		}
	})

	t.Run("负余额可见且禁止继续兑换", func(t *testing.T) {
		app.mustSuccess(t, http.MethodPost, adjustPath, financeToken,
			map[string]interface{}{"amount": 1301, "operation": "subtract", "reason": "冲正超额扣减"}, idem("smoke-negative"))
		account := app.mustSuccess(t, http.MethodGet, "/api/v1/points/account", tokenA, nil, nil)
		if got := mapInt64(account.object(), "balance"); got != -500 {
			t.Fatalf("negative balance must stay visible, got %d", got)
		}
		redeem := app.do(t, http.MethodPost, "/api/v1/points/exchange-orders", tokenA,
			map[string]interface{}{"product_id": productID}, idem("smoke-negative-redeem"))
		if redeem.statusCode == 0 {
			t.Fatalf("negative balance must not redeem: %s", redeem.raw)
		}
	})

	t.Logf("冒烟链路完成：user_a=%d user_b=%d product=%d order_a=%d", userA, userB, productID, orderA)
}
