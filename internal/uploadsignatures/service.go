package uploadsignatures

import (
	"fmt"

	"github.com/CodeEnthusiast09/x-clone-api/internal/cloudinary"
	"github.com/google/uuid"
)

type Service struct {
	cdn          *cloudinary.Client
	uploadPreset string
}

func NewService(cdn *cloudinary.Client, uploadPreset string) *Service {
	return &Service{
		cdn:          cdn,
		uploadPreset: uploadPreset,
	}
}

// Sign returns a fresh signed upload token. The public_id is pinned to
// <namespace>/<clerkID>/<random-uuid> so Cloudinary stores the asset under
// a path that proves who owns it. The mobile client cannot change the
// public_id without invalidating the signature.
func (s *Service) Sign(clerkID, namespace string, maxBytes int64) *cloudinary.UploadSignature {
	publicID := fmt.Sprintf("%s/%s/%s", namespace, clerkID, uuid.NewString())
	return s.cdn.Sign(s.uploadPreset, maxBytes, publicID)
}
