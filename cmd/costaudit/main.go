// costaudit 是 HCZ 商品成本治理的只读审计工具。
//
// 用法：
//
//	go run ./cmd/costaudit -dsn ./db/hcz.db -out docs/audits/PRODUCT_COST_AUDIT.md
//
// 只读打开开发库，遍历 is_active=true 的商品，按真实成本状态分类并输出 Markdown 审计报告。
// 本工具绝不写入或修改任何商品数据；对 COST_MISSING 商品只标记 ADMIN_INPUT_REQUIRED。
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type row struct {
	ID             uint   `gorm:"column:id"`
	TitleZhCN      string `gorm:"column:title_zh"`
	IsActive       bool   `gorm:"column:is_active"`
	PriceAmount    string `gorm:"column:price_amount"`
	CostPriceAmount string `gorm:"column:cost_price_amount"`
	IsCostExempt   bool   `gorm:"column:is_cost_exempt"`
}

func classify(r row) string {
	switch {
	case r.IsCostExempt:
		return "COST_EXEMPT"
	case toF(r.CostPriceAmount) > 0:
		// 成本倒挂：成本 >= 售价
		if toF(r.CostPriceAmount) >= toF(r.PriceAmount) && toF(r.PriceAmount) > 0 {
			return "UNPROFITABLE"
		}
		return "VALID_COST"
	default:
		// cost <= 0 且未豁免
		return "COST_MISSING"
	}
}

func toF(s string) float64 {
	var v float64
	_, _ = fmt.Sscanf(s, "%f", &v)
	return v
}

func main() {
	dsn := flag.String("dsn", "./db/hcz.db", "sqlite dsn（只读打开）")
	out := flag.String("out", "docs/audits/PRODUCT_COST_AUDIT.md", "输出 markdown 路径")
	flag.Parse()

	// 只读打开，避免误写开发库。
	ro := "file:" + *dsn + "?mode=ro"
	db, err := gorm.Open(sqlite.Open(ro), &gorm.Config{})
	if err != nil {
		log.Fatalf("open db: %v", err)
	}

	// is_cost_exempt 列由 GORM AutoMigrate 在建表/启动时补齐；只读审计跑在旧库上时可能尚未补列。
	hasExempt := db.Migrator().HasColumn("products", "is_cost_exempt")
	exemptCol := "0"
	if hasExempt {
		exemptCol = "is_cost_exempt"
	}

	var rows []row
	// deleted_at IS NULL：排除软删除；is_active=1：仅上架商品。
	sql := `SELECT id,
		COALESCE(json_extract(title_json, '$.zh-CN'), json_extract(title_json, '$.en-US'), '') AS title_zh,
		is_active,
		CAST(price_amount AS TEXT) AS price_amount,
		CAST(cost_price_amount AS TEXT) AS cost_price_amount,
		` + exemptCol + ` AS is_cost_exempt
		FROM products
		WHERE is_active = 1 AND deleted_at IS NULL
		ORDER BY id`
	if err := db.Raw(sql).Scan(&rows).Error; err != nil {
		log.Fatalf("query products: %v", err)
	}

	buckets := map[string][]row{}
	for _, r := range rows {
		buckets[classify(r)] = append(buckets[classify(r)], r)
	}

	md := "# PRODUCT_COST_AUDIT\n\n"
	md += fmt.Sprintf("- 生成时间：%s\n", time.Now().Format(time.RFC3339))
	md += fmt.Sprintf("- 数据源：`%s`（只读）\n", *dsn)
	md += fmt.Sprintf("- 范围：`is_active=1 AND deleted_at IS NULL`\n")
	md += "- 自动修改：**否**（COST_MISSING 一律标记 `ADMIN_INPUT_REQUIRED`，禁止脚本填值）\n"
	if !hasExempt {
		md += "- 注意：当前库尚未 `is_cost_exempt` 列（GORM AutoMigrate 将在下次启动时补齐），豁免列按 0 处理。\n"
	}
	md += "\n"
	md += "## 分类汇总\n\n"
	md += "| 分类 | 数量 | 含义 |\n|---|---|---|\n"
	md += fmt.Sprintf("| VALID_COST | %d | 成本已配置且 > 0 |\n", len(buckets["VALID_COST"]))
	md += fmt.Sprintf("| COST_MISSING | %d | 成本 <= 0 且未豁免 → ADMIN_INPUT_REQUIRED |\n", len(buckets["COST_MISSING"]))
	md += fmt.Sprintf("| COST_EXEMPT | %d | 真零成本豁免（数字权益/赠品/内测） |\n", len(buckets["COST_EXEMPT"]))
	md += fmt.Sprintf("| UNPROFITABLE | %d | 成本 >= 售价，价格倒挂 |\n", len(buckets["UNPROFITABLE"]))
	md += fmt.Sprintf("| **合计上架商品** | **%d** | |\n\n", len(rows))

	writeSection := func(title, code string, withAdminRequired bool) {
		list := buckets[code]
		md += fmt.Sprintf("## %s（%d）\n\n", title, len(list))
		if len(list) == 0 {
			md += "_无_\n\n"
			return
		}
		md += "| product_id | name | enabled | sale_price | cost_price | is_cost_exempt | profit_guard_status |\n"
		md += "|---|---|---|---|---|---|---|\n"
		for _, r := range list {
			status := code
			if withAdminRequired {
				status = "ADMIN_INPUT_REQUIRED"
			}
			name := r.TitleZhCN
			if name == "" {
				name = fmt.Sprintf("(product #%d)", r.ID)
			}
			md += fmt.Sprintf("| %d | %s | %v | %s | %s | %v | %s |\n",
				r.ID, name, r.IsActive, r.PriceAmount, r.CostPriceAmount, r.IsCostExempt, status)
		}
		md += "\n"
	}

	writeSection("COST_MISSING（成本缺失，需人工录入）", "COST_MISSING", true)
	writeSection("UNPROFITABLE（价格倒挂）", "UNPROFITABLE", true)
	writeSection("VALID_COST（成本正常）", "VALID_COST", false)
	writeSection("COST_EXEMPT（豁免）", "COST_EXEMPT", false)

	md += "## 处置原则\n\n"
	md += "1. COST_MISSING：**绝不自动填值**（不填售价/0/随机/估算）。由管理员人工录入真实采购成本后，再在 Pricing 页确认 Pricing Preview。\n"
	md += "2. UNPROFITABLE：成本 >= 售价，需管理员重新定价或调整成本。\n"
	md += "3. 在 Profit Guard 开启 `RequireCostPrice` 前，必须先清零 COST_MISSING 或对确属零成本商品打 `is_cost_exempt`。\n"

	if err := os.WriteFile(*out, []byte(md), 0o644); err != nil {
		log.Fatalf("write report: %v", err)
	}
	fmt.Printf("wrote %s (%d active products scanned)\n", *out, len(rows))
}
