package follows

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// RegisterProtected mounts the follow toggle endpoints under /users.
// The caller is responsible for attaching the auth middleware to rg.
func RegisterProtected(rg *gin.RouterGroup, db *gorm.DB) {
	svc := NewService(db)
	h := NewHandler(svc)

	g := rg.Group("/users")
	{
		g.POST("/:username/follow", h.Follow)
		g.DELETE("/:username/follow", h.Unfollow)
	}
}
