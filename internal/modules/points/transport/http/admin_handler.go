package pointsphttp

import (
	"errors"
	"strings"
	"time"

	userdomain "github.com/Aether-v1/hcz/internal/modules/identity/user/domain"
	pointscontract "github.com/Aether-v1/hcz/internal/modules/points/contract"
	pointsdomain "github.com/Aether-v1/hcz/internal/modules/points/domain"

	"github.com/Aether-v1/hcz/internal/platform/http/ginutil"
	"github.com/Aether-v1/hcz/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

// AdminPointsService 是后台积分管理所需的最小端口。
// 查询一律使用 contract 过滤结构体：transport 层不拼 SQL，也不自建第二套查询逻辑。
type AdminPointsService interface {
	GetAccount(userID uint) (*pointsdomain.Account, error)
	ListLedgerEntries(filter pointscontract.LedgerListFilter) ([]pointsdomain.LedgerEntry, int64, error)
	ListAccounts(filter pointscontract.AccountListFilter) ([]pointscontract.AccountWithUser, int64, error)
	AdminAdjust(input pointscontract.AdjustInput) (*pointsdomain.Account, *pointsdomain.LedgerEntry, error)
	AdminCompensate(input pointscontract.CompensateInput) (*pointsdomain.Account, *pointsdomain.LedgerEntry, error)
	GetStats(now time.Time) (*pointscontract.StatsResult, error)
}

// AdminUserReader 用于确认调整目标用户存在。
type AdminUserReader interface {
	GetByID(id uint) (*userdomain.User, error)
}

// AdminHandler 处理后台积分 HTTP 请求。
type AdminHandler struct {
	points AdminPointsService
	users  AdminUserReader
}

func NewAdminHandler(points AdminPointsService, users AdminUserReader) *AdminHandler {
	if points == nil || users == nil {
		panic("points admin handler: required dependency is nil")
	}
	return &AdminHandler{points: points, users: users}
}

// GetUserPoints 后台查看用户积分账户（余额 / 累计获得 / 累计消费）。
// total_spent 是历史累计消费口径（含已返还），不是净消费，见 API 文档语义说明。
func (h *AdminHandler) GetUserPoints(c *gin.Context) {
	user, _, ok := h.requireTargetUser(c)
	if !ok {
		return
	}
	account, err := h.points.GetAccount(user.ID)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.points_account_fetch_failed", err)
		return
	}
	response.Success(c, gin.H{
		"user":    user,
		"account": normalizeAccountView(account),
	})
}

// GetUserLedgerEntries 后台分页查看用户积分流水。
// 过滤白名单：action_type / source_type / reference / direction / 时间区间；
// 排序固定 created_at DESC, id DESC，不接受客户端排序字段。
func (h *AdminHandler) GetUserLedgerEntries(c *gin.Context) {
	user, _, ok := h.requireTargetUser(c)
	if !ok {
		return
	}
	filter, ok := ledgerFilterFromRequest(c)
	if !ok {
		return
	}
	filter.UserID = user.ID

	entries, total, err := h.points.ListLedgerEntries(filter)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.points_ledger_fetch_failed", err)
		return
	}
	pagination := response.BuildPagination(filter.Page, filter.PageSize, total)
	response.SuccessWithPage(c, entries, pagination)
}

// AdminAdjustUserPointsRequest 后台增减积分请求体。
// Amount 恒为正数；扣减由 Operation=subtract 表达（禁止负数金额，避免双重负数错误）。
type AdminAdjustUserPointsRequest struct {
	Amount    int64  `json:"amount" binding:"required"`
	Operation string `json:"operation"` // add / subtract
	Reason    string `json:"reason" binding:"required"`
}

// AdminCompensateUserPointsRequest 后台人工补偿请求体（P4）。
// 补偿恒为入账；OrderID 可选，仅用于把流水关联到发生异常的订单。
type AdminCompensateUserPointsRequest struct {
	Amount  int64  `json:"amount" binding:"required"`
	Reason  string `json:"reason" binding:"required"`
	OrderID uint   `json:"order_id"`
}

// AdjustUserPoints 后台增减用户积分。
//
// 要求：
//   - Admin JWT（ginutil.GetAdminID）+ RBAC（路由层）；
//   - Idempotency-Key 请求头必填且限长（重复请求不重复入账）；
//   - Reason 必填；
//   - 行锁与账户更新由 application 层单事务保证。
func (h *AdminHandler) AdjustUserPoints(c *gin.Context) {
	user, adminID, ok := h.requireTargetUser(c)
	if !ok {
		return
	}
	userID := user.ID

	var req AdminAdjustUserPointsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}

	idemKey, code, msg := parseIdempotencyKey(c)
	if msg != "" {
		ginutil.RespondError(c, code, msg, nil)
		return
	}
	op := strings.ToLower(strings.TrimSpace(req.Operation))
	if op != "add" && op != "subtract" {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}

	account, entry, err := h.points.AdminAdjust(pointscontract.AdjustInput{
		UserID:          userID,
		OperatorAdminID: adminID,
		Operation:       op,
		Amount:          req.Amount,
		Reason:          strings.TrimSpace(req.Reason),
		Reference:       pointscontract.AdminAdjustReference(idemKey),
	})
	if err != nil {
		respondPointsMutationError(c, err)
		return
	}

	response.Success(c, gin.H{
		"account": normalizeAccountView(account),
		"ledger":  entry,
	})
}

// CompensateUserPoints 后台人工补偿积分（P4）。
//
// 用于系统生命周期之外的一次性补发（例如历史订单漏发积分）。
// 独立 action ADMIN_COMPENSATION：不伪造 ORDER_REWARD，也不重放订单完成生命周期
// （重放会重复触发佣金/通知等副作用）。补偿不恢复任何库存、不改变任何订单状态。
func (h *AdminHandler) CompensateUserPoints(c *gin.Context) {
	user, adminID, ok := h.requireTargetUser(c)
	if !ok {
		return
	}

	var req AdminCompensateUserPointsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	idemKey, code, msg := parseIdempotencyKey(c)
	if msg != "" {
		ginutil.RespondError(c, code, msg, nil)
		return
	}

	account, entry, err := h.points.AdminCompensate(pointscontract.CompensateInput{
		UserID:          user.ID,
		OperatorAdminID: adminID,
		Amount:          req.Amount,
		Reason:          strings.TrimSpace(req.Reason),
		Reference:       pointscontract.AdminCompensationReference(idemKey),
		OrderID:         req.OrderID,
	})
	if err != nil {
		respondPointsMutationError(c, err)
		return
	}
	response.Success(c, gin.H{
		"account": normalizeAccountView(account),
		"ledger":  entry,
	})
}

// requireTargetUser 校验 Admin 身份并确认 :id 指向真实用户（存在性 + 正确域）。
// 失败时响应已写出，返回 ok=false；调用方必须立即 return。
func (h *AdminHandler) requireTargetUser(c *gin.Context) (*userdomain.User, uint, bool) {
	userID, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.user_id_invalid", nil)
		return nil, 0, false
	}
	adminID, ok := ginutil.GetAdminID(c)
	if !ok {
		return nil, 0, false
	}
	user, err := h.users.GetByID(userID)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
		return nil, 0, false
	}
	if user == nil {
		ginutil.RespondError(c, response.CodeNotFound, "error.user_not_found", nil)
		return nil, 0, false
	}
	return user, adminID, true
}

// parseIdempotencyKey 读取并校验 Idempotency-Key（必填 + 限长）。
// 超长 key 必须显式拒绝：否则派生的 reference 超列宽只会得到 DB 错误（500）或静默截断。
func parseIdempotencyKey(c *gin.Context) (string, int, string) {
	idemKey := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	if idemKey == "" {
		return "", response.CodeBadRequest, "error.idempotency_key_required"
	}
	if len([]rune(idemKey)) > pointscontract.MaxIdempotencyKeyLength {
		return "", response.CodeBadRequest, "error.points_idempotency_key_too_long"
	}
	return idemKey, 0, ""
}

// respondPointsMutationError 把积分 mutation 的领域错误映射为稳定 HTTP 契约。
func respondPointsMutationError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, pointscontract.ErrIdempotencyConflict):
		ginutil.RespondError(c, response.CodeConflict, "error.idempotency_conflict", nil)
	case errors.Is(err, pointscontract.ErrInvalidOperation):
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
	case errors.Is(err, pointscontract.ErrInvalidAmount),
		errors.Is(err, pointscontract.ErrAmountTooLarge):
		ginutil.RespondError(c, response.CodeBadRequest, "error.points_amount_invalid", nil)
	case errors.Is(err, pointscontract.ErrReasonRequired):
		ginutil.RespondError(c, response.CodeBadRequest, "error.points_adjust_remark_required", nil)
	case errors.Is(err, pointscontract.ErrReasonTooLong):
		ginutil.RespondError(c, response.CodeBadRequest, "error.points_reason_too_long", nil)
	case errors.Is(err, pointscontract.ErrReferenceRequired),
		errors.Is(err, pointscontract.ErrReferenceTooLong):
		ginutil.RespondError(c, response.CodeBadRequest, "error.points_idempotency_key_too_long", nil)
	case errors.Is(err, pointscontract.ErrAccountNotFound):
		ginutil.RespondError(c, response.CodeNotFound, "error.user_not_found", nil)
	case errors.Is(err, pointscontract.ErrNegativeNotAllowed):
		ginutil.RespondError(c, response.CodeBadRequest, "error.points_balance_insufficient", nil)
	default:
		ginutil.RespondError(c, response.CodeInternal, "error.points_adjust_failed", err)
	}
}
