package conversations

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Register mounts the read endpoints. The caller is responsible for attaching
// the auth middleware to rg.
func Register(rg *gin.RouterGroup, db *gorm.DB) {
	svc := NewService(db)
	h := NewHandler(svc)

	rg.GET("/conversations", h.List)
}

// RegisterProtected mounts the write endpoints. The caller is responsible for
// attaching the auth middleware to rg.
func RegisterProtected(rg *gin.RouterGroup, db *gorm.DB) {
	svc := NewService(db)
	h := NewHandler(svc)

	rg.POST("/conversations", h.StartOrGet)
}
