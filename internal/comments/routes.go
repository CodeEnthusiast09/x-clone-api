package comments

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func Register(rg *gin.RouterGroup, db *gorm.DB) {
	svc := NewService(db)
	h := NewHandler(svc)

	g := rg.Group("/comments")
	{
		g.GET("/post/:postId", h.ListByPost)
	}
}
