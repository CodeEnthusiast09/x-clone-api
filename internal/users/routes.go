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
