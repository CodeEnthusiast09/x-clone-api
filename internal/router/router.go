package router

import (
	"net/http"
	"time"

	"github.com/CodeEnthusiast09/x-clone-api/internal/common"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func New(env string, _ *gorm.DB) *gin.Engine {
	if env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.Default()

	startedAt := time.Now()

	r.GET("/health", func(c *gin.Context) {
		common.Success(c, http.StatusOK, "ok", gin.H{
			"uptime":      time.Since(startedAt).String(),
			"environment": env,
		})
	})

	return r
}
