package notifications

import (
	"log"
	"net/http"
	"strconv"

	"github.com/CodeEnthusiast09/x-clone-api/internal/common"
	"github.com/CodeEnthusiast09/x-clone-api/internal/middleware"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) List(c *gin.Context) {
	clerkID := c.GetString(middleware.ContextClerkID)
	recipientID, err := h.svc.userIDFromClerk(clerkID)
	if err != nil {
		common.Error(c, http.StatusUnauthorized, "user not synced")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 20
	}

	items, total, err := h.svc.List(recipientID, page, limit)
	if err != nil {
		log.Printf("notifications.List: %v", err)
		common.Error(c, http.StatusInternalServerError, "failed to fetch notifications")
		return
	}

	totalPages := int(total) / limit
	if int(total)%limit != 0 {
		totalPages++
	}

	common.Paginated(c, http.StatusOK, "notifications fetched", items, common.PaginationMeta{
		Total:      total,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
	})
}

type pushTokenInput struct {
	Token string `json:"token" binding:"required"`
}

func (h *Handler) RegisterPushToken(c *gin.Context) {
	clerkID := c.GetString(middleware.ContextClerkID)
	userID, err := h.svc.userIDFromClerk(clerkID)
	if err != nil {
		common.Error(c, http.StatusUnauthorized, "user not synced")
		return
	}

	var in pushTokenInput
	if err := c.ShouldBindJSON(&in); err != nil {
		common.Error(c, http.StatusBadRequest, "token is required")
		return
	}

	if err := h.svc.UpsertPushToken(userID, in.Token); err != nil {
		log.Printf("notifications.RegisterPushToken: %v", err)
		common.Error(c, http.StatusInternalServerError, "failed to register push token")
		return
	}

	common.Success(c, http.StatusOK, "push token registered", nil)
}

func (h *Handler) UnregisterPushToken(c *gin.Context) {
	clerkID := c.GetString(middleware.ContextClerkID)
	userID, err := h.svc.userIDFromClerk(clerkID)
	if err != nil {
		common.Error(c, http.StatusUnauthorized, "user not synced")
		return
	}

	var in pushTokenInput
	if err := c.ShouldBindJSON(&in); err != nil {
		common.Error(c, http.StatusBadRequest, "token is required")
		return
	}

	if err := h.svc.DeletePushToken(userID, in.Token); err != nil {
		log.Printf("notifications.UnregisterPushToken: %v", err)
		common.Error(c, http.StatusInternalServerError, "failed to unregister push token")
		return
	}

	common.Success(c, http.StatusOK, "push token removed", nil)
}

func (h *Handler) UnreadCount(c *gin.Context) {
	clerkID := c.GetString(middleware.ContextClerkID)
	recipientID, err := h.svc.userIDFromClerk(clerkID)
	if err != nil {
		common.Error(c, http.StatusUnauthorized, "user not synced")
		return
	}

	count, err := h.svc.UnreadCount(recipientID)
	if err != nil {
		log.Printf("notifications.UnreadCount: %v", err)
		common.Error(c, http.StatusInternalServerError, "failed to fetch unread count")
		return
	}

	common.Success(c, http.StatusOK, "unread count fetched", map[string]int64{"count": count})
}

func (h *Handler) MarkAllRead(c *gin.Context) {
	clerkID := c.GetString(middleware.ContextClerkID)
	recipientID, err := h.svc.userIDFromClerk(clerkID)
	if err != nil {
		common.Error(c, http.StatusUnauthorized, "user not synced")
		return
	}

	if err := h.svc.MarkAllRead(recipientID); err != nil {
		log.Printf("notifications.MarkAllRead: %v", err)
		common.Error(c, http.StatusInternalServerError, "failed to mark notifications read")
		return
	}

	common.Success(c, http.StatusOK, "notifications marked as read", nil)
}
