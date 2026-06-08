package uploadsignatures

import (
	"github.com/CodeEnthusiast09/x-clone-api/internal/cloudinary"
	"github.com/gin-gonic/gin"
)

// Mount describes one signature endpoint: the URL it lives at, the
// owner-scoped public_id prefix the generated uploads land under, and the
// max upload size for that asset type.
type Mount struct {
	Path      string
	Namespace string
	MaxBytes  int64
}

// RegisterProtected mounts one POST handler per Mount. The caller is responsible
// for attaching the auth middleware to rg. Each generated handler signs uploads
// for its bound namespace + size cap; the same upload preset is reused.
func RegisterProtected(rg *gin.RouterGroup, cdn *cloudinary.Client, uploadPreset string, mounts []Mount) {
	svc := NewService(cdn, uploadPreset)
	h := NewHandler(svc)

	for _, m := range mounts {
		rg.POST(m.Path, h.CreateFor(m.Namespace, m.MaxBytes))
	}
}
