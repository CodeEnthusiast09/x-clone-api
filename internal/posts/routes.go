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
		g.GET("/user/:username", h.ListByUsername)
	}
}
