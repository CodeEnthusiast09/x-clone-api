package uploadsignatures

import (
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

// CreateFor returns a gin.HandlerFunc bound to a specific upload namespace and
// size cap. Used to register multiple signature endpoints (e.g. /posts,
// /banners) off a single Handler — each endpoint produces signed params that
// pin the upload to a different owner-scoped Cloudinary path.
func (h *Handler) CreateFor(namespace string, maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		clerkID := c.GetString(middleware.ContextClerkID)
		if clerkID == "" {
			common.Error(c, http.StatusUnauthorized, "missing authentication context")
			return
		}
		sig := h.svc.Sign(clerkID, namespace, maxBytes)
		common.Success(c, http.StatusOK, "upload signature generated", sig)
	}
}
