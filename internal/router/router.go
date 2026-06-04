package router

import (
	"net/http"
	"time"

	"github.com/CodeEnthusiast09/x-clone-api/internal/comments"
	"github.com/CodeEnthusiast09/x-clone-api/internal/common"
	"github.com/CodeEnthusiast09/x-clone-api/internal/config"
	"github.com/CodeEnthusiast09/x-clone-api/internal/middleware"
	"github.com/CodeEnthusiast09/x-clone-api/internal/posts"
	"github.com/CodeEnthusiast09/x-clone-api/internal/users"
	"github.com/CodeEnthusiast09/x-clone-api/internal/webhooks"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func New(cfg *config.Config, db *gorm.DB) *gin.Engine {
	if cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.Default()

	startedAt := time.Now()

	r.GET("/health", func(c *gin.Context) {
		common.Success(c, http.StatusOK, "ok", gin.H{
			"uptime":      time.Since(startedAt).String(),
			"environment": cfg.Env,
		})
	})

	api := r.Group("/api")

	// Public routes — read endpoints and webhooks (webhook auth happens via Svix signature).
	users.Register(api, db)
	posts.Register(api, db)
	posts.RegisterUnderUsers(api, db)
	comments.Register(api, db)
	webhooks.Register(api, db, cfg.ClerkWebhookSecret)

	// Protected routes — require a valid Clerk JWT.
	protected := api.Group("", middleware.RequireAuth())
	users.RegisterProtected(protected, db)

	return r
}
