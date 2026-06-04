package posts

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func Register(rg *gin.RouterGroup, db *gorm.DB) {
	svc := NewService(db)
	h := NewHandler(svc)

	g := rg.Group("/posts")
	{
		g.GET("", h.List)
		g.GET("/:postId", h.GetByID)
	}
}

// RegisterUnderUsers mounts post routes that are scoped to a specific user.
// Lives under the /users prefix so URLs read naturally (e.g. /users/alice/posts).
func RegisterUnderUsers(rg *gin.RouterGroup, db *gorm.DB) {
	svc := NewService(db)
	h := NewHandler(svc)

	g := rg.Group("/users")
	g.GET("/:username/posts", h.ListByUsername)
}
