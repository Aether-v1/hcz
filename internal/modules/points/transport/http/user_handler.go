package pointsphttp

import (
	pointscontract "github.com/Aether-v1/hcz/internal/modules/points/contract"
	pointsdomain "github.com/Aether-v1/hcz/internal/modules/points/domain"

	"github.com/Aether-v1/hcz/internal/platform/http/ginutil"
	"github.com/Aether-v1/hcz/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

// UserPointsService 是用户端积分查询所需的最小端口。
// 过滤结构体与后台一致：用户端只允许用 JWT 中的 user_id 限定范围，不接受客户端传入的用户维度。
type UserPointsService interface {
	GetAccount(userID uint) (*pointsdomain.Account, error)
	ListLedgerEntries(filter pointscontract.LedgerListFilter) ([]pointsdomain.LedgerEntry, int64, error)
}

// UserHandler 处理用户端积分 HTTP 请求。
type UserHandler struct {
	points UserPointsService
}

func NewUserHandler(points UserPointsService) *UserHandler {
	if points == nil {
		panic("points user handler: required dependency is nil")
	}
	return &UserHandler{points: points}
}

// accountView 是积分账户的对外视图。
// 用户从未产生积分时返回零值账户（不 404、不创建账户）。
type accountView struct {
	Balance     int64 `json:"balance"`
	TotalEarned int64 `json:"total_earned"`
	TotalSpent  int64 `json:"total_spent"`
}

func normalizeAccountView(account *pointsdomain.Account) accountView {
	if account == nil {
		return accountView{}
	}
	return accountView{Balance: account.Balance, TotalEarned: account.TotalEarned, TotalSpent: account.TotalSpent}
}

// GetAccount 返回当前登录用户的积分账户。
// user_id 仅取自 JWT 上下文，不接受客户端传入，杜绝 IDOR。
func (h *UserHandler) GetAccount(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	account, err := h.points.GetAccount(uid)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.points_account_fetch_failed", err)
		return
	}
	response.Success(c, normalizeAccountView(account))
}

// ListLedgerEntries 返回当前登录用户的积分流水（分页）。
// user_id 仅取自 JWT 上下文，不接受客户端传入，杜绝 IDOR。
func (h *UserHandler) ListLedgerEntries(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	page, pageSize := ginutil.ParsePagination(c)
	entries, total, err := h.points.ListLedgerEntries(pointscontract.LedgerListFilter{
		UserID:   uid,
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.points_ledger_fetch_failed", err)
		return
	}
	pagination := response.BuildPagination(page, pageSize, total)
	response.SuccessWithPage(c, entries, pagination)
}
