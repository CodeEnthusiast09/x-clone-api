package users

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func Register(rg *gin.RouterGroup, db *gorm.DB) {
	svc := NewService(db)
	h := NewHandler(svc)

	g := rg.Group("/users")
	{
		g.GET("/:username", h.GetByUsername)
	}
}

// RegisterProtected registers routes that require an authenticated Clerk session.
// The caller is responsible for attaching the auth middleware to rg.
func RegisterProtected(rg *gin.RouterGroup, db *gorm.DB) {
	svc := NewService(db)
	h := NewHandler(svc)

	rg.GET("/me", h.Me)
	rg.POST("/auth/sync", h.Sync)
}
