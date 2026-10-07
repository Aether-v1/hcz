package authz

import "testing"

// TestPointsRBACFinalMatrix 是 P4 §43 的后台积分/签到/商城权限终局矩阵。
//
// 每条断言都是一个显式决策，而不是"通配顺手放权"：
//   - 资金写入（adjust / compensate）只属于 finance；
//   - 只读能力（account / ledger / stats / 签到历史）属于 readonly_auditor，
//     并被 operations / finance / system_admin 继承；
//   - 负余额排查列表（points/accounts）属于 finance；
//   - 商城商品维护属于 operations（+ system_admin 全量），
//     兑换订单 fail/cancel 涉及积分返还与库存恢复，属于 finance（operations 仅 process/complete）；
//   - 后台没有任何签到写入口（补签/改签/删除），因此任何角色的写请求都必须被拒。
func TestPointsRBACFinalMatrix(t *testing.T) {
	svc := setupPointsPolicyTest(t)

	const (
		auditor = uint(601)
		ops     = uint(602)
		fin     = uint(603)
		sys     = uint(604)
		none    = uint(605)
	)
	roles := map[uint]string{auditor: "readonly_auditor", ops: "operations", fin: "finance", sys: "system_admin"}
	for adminID, role := range roles {
		if err := svc.SetAdminRoles(adminID, []string{role}); err != nil {
			t.Fatalf("set role %s: %v", role, err)
		}
	}

	cases := []struct {
		name      string
		adminID   uint
		path      string
		action    string
		wantAllow bool
	}{
		// 只读能力
		{"auditor reads points", auditor, "/api/v1/admin/users/42/points", "GET", true},
		{"auditor reads ledger", auditor, "/api/v1/admin/users/42/points/ledger", "GET", true},
		{"auditor reads stats", auditor, "/api/v1/admin/points/stats", "GET", true},
		{"auditor reads checkins", auditor, "/api/v1/admin/users/42/checkins", "GET", true},
		{"auditor cannot list accounts", auditor, "/api/v1/admin/points/accounts", "GET", false},

		// 资金写入：仅 finance
		{"auditor cannot adjust", auditor, "/api/v1/admin/users/42/points/adjust", "POST", false},
		{"auditor cannot compensate", auditor, "/api/v1/admin/users/42/points/compensate", "POST", false},
		{"operations cannot adjust", ops, "/api/v1/admin/users/42/points/adjust", "POST", false},
		{"operations cannot compensate", ops, "/api/v1/admin/users/42/points/compensate", "POST", false},
		{"system_admin cannot adjust", sys, "/api/v1/admin/users/42/points/adjust", "POST", false},
		{"system_admin cannot compensate", sys, "/api/v1/admin/users/42/points/compensate", "POST", false},
		{"finance adjusts", fin, "/api/v1/admin/users/42/points/adjust", "POST", true},
		{"finance compensates", fin, "/api/v1/admin/users/42/points/compensate", "POST", true},
		{"finance lists accounts", fin, "/api/v1/admin/points/accounts", "GET", true},
		{"system_admin cannot list accounts", sys, "/api/v1/admin/points/accounts", "GET", false},
		{"operations cannot list accounts", ops, "/api/v1/admin/points/accounts", "GET", false},

		// 签到：后台只读
		{"operations reads checkins", ops, "/api/v1/admin/users/42/checkins", "GET", true},
		{"finance reads checkins", fin, "/api/v1/admin/users/42/checkins", "GET", true},
		{"system_admin reads checkins", sys, "/api/v1/admin/users/42/checkins", "GET", true},
		{"auditor cannot write checkins", auditor, "/api/v1/admin/users/42/checkins", "POST", false},
		{"finance cannot delete checkins", fin, "/api/v1/admin/users/42/checkins", "DELETE", false},

		// 签到配置：system_admin 专属（finance 无 settings 权限）
		{"system_admin updates checkin config", sys, "/api/v1/admin/settings/checkin", "PUT", true},
		{"finance updates checkin config", fin, "/api/v1/admin/settings/checkin", "PUT", false},
		{"auditor reads checkin config", auditor, "/api/v1/admin/settings/checkin", "GET", false},

		// 商城商品维护
		{"operations creates product", ops, "/api/v1/admin/points/products", "POST", true},
		{"finance creates product", fin, "/api/v1/admin/points/products", "POST", false},
		{"system_admin creates product", sys, "/api/v1/admin/points/products", "POST", true},
		{"auditor lists products", auditor, "/api/v1/admin/points/products", "GET", true},

		// 兑换订单履约：operations 仅 process/complete；返还类动作属于 finance
		{"operations processes order", ops, "/api/v1/admin/points/exchange-orders/7/process", "POST", true},
		{"operations completes order", ops, "/api/v1/admin/points/exchange-orders/7/complete", "POST", true},
		{"operations fails order", ops, "/api/v1/admin/points/exchange-orders/7/fail", "POST", false},
		{"operations cancels order", ops, "/api/v1/admin/points/exchange-orders/7/cancel", "POST", false},
		{"finance fails order", fin, "/api/v1/admin/points/exchange-orders/7/fail", "POST", true},
		{"finance cancels order", fin, "/api/v1/admin/points/exchange-orders/7/cancel", "POST", true},
		{"auditor lists orders", auditor, "/api/v1/admin/points/exchange-orders", "GET", true},
		{"auditor fails order", auditor, "/api/v1/admin/points/exchange-orders/7/fail", "POST", false},

		// 无角色管理员：一律拒绝
		{"roleless admin reads points", none, "/api/v1/admin/users/42/points", "GET", false},
		{"roleless admin adjusts", none, "/api/v1/admin/users/42/points/adjust", "POST", false},
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
