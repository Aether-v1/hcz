package authz

import (
	"fmt"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupPointsPolicyTest(t *testing.T) *Service {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	svc, err := NewService(db)
	if err != nil {
		t.Fatalf("new authz service failed: %v", err)
	}
	if err := svc.BootstrapBuiltinRoles(); err != nil {
		t.Fatalf("bootstrap builtin roles failed: %v", err)
	}
	return svc
}

// TestPointsAdminPolicies 验证积分后台端点已正式登记 RBAC：
//   - finance：可读积分账户/流水，可调整积分；
//   - readonly_auditor：只读积分账户/流水，不可调整；
//   - 未授权管理员：全部拒绝。
func TestPointsAdminPolicies(t *testing.T) {
	svc := setupPointsPolicyTest(t)

	const financeAdmin = 501
	const auditorAdmin = 502
	const otherAdmin = 503

	if err := svc.SetAdminRoles(financeAdmin, []string{"finance"}); err != nil {
		t.Fatalf("set finance role: %v", err)
	}
	if err := svc.SetAdminRoles(auditorAdmin, []string{"readonly_auditor"}); err != nil {
		t.Fatalf("set auditor role: %v", err)
	}

	cases := []struct {
		name       string
		adminID    uint
		path       string
		action     string
		wantAllow  bool
	}{
		{"finance read account", financeAdmin, "/api/v1/admin/users/42/points", "GET", true},
		{"finance read ledger", financeAdmin, "/api/v1/admin/users/42/points/ledger", "GET", true},
		{"finance adjust", financeAdmin, "/api/v1/admin/users/42/points/adjust", "POST", true},
		{"auditor read account", auditorAdmin, "/api/v1/admin/users/42/points", "GET", true},
		{"auditor read ledger", auditorAdmin, "/api/v1/admin/users/42/points/ledger", "GET", true},
		{"auditor cannot adjust", auditorAdmin, "/api/v1/admin/users/42/points/adjust", "POST", false},
		{"unauthorized admin read", otherAdmin, "/api/v1/admin/users/42/points", "GET", false},
		{"unauthorized admin adjust", otherAdmin, "/api/v1/admin/users/42/points/adjust", "POST", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			allow, err := svc.EnforceAdmin(tc.adminID, tc.path, tc.action)
			if err != nil {
				t.Fatalf("enforce: %v", err)
			}
			if allow != tc.wantAllow {
				t.Fatalf("EnforceAdmin(%d, %s, %s) = %v, want %v",
					tc.adminID, tc.path, tc.action, allow, tc.wantAllow)
			}
		})
	}
}
