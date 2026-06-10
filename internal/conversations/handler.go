package conversations

import (
	"errors"
	"net/http"

	"github.com/CodeEnthusiast09/x-clone-api/internal/common"
	"github.com/CodeEnthusiast09/x-clone-api/internal/middleware"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// StartOrGet  POST /api/conversations
func (h *Handler) StartOrGet(c *gin.Context) {
	clerkID := c.GetString(middleware.ContextClerkID)
	if clerkID == "" {
		common.Error(c, 401, "unauthorized")
		return
	}

	var in struct {
		RecipientID uuid.UUID `json:"recipientId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		common.Error(c, 400, err.Error())
		return
	}

	callerDBID, err := h.svc.userIDFromClerk(clerkID)
	if err != nil {
		common.Error(c, 500, "failed to resolve caller")
		return
	}
	if callerDBID == in.RecipientID {
		common.Error(c, 400, "cannot start a conversation with yourself")
		return
	}

	conv, err := h.svc.GetOrCreate(clerkID, in.RecipientID)
	if err != nil {
		if errors.Is(err, ErrRecipientNotFound) {
			common.Error(c, 404, "recipient not found")
			return
		}
		common.Error(c, 500, "failed to get or create conversation")
		return
	}

	common.Success(c, 200, "ok", conv)
}

// Delete  DELETE /api/conversations/:conversationId
func (h *Handler) Delete(c *gin.Context) {
	clerkID := c.GetString(middleware.ContextClerkID)
	if clerkID == "" {
		common.Error(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	convID, err := uuid.Parse(c.Param("conversationId"))
	if err != nil {
		common.Error(c, http.StatusBadRequest, "invalid conversation id")
		return
	}

	if err := h.svc.Delete(clerkID, convID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			common.Error(c, http.StatusNotFound, "conversation not found or access denied")
			return
		}
		if errors.Is(err, ErrUserNotSynced) {
			common.Error(c, http.StatusUnauthorized, "user not synced")
			return
		}
		common.Error(c, http.StatusInternalServerError, "failed to delete conversation")
		return
	}

	common.Success(c, http.StatusOK, "conversation deleted", nil)
}

// List  GET /api/conversations
func (h *Handler) List(c *gin.Context) {
	clerkID := c.GetString(middleware.ContextClerkID)
	if clerkID == "" {
		common.Error(c, 401, "unauthorized")
		return
	}

	views, err := h.svc.ListForUser(clerkID)
	if err != nil {
		common.Error(c, 500, "failed to list conversations")
		return
	}

	common.Success(c, 200, "ok", views)
}
