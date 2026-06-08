package messages

import (
	"strconv"

	"github.com/CodeEnthusiast09/x-clone-api/internal/common"
	"github.com/CodeEnthusiast09/x-clone-api/internal/conversations"
	"github.com/CodeEnthusiast09/x-clone-api/internal/middleware"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	svc     *Service
	convSvc *conversations.Service
}

func NewHandler(svc *Service, convSvc *conversations.Service) *Handler {
	return &Handler{svc: svc, convSvc: convSvc}
}

// List  GET /api/conversations/:conversationId/messages
func (h *Handler) List(c *gin.Context) {
	clerkID := c.GetString(middleware.ContextClerkID)
	if clerkID == "" {
		common.Error(c, 401, "unauthorized")
		return
	}

	convID, err := uuid.Parse(c.Param("conversationId"))
	if err != nil {
		common.Error(c, 400, "invalid conversation id")
		return
	}

	conv, err := h.convSvc.GetByID(convID, clerkID)
	if err != nil {
		common.Error(c, 500, "failed to fetch conversation")
		return
	}
	if conv == nil {
		common.Error(c, 404, "conversation not found")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 50
	}

	msgs, total, err := h.svc.ListForConversation(convID, page, limit)
	if err != nil {
		common.Error(c, 500, "failed to fetch messages")
		return
	}

	totalPages := int(total) / limit
	if int(total)%limit != 0 {
		totalPages++
	}
	common.Paginated(c, 200, "ok", msgs, common.PaginationMeta{
		Total:      total,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
	})
}

// MarkRead  PATCH /api/conversations/:conversationId/read
func (h *Handler) MarkRead(c *gin.Context) {
	clerkID := c.GetString(middleware.ContextClerkID)
	if clerkID == "" {
		common.Error(c, 401, "unauthorized")
		return
	}

	convID, err := uuid.Parse(c.Param("conversationId"))
	if err != nil {
		common.Error(c, 400, "invalid conversation id")
		return
	}

	conv, err := h.convSvc.GetByID(convID, clerkID)
	if err != nil {
		common.Error(c, 500, "failed to fetch conversation")
		return
	}
	if conv == nil {
		common.Error(c, 404, "conversation not found")
		return
	}

	updated, err := h.svc.MarkRead(convID, clerkID)
	if err != nil {
		common.Error(c, 500, "failed to mark messages as read")
		return
	}

	common.Success(c, 200, "ok", gin.H{"markedRead": updated})
}
