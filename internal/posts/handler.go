package posts

import (
	"errors"
	"log"
	"math"
	"net/http"
	"strconv"

	"github.com/CodeEnthusiast09/x-clone-api/internal/common"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
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
