package v1

import (
	"context"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/sailboxhq/sailbox/apps/api/internal/apierr"
	"github.com/sailboxhq/sailbox/apps/api/internal/httputil"
	"github.com/sailboxhq/sailbox/apps/api/internal/orchestrator"
	"github.com/sailboxhq/sailbox/apps/api/internal/store"
)

type ClusterHandler struct {
	orch  orchestrator.Orchestrator
	store store.Store
}

func NewClusterHandler(orch orchestrator.Orchestrator, s store.Store) *ClusterHandler {
	return &ClusterHandler{orch: orch, store: s}
}

func (h *ClusterHandler) GetNodes(c *gin.Context) {
	nodes, err := h.orch.GetNodes(c.Request.Context())
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondList(c, nodes)
}

func (h *ClusterHandler) GetMetrics(c *gin.Context) {
	metrics, err := h.orch.GetClusterMetrics(c.Request.Context())
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, metrics)
}

func (h *ClusterHandler) GetAllPods(c *gin.Context) {
	pods, err := h.orch.GetAllPods(c.Request.Context())
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondList(c, pods)
}

func (h *ClusterHandler) GetEvents(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "100")
	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	events, err := h.orch.GetClusterEvents(c.Request.Context(), limit)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondList(c, events)
}

func (h *ClusterHandler) GetPVCs(c *gin.Context) {
	pvcs, err := h.orch.GetPVCs(c.Request.Context())
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondList(c, pvcs)
}

func (h *ClusterHandler) GetNamespaces(c *gin.Context) {
	namespaces, err := h.orch.GetNamespaces(c.Request.Context())
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondList(c, namespaces)
}

func (h *ClusterHandler) GetNodeMetrics(c *gin.Context) {
	metrics, err := h.orch.GetNodeMetrics(c.Request.Context())
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondList(c, metrics)
}

func (h *ClusterHandler) GetTopology(c *gin.Context) {
	topo, err := h.orch.GetClusterTopology(c.Request.Context())
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, topo)
}

func (h *ClusterHandler) GetNodePools(c *gin.Context) {
	pools, err := h.orch.GetNodePools(c.Request.Context())
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, pools)
}

func (h *ClusterHandler) SetNodePool(c *gin.Context) {
	nodeName := c.Param("name")
	var body struct {
		Pool string `json:"pool"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail(err.Error()))
		return
	}
	if body.Pool == "" {
		// Remove pool label
		if err := h.orch.RemoveNodeLabel(c.Request.Context(), nodeName, "sailbox/pool"); err != nil {
			httputil.RespondError(c, err)
			return
		}
	} else {
		if err := h.orch.SetNodeLabel(c.Request.Context(), nodeName, "sailbox/pool", body.Pool); err != nil {
			httputil.RespondError(c, err)
			return
		}
	}
	httputil.RespondOK(c, gin.H{"message": "node pool updated"})
}

func (h *ClusterHandler) GetTraefikConfig(c *gin.Context) {
	yaml, err := h.orch.GetTraefikConfig(c.Request.Context())
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, gin.H{"yaml": yaml})
}

func (h *ClusterHandler) UpdateTraefikConfig(c *gin.Context) {
	var body struct {
		YAML string `json:"yaml" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail(err.Error()))
		return
	}
	if err := h.orch.UpdateTraefikConfig(c.Request.Context(), body.YAML); err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, gin.H{"message": "traefik config updated"})
}

func (h *ClusterHandler) GetHelmReleases(c *gin.Context) {
	releases, err := h.orch.GetHelmReleases(c.Request.Context())
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondList(c, releases)
}

func (h *ClusterHandler) GetDaemonSets(c *gin.Context) {
	daemonSets, err := h.orch.GetDaemonSets(c.Request.Context())
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondList(c, daemonSets)
}

func (h *ClusterHandler) DeletePVC(c *gin.Context) {
	namespace := c.Param("namespace")
	name := c.Param("name")
	if namespace == "" || name == "" {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("namespace and name are required"))
		return
	}
	if err := h.orch.DeleteVolume(c.Request.Context(), name, namespace); err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, gin.H{"message": "PVC deleted"})
}

func (h *ClusterHandler) GetCleanupStats(c *gin.Context) {
	ctx := c.Request.Context()
	stats, err := h.orch.GetCleanupStats(ctx)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}

	// Enrich with orphan ingress detection (requires DB access)
	validHosts := h.getValidDomainHosts(ctx)
	orphans, orphanErr := h.orch.GetOrphanIngresses(ctx, validHosts)
	if orphanErr == nil && orphans != nil {
		stats.OrphanIngresses = len(orphans)
		stats.OrphanIngressNames = orphans
	}
	if stats.OrphanIngressNames == nil {
		stats.OrphanIngressNames = []string{}
	}

	httputil.RespondOK(c, stats)
}

// getValidDomainHosts returns a set of all domain hosts in the database.
func (h *ClusterHandler) getValidDomainHosts(ctx context.Context) map[string]bool {
	hosts := make(map[string]bool)
	// List all apps, get their domains
	apps, _, _ := h.store.Applications().ListAll(ctx, store.ListParams{Page: 1, PerPage: 1000}, store.AppListFilter{})
	for _, app := range apps {
		domains, _ := h.store.Domains().ListByApp(ctx, app.ID)
		for _, d := range domains {
			hosts[d.Host] = true
		}
	}
	return hosts
}

func (h *ClusterHandler) CleanupEvictedPods(c *gin.Context) {
	result, err := h.orch.CleanupEvictedPods(c.Request.Context())
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, result)
}

func (h *ClusterHandler) CleanupFailedPods(c *gin.Context) {
	result, err := h.orch.CleanupFailedPods(c.Request.Context())
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, result)
}

func (h *ClusterHandler) CleanupCompletedPods(c *gin.Context) {
	result, err := h.orch.CleanupCompletedPods(c.Request.Context())
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, result)
}

func (h *ClusterHandler) CleanupStaleReplicaSets(c *gin.Context) {
	result, err := h.orch.CleanupStaleReplicaSets(c.Request.Context())
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, result)
}

func (h *ClusterHandler) CleanupCompletedJobs(c *gin.Context) {
	result, err := h.orch.CleanupCompletedJobs(c.Request.Context())
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, result)
}

func (h *ClusterHandler) CleanupOrphanIngresses(c *gin.Context) {
	ctx := c.Request.Context()
	validHosts := h.getValidDomainHosts(ctx)
	result, err := h.orch.CleanupOrphanIngresses(ctx, validHosts)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, result)
}

func (h *ClusterHandler) ExpandPVC(c *gin.Context) {
	namespace := c.Param("namespace")
	name := c.Param("name")
	if namespace == "" || name == "" {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("namespace and name are required"))
		return
	}
	var body struct {
		Size string `json:"size" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail(err.Error()))
		return
	}
	if err := h.orch.ExpandVolume(c.Request.Context(), name, namespace, body.Size); err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, gin.H{"message": "PVC expanded"})
}
