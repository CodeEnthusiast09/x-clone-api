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
	// bot heuristics. Mount before any Arcjet-protected sub-group so this
	// path inherits no Arcjet middleware.
	webhooks.Register(api, db, cfg.ClerkWebhookSecret)

	// Three rate-limit tiers, each derived from the base Arcjet client by
	// adding a TokenBucket rule keyed on ip.src. One Arcjet RPC per request.
	publicLimit, err := middleware.RateLimit(ajClient, cfg.ArcjetPublicRPM)
	if err != nil {
		log.Fatalf("arcjet public rate-limit init: %v", err)
	}
	authLimit, err := middleware.RateLimit(ajClient, cfg.ArcjetAuthRPM)
	if err != nil {
		log.Fatalf("arcjet auth rate-limit init: %v", err)
	}
	writeLimit, err := middleware.RateLimit(ajClient, cfg.ArcjetWriteRPM)
	if err != nil {
		log.Fatalf("arcjet write rate-limit init: %v", err)
	}

	// Public reads — unauthenticated GETs. Highest budget.
	publicReads := api.Group("", publicLimit)
	users.Register(publicReads, db)
	posts.Register(publicReads, db)
	posts.RegisterUnderUsers(publicReads, db)
	comments.Register(publicReads, db)

	// Authed reads / lightweight profile actions — require Clerk JWT.
	authedReads := api.Group("", authLimit, middleware.RequireAuth())
	users.RegisterProtected(authedReads, db)

	// Authed writes — every mutation goes here. Lowest budget.
	authedWrites := api.Group("", writeLimit, middleware.RequireAuth())
	uploadsignatures.RegisterProtected(authedWrites, cdn, cfg.CloudinaryUploadPreset, []uploadsignatures.Mount{
		{Path: "/upload-signatures/posts", Namespace: posts.PostImageNamespace, MaxBytes: cfg.PostImageMaxBytes},
		{Path: "/upload-signatures/banners", Namespace: users.BannerImageNamespace, MaxBytes: cfg.BannerImageMaxBytes},
	})
	posts.RegisterProtected(authedWrites, db, cdn)
	comments.RegisterUnderPosts(authedWrites, db)
	comments.RegisterProtected(authedWrites, db)
	follows.RegisterProtected(authedWrites, db)

	return r
}
