package v1

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sailboxhq/sailbox/apps/api/internal/api/middleware"
	"github.com/sailboxhq/sailbox/apps/api/internal/apierr"
	"github.com/sailboxhq/sailbox/apps/api/internal/httputil"
	"github.com/sailboxhq/sailbox/apps/api/internal/model"
	"github.com/sailboxhq/sailbox/apps/api/internal/service"
)

type ResourceHandler struct {
	svc *service.ResourceService
}

func NewResourceHandler(svc *service.ResourceService) *ResourceHandler {
	return &ResourceHandler{svc: svc}
}

func (h *ResourceHandler) List(c *gin.Context) {
	orgID := middleware.GetOrgID(c)
	resourceType := c.Query("type")
	resources, err := h.svc.List(c.Request.Context(), orgID, resourceType)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondList(c, redactResources(resources))
}

func (h *ResourceHandler) Create(c *gin.Context) {
	orgID := middleware.GetOrgID(c)
	var input service.CreateResourceInput
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}
	resource, err := h.svc.Create(c.Request.Context(), orgID, input)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondCreated(c, resource.Redacted(), "")
}

func (h *ResourceHandler) Update(c *gin.Context) {
	orgID := middleware.GetOrgID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid resource ID"))
		return
	}
	var input service.UpdateResourceInput
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}
	resource, err := h.svc.Update(c.Request.Context(), orgID, id, input)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, resource.Redacted())
}

func (h *ResourceHandler) Delete(c *gin.Context) {
	orgID := middleware.GetOrgID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid resource ID"))
		return
	}
	if err := h.svc.Delete(c.Request.Context(), orgID, id); err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondNoContent(c)
}

func (h *ResourceHandler) TestConnection(c *gin.Context) {
	orgID := middleware.GetOrgID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid resource ID"))
		return
	}
	ok, msg, err := h.svc.TestConnection(c.Request.Context(), orgID, id)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, gin.H{"success": ok, "message": msg})
}

func (h *ResourceHandler) ListRepos(c *gin.Context) {
	orgID := middleware.GetOrgID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid resource ID"))
		return
	}
	repos, err := h.svc.ListRepos(c.Request.Context(), orgID, id)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	if repos == nil {
		repos = []service.GitRepo{}
	}
	httputil.RespondOK(c, repos)
}

func (h *ResourceHandler) GenerateSSHKey(c *gin.Context) {
	orgID := middleware.GetOrgID(c)
	var input struct {
		Algorithm string `json:"algorithm"` // ed25519 | rsa-4096
		Name      string `json:"name"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}
	if input.Algorithm == "" {
		input.Algorithm = "ed25519"
	}
	resource, err := h.svc.GenerateSSHKey(c.Request.Context(), orgID, input.Algorithm, input.Name)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	// The private key stays server-side; the caller gets the public half, which
	// is what has to be registered with the git host.
	httputil.RespondCreated(c, resource.Redacted(), "")
}

// redactResources strips credentials from a list of resources before it leaves
// the API. Internal callers keep using the service/store directly, which still
// return the real values.
func redactResources(resources []model.SharedResource) []model.SharedResource {
	out := make([]model.SharedResource, 0, len(resources))
	for i := range resources {
		out = append(out, *resources[i].Redacted())
	}
	return out
}
