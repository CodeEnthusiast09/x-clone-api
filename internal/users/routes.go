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

	// Static /users/me routes must be registered before /users/:username so Gin
	// matches them as exact paths rather than routing to GetByUsername.
	g := rg.Group("/users")
	{
		g.GET("/me", h.Me)
		g.PATCH("/me", h.UpdateMe)
	}

	rg.POST("/auth/sync", h.Sync)
}
