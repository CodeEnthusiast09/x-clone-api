package chat

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/CodeEnthusiast09/x-clone-api/internal/common"
	"github.com/CodeEnthusiast09/x-clone-api/internal/conversations"
	"github.com/CodeEnthusiast09/x-clone-api/internal/messages"
	"github.com/CodeEnthusiast09/x-clone-api/internal/middleware"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type Handler struct {
	hub             *Hub
	msgSvc          *messages.Service
	convSvc         *conversations.Service
	upgrader        websocket.Upgrader
}

func NewHandler(hub *Hub, msgSvc *messages.Service, convSvc *conversations.Service, env string, allowedOrigins map[string]bool) *Handler {
	return &Handler{
		hub:     hub,
		msgSvc:  msgSvc,
		convSvc: convSvc,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			// Native mobile clients (Expo) omit Origin entirely — always allow.
			// Browser clients send Origin; allow all in dev, check allowlist in prod.
			CheckOrigin: func(r *http.Request) bool {
				origin := r.Header.Get("Origin")
				if origin == "" {
					return true // native mobile client
				}
				if env != "production" {
					return true // dev/staging: allow all for testing tools
				}
				return allowedOrigins[origin]
			},
		},
	}
}

// ServeWS  GET /api/conversations/:conversationId/ws
//
// Verifies the caller is a participant, upgrades to WebSocket, then:
//   - registers the client with the hub
//   - marks all unread incoming messages as read and broadcasts a "read" event
//   - starts read/write pump goroutines
func (h *Handler) ServeWS(c *gin.Context) {
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

	conv, err := h.convSvc.GetByID(convID, clerkID)
	if err != nil {
		common.Error(c, http.StatusInternalServerError, "failed to fetch conversation")
		return
	}
	if conv == nil {
		common.Error(c, http.StatusForbidden, "conversation not found or access denied")
		return
	}

	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("ws upgrade failed: conv=%s clerk=%s: %v", convID, clerkID, err)
		return
	}

	client := &Client{
		hub:            h.hub,
		conn:           conn,
		conversationID: convID.String(),
		clerkID:        clerkID,
		send:           make(chan []byte, 256),
	}
	h.hub.register <- client

	// Mark all unread messages in this conversation as read for this caller,
	// then broadcast a "read" event so the sender's client can update their UI.
	updated, err := h.msgSvc.MarkRead(convID, clerkID)
	if err == nil && updated > 0 {
		if payload, jsonErr := json.Marshal(WSEvent{
			Type: "read",
			Data: gin.H{"conversationId": convID.String(), "readerId": clerkID},
		}); jsonErr == nil {
			h.hub.broadcast <- &BroadcastMsg{
				ConversationID: convID.String(),
				Payload:        payload,
				Exclude:        client,
			}
		}
	}

	go client.WritePump()

	// ReadPump runs in the current goroutine (blocks until disconnect).
	client.ReadPump(func(body string) {
		msg, err := h.msgSvc.Create(convID, clerkID, body)
		if err != nil {
			log.Printf("ws message persist failed: conv=%s clerk=%s: %v", convID, clerkID, err)
			return
		}
		payload, err := json.Marshal(WSEvent{Type: "message", Data: msg})
		if err != nil {
			return
		}
		h.hub.broadcast <- &BroadcastMsg{
			ConversationID: convID.String(),
			Payload:        payload,
		}
	})
}
