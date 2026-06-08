package messages

import (
	"github.com/CodeEnthusiast09/x-clone-api/internal/conversations"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Register mounts the read endpoints. The caller is responsible for attaching
// the auth middleware to rg.
func Register(rg *gin.RouterGroup, db *gorm.DB) {
	svc := NewService(db)
	convSvc := conversations.NewService(db)
	h := NewHandler(svc, convSvc)

	rg.GET("/conversations/:conversationId/messages", h.List)
}

// RegisterProtected mounts the write endpoints. The caller is responsible for
// attaching the auth middleware to rg.
func RegisterProtected(rg *gin.RouterGroup, db *gorm.DB) {
	svc := NewService(db)
	convSvc := conversations.NewService(db)
	h := NewHandler(svc, convSvc)

	rg.PATCH("/conversations/:conversationId/read", h.MarkRead)
}
