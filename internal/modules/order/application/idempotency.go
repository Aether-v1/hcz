package application

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
)

// orderIdempotencyFingerprint 计算订单创建请求的 payload 指纹。
// 用于检测同一个 Idempotency-Key 是否被用于不同的业务请求（payload conflict）。
// 指纹覆盖：items（按 product_id+sku_id 排序）、coupon_code、affiliate_code。
// 不覆盖：client_ip（同一次重试可能来自不同出口 IP）、tenant（由用户上下文决定）、manual_form_data（结构复杂且非订单核心标识）。
func orderIdempotencyFingerprint(items []CreateOrderItem, couponCode, affiliateCode string) string {
	type fingerprintItem struct {
		ProductID uint `json:"product_id"`
		SKUID     uint `json:"sku_id"`
		Quantity  int  `json:"quantity"`
	}

	fpItems := make([]fingerprintItem, 0, len(items))
	for _, item := range items {
		fpItems = append(fpItems, fingerprintItem{
			ProductID: item.ProductID,
			SKUID:     item.SKUID,
			Quantity:  item.Quantity,
		})
	}
	sort.Slice(fpItems, func(i, j int) bool {
		if fpItems[i].ProductID != fpItems[j].ProductID {
			return fpItems[i].ProductID < fpItems[j].ProductID
		}
		return fpItems[i].SKUID < fpItems[j].SKUID
	})

	payload := struct {
		Items         []fingerprintItem `json:"items"`
		CouponCode    string            `json:"coupon_code"`
		AffiliateCode string            `json:"affiliate_code"`
	}{
		Items:         fpItems,
		CouponCode:    couponCode,
		AffiliateCode: affiliateCode,
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		// json.Marshal 对这种结构不会失败；兜底返回空串由调用方处理。
		return ""
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// isUniqueConstraintViolation 检测错误是否为数据库唯一约束冲突。
// 兼容 SQLite（UNIQUE constraint failed）和 PostgreSQL（duplicate key value violates unique constraint）。
// 用于并发幂等竞态兜底：唯一索引拦截第二个插入后，重新查询返回已有订单。
func isUniqueConstraintViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique constraint") ||
		strings.Contains(msg, "duplicate key") ||
		strings.Contains(msg, "error 1062") || // MySQL
		strings.Contains(msg, "sqlstate 23505") // PostgreSQL
}

// 确保 errors 包被引用（避免未使用导入）。
var _ = errors.New
