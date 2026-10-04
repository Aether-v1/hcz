package withdrawalhttp

import (
	"strings"

	withdrawalcontract "github.com/Aether-v1/hcz/internal/modules/walletwithdrawal/contract"
	withdrawalpresenter "github.com/Aether-v1/hcz/internal/modules/walletwithdrawal/transport/presenter"
	"github.com/Aether-v1/hcz/internal/platform/http/ginutil"
	"github.com/Aether-v1/hcz/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

// ListAddresses 列出用户提现地址。
func (h *UserHandler) ListAddresses(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	rows, err := h.svc.ListAddresses(uid)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.internal", err)
		return
	}
	response.Success(c, withdrawalpresenter.NewAddressRespList(rows))
}

type createAddressRequest struct {
	Network string `json:"network" binding:"required"`
	Address string `json:"address" binding:"required"`
	Label   string `json:"label"`
}

// CreateAddress 新增提现地址。
func (h *UserHandler) CreateAddress(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	var req createAddressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	a, err := h.svc.CreateAddress(withdrawalcontract.CreateAddressInput{
		UserID:  uid,
		Network: strings.TrimSpace(req.Network),
		Address: strings.TrimSpace(req.Address),
		Label:   strings.TrimSpace(req.Label),
	})
	if err != nil {
		respondUserError(c, err)
		return
	}
	response.Success(c, withdrawalpresenter.NewAddressResp(a))
}

// DeleteAddress 删除提现地址。
func (h *UserHandler) DeleteAddress(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	if err := h.svc.DeleteAddress(uid, id); err != nil {
		respondUserError(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

// SetDefaultAddress 设置默认地址。
func (h *UserHandler) SetDefaultAddress(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	a, err := h.svc.SetDefaultAddress(uid, id)
	if err != nil {
		respondUserError(c, err)
		return
	}
	response.Success(c, withdrawalpresenter.NewAddressResp(a))
}
