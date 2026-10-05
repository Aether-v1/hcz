// Package http 实现工单系统薄层 Handler：只做参数解析、鉴权、调用 service、返回响应。
package http

import (
	"errors"
	"net/http"
	"strings"

	supportapp "github.com/Aether-v1/hcz/internal/modules/supportticket/application"
	supportcontract "github.com/Aether-v1/hcz/internal/modules/supportticket/contract"
	"github.com/Aether-v1/hcz/internal/platform/http/ginutil"
	"github.com/Aether-v1/hcz/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

// UserHandler 用户侧工单 Handler。
type UserHandler struct {
	svc *supportapp.Service
}

// NewUserHandler 创建用户侧 Handler。
func NewUserHandler(svc *supportapp.Service) *UserHandler {
	if svc == nil {
		panic("supportticket user handler: service is nil")
	}
	return &UserHandler{svc: svc}
}

// respondUserError 把领域错误映射为 HTTP 响应。
func respondUserError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, supportcontract.ErrTicketNotFound),
		errors.Is(err, supportcontract.ErrAttachmentNotFound):
		ginutil.RespondError(c, response.CodeNotFound, "error.ticket_not_found", nil)
	case errors.Is(err, supportcontract.ErrNotOwner):
		ginutil.RespondError(c, response.CodeNotFound, "error.ticket_not_found", nil)
	case errors.Is(err, supportcontract.ErrTicketClosed):
		ginutil.RespondError(c, response.CodeBadRequest, "error.ticket_closed", nil)
	case errors.Is(err, supportcontract.ErrReopenExpired):
		ginutil.RespondError(c, response.CodeBadRequest, "error.ticket_reopen_expired", nil)
	case errors.Is(err, supportcontract.ErrCategoryDisabled):
		ginutil.RespondError(c, response.CodeBadRequest, "error.ticket_category_disabled", nil)
	case errors.Is(err, supportcontract.ErrCategoryNotFound):
		ginutil.RespondError(c, response.CodeBadRequest, "error.ticket_category_disabled", nil)
	case errors.Is(err, supportcontract.ErrInvalidBizOwnership):
		ginutil.RespondError(c, response.CodeBadRequest, "error.ticket_invalid_biz_ownership", nil)
	case errors.Is(err, supportcontract.ErrInvalidSubject),
		errors.Is(err, supportcontract.ErrInvalidBody):
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
	case errors.Is(err, supportcontract.ErrTicketStatusInvalid):
		ginutil.RespondError(c, response.CodeBadRequest, "error.ticket_status_invalid", nil)
	default:
		ginutil.RespondError(c, response.CodeInternal, "error.internal", err)
	}
}

// ListCategories GET /categories
func (h *UserHandler) ListCategories(c *gin.Context) {
	rows, err := h.svc.ListEnabledCategories()
	if err != nil {
		respondUserError(c, err)
		return
	}
	items := make([]gin.H, 0, len(rows))
	for i := range rows {
		items = append(items, gin.H{
			"id":               rows[i].ID,
			"code":             rows[i].Code,
			"name":             rows[i].Name,
			"default_priority": rows[i].DefaultPriority,
		})
	}
	response.Success(c, gin.H{"items": items})
}

// ListTickets GET /tickets
func (h *UserHandler) ListTickets(c *gin.Context) {
	userID, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	page, pageSize := ginutil.ParsePagination(c)
	rows, total, err := h.svc.ListMyTicketsView(userID, supportcontract.TicketListFilter{
		Status:   strings.TrimSpace(c.Query("status")),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		respondUserError(c, err)
		return
	}
	response.SuccessWithPage(c, gin.H{"items": rows}, response.BuildPagination(page, pageSize, total))
}

// CreateTicket POST /tickets
func (h *UserHandler) CreateTicket(c *gin.Context) {
	userID, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	var req createTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	ticket, err := h.svc.CreateTicket(c.Request.Context(), supportcontract.CreateTicketInput{
		UserID:        userID,
		CategoryID:    req.CategoryID,
		Subject:       req.Subject,
		Body:          req.Body,
		BizType:       req.BizType,
		BizID:         req.BizID,
		AttachmentIDs: req.AttachmentIDs,
	})
	if err != nil {
		respondUserError(c, err)
		return
	}
	response.Success(c, gin.H{"id": ticket.ID, "ticket_no": ticket.TicketNo})
}

// GetTicket GET /tickets/:id
func (h *UserHandler) GetTicket(c *gin.Context) {
	userID, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	detail, err := h.svc.GetMyTicketView(c.Request.Context(), userID, id)
	if err != nil {
		respondUserError(c, err)
		return
	}
	response.Success(c, detail)
}

// ReplyTicket POST /tickets/:id/replies
func (h *UserHandler) ReplyTicket(c *gin.Context) {
	userID, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	var req replyTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	ticket, err := h.svc.ReplyToTicket(c.Request.Context(), supportcontract.ReplyTicketInput{
		UserID:        userID,
		TicketID:      id,
		Body:          req.Body,
		AttachmentIDs: req.AttachmentIDs,
	})
	if err != nil {
		respondUserError(c, err)
		return
	}
	response.Success(c, gin.H{"id": ticket.ID, "status": ticket.Status})
}

// CloseTicket POST /tickets/:id/close
func (h *UserHandler) CloseTicket(c *gin.Context) {
	userID, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	ticket, err := h.svc.CloseMyTicket(c.Request.Context(), userID, id)
	if err != nil {
		respondUserError(c, err)
		return
	}
	response.Success(c, gin.H{"id": ticket.ID, "status": ticket.Status})
}

// ReopenTicket POST /tickets/:id/reopen
func (h *UserHandler) ReopenTicket(c *gin.Context) {
	userID, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	ticket, err := h.svc.ReopenMyTicket(c.Request.Context(), userID, id)
	if err != nil {
		respondUserError(c, err)
		return
	}
	response.Success(c, gin.H{"id": ticket.ID, "status": ticket.Status})
}

// UploadAttachment POST /attachments
func (h *UserHandler) UploadAttachment(c *gin.Context) {
	userID, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	file, err := c.FormFile("file")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.file_missing", nil)
		return
	}
	att, err := h.svc.UploadAttachment(userID, file)
	if err != nil {
		respondUserError(c, err)
		return
	}
	response.Success(c, gin.H{
		"id":        att.ID,
		"file_name": att.FileName,
		"mime_type": att.MimeType,
		"file_size": att.FileSize,
	})
}

// DownloadAttachment GET /attachments/:id
func (h *UserHandler) DownloadAttachment(c *gin.Context) {
	userID, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	dl, err := h.svc.DownloadAttachment(userID, id)
	if err != nil {
		respondUserError(c, err)
		return
	}
	defer dl.Reader.Close()
	c.Header("Content-Disposition", "attachment; filename=\""+dl.FileName+"\"")
	c.DataFromReader(http.StatusOK, dl.Attachment.FileSize, dl.MimeType, dl.Reader, nil)
}
