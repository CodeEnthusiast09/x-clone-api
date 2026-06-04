package uploadsignatures

import (
	"github.com/CodeEnthusiast09/x-clone-api/internal/cloudinary"
	"github.com/gin-gonic/gin"
)

// RegisterProtected mounts POST /upload-signatures. The caller is responsible
// for attaching the auth middleware to rg. namespace is the public_id prefix
// for the generated uploads (e.g. "x_clone/posts/users") — must match the
// prefix the downstream consumer (posts) validates against.
func RegisterProtected(rg *gin.RouterGroup, cdn *cloudinary.Client, uploadPreset string, maxBytes int64, namespace string) {
	svc := NewService(cdn, uploadPreset, maxBytes, namespace)
	h := NewHandler(svc)

	rg.POST("/upload-signatures", h.Create)
}
