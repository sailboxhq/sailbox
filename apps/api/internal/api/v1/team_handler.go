package v1

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/sailboxhq/sailbox/apps/api/internal/api/middleware"
	"github.com/sailboxhq/sailbox/apps/api/internal/apierr"
	"github.com/sailboxhq/sailbox/apps/api/internal/httputil"
	"github.com/sailboxhq/sailbox/apps/api/internal/service"
)

type TeamHandler struct {
	svc *service.TeamService
}

func NewTeamHandler(svc *service.TeamService) *TeamHandler {
	return &TeamHandler{svc: svc}
}

// ListMembers returns all members of the current organization.
func (h *TeamHandler) ListMembers(c *gin.Context) {
	orgID := middleware.GetOrgID(c)
	members, err := h.svc.ListMembers(c.Request.Context(), orgID)
	if err != nil {
		httputil.RespondError(c, apierr.ErrInternal.WithDetail(err.Error()))
		return
	}

	httputil.RespondList(c, members)
}

// UpdateMemberRole changes a member's role within the organization.
func (h *TeamHandler) UpdateMemberRole(c *gin.Context) {
	memberID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail("invalid member id"))
		return
	}

	var input struct {
		Role string `json:"role" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}

	// Prevent self-modification
	requesterID := middleware.GetUserID(c)
	if requesterID == memberID {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("cannot change your own role"))
		return
	}

	orgID := middleware.GetOrgID(c)
	if err := h.svc.UpdateMemberRole(c.Request.Context(), orgID, memberID, input.Role); err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail(err.Error()))
		return
	}

	httputil.RespondOK(c, gin.H{"message": "member role updated"})
}

// RemoveMember removes a member from the organization.
func (h *TeamHandler) RemoveMember(c *gin.Context) {
	memberID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail("invalid member id"))
		return
	}

	orgID := middleware.GetOrgID(c)
	if err := h.svc.RemoveMember(c.Request.Context(), orgID, memberID); err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail(err.Error()))
		return
	}

	httputil.RespondNoContent(c)
}

// InviteMember creates an invitation for a new member.
func (h *TeamHandler) InviteMember(c *gin.Context) {
	var input struct {
		Email string `json:"email" binding:"required,email"`
		Role  string `json:"role" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}

	orgID := middleware.GetOrgID(c)
	userID := middleware.GetUserID(c)

	inv, err := h.svc.InviteMember(c.Request.Context(), orgID, userID, input.Email, input.Role)
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail(err.Error()))
		return
	}

	scheme := "https"
	if c.Request.TLS == nil {
		scheme = "http"
	}
	inviteURL := fmt.Sprintf("%s://%s/auth/invite?token=%s", scheme, c.Request.Host, inv.Token)
	httputil.RespondCreated(c, gin.H{
		"invitation": inv,
		"invite_url": inviteURL,
	}, "")
}

// AcceptInvitation accepts a pending invitation.
func (h *TeamHandler) AcceptInvitation(c *gin.Context) {
	var input struct {
		Token string `json:"token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}

	userID := middleware.GetUserID(c)
	if err := h.svc.AcceptInvitation(c.Request.Context(), input.Token, userID); err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail(err.Error()))
		return
	}

	httputil.RespondOK(c, gin.H{"message": "invitation accepted"})
}

// ListInvitations returns all invitations for the current organization.
func (h *TeamHandler) ListInvitations(c *gin.Context) {
	orgID := middleware.GetOrgID(c)
	invitations, err := h.svc.ListInvitations(c.Request.Context(), orgID)
	if err != nil {
		httputil.RespondError(c, apierr.ErrInternal.WithDetail(err.Error()))
		return
	}

	httputil.RespondList(c, invitations)
}

// CancelInvitation deletes a pending invitation.
func (h *TeamHandler) CancelInvitation(c *gin.Context) {
	invID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail("invalid invitation id"))
		return
	}

	if err := h.svc.CancelInvitation(c.Request.Context(), invID); err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail(err.Error()))
		return
	}

	httputil.RespondNoContent(c)
}

// SetProjectAccess grants or updates a user's access to a project.
func (h *TeamHandler) SetProjectAccess(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("projectId"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail("invalid project id"))
		return
	}

	userID, err := uuid.Parse(c.Param("userId"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail("invalid user id"))
		return
	}

	var input struct {
		Role string `json:"role" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}

	if err := h.svc.SetProjectAccess(c.Request.Context(), projectID, userID, input.Role); err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail(err.Error()))
		return
	}

	httputil.RespondOK(c, gin.H{"message": "project access updated"})
}

// RemoveProjectAccess revokes a user's access to a project.
func (h *TeamHandler) RemoveProjectAccess(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("projectId"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail("invalid project id"))
		return
	}

	userID, err := uuid.Parse(c.Param("userId"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail("invalid user id"))
		return
	}

	if err := h.svc.RemoveProjectAccess(c.Request.Context(), projectID, userID); err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail(err.Error()))
		return
	}

	httputil.RespondNoContent(c)
}
