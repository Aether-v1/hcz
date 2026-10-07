package pointsphttp

import (
	"strings"
	"time"

	pointscontract "github.com/Aether-v1/hcz/internal/modules/points/contract"

	"github.com/Aether-v1/hcz/internal/platform/http/ginutil"
	"github.com/Aether-v1/hcz/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

// ListAccounts 后台分页查询积分账户（P4 负余额排查）。
//
//	query: user_id / negative_only=true / page / page_size
//	负余额原样返回，不做钳制——它是退款冲正或 Admin 扣减后的真实结果。
func (h *AdminHandler) ListAccounts(c *gin.Context) {
	if _, ok := ginutil.GetAdminID(c); !ok {
		return
	}
	page, pageSize := ginutil.ParsePagination(c)

	userID, err := ginutil.ParseQueryUint(strings.TrimSpace(c.Query("user_id")), true)
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.user_id_invalid", err)
		return
	}
	negativeOnly, err := ginutil.ParseQueryBool(c, "negative_only")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}

	accounts, total, err := h.points.ListAccounts(pointscontract.AccountListFilter{
		UserID:       userID,
		NegativeOnly: negativeOnly,
		Page:         page,
		PageSize:     pageSize,
	})
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.points_account_fetch_failed", err)
		return
	}
	response.SuccessWithPage(c, accounts, response.BuildPagination(page, pageSize, total))
}

// GetStats 后台积分运营统计（P4）。
// 指标全部直接聚合自 points_ledger / user_checkins / points_exchange_orders，
// 时间口径是签到域的业务日（Asia/Shanghai），不存在第二套累计总表。
func (h *AdminHandler) GetStats(c *gin.Context) {
	if _, ok := ginutil.GetAdminID(c); !ok {
		return
	}
	stats, err := h.points.GetStats(time.Now())
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.points_stats_fetch_failed", err)
		return
	}
	response.Success(c, stats)
}

// ledgerFilterFromRequest 解析流水过滤条件（白名单枚举 + RFC3339 时间区间 + 分页）。
// 排序字段不接受客户端输入：固定 created_at DESC, id DESC（在 store 层写死）。
// 校验失败时响应已写出，返回 ok=false。
func ledgerFilterFromRequest(c *gin.Context) (pointscontract.LedgerListFilter, bool) {
	page, pageSize := ginutil.ParsePagination(c)
	filter := pointscontract.LedgerListFilter{Page: page, PageSize: pageSize}

	filter.ActionType = strings.TrimSpace(c.Query("action_type"))
	if filter.ActionType != "" && !pointscontract.IsValidActionType(filter.ActionType) {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return filter, false
	}
	filter.SourceType = strings.TrimSpace(c.Query("source_type"))
	if filter.SourceType != "" && !pointscontract.IsValidSourceType(filter.SourceType) {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return filter, false
	}
	filter.Direction = strings.TrimSpace(c.Query("direction"))
	if !pointscontract.IsValidLedgerDirection(filter.Direction) {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return filter, false
	}
	filter.Reference = strings.TrimSpace(c.Query("reference"))
	if len([]rune(filter.Reference)) > pointscontract.MaxReferenceLength {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return filter, false
	}

	createdFrom, createdTo, err := ginutil.ParseQueryTimeRange(c, "created_from", "created_to")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return filter, false
	}
	if createdFrom != nil {
		filter.CreatedFrom = *createdFrom
	}
	if createdTo != nil {
		filter.CreatedTo = *createdTo
	}
	return filter, true
}
