package sitebuilderhttp

import (
	"strconv"

	"github.com/Aether-v1/hcz/internal/cache"
	sitebuilderapp "github.com/Aether-v1/hcz/internal/modules/sitebuilder/application"
	"github.com/Aether-v1/hcz/internal/modules/sitebuilder/infrastructure/gormstore"
	ginutil "github.com/Aether-v1/hcz/internal/platform/http/ginutil"
	"github.com/Aether-v1/hcz/internal/platform/http/response"
	"github.com/Aether-v1/hcz/internal/shared/jsonmap"

	"github.com/gin-gonic/gin"
)

// Services sitebuilder admin handler 依赖的业务服务集合。
type Services struct {
	HomeEntries *sitebuilderapp.HomeEntryService
	Discovery   *sitebuilderapp.DiscoveryBlockService
	Brand       *sitebuilderapp.BrandService
	Template    *sitebuilderapp.TemplateService
	Audit       *sitebuilderapp.AuditService
}

// AdminHandler 处理站点装修后台管理请求。
type AdminHandler struct {
	svc Services
}

// NewAdminHandler 创建 AdminHandler。
func NewAdminHandler(svc Services) *AdminHandler {
	return &AdminHandler{svc: svc}
}

func (h *AdminHandler) invalidate(c *gin.Context) {
	_ = cache.DelAllPublicConfig(c.Request.Context())
}

// ==================== Home Entries ====================

func (h *AdminHandler) ListHomeEntries(c *gin.Context) {
	entries, err := h.svc.HomeEntries.ListAdmin()
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.sitebuilder_fetch_failed", err)
		return
	}
	response.Success(c, entries)
}

func (h *AdminHandler) GetHomeEntry(c *gin.Context) {
	id, ok := pathUint(c)
	if !ok {
		return
	}
	entry, err := h.svc.HomeEntries.Get(id)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.sitebuilder_fetch_failed", err)
		return
	}
	if entry == nil {
		ginutil.RespondErrorWithMsg(c, response.CodeNotFound, "home entry not found", nil)
		return
	}
	response.Success(c, entry)
}

type homeEntryRequest struct {
	Key          string `json:"key"`
	Title        string `json:"title" binding:"required"`
	Subtitle     string `json:"subtitle"`
	Icon         string `json:"icon"`
	ActionType   string `json:"action_type" binding:"required"`
	ActionTarget string `json:"action_target" binding:"required"`
	Badge        string `json:"badge"`
	Recommended  *bool  `json:"recommended"`
	Enabled      *bool  `json:"enabled"`
	SortOrder    int    `json:"sort_order"`
}

func (h *AdminHandler) CreateHomeEntry(c *gin.Context) {
	var req homeEntryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	entry, err := h.svc.HomeEntries.Create(sitebuilderapp.HomeEntryInput{
		Key: req.Key, Title: req.Title, Subtitle: req.Subtitle, Icon: req.Icon,
		ActionType: req.ActionType, ActionTarget: req.ActionTarget, Badge: req.Badge,
		Recommended: req.Recommended, Enabled: req.Enabled, SortOrder: req.SortOrder,
	})
	if err != nil {
		ginutil.RespondErrorWithMsg(c, response.CodeBadRequest, err.Error(), err)
		return
	}
	adminID, _ := ginutil.GetAdminID(c)
	h.svc.Audit.Record(adminID, "home_entries", "create", nil, entry)
	h.invalidate(c)
	response.Success(c, entry)
}

func (h *AdminHandler) UpdateHomeEntry(c *gin.Context) {
	id, ok := pathUint(c)
	if !ok {
		return
	}
	var req homeEntryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	before, _ := h.svc.HomeEntries.Get(id)
	entry, err := h.svc.HomeEntries.Update(id, sitebuilderapp.HomeEntryInput{
		Title: req.Title, Subtitle: req.Subtitle, Icon: req.Icon,
		ActionType: req.ActionType, ActionTarget: req.ActionTarget, Badge: req.Badge,
		Recommended: req.Recommended, Enabled: req.Enabled, SortOrder: req.SortOrder,
	})
	if err != nil {
		ginutil.RespondErrorWithMsg(c, response.CodeBadRequest, err.Error(), err)
		return
	}
	adminID, _ := ginutil.GetAdminID(c)
	h.svc.Audit.Record(adminID, "home_entries", "update", before, entry)
	h.invalidate(c)
	response.Success(c, entry)
}

func (h *AdminHandler) DeleteHomeEntry(c *gin.Context) {
	id, ok := pathUint(c)
	if !ok {
		return
	}
	before, _ := h.svc.HomeEntries.Get(id)
	if err := h.svc.HomeEntries.Delete(id); err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.sitebuilder_save_failed", err)
		return
	}
	adminID, _ := ginutil.GetAdminID(c)
	h.svc.Audit.Record(adminID, "home_entries", "delete", before, nil)
	h.invalidate(c)
	response.Success(c, gin.H{"id": id})
}

func (h *AdminHandler) ToggleHomeEntry(c *gin.Context) {
	id, ok := pathUint(c)
	if !ok {
		return
	}
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	before, _ := h.svc.HomeEntries.Get(id)
	entry, err := h.svc.HomeEntries.SetEnabled(id, req.Enabled)
	if err != nil {
		ginutil.RespondErrorWithMsg(c, response.CodeBadRequest, err.Error(), err)
		return
	}
	adminID, _ := ginutil.GetAdminID(c)
	h.svc.Audit.Record(adminID, "home_entries", "toggle", before, entry)
	h.invalidate(c)
	response.Success(c, entry)
}

type reorderRequest struct {
	Items []struct {
		ID        uint `json:"id" binding:"required"`
		SortOrder int  `json:"sort_order"`
	} `json:"items" binding:"required"`
}

func (h *AdminHandler) ReorderHomeEntries(c *gin.Context) {
	var req reorderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	items := make([]gormstore.ReorderItem, 0, len(req.Items))
	for i := range req.Items {
		items = append(items, gormstore.ReorderItem{ID: req.Items[i].ID, SortOrder: req.Items[i].SortOrder})
	}
	if err := h.svc.HomeEntries.Reorder(items); err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.sitebuilder_save_failed", err)
		return
	}
	adminID, _ := ginutil.GetAdminID(c)
	h.svc.Audit.Record(adminID, "home_entries", "reorder", nil, items)
	h.invalidate(c)
	response.Success(c, gin.H{"reordered": len(items)})
}

// ==================== Discovery Blocks ====================

func (h *AdminHandler) ListDiscoveryBlocks(c *gin.Context) {
	blocks, err := h.svc.Discovery.ListAdmin()
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.sitebuilder_fetch_failed", err)
		return
	}
	response.Success(c, blocks)
}

func (h *AdminHandler) GetDiscoveryBlock(c *gin.Context) {
	id, ok := pathUint(c)
	if !ok {
		return
	}
	block, err := h.svc.Discovery.Get(id)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.sitebuilder_fetch_failed", err)
		return
	}
	if block == nil {
		ginutil.RespondErrorWithMsg(c, response.CodeNotFound, "discovery block not found", nil)
		return
	}
	response.Success(c, block)
}

type discoveryBlockRequest struct {
	Type      string       `json:"type" binding:"required"`
	Title     string       `json:"title"`
	Config    jsonmap.JSON `json:"config"`
	Enabled   *bool        `json:"enabled"`
	SortOrder int          `json:"sort_order"`
}

func (h *AdminHandler) CreateDiscoveryBlock(c *gin.Context) {
	var req discoveryBlockRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	block, err := h.svc.Discovery.Create(sitebuilderapp.DiscoveryBlockInput{
		Type: req.Type, Title: req.Title, Config: req.Config, Enabled: req.Enabled, SortOrder: req.SortOrder,
	})
	if err != nil {
		ginutil.RespondErrorWithMsg(c, response.CodeBadRequest, err.Error(), err)
		return
	}
	adminID, _ := ginutil.GetAdminID(c)
	h.svc.Audit.Record(adminID, "discovery", "create", nil, block)
	h.invalidate(c)
	response.Success(c, block)
}

func (h *AdminHandler) UpdateDiscoveryBlock(c *gin.Context) {
	id, ok := pathUint(c)
	if !ok {
		return
	}
	var req discoveryBlockRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	before, _ := h.svc.Discovery.Get(id)
	block, err := h.svc.Discovery.Update(id, sitebuilderapp.DiscoveryBlockInput{
		Type: req.Type, Title: req.Title, Config: req.Config, Enabled: req.Enabled, SortOrder: req.SortOrder,
	})
	if err != nil {
		ginutil.RespondErrorWithMsg(c, response.CodeBadRequest, err.Error(), err)
		return
	}
	adminID, _ := ginutil.GetAdminID(c)
	h.svc.Audit.Record(adminID, "discovery", "update", before, block)
	h.invalidate(c)
	response.Success(c, block)
}

func (h *AdminHandler) DeleteDiscoveryBlock(c *gin.Context) {
	id, ok := pathUint(c)
	if !ok {
		return
	}
	before, _ := h.svc.Discovery.Get(id)
	if err := h.svc.Discovery.Delete(id); err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.sitebuilder_save_failed", err)
		return
	}
	adminID, _ := ginutil.GetAdminID(c)
	h.svc.Audit.Record(adminID, "discovery", "delete", before, nil)
	h.invalidate(c)
	response.Success(c, gin.H{"id": id})
}

func (h *AdminHandler) ToggleDiscoveryBlock(c *gin.Context) {
	id, ok := pathUint(c)
	if !ok {
		return
	}
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	before, _ := h.svc.Discovery.Get(id)
	block, err := h.svc.Discovery.SetEnabled(id, req.Enabled)
	if err != nil {
		ginutil.RespondErrorWithMsg(c, response.CodeBadRequest, err.Error(), err)
		return
	}
	adminID, _ := ginutil.GetAdminID(c)
	h.svc.Audit.Record(adminID, "discovery", "toggle", before, block)
	h.invalidate(c)
	response.Success(c, block)
}

func (h *AdminHandler) ReorderDiscoveryBlocks(c *gin.Context) {
	var req reorderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	items := make([]gormstore.ReorderItem, 0, len(req.Items))
	for i := range req.Items {
		items = append(items, gormstore.ReorderItem{ID: req.Items[i].ID, SortOrder: req.Items[i].SortOrder})
	}
	if err := h.svc.Discovery.Reorder(items); err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.sitebuilder_save_failed", err)
		return
	}
	adminID, _ := ginutil.GetAdminID(c)
	h.svc.Audit.Record(adminID, "discovery", "reorder", nil, items)
	h.invalidate(c)
	response.Success(c, gin.H{"reordered": len(items)})
}

// ==================== Brand / Template ====================

func (h *AdminHandler) GetBrand(c *gin.Context) {
	brand, err := h.svc.Brand.Get()
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.sitebuilder_fetch_failed", err)
		return
	}
	response.Success(c, brand)
}

type brandRequest struct {
	PrimaryColor string `json:"primary_color"`
	Copyright    string `json:"copyright"`
}

func (h *AdminHandler) UpdateBrand(c *gin.Context) {
	var req brandRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	before, _ := h.svc.Brand.Get()
	brand, err := h.svc.Brand.Update(req.PrimaryColor, req.Copyright)
	if err != nil {
		ginutil.RespondErrorWithMsg(c, response.CodeBadRequest, err.Error(), err)
		return
	}
	adminID, _ := ginutil.GetAdminID(c)
	h.svc.Audit.Record(adminID, "brand", "update", before, brand)
	h.invalidate(c)
	response.Success(c, brand)
}

func (h *AdminHandler) UpdateTemplate(c *gin.Context) {
	var req struct {
		Template string `json:"template" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	before, _ := h.svc.Template.Get()
	tpl, err := h.svc.Template.Switch(req.Template)
	if err != nil {
		ginutil.RespondErrorWithMsg(c, response.CodeBadRequest, err.Error(), err)
		return
	}
	adminID, _ := ginutil.GetAdminID(c)
	h.svc.Audit.Record(adminID, "template", "switch", before, tpl)
	h.invalidate(c)
	response.Success(c, gin.H{"template": tpl})
}

// ==================== Audit Logs ====================

func (h *AdminHandler) ListAuditLogs(c *gin.Context) {
	limit := 50
	if v := c.Query("limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 && parsed <= 200 {
			limit = parsed
		}
	}
	offset := 0
	if v := c.Query("offset"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed >= 0 {
			offset = parsed
		}
	}
	logs, err := h.svc.Audit.List(limit, offset)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.sitebuilder_fetch_failed", err)
		return
	}
	response.Success(c, logs)
}

// pathUint 解析路径中的 :id 为 uint。
func pathUint(c *gin.Context) (uint, bool) {
	raw := c.Param("id")
	parsed, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || parsed == 0 {
		ginutil.RespondErrorWithMsg(c, response.CodeBadRequest, "invalid id", err)
		return 0, false
	}
	return uint(parsed), true
}
