package uploadsignatures

import (
	"fmt"

	"github.com/CodeEnthusiast09/x-clone-api/internal/cloudinary"
	"github.com/google/uuid"
)

type Service struct {
	cdn          *cloudinary.Client
	uploadPreset string
	maxBytes     int64
	namespace    string
}

// NewService builds the signer. namespace is the public_id prefix that all
// generated uploads will live under — combined with the caller's clerkID it
// produces an owner-scoped path the rest of the API can authorize against.
func NewService(cdn *cloudinary.Client, uploadPreset string, maxBytes int64, namespace string) *Service {
	return &Service{
		cdn:          cdn,
		uploadPreset: uploadPreset,
		maxBytes:     maxBytes,
		namespace:    namespace,
	}
}

// SignPostUpload returns a fresh signed upload token for a post image.
// The public_id is pinned to <namespace>/<clerkID>/<random-uuid> so Cloudinary
// stores the asset under a path that proves who owns it. The mobile client
// cannot change the public_id without invalidating the signature.
func (s *Service) SignPostUpload(clerkID string) *cloudinary.UploadSignature {
	publicID := fmt.Sprintf("%s/%s/%s", s.namespace, clerkID, uuid.NewString())
	return s.cdn.Sign(s.uploadPreset, s.maxBytes, publicID)
}
