package v1

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sailboxhq/sailbox/apps/api/internal/api/middleware"
	"github.com/sailboxhq/sailbox/apps/api/internal/apierr"
	"github.com/sailboxhq/sailbox/apps/api/internal/httputil"
	"github.com/sailboxhq/sailbox/apps/api/internal/model"
	"github.com/sailboxhq/sailbox/apps/api/internal/store"
)

// Guard builds middleware that verifies the caller may touch the resource named
// by the ":id" path parameter.
//
// Two checks run for every guarded route:
//
//  1. Org ownership — the resource's project must belong to the caller's org.
//  2. Project-level access — if the caller has been granted explicit project
//     memberships, those are authoritative: they may only reach projects they
//     were granted, and a "viewer" grant is read-only. A user with no grants at
//     all keeps org-wide access, so granting access to one project is what
//     scopes a user down rather than silently locking out every existing member.
//
// Owners and org admins bypass the project-level check.
type Guard struct {
	store store.Store
}

func NewGuard(s store.Store) *Guard {
	return &Guard{store: s}
}

// Project guards routes whose ":id" is a project ID.
func (g *Guard) Project() gin.HandlerFunc {
	return g.guard("project", "id", func(ctx context.Context, id uuid.UUID) (uuid.UUID, uuid.UUID, error) {
		project, err := g.store.Projects().GetByID(ctx, id)
		if err != nil {
			return uuid.Nil, uuid.Nil, err
		}
		return project.OrgID, project.ID, nil
	})
}

// App guards routes whose ":id" is an application ID.
func (g *Guard) App() gin.HandlerFunc {
	return g.guard("application", "id", g.resolveApp)
}

// AppParam is App for routes that name the application differently, such as the
// WebSocket routes using ":appId".
func (g *Guard) AppParam(param string) gin.HandlerFunc {
	return g.guard("application", param, g.resolveApp)
}

func (g *Guard) resolveApp(ctx context.Context, id uuid.UUID) (uuid.UUID, uuid.UUID, error) {
	app, err := g.store.Applications().GetByID(ctx, id)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	project, err := g.store.Projects().GetByID(ctx, app.ProjectID)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	return project.OrgID, project.ID, nil
}

// Database guards routes whose ":id" is a managed database ID.
func (g *Guard) Database() gin.HandlerFunc {
	return g.guard("database", "id", func(ctx context.Context, id uuid.UUID) (uuid.UUID, uuid.UUID, error) {
		db, err := g.store.ManagedDatabases().GetByID(ctx, id)
		if err != nil {
			return uuid.Nil, uuid.Nil, err
		}
		project, err := g.store.Projects().GetByID(ctx, db.ProjectID)
		if err != nil {
			return uuid.Nil, uuid.Nil, err
		}
		return project.OrgID, project.ID, nil
	})
}

// CronJob guards routes whose ":id" is a cron job ID.
func (g *Guard) CronJob() gin.HandlerFunc {
	return g.guard("cron job", "id", func(ctx context.Context, id uuid.UUID) (uuid.UUID, uuid.UUID, error) {
		cj, err := g.store.CronJobs().GetByID(ctx, id)
		if err != nil {
			return uuid.Nil, uuid.Nil, err
		}
		project, err := g.store.Projects().GetByID(ctx, cj.ProjectID)
		if err != nil {
			return uuid.Nil, uuid.Nil, err
		}
		return project.OrgID, project.ID, nil
	})
}

// Domain guards routes whose ":id" is a domain ID.
func (g *Guard) Domain() gin.HandlerFunc {
	return g.guard("domain", "id", func(ctx context.Context, id uuid.UUID) (uuid.UUID, uuid.UUID, error) {
		domain, err := g.store.Domains().GetByID(ctx, id)
		if err != nil {
			return uuid.Nil, uuid.Nil, err
		}
		app, err := g.store.Applications().GetByID(ctx, domain.AppID)
		if err != nil {
			return uuid.Nil, uuid.Nil, err
		}
		project, err := g.store.Projects().GetByID(ctx, app.ProjectID)
		if err != nil {
			return uuid.Nil, uuid.Nil, err
		}
		return project.OrgID, project.ID, nil
	})
}

// Deployment guards routes whose ":id" is a deployment ID.
func (g *Guard) Deployment() gin.HandlerFunc {
	return g.guard("deployment", "id", func(ctx context.Context, id uuid.UUID) (uuid.UUID, uuid.UUID, error) {
		deploy, err := g.store.Deployments().GetByID(ctx, id)
		if err != nil {
			return uuid.Nil, uuid.Nil, err
		}
		app, err := g.store.Applications().GetByID(ctx, deploy.AppID)
		if err != nil {
			return uuid.Nil, uuid.Nil, err
		}
		project, err := g.store.Projects().GetByID(ctx, app.ProjectID)
		if err != nil {
			return uuid.Nil, uuid.Nil, err
		}
		return project.OrgID, project.ID, nil
	})
}

// VisibleProjectIDs returns the projects the caller is allowed to see, or nil
// when they are not restricted to a subset (an owner/admin, or a member with no
// explicit grants).
//
// Cross-project listings must apply this on top of the org filter. Without it a
// scoped member still sees every project's apps and deployments, and the IDs in
// those listings are enough to drive the per-resource routes.
func (g *Guard) VisibleProjectIDs(c *gin.Context) ([]uuid.UUID, error) {
	return grantedProjectIDs(c, g.store)
}

// grantedProjectIDs returns the caller's explicit project grants: a nil slice
// means "not restricted" (owner/admin, or a member with no grants at all).
//
// A lookup failure is returned as an error and never as nil — nil is the
// unrestricted case, so swallowing the error here would widen access exactly
// when the database is in trouble.
func grantedProjectIDs(c *gin.Context, s store.Store) ([]uuid.UUID, error) {
	role := middleware.GetUserRole(c)
	if role == string(model.RoleOwner) || role == string(model.RoleAdmin) {
		return nil, nil
	}

	grants, err := s.ProjectMembers().ListByUserInOrg(c.Request.Context(), middleware.GetUserID(c), middleware.GetOrgID(c))
	if err != nil {
		return nil, err
	}
	if len(grants) == 0 {
		// No explicit grants — org membership still carries project access.
		return nil, nil
	}

	ids := make([]uuid.UUID, 0, len(grants))
	for _, grant := range grants {
		ids = append(ids, grant.ProjectID)
	}
	return ids, nil
}

// projectAccess applies the per-project rule for one project. write selects the
// stricter check: a "viewer" grant allows reads but not modifications.
func projectAccess(c *gin.Context, s store.Store, projectID uuid.UUID, write bool) error {
	role := middleware.GetUserRole(c)
	if role == string(model.RoleOwner) || role == string(model.RoleAdmin) {
		return nil
	}

	grants, err := s.ProjectMembers().ListByUserInOrg(c.Request.Context(), middleware.GetUserID(c), middleware.GetOrgID(c))
	if err != nil {
		return errAccessDenied
	}
	if len(grants) == 0 {
		return nil
	}

	for _, grant := range grants {
		if grant.ProjectID != projectID {
			continue
		}
		if write && grant.Role == projectRoleViewer {
			return errReadOnlyAccess
		}
		return nil
	}
	return errAccessDenied
}

// RequireProjectWrite verifies that the caller may create or modify resources in
// projectID: the project must belong to their org, and any explicit grant must
// allow writes. It responds and returns false when it does not, so handlers that
// take a project_id in the request body get the same rule as the :id routes.
func RequireProjectWrite(c *gin.Context, s store.Store, projectID uuid.UUID) bool {
	project, err := s.Projects().GetByID(c.Request.Context(), projectID)
	if err != nil {
		httputil.RespondError(c, apierr.ErrNotFound.WithDetail("project not found"))
		return false
	}
	if project.OrgID != middleware.GetOrgID(c) {
		httputil.RespondError(c, apierr.ErrForbidden.WithDetail("access denied"))
		return false
	}
	if err := projectAccess(c, s, projectID, true); err != nil {
		httputil.RespondError(c, apierr.ErrForbidden.WithDetail(err.Error()))
		return false
	}
	return true
}

// resolver maps the ":id" path parameter to the owning org and project.
type resolver func(ctx context.Context, id uuid.UUID) (orgID, projectID uuid.UUID, err error)

func (g *Guard) guard(kind, param string, resolve resolver) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param(param))
		if err != nil {
			httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid "+kind+" ID"))
			c.Abort()
			return
		}

		ctx := c.Request.Context()
		orgID, projectID, err := resolve(ctx, id)
		if err != nil {
			httputil.RespondError(c, apierr.ErrNotFound.WithDetail(kind+" not found"))
			c.Abort()
			return
		}

		if orgID != middleware.GetOrgID(c) {
			httputil.RespondError(c, apierr.ErrForbidden.WithDetail("access denied"))
			c.Abort()
			return
		}

		if err := g.checkProjectAccess(c, projectID); err != nil {
			httputil.RespondError(c, apierr.ErrForbidden.WithDetail(err.Error()))
			c.Abort()
			return
		}

		c.Set(ctxGuardProjectID, projectID)
		c.Next()
	}
}

// ctxGuardProjectID holds the project a guard resolved, so middleware running
// after it can apply further checks without re-resolving.
const ctxGuardProjectID = "guard_project_id"

// RequireWrite demands write access to the project a preceding guard resolved.
//
// The method-based rule treats every GET as viewer-safe, which is wrong for the
// handful of reads that hand back a credential: a webhook signing secret lets a
// viewer forge a push and trigger a deployment, and a terminal session is a
// write no matter which verb opens it.
func (g *Guard) RequireWrite() gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID, ok := c.Get(ctxGuardProjectID)
		if !ok {
			httputil.RespondError(c, apierr.ErrForbidden.WithDetail("access denied"))
			c.Abort()
			return
		}
		if err := projectAccess(c, g.store, projectID.(uuid.UUID), true); err != nil {
			httputil.RespondError(c, apierr.ErrForbidden.WithDetail(err.Error()))
			c.Abort()
			return
		}
		c.Next()
	}
}

// checkProjectAccess enforces per-project grants for non-admin callers. Reads
// are allowed on a "viewer" grant; anything else counts as a write.
func (g *Guard) checkProjectAccess(c *gin.Context, projectID uuid.UUID) error {
	return projectAccess(c, g.store, projectID, !isReadMethod(c.Request.Method))
}

const projectRoleViewer = "viewer"

func isReadMethod(method string) bool {
	return method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions
}

type guardError string

func (e guardError) Error() string { return string(e) }

const (
	errAccessDenied   = guardError("access denied")
	errReadOnlyAccess = guardError("read-only access to this project")
)
