package comments

import (
	"log"
	"net/http"

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

func (h *Handler) ListByPost(c *gin.Context) {
	idStr := c.Param("postId")
	postID, err := uuid.Parse(idStr)
	if err != nil {
		common.Error(c, http.StatusBadRequest, "invalid post id")
		return
	}

	out, err := h.svc.ListByPost(postID)
	if err != nil {
		log.Printf("comments.ListByPost: %v", err)
		common.Error(c, http.StatusInternalServerError, "failed to list comments")
		return
	}

	common.Success(c, http.StatusOK, "comments fetched", out)
}
