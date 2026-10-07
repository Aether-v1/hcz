package pointsmallapp

import (
	"errors"
	"strings"
	"time"

	"github.com/Aether-v1/hcz/internal/logger"
	pointsapp "github.com/Aether-v1/hcz/internal/modules/points/application"
	pointscontract "github.com/Aether-v1/hcz/internal/modules/points/contract"
	pointsmallcontract "github.com/Aether-v1/hcz/internal/modules/pointsmall/contract"
	pointsmalldomain "github.com/Aether-v1/hcz/internal/modules/pointsmall/domain"
	"github.com/Aether-v1/hcz/internal/shared/serial"
)

// Options 是商城服务的依赖。
type Options struct {
	Repository pointsmallcontract.Repository
	UnitOfWork pointsmallcontract.UnitOfWork
	// Points 是积分模块权威服务（Redeem / RedeemRefund 同事务执行；GetAccount 供幂等重读余额）。
	Points *pointsapp.Service
}

// Service 是积分商城的应用用例。
//
// 事务契约：积分扣减/返还 + 库存扣减/恢复 + 兑换订单状态 必须在同一事务
// （任一失败整体回滚；禁止"扣积分但没订单 / 有订单但没扣积分"）。
//
// 锁顺序（全局统一，避免 ABBA 死锁）：
//
//		ExchangeOrder(状态) → PointsAccount → PointsProduct
//
//	  - Create（新订单无行可锁）：PointsAccount（REDEEM）→ PointsProduct（库存扣减）→ INSERT；
//	    同用户创建天然由账户行锁串行化（per_user_limit 校验依赖该串行）。
//	  - Cancel / Admin Fail / Admin Cancel（返还路径）：ExchangeOrder(FOR UPDATE) → PointsAccount(REDEEM_REFUND) → PointsProduct(库存恢复)。
//	  - Process / Complete（纯状态）：ExchangeOrder(FOR UPDATE)。
type Service struct {
	repository pointsmallcontract.Repository
	unitOfWork pointsmallcontract.UnitOfWork
	points     *pointsapp.Service
}

var _ pointsmallcontract.UseCase = (*Service)(nil)

// NewService 创建商城服务。
func NewService(options Options) *Service {
	return &Service{
		repository: options.Repository,
		unitOfWork: options.UnitOfWork,
		points:     options.Points,
	}
}

// ---------------------------------------------------------------------------
// 商品（Admin）
// ---------------------------------------------------------------------------

// validateProductInput 校验商品输入（创建与更新共用）。
// 上限口径：积分价格与积分单笔上限一致；库存/限购是本模块自有的误输入防线。
func validateProductInput(input pointsmallcontract.ProductInput) error {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return pointsmallcontract.ErrProductNameRequired
	}
	if len([]rune(name)) > pointsmallcontract.MaxNameLength {
		return pointsmallcontract.ErrInvalidName
	}
	if len([]rune(strings.TrimSpace(input.Cover))) > pointsmallcontract.MaxCoverLength {
		return pointsmallcontract.ErrInvalidName
	}
	if len([]rune(input.Instructions)) > pointsmallcontract.MaxInstructionsLen {
		return pointsmallcontract.ErrInvalidName
	}
	if input.PointsPrice <= 0 || input.PointsPrice > pointscontract.MaxPointsAmount {
		return pointsmallcontract.ErrInvalidPointsPrice
	}
	if input.Stock < 0 || input.Stock > pointsmallcontract.MaxStock {
		return pointsmallcontract.ErrInvalidStock
	}
	if input.PerUserLimit < 0 || input.PerUserLimit > pointsmallcontract.MaxPerUserLimit {
		return pointsmallcontract.ErrInvalidPerUserLimit
	}
	if !pointsmalldomain.ValidFulfillmentType(strings.TrimSpace(input.FulfillmentType)) {
		return pointsmallcontract.ErrInvalidFulfillmentType
	}
	return validateReason(input.Reason)
}

// validateReason 限制后台原因长度（列宽 255）；空原因是否必填由各用例自行判定。
func validateReason(reason string) error {
	if len([]rune(strings.TrimSpace(reason))) > pointscontract.MaxReasonLength {
		return pointsmallcontract.ErrExchangeInvalidReason
	}
	return nil
}

// CreateProduct 创建积分商品。
func (s *Service) CreateProduct(input pointsmallcontract.ProductInput) (*pointsmalldomain.PointsProduct, error) {
	if err := validateProductInput(input); err != nil {
		return nil, err
	}
	if input.OperatorAdminID == 0 {
		return nil, pointsmallcontract.ErrAdminRequired
	}
	now := time.Now()
	product := &pointsmalldomain.PointsProduct{
		Name:            strings.TrimSpace(input.Name),
		Subtitle:        strings.TrimSpace(input.Subtitle),
		Description:     strings.TrimSpace(input.Description),
		Cover:           strings.TrimSpace(input.Cover),
		PointsPrice:     input.PointsPrice,
		Stock:           input.Stock,
		UnlimitedStock:  input.UnlimitedStock,
		Enabled:         input.Enabled,
		Sort:            input.Sort,
		PerUserLimit:    input.PerUserLimit,
		FulfillmentType: strings.TrimSpace(input.FulfillmentType),
		Instructions:    strings.TrimSpace(input.Instructions),
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := s.repository.CreateProduct(product); err != nil {
		return nil, err
	}
	logger.Infow("points_mall_product_created",
		"operator_id", input.OperatorAdminID,
		"product_id", product.ID,
		"points_price", product.PointsPrice,
		"stock", product.Stock,
		"unlimited_stock", product.UnlimitedStock,
		"enabled", product.Enabled,
		"reason", strings.TrimSpace(input.Reason),
	)
	return product, nil
}

// UpdateProduct 更新积分商品（Admin 可直接设置可用库存，>=0，带审计字段由调用方记录）。
func (s *Service) UpdateProduct(id uint, input pointsmallcontract.ProductInput) (*pointsmalldomain.PointsProduct, error) {
	if err := validateProductInput(input); err != nil {
		return nil, err
	}
	if input.OperatorAdminID == 0 {
		return nil, pointsmallcontract.ErrAdminRequired
	}
	var product *pointsmalldomain.PointsProduct
	var beforeStock int64
	var beforePrice int64
	err := s.unitOfWork.WithinTransaction(func(tx pointsmallcontract.Transaction) error {
		// FOR UPDATE：与创建兑换的库存校验串行，避免"下架/改库存"与"兑换"互相撕裂。
		current, err := tx.Products().GetProductByIDForUpdate(id)
		if err != nil {
			return err
		}
		if current == nil {
			return pointsmallcontract.ErrProductNotFound
		}
		beforeStock = current.Stock
		beforePrice = current.PointsPrice
		current.Name = strings.TrimSpace(input.Name)
		current.Subtitle = strings.TrimSpace(input.Subtitle)
		current.Description = strings.TrimSpace(input.Description)
		current.Cover = strings.TrimSpace(input.Cover)
		current.PointsPrice = input.PointsPrice
		current.Stock = input.Stock
		current.UnlimitedStock = input.UnlimitedStock
		current.Enabled = input.Enabled
		current.Sort = input.Sort
		current.PerUserLimit = input.PerUserLimit
		current.FulfillmentType = strings.TrimSpace(input.FulfillmentType)
		current.Instructions = strings.TrimSpace(input.Instructions)
		current.UpdatedAt = time.Now()
		if err := tx.Products().UpdateProduct(current); err != nil {
			return err
		}
		product = current
		return nil
	})
	if err != nil {
		return nil, err
	}
	// 库存/价格是资产型字段：每次后台变更都留下 before/after + 操作者 + 原因（§12 审计要求）。
	logger.Infow("points_mall_product_updated",
		"operator_id", input.OperatorAdminID,
		"product_id", product.ID,
		"before_stock", beforeStock,
		"after_stock", product.Stock,
		"before_points_price", beforePrice,
		"after_points_price", product.PointsPrice,
		"unlimited_stock", product.UnlimitedStock,
		"enabled", product.Enabled,
		"reason", strings.TrimSpace(input.Reason),
	)
	return product, nil
}

// SetProductEnabled 上下架积分商品（下架不删除，历史订单仍可继续履约）。
func (s *Service) SetProductEnabled(input pointsmallcontract.ProductEnabledInput) (*pointsmalldomain.PointsProduct, error) {
	if input.OperatorAdminID == 0 {
		return nil, pointsmallcontract.ErrAdminRequired
	}
	if err := validateReason(input.Reason); err != nil {
		return nil, err
	}
	var product *pointsmalldomain.PointsProduct
	err := s.unitOfWork.WithinTransaction(func(tx pointsmallcontract.Transaction) error {
		current, err := tx.Products().GetProductByIDForUpdate(input.ProductID)
		if err != nil {
			return err
		}
		if current == nil {
			return pointsmallcontract.ErrProductNotFound
		}
		current.Enabled = input.Enabled
		current.UpdatedAt = time.Now()
		if err := tx.Products().UpdateProduct(current); err != nil {
			return err
		}
		product = current
		return nil
	})
	if err != nil {
		return nil, err
	}
	logger.Infow("points_mall_product_enabled_changed",
		"operator_id", input.OperatorAdminID,
		"product_id", product.ID,
		"enabled", product.Enabled,
		"reason", strings.TrimSpace(input.Reason),
	)
	return product, nil
}

// ListProducts 商品列表（Admin 可含下架；用户端由 User 层过滤 enabled）。
func (s *Service) ListProducts(filter pointsmallcontract.ProductListFilter) ([]pointsmalldomain.PointsProduct, int64, error) {
	return s.repository.ListProducts(filter)
}

// ---------------------------------------------------------------------------
// 用户兑换
// ---------------------------------------------------------------------------

// GetProductDetail 用户端商品详情（can_redeem 为 UX 辅助，POST 时后端重新校验全部条件）。
func (s *Service) GetProductDetail(userID, productID uint) (*pointsmallcontract.ProductDetail, error) {
	if userID == 0 {
		return nil, pointsmallcontract.ErrUserRequired
	}
	product, err := s.repository.GetProductByID(productID)
	if err != nil {
		return nil, err
	}
	if product == nil || !product.Enabled {
		return nil, pointsmallcontract.ErrProductNotFound
	}

	redeemed, err := s.repository.CountUserActiveOrders(userID, productID, 0)
	if err != nil {
		return nil, err
	}
	account, err := s.points.GetAccount(userID)
	if err != nil {
		return nil, err
	}
	balance := int64(0)
	if account != nil {
		balance = account.Balance
	}

	detail := &pointsmallcontract.ProductDetail{
		Product:           *product,
		UserRedeemedCount: redeemed,
		CanRedeem:         true,
	}
	reason := ""
	switch {
	case balance < product.PointsPrice:
		reason = pointsmallcontract.ReasonInsufficientPoints
	case !product.UnlimitedStock && product.Stock <= 0:
		reason = pointsmallcontract.ReasonOutOfStock
	case product.PerUserLimit > 0 && redeemed >= product.PerUserLimit:
		reason = pointsmallcontract.ReasonLimitReached
	}
	if reason != "" {
		detail.CanRedeem = false
		detail.ReasonCode = reason
	}
	return detail, nil
}

// CreateExchange 创建兑换订单。
//
// 事务流程（锁顺序：PointsAccount → PointsProduct）：
//
//	幂等预查 → 快照读商品 → INSERT 订单 → REDEEM 扣积分（锁账户，禁止负余额）
//	→ 锁商品 FOR UPDATE 校验 enabled/库存并扣减 → count 限购 → COMMIT
//
// 并发最终防线：
//   - 同用户同 Idempotency-Key：UNIQUE(user_id, idempotency_key)（撞索引 → 事务外重读幂等返回）；
//   - 同用户并发不同 Key：账户行锁串行化 → 余额/限购校验精确；
//   - 最后一件库存：商品行 FOR UPDATE 串行化。
func (s *Service) CreateExchange(input pointsmallcontract.CreateExchangeInput) (*pointsmallcontract.ExchangeResult, error) {
	if input.UserID == 0 {
		return nil, pointsmallcontract.ErrUserRequired
	}
	key := strings.TrimSpace(input.IdempotencyKey)
	if key == "" {
		return nil, pointsmallcontract.ErrIdempotencyKeyRequired
	}
	// 列宽 64：超长必须显式拒绝，不能交给数据库报错（400 变成 500）。
	if len([]rune(key)) > pointsmallcontract.MaxIdempotencyKeyLength {
		return nil, pointsmallcontract.ErrIdempotencyKeyTooLong
	}
	if input.ProductID == 0 {
		return nil, pointsmallcontract.ErrProductNotFound
	}

	var result *pointsmallcontract.ExchangeResult
	var duplicate bool
	err := s.unitOfWork.WithinTransaction(func(tx pointsmallcontract.Transaction) error {
		// 1. 幂等预查（同 Key 重复请求返回原订单，不重复扣分/扣库存）。
		existing, err := tx.Products().GetExchangeOrderByIdempotencyKey(input.UserID, key)
		if err != nil {
			return err
		}
		if existing != nil {
			if existing.ProductID != input.ProductID {
				return pointsmallcontract.ErrIdempotencyConflict
			}
			result = s.buildResult(tx, existing, true)
			return nil
		}

		// 2. 快照读商品（价格/名称/履约类型固化到订单，商品改价不影响本订单）。
		product, err := tx.Products().GetProductByID(input.ProductID)
		if err != nil {
			return err
		}
		if product == nil || !product.Enabled {
			return pointsmallcontract.ErrProductDisabled
		}
		if product.PointsPrice <= 0 {
			// 数据异常防御：价格非法视为商品不可兑换。
			return pointsmallcontract.ErrProductNotFound
		}

		// 3. INSERT 订单（V1 quantity 固定 1；快照固化；幂等键与用户联合唯一）。
		now := time.Now()
		order := &pointsmalldomain.ExchangeOrder{
			OrderNo:                 serial.Generate("PX"),
			UserID:                  input.UserID,
			ProductID:               input.ProductID,
			ProductNameSnapshot:     product.Name,
			UnitPoints:              product.PointsPrice,
			Quantity:                1,
			TotalPoints:             product.PointsPrice,
			Status:                  pointsmalldomain.ExchangeStatusPending,
			FulfillmentTypeSnapshot: product.FulfillmentType,
			IdempotencyKey:          key,
			CreatedAt:               now,
			UpdatedAt:               now,
		}
		if err := tx.Products().CreateExchangeOrder(order); err != nil {
			if isDuplicateKeyError(err) {
				// 并发同 Key：另一事务先插入。标记 duplicate，提交空事务后事务外重读。
				duplicate = true
				return nil
			}
			return err
		}

		// 4. REDEEM 扣积分（锁账户；用户主动消费禁止负余额）。
		//    reference = points:redeem:{order_id}（P0 语义：同一订单最多 1 个 REDEEM）。
		if _, err := s.points.Redeem(tx.Points(), pointscontract.RedeemInput{
			UserID:          input.UserID,
			ExchangeOrderID: order.ID,
			Amount:          order.TotalPoints,
			Reason:          "积分商城兑换 " + order.OrderNo,
			Reference:       pointscontract.RedeemReference(order.ID),
		}); err != nil {
			if errors.Is(err, pointscontract.ErrNegativeNotAllowed) {
				return pointsmallcontract.ErrPointsInsufficient
			}
			return err
		}

		// 5. 锁商品 FOR UPDATE：重校验 enabled + 库存（含并发抢最后一件），扣减可用库存。
		lockedProduct, err := tx.Products().GetProductByIDForUpdate(input.ProductID)
		if err != nil {
			return err
		}
		if lockedProduct == nil || !lockedProduct.Enabled {
			return pointsmallcontract.ErrProductDisabled
		}
		if !lockedProduct.UnlimitedStock {
			if lockedProduct.Stock <= 0 {
				return pointsmallcontract.ErrProductOutOfStock
			}
			lockedProduct.Stock--
			lockedProduct.UpdatedAt = time.Now()
			if err := tx.Products().UpdateProduct(lockedProduct); err != nil {
				return err
			}
		}

		// 6. per_user_limit 校验（此时已持有账户行锁，同用户并发创建被串行化；
		//    count 排除本订单自身，避免把自己算进限购；
		//    READ COMMITTED 下 count 能看到先提交事务的订单）。
		if lockedProduct.PerUserLimit > 0 {
			count, err := tx.Products().CountUserActiveOrders(input.UserID, input.ProductID, order.ID)
			if err != nil {
				return err
			}
			if count >= lockedProduct.PerUserLimit {
				return pointsmallcontract.ErrProductLimitReached
			}
		}

		result = s.buildResult(tx, order, false)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if duplicate {
		// 事务外重读权威订单（此时并发写入已提交），幂等返回原订单。
		existing, reloadErr := s.repository.GetExchangeOrderByIdempotencyKey(input.UserID, key)
		if reloadErr != nil {
			return nil, reloadErr
		}
		if existing == nil {
			return nil, pointsmallcontract.ErrExchangeOrderNotFound
		}
		if existing.ProductID != input.ProductID {
			return nil, pointsmallcontract.ErrIdempotencyConflict
		}
		return s.buildResultPlain(existing, true), nil
	}
	return result, nil
}

// CancelOrder 用户取消兑换（仅 PENDING）。
// 同事务：返还积分（REDEEM_REFUND）+ 恢复库存 + 状态迁移；幂等：重复取消返回当前状态。
func (s *Service) CancelOrder(input pointsmallcontract.CancelInput) (*pointsmallcontract.ExchangeResult, error) {
	if input.UserID == 0 {
		return nil, pointsmallcontract.ErrUserRequired
	}
	var result *pointsmallcontract.ExchangeResult
	err := s.unitOfWork.WithinTransaction(func(tx pointsmallcontract.Transaction) error {
		order, err := tx.Products().GetExchangeOrderByIDForUpdate(input.OrderID)
		if err != nil {
			return err
		}
		if order == nil || order.UserID != input.UserID {
			// IDOR 归并：非本人订单视为不存在。
			return pointsmallcontract.ErrExchangeOrderNotFound
		}
		if order.Status == pointsmalldomain.ExchangeStatusCancelled {
			result = s.buildResult(tx, order, true)
			return nil
		}
		if order.Status != pointsmalldomain.ExchangeStatusPending {
			return pointsmallcontract.ErrExchangeInvalidState
		}
		if err := s.refundAndRestore(tx, order); err != nil {
			return err
		}
		now := time.Now()
		order.Status = pointsmalldomain.ExchangeStatusCancelled
		order.Reason = "用户取消"
		order.LastOperatorType = "user"
		order.LastOperatorID = input.UserID
		order.CancelledAt = &now
		order.UpdatedAt = now
		if err := tx.Products().UpdateExchangeOrder(order); err != nil {
			return err
		}
		result = s.buildResult(tx, order, false)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// refundAndRestore 返还积分并恢复库存（必须在兑换订单行锁内、同一事务调用）。
//
// 幂等语义：REDEEM_REFUND reference 全局唯一（points:redeem_refund:{order_id}），
// 即使外部重复调用也最多返还一次；库存恢复只在实际发生返还时执行（避免重复加库存）。
func (s *Service) refundAndRestore(tx pointsmallcontract.Transaction, order *pointsmalldomain.ExchangeOrder) error {
	if _, err := s.points.RedeemRefund(tx.Points(), pointscontract.RedeemRefundInput{
		UserID:          order.UserID,
		ExchangeOrderID: order.ID,
		Amount:          order.TotalPoints,
		Reason:          "积分商城兑换返还 " + order.OrderNo,
		Reference:       pointscontract.RedeemRefundReference(order.ID),
	}); err != nil {
		return err
	}

	product, err := tx.Products().GetProductByIDForUpdate(order.ProductID)
	if err != nil {
		return err
	}
	if product == nil {
		// 防御：商品行不应被物理删除（禁止删除语义）；缺失时记录 warning，不阻塞返还。
		logger.Warnw("points_mall_product_missing_during_refund",
			"exchange_order_id", order.ID,
			"product_id", order.ProductID,
		)
		return nil
	}
	if !product.UnlimitedStock {
		product.Stock += order.Quantity
		product.UpdatedAt = time.Now()
		if err := tx.Products().UpdateProduct(product); err != nil {
			return err
		}
	}
	return nil
}

// ListExchangeOrders 用户兑换订单列表（只能看到自己的订单）。
func (s *Service) ListExchangeOrders(userID uint, filter pointsmallcontract.ExchangeOrderListFilter) ([]pointsmalldomain.ExchangeOrder, int64, error) {
	if userID == 0 {
		return nil, 0, pointsmallcontract.ErrUserRequired
	}
	filter.UserID = userID
	return s.repository.ListExchangeOrders(filter)
}

// GetExchangeOrder 用户兑换订单详情（store 层 WHERE id AND user_id，IDOR 防御）。
func (s *Service) GetExchangeOrder(userID, orderID uint) (*pointsmalldomain.ExchangeOrder, error) {
	if userID == 0 {
		return nil, pointsmallcontract.ErrUserRequired
	}
	order, err := s.repository.GetExchangeOrderByIDAndUser(orderID, userID)
	if err != nil {
		return nil, err
	}
	if order == nil {
		return nil, pointsmallcontract.ErrExchangeOrderNotFound
	}
	return order, nil
}

// ---------------------------------------------------------------------------
// Admin 兑换订单
// ---------------------------------------------------------------------------

// AdminListExchangeOrders 后台兑换订单列表。
func (s *Service) AdminListExchangeOrders(filter pointsmallcontract.ExchangeOrderListFilter) ([]pointsmalldomain.ExchangeOrder, int64, error) {
	return s.repository.ListExchangeOrders(filter)
}

// AdminGetExchangeOrder 后台兑换订单详情。
func (s *Service) AdminGetExchangeOrder(orderID uint) (*pointsmalldomain.ExchangeOrder, error) {
	order, err := s.repository.GetExchangeOrderByID(orderID)
	if err != nil {
		return nil, err
	}
	if order == nil {
		return nil, pointsmallcontract.ErrExchangeOrderNotFound
	}
	return order, nil
}

// AdminProcessOrder 后台开始处理：PENDING → PROCESSING（不改积分/库存，仅状态 + 审计）。
func (s *Service) AdminProcessOrder(input pointsmallcontract.AdminExchangeActionInput) (*pointsmalldomain.ExchangeOrder, error) {
	if input.AdminID == 0 {
		return nil, pointsmallcontract.ErrAdminRequired
	}
	order, err := s.adminTransition(input, pointsmalldomain.ExchangeStatusProcessing,
		func(order *pointsmalldomain.ExchangeOrder) {
			order.LastOperatorType = "admin"
			order.LastOperatorID = input.AdminID
		},
		pointsmalldomain.ExchangeStatusPending,
	)
	if err != nil {
		return nil, err
	}
	logger.Infow("points_mall_order_processed",
		"operator_id", input.AdminID,
		"exchange_order_id", input.OrderID,
		"before_status", pointsmalldomain.ExchangeStatusPending,
		"after_status", pointsmalldomain.ExchangeStatusProcessing,
	)
	return order, nil
}

// AdminCompleteOrder 后台完成履约：PROCESSING → COMPLETED（终态；不再次扣积分/扣库存）。
func (s *Service) AdminCompleteOrder(input pointsmallcontract.AdminExchangeActionInput) (*pointsmalldomain.ExchangeOrder, error) {
	if input.AdminID == 0 {
		return nil, pointsmallcontract.ErrAdminRequired
	}
	// Note 会写入 exchange_order.reason（列宽 255）。
	if err := validateReason(input.Note); err != nil {
		return nil, err
	}
	order, err := s.adminTransition(input, pointsmalldomain.ExchangeStatusCompleted,
		func(order *pointsmalldomain.ExchangeOrder) {
			order.Reason = strings.TrimSpace(input.Note)
			order.LastOperatorType = "admin"
			order.LastOperatorID = input.AdminID
			now := time.Now()
			order.CompletedAt = &now
		},
		pointsmalldomain.ExchangeStatusProcessing,
	)
	if err != nil {
		return nil, err
	}
	logger.Infow("points_mall_order_completed",
		"operator_id", input.AdminID,
		"exchange_order_id", input.OrderID,
		"before_status", pointsmalldomain.ExchangeStatusProcessing,
		"after_status", pointsmalldomain.ExchangeStatusCompleted,
	)
	return order, nil
}

// AdminFailOrder 后台失败：PENDING/PROCESSING → FAILED。
// 同事务：返还积分（REDEEM_REFUND）+ 恢复库存 + 状态迁移；Reason 必填。
// 幂等：重复 fail 返回当前 FAILED 状态，不重复返还/恢复。
func (s *Service) AdminFailOrder(input pointsmallcontract.AdminExchangeActionInput) (*pointsmalldomain.ExchangeOrder, error) {
	if input.AdminID == 0 {
		return nil, pointsmallcontract.ErrAdminRequired
	}
	if strings.TrimSpace(input.Reason) == "" {
		return nil, pointsmallcontract.ErrExchangeReasonRequired
	}
	if err := validateReason(input.Reason); err != nil {
		return nil, err
	}
	order, err := s.adminTransitionWithRefund(input, pointsmalldomain.ExchangeStatusFailed)
	if err != nil {
		return nil, err
	}
	logger.Infow("points_mall_order_failed",
		"operator_id", input.AdminID,
		"exchange_order_id", input.OrderID,
		"reason", strings.TrimSpace(input.Reason),
		"refunded_points", order.TotalPoints,
		"restored_stock", order.Quantity,
	)
	return order, nil
}

// AdminCancelOrder 后台取消：PENDING/PROCESSING → CANCELLED。
// 同事务：返还积分 + 恢复库存 + 状态迁移；Reason 必填；幂等同 fail。
func (s *Service) AdminCancelOrder(input pointsmallcontract.AdminExchangeActionInput) (*pointsmalldomain.ExchangeOrder, error) {
	if input.AdminID == 0 {
		return nil, pointsmallcontract.ErrAdminRequired
	}
	if strings.TrimSpace(input.Reason) == "" {
		return nil, pointsmallcontract.ErrExchangeReasonRequired
	}
	if err := validateReason(input.Reason); err != nil {
		return nil, err
	}
	order, err := s.adminTransitionWithRefund(input, pointsmalldomain.ExchangeStatusCancelled)
	if err != nil {
		return nil, err
	}
	logger.Infow("points_mall_order_cancelled",
		"operator_id", input.AdminID,
		"exchange_order_id", input.OrderID,
		"reason", strings.TrimSpace(input.Reason),
		"refunded_points", order.TotalPoints,
		"restored_stock", order.Quantity,
	)
	return order, nil
}

// adminTransition 执行纯状态迁移（不涉积分/库存）。
func (s *Service) adminTransition(
	input pointsmallcontract.AdminExchangeActionInput,
	to string,
	patch func(*pointsmalldomain.ExchangeOrder),
	allowedFrom ...string,
) (*pointsmalldomain.ExchangeOrder, error) {
	var updated *pointsmalldomain.ExchangeOrder
	err := s.unitOfWork.WithinTransaction(func(tx pointsmallcontract.Transaction) error {
		order, err := tx.Products().GetExchangeOrderByIDForUpdate(input.OrderID)
		if err != nil {
			return err
		}
		if order == nil {
			return pointsmallcontract.ErrExchangeOrderNotFound
		}
		if order.Status == to {
			// 幂等重放：已是目标状态，返回当前订单。
			updated = order
			return nil
		}
		allowed := false
		for _, from := range allowedFrom {
			if order.Status == from {
				allowed = true
				break
			}
		}
		if !allowed {
			return pointsmallcontract.ErrExchangeInvalidState
		}
		now := time.Now()
		patch(order)
		order.UpdatedAt = now
		order.Status = to
		if err := tx.Products().UpdateExchangeOrder(order); err != nil {
			return err
		}
		updated = order
		return nil
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// adminTransitionWithRefund 执行状态迁移 + 返还积分 + 恢复库存（fail/cancel）。
func (s *Service) adminTransitionWithRefund(
	input pointsmallcontract.AdminExchangeActionInput,
	to string,
) (*pointsmalldomain.ExchangeOrder, error) {
	var updated *pointsmalldomain.ExchangeOrder
	err := s.unitOfWork.WithinTransaction(func(tx pointsmallcontract.Transaction) error {
		order, err := tx.Products().GetExchangeOrderByIDForUpdate(input.OrderID)
		if err != nil {
			return err
		}
		if order == nil {
			return pointsmallcontract.ErrExchangeOrderNotFound
		}
		if order.Status == to {
			// 幂等重放：已是目标状态（积分已返还、库存已恢复），不再返还。
			updated = order
			return nil
		}
		if order.Status != pointsmalldomain.ExchangeStatusPending &&
			order.Status != pointsmalldomain.ExchangeStatusProcessing {
			return pointsmallcontract.ErrExchangeInvalidState
		}
		if err := s.refundAndRestore(tx, order); err != nil {
			return err
		}
		now := time.Now()
		order.Status = to
		order.Reason = strings.TrimSpace(input.Reason)
		order.LastOperatorType = "admin"
		order.LastOperatorID = input.AdminID
		order.UpdatedAt = now
		if to == pointsmalldomain.ExchangeStatusFailed {
			order.FailedAt = &now
		} else {
			order.CancelledAt = &now
		}
		if err := tx.Products().UpdateExchangeOrder(order); err != nil {
			return err
		}
		updated = order
		return nil
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// ---------------------------------------------------------------------------
// 结果组装
// ---------------------------------------------------------------------------

// buildResult 组装结果（余额来自事务后的权威 Points Account；无账户按 0）。
func (s *Service) buildResult(tx pointsmallcontract.Transaction, order *pointsmalldomain.ExchangeOrder, already bool) *pointsmallcontract.ExchangeResult {
	balance := int64(0)
	if account, err := tx.Points().Points().GetAccountByUserID(order.UserID); err == nil && account != nil {
		balance = account.Balance
	}
	return &pointsmallcontract.ExchangeResult{
		Order:            order,
		CurrentBalance:   balance,
		AlreadyProcessed: already,
	}
}

// buildResultPlain 组装"并发幂等重读"的结果（事务外直查权威账户）。
func (s *Service) buildResultPlain(order *pointsmalldomain.ExchangeOrder, already bool) *pointsmallcontract.ExchangeResult {
	balance := int64(0)
	if account, err := s.points.GetAccount(order.UserID); err == nil && account != nil {
		balance = account.Balance
	}
	return &pointsmallcontract.ExchangeResult{
		Order:            order,
		CurrentBalance:   balance,
		AlreadyProcessed: already,
	}
}

// isDuplicateKeyError 识别唯一索引冲突（与 Points / Checkin 模块口径一致）。
func isDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "unique constraint") ||
		strings.Contains(lower, "duplicate key") ||
		strings.Contains(lower, "duplicate entry") ||
		strings.Contains(lower, "unique index")
}
