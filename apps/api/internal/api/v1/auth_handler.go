package v1

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sailboxhq/sailbox/apps/api/internal/api/middleware"
	"github.com/sailboxhq/sailbox/apps/api/internal/apierr"
	"github.com/sailboxhq/sailbox/apps/api/internal/httputil"
	"github.com/sailboxhq/sailbox/apps/api/internal/service"
)

type AuthHandler struct {
	svc *service.AuthService
}

func NewAuthHandler(svc *service.AuthService) *AuthHandler {
	return &AuthHandler{svc: svc}
}

func (h *AuthHandler) SetupStatus(c *gin.Context) {
	status, err := h.svc.GetSetupStatus(c.Request.Context())
	if err != nil {
		httputil.RespondError(c, apierr.ErrInternal.WithDetail(err.Error()))
		return
	}

	httputil.RespondOK(c, status)
}

func (h *AuthHandler) Register(c *gin.Context) {
	var input service.RegisterInput
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}

	result, err := h.svc.Register(c.Request.Context(), input)
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail(err.Error()))
		return
	}

	httputil.RespondCreated(c, result, "")
}

func (h *AuthHandler) Login(c *gin.Context) {
	var input service.LoginInput
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}

	result, err := h.svc.Login(c.Request.Context(), input)
	if err != nil {
		c.JSON(http.StatusUnauthorized, apierr.ErrUnauthorized.WithDetail(err.Error()))
		return
	}

	httputil.RespondOK(c, result)
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	var input service.RefreshInput
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}

	result, err := h.svc.Refresh(c.Request.Context(), input)
	if err != nil {
		c.JSON(http.StatusUnauthorized, apierr.ErrUnauthorized.WithDetail(err.Error()))
		return
	}

	httputil.RespondOK(c, result)
}

func (h *AuthHandler) Me(c *gin.Context) {
	userID := middleware.GetUserID(c)
	user, err := h.svc.GetUser(c.Request.Context(), userID)
	if err != nil {
		httputil.RespondError(c, apierr.ErrNotFound.WithDetail("user not found"))
		return
	}

	httputil.RespondOK(c, user)
}

func (h *AuthHandler) UpdateProfile(c *gin.Context) {
	var input service.UpdateProfileInput
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}

	userID := middleware.GetUserID(c)
	user, err := h.svc.UpdateProfile(c.Request.Context(), userID, input)
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail(err.Error()))
		return
	}

	httputil.RespondOK(c, user)
}

func (h *AuthHandler) ChangePassword(c *gin.Context) {
	var input service.ChangePasswordInput
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}

	userID := middleware.GetUserID(c)
	if err := h.svc.ChangePassword(c.Request.Context(), userID, input); err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail(err.Error()))
		return
	}

	httputil.RespondOK(c, gin.H{"message": "password changed successfully"})
}

func (h *AuthHandler) ListAvatars(c *gin.Context) {
	httputil.RespondOK(c, h.svc.ListAvatars())
}

func (h *AuthHandler) Setup2FA(c *gin.Context) {
	userID := middleware.GetUserID(c)
	result, err := h.svc.Setup2FA(c.Request.Context(), userID)
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail(err.Error()))
		return
	}

	httputil.RespondOK(c, result)
}

func (h *AuthHandler) Verify2FA(c *gin.Context) {
	var input service.Verify2FAInput
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}

	userID := middleware.GetUserID(c)
	if err := h.svc.Verify2FA(c.Request.Context(), userID, input.Code); err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail(err.Error()))
		return
	}

	httputil.RespondOK(c, gin.H{"message": "2FA enabled successfully"})
}

func (h *AuthHandler) Disable2FA(c *gin.Context) {
	var input service.Verify2FAInput
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}

	userID := middleware.GetUserID(c)
	if err := h.svc.Disable2FA(c.Request.Context(), userID, input.Code); err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail(err.Error()))
		return
	}

	httputil.RespondOK(c, gin.H{"message": "2FA disabled successfully"})
}
