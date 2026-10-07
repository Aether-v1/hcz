package contract

import (
	pointscontract "github.com/Aether-v1/hcz/internal/modules/points/contract"
	pointsmalldomain "github.com/Aether-v1/hcz/internal/modules/pointsmall/domain"
)

// Repository 拥有积分商城聚合（商品 + 兑换订单）的持久化。
// 事务调用方通过 Transaction 拿到绑定到同一事务的同一端口。
type Repository interface {
	// Products
	ListProducts(filter ProductListFilter) ([]pointsmalldomain.PointsProduct, int64, error)
	GetProductByID(id uint) (*pointsmalldomain.PointsProduct, error)
	GetProductByIDForUpdate(id uint) (*pointsmalldomain.PointsProduct, error)
	CreateProduct(product *pointsmalldomain.PointsProduct) error
	UpdateProduct(product *pointsmalldomain.PointsProduct) error

	// Exchange orders
	// CountUserActiveOrders 统计用户对某商品的有效兑换数（PENDING/PROCESSING/COMPLETED；
	// FAILED/CANCELLED 已返还积分与库存，不计入限购）。excludeOrderID>0 时排除自身订单
	// （创建流程中 count 不得把自己算进限购）。
	CountUserActiveOrders(userID, productID, excludeOrderID uint) (int64, error)
	CreateExchangeOrder(order *pointsmalldomain.ExchangeOrder) error
	GetExchangeOrderByID(id uint) (*pointsmalldomain.ExchangeOrder, error)
	GetExchangeOrderByIDForUpdate(id uint) (*pointsmalldomain.ExchangeOrder, error)
	GetExchangeOrderByIDAndUser(id, userID uint) (*pointsmalldomain.ExchangeOrder, error)
	GetExchangeOrderByIdempotencyKey(userID uint, key string) (*pointsmalldomain.ExchangeOrder, error)
	UpdateExchangeOrder(order *pointsmalldomain.ExchangeOrder) error
	ListExchangeOrders(filter ExchangeOrderListFilter) ([]pointsmalldomain.ExchangeOrder, int64, error)
}

// Transaction 是已打开数据库事务的商城域视图，不暴露 ORM 原语。
type Transaction interface {
	Products() Repository
	// Points 返回绑定到同一事务的积分域视图（REDEEM / REDEEM_REFUND 同事务）。
	Points() pointscontract.Transaction
}

// UnitOfWork 暴露开启事务的能力。
type UnitOfWork interface {
	WithinTransaction(fn func(Transaction) error) error
}

// UseCase 是商城模块对外的应用端口。
type UseCase interface {
	// 商品
	ListProducts(filter ProductListFilter) ([]pointsmalldomain.PointsProduct, int64, error)
	GetProductDetail(userID, productID uint) (*ProductDetail, error)
	CreateProduct(input ProductInput) (*pointsmalldomain.PointsProduct, error)
	UpdateProduct(id uint, input ProductInput) (*pointsmalldomain.PointsProduct, error)
	SetProductEnabled(input ProductEnabledInput) (*pointsmalldomain.PointsProduct, error)

	// 兑换
	CreateExchange(input CreateExchangeInput) (*ExchangeResult, error)
	CancelOrder(input CancelInput) (*ExchangeResult, error) // 用户取消（仅 PENDING）
	ListExchangeOrders(userID uint, filter ExchangeOrderListFilter) ([]pointsmalldomain.ExchangeOrder, int64, error)
	GetExchangeOrder(userID, orderID uint) (*pointsmalldomain.ExchangeOrder, error)

	// Admin
	AdminListExchangeOrders(filter ExchangeOrderListFilter) ([]pointsmalldomain.ExchangeOrder, int64, error)
	AdminGetExchangeOrder(orderID uint) (*pointsmalldomain.ExchangeOrder, error)
	AdminProcessOrder(input AdminExchangeActionInput) (*pointsmalldomain.ExchangeOrder, error)
	AdminCompleteOrder(input AdminExchangeActionInput) (*pointsmalldomain.ExchangeOrder, error)
	AdminFailOrder(input AdminExchangeActionInput) (*pointsmalldomain.ExchangeOrder, error)
	AdminCancelOrder(input AdminExchangeActionInput) (*pointsmalldomain.ExchangeOrder, error)
}
