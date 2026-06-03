package webhooks

import (
	"github.com/CodeEnthusiast09/x-clone-api/internal/users"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func Register(rg *gin.RouterGroup, db *gorm.DB, clerkWebhookSecret string) {
	usersSvc := users.NewService(db)
	clerk := NewClerkHandler(usersSvc, clerkWebhookSecret)

	g := rg.Group("/webhooks")
	{
		g.POST("/clerk", clerk.Handle)
	}
}
