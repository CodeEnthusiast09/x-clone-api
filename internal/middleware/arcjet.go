package middleware

import (
	"log"
	"net/http"

	"github.com/CodeEnthusiast09/x-clone-api/internal/common"
	"github.com/arcjet/arcjet-go"
	"github.com/gin-gonic/gin"
)

// NewArcjetClient builds the base Arcjet client used by every protected route.
// Shield always runs in Live mode -- it just blocks malicious request patterns
// (SQLi, XSS, etc.) and there's no reason to dry-run that. Bot detection mode
// follows the environment: DryRun in development (so curl/Postman/test scripts
// get logged-but-allowed) and Live in production (blocks automated callers).
func NewArcjetClient(key, env string) (*arcjet.Client, error) {
	botMode := arcjet.ModeLive
	if env != "production" {
		botMode = arcjet.ModeDryRun
	}

	return arcjet.NewClient(arcjet.Config{
		Key: key,
		Rules: []arcjet.Rule{
			arcjet.Shield(arcjet.ShieldOptions{Mode: arcjet.ModeLive}),
			arcjet.DetectBot(arcjet.BotOptions{
				Mode: botMode,
				// Empty Allow blocks every detected bot when Mode == Live.
				// In DryRun (dev), denials are logged on the Arcjet dashboard
				// but the request still goes through.
				Allow: []string{},
			}),
		},
	})
}

// Arcjet returns a gin.HandlerFunc that evaluates the request against the
// configured rules. Fails open on Arcjet errors -- per Arcjet's own guidance,
// a security middleware should not take down a working API when the upstream
// decide service blips.
func Arcjet(client *arcjet.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		decision, err := client.Protect(c.Request.Context(), c.Request)
		if err != nil {
			log.Printf("arcjet protect error (fail-open): %v", err)
			c.Next()
			return
		}

		// Surface RateLimit-* response headers regardless of decision so
		// well-behaved clients can self-throttle before hitting 429.
		arcjet.SetRateLimitHeaders(c.Writer, decision)

		if !decision.IsDenied() {
			c.Next()
			return
		}

		status := http.StatusForbidden
		msg := "request denied"
		switch {
		case decision.Reason.IsRateLimit():
			status = http.StatusTooManyRequests
			msg = "rate limit exceeded"
		case decision.Reason.IsBot():
			msg = "automated requests are not allowed"
		case decision.Reason.IsShield():
			msg = "request failed security check"
		}

		log.Printf("arcjet denied: id=%s ip=%s reason=%s", decision.ID, c.ClientIP(), decision.Reason.Message)
		common.Error(c, status, msg)
		c.Abort()
	}
}
