package search

import (
	"strconv"
	"strings"

	"github.com/CodeEnthusiast09/x-clone-api/internal/common"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Search  GET /api/search?q=<term>&limit=<n>
func (h *Handler) Search(c *gin.Context) {
	q := strings.TrimSpace(c.Query("q"))
	if q == "" {
		common.Error(c, 400, "q is required")
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if limit < 1 || limit > 50 {
		limit = 10
	}

	results, err := h.svc.Search(q, limit)
	if err != nil {
		common.Error(c, 500, "search failed")
		return
	}

	common.Success(c, 200, "ok", results)
}
