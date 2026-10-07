package productcontract

import "errors"

// 商品查询、后台与写入用例共享的稳定错误 identity。
var (
	ErrNotFound                     = errors.New("not found")
	ErrSlugExists                   = errors.New("slug exists")
	ErrProductCategoryInvalid       = errors.New("product category invalid")
	ErrProductPriceInvalid          = errors.New("product price invalid")
	ErrProductPurchaseInvalid       = errors.New("product purchase invalid")
	ErrProductPurchaseLimitInvalid  = errors.New("product purchase limit invalid")
	ErrProductStockDisplayInvalid   = errors.New("product stock display invalid")
	ErrFulfillmentInvalid           = errors.New("fulfillment invalid")
	ErrManualStockInvalid           = errors.New("manual stock invalid")
	ErrProductSKUInvalid            = errors.New("product sku invalid")
	ErrProductSKUHasCardSecretStock = errors.New("product sku has card secret stock")
	ErrProductHasStock              = errors.New("product has stock")
	ErrProductHasOrderRecord        = errors.New("product has order record")
	ErrResellerProductNotListed     = errors.New("reseller product not listed")
	// P1：商品固定积分奖励配置校验失败（reward_enabled=true 时 reward_points 必须 > 0）。
	ErrRewardPointsInvalid = errors.New("reward points invalid")
)
