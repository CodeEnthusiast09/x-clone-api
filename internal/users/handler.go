package users

import (
	"errors"
	"log"
	"net/http"

	"github.com/CodeEnthusiast09/x-clone-api/internal/common"
	"github.com/CodeEnthusiast09/x-clone-api/internal/middleware"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) GetByUsername(c *gin.Context) {
	username := c.Param("username")
	if username == "" {
		common.Error(c, http.StatusBadRequest, "username is required")
		return
	}

	callerID := h.svc.callerID(c.GetString(middleware.ContextClerkID))

	profile, err := h.svc.GetProfile(callerID, username)
	if errors.Is(err, ErrUserNotFound) {
		common.Error(c, http.StatusNotFound, "user not found")
		return
	}
	if err != nil {
		log.Printf("users.GetByUsername: %v", err)
		common.Error(c, http.StatusInternalServerError, "failed to fetch user")
		return
	}

	common.Success(c, http.StatusOK, "user fetched", profile)
}

// Me returns the currently-authenticated user's record from our DB.
// If the user hasn't been synced yet, hints to call /api/auth/sync.
func (h *Handler) Me(c *gin.Context) {
	clerkID := c.GetString(middleware.ContextClerkID)
	if clerkID == "" {
		common.Error(c, http.StatusUnauthorized, "missing authentication context")
		return
	}

	user, err := h.svc.GetByClerkID(clerkID)
	if errors.Is(err, ErrUserNotFound) {
		common.Error(c, http.StatusNotFound, "user not synced; call POST /api/auth/sync")
		return
	}
	if err != nil {
		log.Printf("users.Me: %v", err)
		common.Error(c, http.StatusInternalServerError, "failed to fetch user")
		return
	}

	common.Success(c, http.StatusOK, "user fetched", user)
}

type updateMeInput struct {
	FirstName   *string `json:"firstName"   binding:"omitempty,max=100"`
	LastName    *string `json:"lastName"    binding:"omitempty,max=100"`
	Bio         *string `json:"bio"         binding:"omitempty,max=160"`
	Location    *string `json:"location"    binding:"omitempty,max=100"`
	BannerImage *string `json:"bannerImage" binding:"omitempty,max=2048"`
}

// UpdateMe applies a partial update to the authenticated user's profile.
// Body fields are all optional; at least one must be provided.
func (h *Handler) UpdateMe(c *gin.Context) {
	clerkID := c.GetString(middleware.ContextClerkID)
	if clerkID == "" {
		common.Error(c, http.StatusUnauthorized, "missing authentication context")
		return
	}

	var in updateMeInput
	if err := c.ShouldBindJSON(&in); err != nil {
		common.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	user, err := h.svc.UpdateProfile(clerkID, UpdateProfileInput(in))
	if errors.Is(err, ErrEmptyUpdate) {
		common.Error(c, http.StatusBadRequest, "at least one field must be provided")
		return
	}
	if errors.Is(err, ErrInvalidBannerURL) {
		common.Error(c, http.StatusBadRequest, "banner URL must come from your own /upload-signatures/banners call")
		return
	}
	if errors.Is(err, ErrUserNotFound) {
		common.Error(c, http.StatusNotFound, "user not synced; call POST /api/auth/sync")
		return
	}
	if err != nil {
		log.Printf("users.UpdateMe: %v", err)
		common.Error(c, http.StatusInternalServerError, "failed to update profile")
		return
	}

	common.Success(c, http.StatusOK, "profile updated", user)
}

// Sync is the fallback for when the webhook hasn't created the user yet.
// Idempotent: if the user already exists, returns them as-is.
func (h *Handler) Sync(c *gin.Context) {
	clerkID := c.GetString(middleware.ContextClerkID)
	if clerkID == "" {
		common.Error(c, http.StatusUnauthorized, "missing authentication context")
		return
	}

	user, err := h.svc.Sync(c.Request.Context(), clerkID)
	if err != nil {
		log.Printf("users.Sync: %v", err)
		common.Error(c, http.StatusInternalServerError, "failed to sync user")
		return
	}

	common.Success(c, http.StatusOK, "user synced", user)
}
