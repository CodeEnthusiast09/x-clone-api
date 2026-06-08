package router

import (
	"log"
	"net/http"
	"time"

	"github.com/CodeEnthusiast09/x-clone-api/internal/cloudinary"
	"github.com/CodeEnthusiast09/x-clone-api/internal/comments"
	"github.com/CodeEnthusiast09/x-clone-api/internal/common"
	"github.com/CodeEnthusiast09/x-clone-api/internal/config"
	"github.com/CodeEnthusiast09/x-clone-api/internal/follows"
	"github.com/CodeEnthusiast09/x-clone-api/internal/middleware"
	"github.com/CodeEnthusiast09/x-clone-api/internal/posts"
	"github.com/CodeEnthusiast09/x-clone-api/internal/uploadsignatures"
	"github.com/CodeEnthusiast09/x-clone-api/internal/users"
	"github.com/CodeEnthusiast09/x-clone-api/internal/webhooks"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func New(cfg *config.Config, db *gorm.DB, cdn *cloudinary.Client) *gin.Engine {
	if cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	ajClient, err := middleware.NewArcjetClient(cfg.ArcjetKey, cfg.Env)
	if err != nil {
		log.Fatalf("arcjet client init: %v", err)
	}

	r := gin.Default()

	startedAt := time.Now()

	// /health stays outside the Arcjet chain so liveness probes always pass.
	r.GET("/health", func(c *gin.Context) {
		common.Success(c, http.StatusOK, "ok", gin.H{
			"uptime":      time.Since(startedAt).String(),
			"environment": cfg.Env,
		})
	})

	api := r.Group("/api")

	// Webhooks are NOT behind Arcjet -- Svix verifies the signature itself,
	// and we want Clerk's high-volume callbacks to never trip rate limits or
	// bot heuristics. Mount before the Arcjet-protected sub-group so this
	// group inherits no Arcjet middleware.
	webhooks.Register(api, db, cfg.ClerkWebhookSecret)

	// Everything else flows through Arcjet first.
	guarded := api.Group("", middleware.Arcjet(ajClient))

	// Public routes — read endpoints, no Clerk auth.
	users.Register(guarded, db)
	posts.Register(guarded, db)
	posts.RegisterUnderUsers(guarded, db)
	comments.Register(guarded, db)

	// Protected routes — require a valid Clerk JWT. Arcjet middleware
	// inherited from the parent guarded group runs before RequireAuth.
	protected := guarded.Group("", middleware.RequireAuth())
	users.RegisterProtected(protected, db)
	uploadsignatures.RegisterProtected(protected, cdn, cfg.CloudinaryUploadPreset, []uploadsignatures.Mount{
		{Path: "/upload-signatures/posts", Namespace: posts.PostImageNamespace, MaxBytes: cfg.PostImageMaxBytes},
		{Path: "/upload-signatures/banners", Namespace: users.BannerImageNamespace, MaxBytes: cfg.BannerImageMaxBytes},
	})
	posts.RegisterProtected(protected, db, cdn)
	comments.RegisterUnderPosts(protected, db)
	comments.RegisterProtected(protected, db)
	follows.RegisterProtected(protected, db)

	return r
}
