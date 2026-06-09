package notifications

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func Register(rg *gin.RouterGroup, db *gorm.DB) {
	svc := NewService(db)
	h := NewHandler(svc)
	rg.GET("/notifications", h.List)
}

func RegisterProtected(rg *gin.RouterGroup, db *gorm.DB) {
	svc := NewService(db)
	h := NewHandler(svc)
	rg.PATCH("/notifications/read", h.MarkAllRead)
	rg.POST("/push-token", h.RegisterPushToken)
	rg.DELETE("/push-token", h.UnregisterPushToken)
}
