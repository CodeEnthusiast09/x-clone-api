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

// Create returns a short-lived signed upload token for a post image. The signed
// params pin the upload to a public_id under the caller's owner-scoped prefix —
// the mobile client must upload using the exact returned params or Cloudinary
// will reject the request.
func (h *Handler) Create(c *gin.Context) {
	clerkID := c.GetString(middleware.ContextClerkID)
	if clerkID == "" {
		common.Error(c, http.StatusUnauthorized, "missing authentication context")
		return
	}
	sig := h.svc.SignPostUpload(clerkID)
	common.Success(c, http.StatusOK, "upload signature generated", sig)
}
