package webhooks

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/CodeEnthusiast09/x-clone-api/internal/common"
	"github.com/CodeEnthusiast09/x-clone-api/internal/users"
	"github.com/gin-gonic/gin"
	svix "github.com/svix/svix-webhooks/go"
)

type ClerkHandler struct {
	usersSvc      *users.Service
	webhookSecret string
}

func NewClerkHandler(usersSvc *users.Service, webhookSecret string) *ClerkHandler {
	return &ClerkHandler{
		usersSvc:      usersSvc,
		webhookSecret: webhookSecret,
	}
}

// clerkEvent is the minimal shape of a Clerk webhook payload we care about.
// We keep Data as RawMessage so we can decode it into the right shape per event type.
type clerkEvent struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type clerkUserData struct {
	ID                    string `json:"id"`
	FirstName             string `json:"first_name"`
	LastName              string `json:"last_name"`
	ImageURL              string `json:"image_url"`
	PrimaryEmailAddressID string `json:"primary_email_address_id"`
	EmailAddresses        []struct {
		ID           string `json:"id"`
		EmailAddress string `json:"email_address"`
	} `json:"email_addresses"`
}

func (h *ClerkHandler) Handle(c *gin.Context) {
	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		common.Error(c, http.StatusBadRequest, "failed to read request body")
		return
	}

	wh, err := svix.NewWebhook(h.webhookSecret)
	if err != nil {
		log.Printf("clerk webhook: bad secret config: %v", err)
		common.Error(c, http.StatusInternalServerError, "webhook misconfigured")
		return
	}

	if err := wh.Verify(payload, c.Request.Header); err != nil {
		log.Printf("clerk webhook: signature verification failed: %v", err)
		common.Error(c, http.StatusUnauthorized, "invalid webhook signature")
		return
	}

	var event clerkEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		common.Error(c, http.StatusBadRequest, "malformed event payload")
		return
	}

	switch event.Type {
	case "user.created":
		h.handleUserCreated(c, event.Data)
	case "user.updated":
		h.handleUserUpdated(c, event.Data)
	case "user.deleted":
		h.handleUserDeleted(c, event.Data)
	default:
		// Acknowledge unhandled events with 200 so Clerk stops retrying.
		log.Printf("clerk webhook: ignoring event type %q", event.Type)
		common.Success(c, http.StatusOK, "event ignored", nil)
	}
}

func (h *ClerkHandler) handleUserCreated(c *gin.Context, data json.RawMessage) {
	var u clerkUserData
	if err := json.Unmarshal(data, &u); err != nil {
		common.Error(c, http.StatusBadRequest, "malformed user payload")
		return
	}

	email := primaryEmail(u)
	if email == "" {
		common.Error(c, http.StatusBadRequest, "user has no email address")
		return
	}

	_, err := h.usersSvc.UpsertFromClerk(u.ID, email, u.FirstName, u.LastName, u.ImageURL)
	if err != nil {
		log.Printf("clerk webhook user.created: %v", err)
		common.Error(c, http.StatusInternalServerError, "failed to create user")
		return
	}

	common.Success(c, http.StatusOK, "user synced", nil)
}

func (h *ClerkHandler) handleUserUpdated(c *gin.Context, data json.RawMessage) {
	var u clerkUserData
	if err := json.Unmarshal(data, &u); err != nil {
		common.Error(c, http.StatusBadRequest, "malformed user payload")
		return
	}

	email := primaryEmail(u)
	if email == "" {
		common.Error(c, http.StatusBadRequest, "user has no email address")
		return
	}

	_, err := h.usersSvc.UpsertFromClerk(u.ID, email, u.FirstName, u.LastName, u.ImageURL)
	if err != nil {
		log.Printf("clerk webhook user.updated: %v", err)
		common.Error(c, http.StatusInternalServerError, "failed to update user")
		return
	}

	common.Success(c, http.StatusOK, "user updated", nil)
}

func (h *ClerkHandler) handleUserDeleted(c *gin.Context, data json.RawMessage) {
	var u clerkUserData
	if err := json.Unmarshal(data, &u); err != nil {
		common.Error(c, http.StatusBadRequest, "malformed user payload")
		return
	}

	if err := h.usersSvc.DeleteByClerkID(u.ID); err != nil {
		log.Printf("clerk webhook user.deleted: %v", err)
		common.Error(c, http.StatusInternalServerError, "failed to delete user")
		return
	}

	common.Success(c, http.StatusOK, "user deleted", nil)
}

// primaryEmail picks the email matching primary_email_address_id, falling back to the first.
func primaryEmail(u clerkUserData) string {
	for _, e := range u.EmailAddresses {
		if e.ID == u.PrimaryEmailAddressID {
			return strings.ToLower(e.EmailAddress)
		}
	}
	if len(u.EmailAddresses) > 0 {
		return strings.ToLower(u.EmailAddresses[0].EmailAddress)
	}
	return ""
}
