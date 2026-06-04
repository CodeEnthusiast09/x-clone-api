package posts

import (
	"errors"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/CodeEnthusiast09/x-clone-api/internal/cloudinary"
	"github.com/CodeEnthusiast09/x-clone-api/internal/common"
	"github.com/CodeEnthusiast09/x-clone-api/internal/middleware"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	svc *Service
	cdn *cloudinary.Client // optional; only write handlers use it
}

// NewHandler builds a posts handler. Pass nil for cdn when registering read-only routes.
func NewHandler(svc *Service, cdn *cloudinary.Client) *Handler {
	return &Handler{svc: svc, cdn: cdn}
}

func (h *Handler) List(c *gin.Context) {
	page, limit := parsePagination(c)

	out, total, err := h.svc.List(page, limit)
	if err != nil {
		log.Printf("posts.List: %v", err)
		common.Error(c, http.StatusInternalServerError, "failed to list posts")
		return
	}

	common.Paginated(c, http.StatusOK, "posts fetched", out, buildMeta(total, page, limit))
}

func (h *Handler) GetByID(c *gin.Context) {
	idStr := c.Param("postId")
	id, err := uuid.Parse(idStr)
	if err != nil {
		common.Error(c, http.StatusBadRequest, "invalid post id")
		return
	}

	p, err := h.svc.GetByID(id)
	if errors.Is(err, ErrPostNotFound) {
		common.Error(c, http.StatusNotFound, "post not found")
		return
	}
	if err != nil {
		log.Printf("posts.GetByID: %v", err)
		common.Error(c, http.StatusInternalServerError, "failed to fetch post")
		return
	}

	common.Success(c, http.StatusOK, "post fetched", p)
}

func (h *Handler) ListByUsername(c *gin.Context) {
	username := c.Param("username")
	if username == "" {
		common.Error(c, http.StatusBadRequest, "username is required")
		return
	}

	page, limit := parsePagination(c)

	out, total, err := h.svc.ListByUsername(username, page, limit)
	if errors.Is(err, ErrUserNotFound) {
		common.Error(c, http.StatusNotFound, "user not found")
		return
	}
	if err != nil {
		log.Printf("posts.ListByUsername: %v", err)
		common.Error(c, http.StatusInternalServerError, "failed to list posts")
		return
	}

	common.Paginated(c, http.StatusOK, "posts fetched", out, buildMeta(total, page, limit))
}

type createPostInput struct {
	Content string `json:"content" binding:"max=280"`
	Image   string `json:"image"   binding:"max=2048"`
}

func (h *Handler) Create(c *gin.Context) {
	clerkID := c.GetString(middleware.ContextClerkID)
	if clerkID == "" {
		common.Error(c, http.StatusUnauthorized, "missing authentication context")
		return
	}

	var in createPostInput
	if err := c.ShouldBindJSON(&in); err != nil {
		common.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	p, err := h.svc.Create(clerkID, in.Content, in.Image)
	if errors.Is(err, ErrEmptyPost) {
		common.Error(c, http.StatusBadRequest, "post must have content or image")
		return
	}
	if errors.Is(err, ErrInvalidImageURL) {
		common.Error(c, http.StatusBadRequest, "image URL must come from your own /upload-signatures call")
		return
	}
	if errors.Is(err, ErrUserNotSynced) {
		common.Error(c, http.StatusConflict, "user not synced; call POST /api/auth/sync first")
		return
	}
	if err != nil {
		log.Printf("posts.Create: %v", err)
		common.Error(c, http.StatusInternalServerError, "failed to create post")
		return
	}

	common.Success(c, http.StatusCreated, "post created", p)
}

func (h *Handler) Delete(c *gin.Context) {
	clerkID, postID, ok := h.authedPostContext(c)
	if !ok {
		return
	}

	imageURL, err := h.svc.Delete(clerkID, postID)
	if errors.Is(err, ErrPostNotFound) {
		common.Error(c, http.StatusNotFound, "post not found")
		return
	}
	if errors.Is(err, ErrUserNotSynced) {
		common.Error(c, http.StatusConflict, "user not synced; call POST /api/auth/sync first")
		return
	}
	if err != nil {
		log.Printf("posts.Delete: %v", err)
		common.Error(c, http.StatusInternalServerError, "failed to delete post")
		return
	}

	// Best-effort cleanup of the Cloudinary image. The post is already gone from the
	// user's perspective, so a destroy failure logs but does not fail the request —
	// orphans can be swept later.
	//
	// Defense in depth: re-validate that the asset's public_id starts with this
	// user's owner-scoped prefix before issuing destroy. Create-time validation
	// should already enforce this, but if a bypass ever lands we still won't let
	// one user destroy another user's Cloudinary asset.
	if imageURL != "" && h.cdn != nil {
		publicID := cloudinary.PublicIDFromURL(imageURL)
		if publicID == "" {
			log.Printf("posts.Delete: could not parse public_id from %q; skipping destroy", imageURL)
		} else if !strings.HasPrefix(publicID, expectedImagePrefix(clerkID)) {
			log.Printf("posts.Delete: public_id %q not owned by %s; skipping destroy", publicID, clerkID)
		} else if derr := h.cdn.Destroy(c.Request.Context(), publicID); derr != nil {
			log.Printf("posts.Delete: cloudinary destroy %s: %v", publicID, derr)
		}
	}

	common.Success(c, http.StatusOK, "post deleted", nil)
}

func (h *Handler) Like(c *gin.Context) {
	clerkID, postID, ok := h.authedPostContext(c)
	if !ok {
		return
	}

	err := h.svc.Like(clerkID, postID)
	if errors.Is(err, ErrPostNotFound) {
		common.Error(c, http.StatusNotFound, "post not found")
		return
	}
	if errors.Is(err, ErrUserNotSynced) {
		common.Error(c, http.StatusConflict, "user not synced; call POST /api/auth/sync first")
		return
	}
	if err != nil {
		log.Printf("posts.Like: %v", err)
		common.Error(c, http.StatusInternalServerError, "failed to like post")
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *Handler) Unlike(c *gin.Context) {
	clerkID, postID, ok := h.authedPostContext(c)
	if !ok {
		return
	}

	err := h.svc.Unlike(clerkID, postID)
	if errors.Is(err, ErrPostNotFound) {
		common.Error(c, http.StatusNotFound, "post not found")
		return
	}
	if errors.Is(err, ErrUserNotSynced) {
		common.Error(c, http.StatusConflict, "user not synced; call POST /api/auth/sync first")
		return
	}
	if err != nil {
		log.Printf("posts.Unlike: %v", err)
		common.Error(c, http.StatusInternalServerError, "failed to unlike post")
		return
	}

	c.Status(http.StatusNoContent)
}

// authedPostContext reads clerkID + :postId from the request. Writes the error
// response and returns ok=false if either is missing/invalid.
func (h *Handler) authedPostContext(c *gin.Context) (clerkID string, postID uuid.UUID, ok bool) {
	clerkID = c.GetString(middleware.ContextClerkID)
	if clerkID == "" {
		common.Error(c, http.StatusUnauthorized, "missing authentication context")
		return
	}
	parsed, err := uuid.Parse(c.Param("postId"))
	if err != nil {
		common.Error(c, http.StatusBadRequest, "invalid post id")
		return
	}
	return clerkID, parsed, true
}

func parsePagination(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return page, limit
}

func buildMeta(total int64, page, limit int) common.PaginationMeta {
	totalPages := 0
	if total > 0 {
		totalPages = int(math.Ceil(float64(total) / float64(limit)))
	}
	return common.PaginationMeta{
		Total:      total,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
	}
}
