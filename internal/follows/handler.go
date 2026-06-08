package follows

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

func (h *Handler) Follow(c *gin.Context) {
	clerkID, username, ok := h.authedFollowContext(c)
	if !ok {
		return
	}

	err := h.svc.Follow(clerkID, username)
	if errors.Is(err, ErrSelfFollow) {
		common.Error(c, http.StatusBadRequest, "cannot follow yourself")
		return
	}
	if errors.Is(err, ErrUserNotFound) {
		common.Error(c, http.StatusNotFound, "user not found")
		return
	}
	if errors.Is(err, ErrUserNotSynced) {
		common.Error(c, http.StatusConflict, "user not synced; call POST /api/auth/sync first")
		return
	}
	if err != nil {
		log.Printf("follows.Follow: %v", err)
		common.Error(c, http.StatusInternalServerError, "failed to follow user")
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *Handler) Unfollow(c *gin.Context) {
	clerkID, username, ok := h.authedFollowContext(c)
	if !ok {
		return
	}

	err := h.svc.Unfollow(clerkID, username)
	if errors.Is(err, ErrSelfFollow) {
		common.Error(c, http.StatusBadRequest, "cannot unfollow yourself")
		return
	}
	if errors.Is(err, ErrUserNotFound) {
		common.Error(c, http.StatusNotFound, "user not found")
		return
	}
	if errors.Is(err, ErrUserNotSynced) {
		common.Error(c, http.StatusConflict, "user not synced; call POST /api/auth/sync first")
		return
	}
	if err != nil {
		log.Printf("follows.Unfollow: %v", err)
		common.Error(c, http.StatusInternalServerError, "failed to unfollow user")
		return
	}

	c.Status(http.StatusNoContent)
}

// authedFollowContext reads clerkID + :username from the request. Writes the
// error response and returns ok=false if either is missing.
func (h *Handler) authedFollowContext(c *gin.Context) (clerkID, username string, ok bool) {
	clerkID = c.GetString(middleware.ContextClerkID)
	if clerkID == "" {
		common.Error(c, http.StatusUnauthorized, "missing authentication context")
		return
	}
	username = c.Param("username")
	if username == "" {
		common.Error(c, http.StatusBadRequest, "username is required")
		return
	}
	return clerkID, username, true
}
