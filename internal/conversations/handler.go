package conversations

import (
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

// StartOrGet  POST /api/conversations
func (h *Handler) StartOrGet(c *gin.Context) {
	callerID := c.MustGet("userID").(uuid.UUID)

	var in struct {
		RecipientID uuid.UUID `json:"recipientId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		common.Error(c, 400, err.Error())
		return
	}

	if callerID == in.RecipientID {
		common.Error(c, 400, "cannot start a conversation with yourself")
		return
	}

	conv, err := h.svc.GetOrCreate(callerID, in.RecipientID)
	if err != nil {
		common.Error(c, 500, "failed to get or create conversation")
		return
	}

	common.Success(c, 200, "ok", conv)
}

// List  GET /api/conversations
func (h *Handler) List(c *gin.Context) {
	callerID := c.MustGet("userID").(uuid.UUID)

	convs, err := h.svc.ListForUser(callerID)
	if err != nil {
		common.Error(c, 500, "failed to list conversations")
		return
	}

	common.Success(c, 200, "ok", convs)
}
