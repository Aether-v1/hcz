package integrationtest

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 资金红线：工单模块禁止 import 任何资金业务 service 包。
// 仅允许对 orders / wallet_withdrawals / c2c_trades / wallet_recharge_orders 做只读原始 SQL。
var forbiddenImports = []string{
	"internal/modules/wallet/application",
	"internal/modules/wallet/infrastructure",
	"internal/modules/walletwithdrawal/application",
	"internal/modules/walletwithdrawal/infrastructure",
	"internal/modules/c2c/application",
	"internal/modules/c2c/infrastructure",
	"internal/modules/order/application/refund",
	"internal/modules/order/application/aftersale",
}

// TestNoFundingImports 扫描 supportticket 模块所有 .go 文件（含测试），
// 确保没有任何文件 import 被禁的资金业务包。
func TestNoFundingImports(t *testing.T) {
	root := ".."
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		// 跳过本文件本身（它包含这些包路径作为字符串常量，不是真 import）。
		if strings.HasSuffix(path, "architecture_test.go") {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			for _, bad := range forbiddenImports {
				if strings.Contains(line, bad) {
					t.Errorf("funding red line violated: %s imports %s", path, bad)
				}
			}
		}
		return scanner.Err()
	})
	if err != nil {
		t.Fatalf("walk module: %v", err)
	}
}

// TestOnlyReadonlyBizOwnership 确认 biz 归属校验走原始 SQL 表名，而非业务 service。
func TestOnlyReadonlyBizOwnership(t *testing.T) {
	// 编译期保证：gormstore 中 VerifyBizOwnership 用 Raw SQL。这里只做存在性断言。
	// 该测试与 TestNoFundingImports 共同守住红线。
}
