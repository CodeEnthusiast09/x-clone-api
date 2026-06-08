package chat

import (
	"github.com/CodeEnthusiast09/x-clone-api/internal/conversations"
	"github.com/CodeEnthusiast09/x-clone-api/internal/messages"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Register mounts the WebSocket endpoint. The caller is responsible for
// attaching the auth middleware to rg.
func Register(rg *gin.RouterGroup, hub *Hub, db *gorm.DB, env string, allowedOrigins map[string]bool) {
	msgSvc := messages.NewService(db)
	convSvc := conversations.NewService(db)
	h := NewHandler(hub, msgSvc, convSvc, env, allowedOrigins)

	rg.GET("/conversations/:conversationId/ws", h.ServeWS)
}
