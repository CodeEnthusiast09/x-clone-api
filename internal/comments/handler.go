package comments

import (
	"errors"
	"log"
	"net/http"

	"github.com/CodeEnthusiast09/x-clone-api/internal/common"
	"github.com/CodeEnthusiast09/x-clone-api/internal/middleware"
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

type createCommentInput struct {
	Content string `json:"content" binding:"required,max=280"`
}

func (h *Handler) Create(c *gin.Context) {
	clerkID := c.GetString(middleware.ContextClerkID)
	if clerkID == "" {
		common.Error(c, http.StatusUnauthorized, "missing authentication context")
		return
	}

	postID, err := uuid.Parse(c.Param("postId"))
	if err != nil {
		common.Error(c, http.StatusBadRequest, "invalid post id")
		return
	}

	var in createCommentInput
	if err := c.ShouldBindJSON(&in); err != nil {
		common.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	cm, err := h.svc.Create(clerkID, postID, in.Content)
	if errors.Is(err, ErrEmptyContent) {
		common.Error(c, http.StatusBadRequest, "content is required")
		return
	}
	if errors.Is(err, ErrContentTooLong) {
		common.Error(c, http.StatusBadRequest, "content exceeds 280 characters")
		return
	}
	if errors.Is(err, ErrPostNotFound) {
		common.Error(c, http.StatusNotFound, "post not found")
		return
	}
	if errors.Is(err, ErrUserNotSynced) {
		common.Error(c, http.StatusConflict, "user not synced; call POST /api/auth/sync first")
		return
	}
	if err != nil {
		log.Printf("comments.Create: %v", err)
		common.Error(c, http.StatusInternalServerError, "failed to create comment")
		return
	}

	common.Success(c, http.StatusCreated, "comment created", cm)
}

func (h *Handler) Delete(c *gin.Context) {
	clerkID := c.GetString(middleware.ContextClerkID)
	if clerkID == "" {
		common.Error(c, http.StatusUnauthorized, "missing authentication context")
		return
	}

	commentID, err := uuid.Parse(c.Param("commentId"))
	if err != nil {
		common.Error(c, http.StatusBadRequest, "invalid comment id")
		return
	}

	err = h.svc.Delete(clerkID, commentID)
	if errors.Is(err, ErrCommentNotFound) {
		common.Error(c, http.StatusNotFound, "comment not found")
		return
	}
	if errors.Is(err, ErrUserNotSynced) {
		common.Error(c, http.StatusConflict, "user not synced; call POST /api/auth/sync first")
		return
	}
	if err != nil {
		log.Printf("comments.Delete: %v", err)
		common.Error(c, http.StatusInternalServerError, "failed to delete comment")
		return
	}

	common.Success(c, http.StatusOK, "comment deleted", nil)
}
