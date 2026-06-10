package posts

import (
	"github.com/CodeEnthusiast09/x-clone-api/internal/cloudinary"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func Register(rg *gin.RouterGroup, db *gorm.DB) {
	svc := NewService(db)
	h := NewHandler(svc, nil)

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
	h := NewHandler(svc, nil)

	g := rg.Group("/users")
	g.GET("/:username/posts", h.ListByUsername)
}

// RegisterProtected mounts the post write endpoints. The caller is responsible
// for attaching the auth middleware to rg.
func RegisterProtected(rg *gin.RouterGroup, db *gorm.DB, cdn *cloudinary.Client) {
	svc := NewService(db)
	h := NewHandler(svc, cdn)

	g := rg.Group("/posts")
	{
		g.POST("", h.Create)
		g.DELETE("/:postId", h.Delete)
		g.POST("/:postId/likes", h.Like)
		g.DELETE("/:postId/likes", h.Unlike)
		g.POST("/:postId/repost", h.Repost)
		g.DELETE("/:postId/repost", h.UnRepost)
	}
}
