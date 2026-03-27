package v1

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sailboxhq/sailbox/apps/api/internal/apierr"
	"github.com/sailboxhq/sailbox/apps/api/internal/httputil"
	"github.com/sailboxhq/sailbox/apps/api/internal/service"
)

type NodeHandler struct {
	svc *service.NodeService
}

func NewNodeHandler(svc *service.NodeService) *NodeHandler {
	return &NodeHandler{svc: svc}
}

func (h *NodeHandler) List(c *gin.Context) {
	nodes, err := h.svc.List(c.Request.Context())
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondList(c, nodes)
}

func (h *NodeHandler) Create(c *gin.Context) {
	var input service.CreateNodeInput
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}
	node, err := h.svc.Create(c.Request.Context(), input)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondCreated(c, node, "")
}

func (h *NodeHandler) Initialize(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid node ID"))
		return
	}
	if err := h.svc.Initialize(c.Request.Context(), id); err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondAccepted(c, gin.H{"message": "initialization started"})
}

func (h *NodeHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid node ID"))
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondNoContent(c)
}
