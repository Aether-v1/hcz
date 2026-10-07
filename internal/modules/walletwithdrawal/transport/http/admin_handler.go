package withdrawalhttp

import (
	"errors"
	"strings"

	withdrawalcontract "github.com/Aether-v1/hcz/internal/modules/walletwithdrawal/contract"
	withdrawaldomain "github.com/Aether-v1/hcz/internal/modules/walletwithdrawal/domain"
	withdrawalpresenter "github.com/Aether-v1/hcz/internal/modules/walletwithdrawal/transport/presenter"
	"github.com/Aether-v1/hcz/internal/platform/http/ginutil"
	"github.com/Aether-v1/hcz/internal/platform/http/response"
	"github.com/Aether-v1/hcz/internal/platform/http/stepup"

	"github.com/gin-gonic/gin"
)

// AdminService 是后台提现管理所需的最小端口。
type AdminService interface {
	ListAdminWithdrawals(filter withdrawalcontract.AdminWithdrawalListFilter) ([]withdrawaldomain.Withdrawal, int64, error)
	GetAdminWithdrawalDetail(id uint) (*withdrawaldomain.Withdrawal, error)
	Approve(input withdrawalcontract.AdminReviewInput) (*withdrawaldomain.Withdrawal, error)
	Reject(input withdrawalcontract.AdminReviewInput) (*withdrawaldomain.Withdrawal, error)
	MarkProcessing(input withdrawalcontract.AdminProcessingInput) (*withdrawaldomain.Withdrawal, error)
	Complete(input withdrawalcontract.AdminCompleteInput) (*withdrawaldomain.Withdrawal, error)
}

// AdminHandler 处理后台提现 HTTP 请求。
type AdminHandler struct {
	svc       AdminService
	challenge stepup.Verifier
}

// NewAdminHandler 创建后台提现 Handler。challenge 为高风险动作（Approve/Complete）
// Step-Up 校验器，可为 nil（fail-closed）。
func NewAdminHandler(svc AdminService, challenge stepup.Verifier) *AdminHandler {
	if svc == nil {
		panic("withdrawal admin handler: service is nil")
	}
	return &AdminHandler{svc: svc, challenge: challenge}
}

// List 后台提现单列表。
func (h *AdminHandler) List(c *gin.Context) {
	page, pageSize := ginutil.ParsePagination(c)
	userID, err := ginutil.ParseQueryUint(c.Query("user_id"), false)
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	createdFrom, createdTo, err := ginutil.ParseQueryTimeRange(c, "created_from", "created_to")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	rows, total, err := h.svc.ListAdminWithdrawals(withdrawalcontract.AdminWithdrawalListFilter{
		Page:         page,
		PageSize:     pageSize,
		Status:       strings.TrimSpace(c.Query("status")),
		UserID:       userID,
		WithdrawalNo: strings.TrimSpace(c.Query("withdrawal_no")),
		CreatedFrom:  createdFrom,
		CreatedTo:    createdTo,
	})
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.internal", err)
		return
	}
	response.SuccessWithPage(c, withdrawalpresenter.NewWithdrawalRespList(rows), response.BuildPagination(page, pageSize, total))
}

// Detail 后台提现单详情。
func (h *AdminHandler) Detail(c *gin.Context) {
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	w, err := h.svc.GetAdminWithdrawalDetail(id)
	if err != nil {
		respondAdminError(c, err)
		return
	}
	response.Success(c, withdrawalpresenter.NewWithdrawalResp(w))
}

type adminNoteRequest struct {
	AdminNote string `json:"admin_note"`
}

// Approve 审批通过。
func (h *AdminHandler) Approve(c *gin.Context) {
	adminID, ok := ginutil.GetAdminID(c)
	if !ok {
		return
	}
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	if _, err := stepup.RequireFor(c, h.challenge, stepup.Scope("withdraw.approve", "id", id)); err != nil {
		stepup.RespondError(c, err)
		return
	}
	var req adminNoteRequest
	_ = c.ShouldBindJSON(&req)
	w, err := h.svc.Approve(withdrawalcontract.AdminReviewInput{
		ID:        id,
		AdminID:   adminID,
		AdminNote: strings.TrimSpace(req.AdminNote),
	})
	if err != nil {
		respondAdminError(c, err)
		return
	}
	response.Success(c, withdrawalpresenter.NewWithdrawalResp(w))
}

type rejectRequest struct {
	RejectReason string `json:"reject_reason" binding:"required"`
	AdminNote    string `json:"admin_note"`
}

// Reject 拒绝。
func (h *AdminHandler) Reject(c *gin.Context) {
	adminID, ok := ginutil.GetAdminID(c)
	if !ok {
		return
	}
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	var req rejectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	w, err := h.svc.Reject(withdrawalcontract.AdminReviewInput{
		ID:           id,
		AdminID:      adminID,
		RejectReason: strings.TrimSpace(req.RejectReason),
		AdminNote:    strings.TrimSpace(req.AdminNote),
	})
	if err != nil {
		respondAdminError(c, err)
		return
	}
	response.Success(c, withdrawalpresenter.NewWithdrawalResp(w))
}

// Processing 标记处理中。
func (h *AdminHandler) Processing(c *gin.Context) {
	adminID, ok := ginutil.GetAdminID(c)
	if !ok {
		return
	}
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	w, err := h.svc.MarkProcessing(withdrawalcontract.AdminProcessingInput{
		ID:      id,
		AdminID: adminID,
	})
	if err != nil {
		respondAdminError(c, err)
		return
	}
	response.Success(c, withdrawalpresenter.NewWithdrawalResp(w))
}

type completeRequest struct {
	Txid      string `json:"txid" binding:"required"`
	AdminNote string `json:"admin_note"`
}

// Complete 打款完成。
func (h *AdminHandler) Complete(c *gin.Context) {
	// 最高危动作（真实打款）：强制 Step-Up，绑定 scope 且单次使用。
	adminID, ok := ginutil.GetAdminID(c)
	if !ok {
		return
	}
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	if _, err := stepup.RequireFor(c, h.challenge, stepup.Scope("withdraw.complete", "id", id)); err != nil {
		stepup.RespondError(c, err)
		return
	}
	var req completeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	// 消费前端传入的 Idempotency-Key，配合状态机防止重复打款。
	idempotencyKey := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	w, err := h.svc.Complete(withdrawalcontract.AdminCompleteInput{
		ID:             id,
		AdminID:        adminID,
		Txid:           strings.TrimSpace(req.Txid),
		AdminNote:      strings.TrimSpace(req.AdminNote),
		IdempotencyKey: idempotencyKey,
	})
	if err != nil {
		respondAdminError(c, err)
		return
	}
	response.Success(c, withdrawalpresenter.NewWithdrawalResp(w))
}

func respondAdminError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, withdrawalcontract.ErrWithdrawalNotFound):
		ginutil.RespondError(c, response.CodeNotFound, "error.withdrawal_not_found", nil)
	case errors.Is(err, withdrawalcontract.ErrWithdrawalStatusInvalid):
		ginutil.RespondError(c, response.CodeBadRequest, "error.withdrawal_status_invalid", nil)
	case errors.Is(err, withdrawalcontract.ErrTxidRequired):
		ginutil.RespondError(c, response.CodeBadRequest, "error.withdrawal_txid_required", nil)
	case errors.Is(err, withdrawalcontract.ErrRejectReasonRequired):
		ginutil.RespondError(c, response.CodeBadRequest, "error.withdrawal_reject_reason_required", nil)
	default:
		ginutil.RespondError(c, response.CodeInternal, "error.internal", err)
	}
}
