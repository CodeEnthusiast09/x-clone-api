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

	user, err := h.svc.GetByUsername(username)
	if errors.Is(err, ErrUserNotFound) {
		common.Error(c, http.StatusNotFound, "user not found")
		return
	}
	if err != nil {
		log.Printf("users.GetByUsername: %v", err)
		common.Error(c, http.StatusInternalServerError, "failed to fetch user")
		return
	}

	common.Success(c, http.StatusOK, "user fetched", user)
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
