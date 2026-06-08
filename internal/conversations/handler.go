package conversations

import (
	"errors"

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
