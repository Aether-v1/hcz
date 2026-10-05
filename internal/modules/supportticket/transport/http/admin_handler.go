package http

import (
	"errors"
	"strings"

	supportapp "github.com/Aether-v1/hcz/internal/modules/supportticket/application"
	supportcontract "github.com/Aether-v1/hcz/internal/modules/supportticket/contract"
	"github.com/Aether-v1/hcz/internal/platform/http/ginutil"
	"github.com/Aether-v1/hcz/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

// AdminHandler 客服侧工单 Handler。
type AdminHandler struct {
	svc *supportapp.Service
}

// NewAdminHandler 创建客服侧 Handler。
func NewAdminHandler(svc *supportapp.Service) *AdminHandler {
	if svc == nil {
		panic("supportticket admin handler: service is nil")
	}
	return &AdminHandler{svc: svc}
}

func respondAdminError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, supportcontract.ErrTicketNotFound):
		ginutil.RespondError(c, response.CodeNotFound, "error.ticket_not_found", nil)
	case errors.Is(err, supportcontract.ErrTicketClosed):
		ginutil.RespondError(c, response.CodeBadRequest, "error.ticket_closed", nil)
	case errors.Is(err, supportcontract.ErrReopenExpired):
		ginutil.RespondError(c, response.CodeBadRequest, "error.ticket_reopen_expired", nil)
	case errors.Is(err, supportcontract.ErrAlreadyAssigned):
		ginutil.RespondError(c, response.CodeBadRequest, "error.ticket_already_assigned", nil)
	case errors.Is(err, supportcontract.ErrInvalidPriority):
		ginutil.RespondError(c, response.CodeBadRequest, "error.ticket_invalid_priority", nil)
	case errors.Is(err, supportcontract.ErrCategoryNotFound),
		errors.Is(err, supportcontract.ErrCategoryDisabled):
		ginutil.RespondError(c, response.CodeBadRequest, "error.ticket_category_disabled", nil)
	case errors.Is(err, supportcontract.ErrDuplicateCategoryCode):
		ginutil.RespondError(c, response.CodeBadRequest, "error.ticket_category_code_exists", nil)
	case errors.Is(err, supportcontract.ErrAdminNotFound):
		ginutil.RespondError(c, response.CodeBadRequest, "error.admin_not_found", nil)
	case errors.Is(err, supportcontract.ErrTicketStatusInvalid):
		ginutil.RespondError(c, response.CodeBadRequest, "error.ticket_status_invalid", nil)
	default:
		ginutil.RespondError(c, response.CodeInternal, "error.internal", err)
	}
}

// Overview GET /overview
func (h *AdminHandler) Overview(c *gin.Context) {
	adminID, ok := ginutil.GetAdminID(c)
	if !ok {
		return
	}
	stats, err := h.svc.Overview(adminID)
	if err != nil {
		respondAdminError(c, err)
		return
	}
	response.Success(c, stats)
}

// ListTickets GET /tickets
func (h *AdminHandler) ListTickets(c *gin.Context) {
	adminID, ok := ginutil.GetAdminID(c)
	if !ok {
		return
	}
	page, pageSize := ginutil.ParsePagination(c)
	filter := supportcontract.AdminTicketListFilter{
		Status:   strings.TrimSpace(c.Query("status")),
		Priority: strings.TrimSpace(c.Query("priority")),
		Search:   strings.TrimSpace(c.Query("search")),
		Page:     page,
		PageSize: pageSize,
	}
	if cid, _ := ginutil.ParseQueryUint(c.Query("category_id"), false); cid != 0 {
		filter.CategoryID = cid
	}
	// assigned_admin_id: 0=unassigned, "me"=current admin.
	if raw := strings.TrimSpace(c.Query("assigned_admin_id")); raw != "" {
		if raw == "me" {
			filter.MyAssignedOnly = adminID
		} else if v, err := ginutil.ParseQueryUint(raw, false); err == nil {
			if v == 0 {
				filter.UnassignedOnly = true
			} else {
				filter.MyAssignedOnly = v
			}
		}
	}
	if unread, _ := ginutil.ParseQueryBool(c, "unread_only"); unread {
		filter.UnreadOnly = true
	}
	rows, total, err := h.svc.ListAdminTicketsView(filter)
	if err != nil {
		respondAdminError(c, err)
		return
	}
	response.SuccessWithPage(c, gin.H{"items": rows}, response.BuildPagination(page, pageSize, total))
}

// GetTicket GET /tickets/:id
func (h *AdminHandler) GetTicket(c *gin.Context) {
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	detail, err := h.svc.GetAdminTicketView(id)
	if err != nil {
		respondAdminError(c, err)
		return
	}
	response.Success(c, detail)
}

// ReplyTicket POST /tickets/:id/replies
func (h *AdminHandler) ReplyTicket(c *gin.Context) {
	adminID, ok := ginutil.GetAdminID(c)
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
	ticket, err := h.svc.AdminReply(c.Request.Context(), supportcontract.AdminReplyInput{
		OperatorAdminID: adminID,
		TicketID:        id,
		Body:            req.Body,
		AttachmentIDs:   req.AttachmentIDs,
	})
	if err != nil {
		respondAdminError(c, err)
		return
	}
	response.Success(c, gin.H{"id": ticket.ID, "status": ticket.Status})
}

// AssignTicket POST /tickets/:id/assign
func (h *AdminHandler) AssignTicket(c *gin.Context) {
	adminID, ok := ginutil.GetAdminID(c)
	if !ok {
		return
	}
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	var req assignTicketRequest
	_ = c.ShouldBindJSON(&req)
	ticket, err := h.svc.AssignTicket(c.Request.Context(), supportcontract.AssignTicketInput{
		OperatorAdminID: adminID,
		TicketID:        id,
		AdminID:         req.AdminID,
	})
	if err != nil {
		respondAdminError(c, err)
		return
	}
	response.Success(c, gin.H{"id": ticket.ID, "assigned_admin_id": ticket.AssignedAdminID})
}

// ChangePriority POST /tickets/:id/change-priority
func (h *AdminHandler) ChangePriority(c *gin.Context) {
	adminID, ok := ginutil.GetAdminID(c)
	if !ok {
		return
	}
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	var req changePriorityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	ticket, err := h.svc.ChangePriority(c.Request.Context(), supportcontract.ChangePriorityInput{
		OperatorAdminID: adminID,
		TicketID:        id,
		Priority:        req.Priority,
	})
	if err != nil {
		respondAdminError(c, err)
		return
	}
	response.Success(c, gin.H{"id": ticket.ID, "priority": ticket.Priority})
}

// ResolveTicket POST /tickets/:id/resolve
func (h *AdminHandler) ResolveTicket(c *gin.Context) {
	adminID, ok := ginutil.GetAdminID(c)
	if !ok {
		return
	}
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	var req resolveTicketRequest
	_ = c.ShouldBindJSON(&req)
	ticket, err := h.svc.ResolveTicket(c.Request.Context(), supportcontract.ResolveTicketInput{
		OperatorAdminID: adminID,
		TicketID:        id,
		Reason:          req.Reason,
	})
	if err != nil {
		respondAdminError(c, err)
		return
	}
	response.Success(c, gin.H{"id": ticket.ID, "status": ticket.Status})
}

// CloseTicket POST /tickets/:id/close
func (h *AdminHandler) CloseTicket(c *gin.Context) {
	adminID, ok := ginutil.GetAdminID(c)
	if !ok {
		return
	}
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	var req closeTicketRequest
	_ = c.ShouldBindJSON(&req)
	ticket, err := h.svc.CloseTicket(c.Request.Context(), supportcontract.CloseTicketInput{
		OperatorAdminID: adminID,
		TicketID:        id,
		Reason:          req.Reason,
	})
	if err != nil {
		respondAdminError(c, err)
		return
	}
	response.Success(c, gin.H{"id": ticket.ID, "status": ticket.Status})
}

// ReopenTicket POST /tickets/:id/reopen
func (h *AdminHandler) ReopenTicket(c *gin.Context) {
	adminID, ok := ginutil.GetAdminID(c)
	if !ok {
		return
	}
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	var req reopenTicketRequest
	_ = c.ShouldBindJSON(&req)
	ticket, err := h.svc.ReopenTicket(c.Request.Context(), supportcontract.ReopenTicketInput{
		OperatorAdminID: adminID,
		TicketID:        id,
		Reason:          req.Reason,
	})
	if err != nil {
		respondAdminError(c, err)
		return
	}
	response.Success(c, gin.H{"id": ticket.ID, "status": ticket.Status})
}

// ListAudits GET /tickets/:id/audits
func (h *AdminHandler) ListAudits(c *gin.Context) {
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	audits, err := h.svc.ListAudits(id)
	if err != nil {
		respondAdminError(c, err)
		return
	}
	response.Success(c, gin.H{"items": audits})
}

// ==================== 分类管理 ====================

// ListCategories GET /categories
func (h *AdminHandler) ListCategories(c *gin.Context) {
	rows, err := h.svc.ListAllCategories()
	if err != nil {
		respondAdminError(c, err)
		return
	}
	response.Success(c, gin.H{"items": rows})
}

// CreateCategory POST /categories
func (h *AdminHandler) CreateCategory(c *gin.Context) {
	var req createCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	cat, err := h.svc.CreateCategory(supportcontract.CreateCategoryInput{
		Code:            req.Code,
		Name:            req.Name,
		Enabled:         req.Enabled,
		SortOrder:       req.SortOrder,
		DefaultPriority: req.DefaultPriority,
	})
	if err != nil {
		respondAdminError(c, err)
		return
	}
	response.Success(c, cat)
}

// UpdateCategory PUT /categories/:id
func (h *AdminHandler) UpdateCategory(c *gin.Context) {
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	var req updateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	cat, err := h.svc.UpdateCategory(id, supportcontract.UpdateCategoryInput{
		Name:            req.Name,
		Enabled:         req.Enabled,
		SortOrder:       req.SortOrder,
		DefaultPriority: req.DefaultPriority,
	})
	if err != nil {
		respondAdminError(c, err)
		return
	}
	response.Success(c, cat)
}

// DeleteCategory DELETE /categories/:id (软禁用)
func (h *AdminHandler) DeleteCategory(c *gin.Context) {
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	if err := h.svc.DisableCategory(id); err != nil {
		respondAdminError(c, err)
		return
	}
	response.Success(c, gin.H{"id": id, "enabled": false})
}
