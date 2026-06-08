package comments

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Register mounts the public read endpoints.
func Register(rg *gin.RouterGroup, db *gorm.DB) {
	svc := NewService(db)
	h := NewHandler(svc)

	g := rg.Group("/comments")
	{
		g.GET("/post/:postId", h.ListByPost)
	}
}

// RegisterUnderPosts mounts comment write endpoints that are scoped to a specific post.
// Lives under the /posts prefix so URLs read naturally (e.g. POST /posts/:postId/comments).
// The caller is responsible for attaching the auth middleware to rg.
func RegisterUnderPosts(rg *gin.RouterGroup, db *gorm.DB) {
	svc := NewService(db)
	h := NewHandler(svc)

	g := rg.Group("/posts")
	g.POST("/:postId/comments", h.Create)
}

// RegisterProtected mounts comment write endpoints that operate on a comment id directly.
// The caller is responsible for attaching the auth middleware to rg.
func RegisterProtected(rg *gin.RouterGroup, db *gorm.DB) {
	svc := NewService(db)
	h := NewHandler(svc)

	g := rg.Group("/comments")
	g.DELETE("/:commentId", h.Delete)
}
