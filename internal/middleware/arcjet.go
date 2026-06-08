package middleware

import (
	"log"
	"net/http"
	"time"

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

// RateLimit derives a tier-specific Arcjet client from the base by adding a
// token-bucket rule keyed on the caller's IP, then returns a gin.HandlerFunc
// that runs Protect against the derived client. One Arcjet RPC per request --
// the bucket rule is composed with Shield + DetectBot from the base.
//
// rpm = sustained requests per minute. Capacity is set to rpm so a fresh
// bucket allows a small burst up to the same value before refill throttling
// kicks in.
func RateLimit(base *arcjet.Client, rpm int) (gin.HandlerFunc, error) {
	// Characteristics is intentionally omitted -- per the Arcjet Go SDK
	// README, omitting it defaults to IP-based bucketing (the SDK auto-
	// derives the source IP from the request). Setting it to
	// []string{"ip.src"} makes Arcjet wait for a per-request value supplied
	// via WithCharacteristics(), which we never provide, so the bucket
	// silently never keys -> no enforcement.
	tierClient, err := base.WithRule(arcjet.TokenBucket(arcjet.TokenBucketOptions{
		Mode:       arcjet.ModeLive,
		RefillRate: rpm,
		Interval:   time.Minute,
		Capacity:   rpm,
	}))
	if err != nil {
		return nil, err
	}
	return protectWith(tierClient), nil
}

// protectWith is the shared decision pipeline used by every Arcjet-backed
// middleware. Fails open on Arcjet errors -- per Arcjet's own guidance, a
// security middleware should not take down a working API when the upstream
// decide service blips.
func protectWith(client *arcjet.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		// WithRequested(1) -- TokenBucket requires the per-request token
		// cost on every Protect call. Without it Arcjet returns an ERROR
		// conclusion and the bucket is never decremented (silent fail-open).
		decision, err := client.Protect(c.Request.Context(), c.Request, arcjet.WithRequested(1))
		if err != nil {
			log.Printf("arcjet protect error (fail-open): %v", err)
			c.Next()
			return
		}

		// Surface RateLimit-* response headers regardless of decision so
		// well-behaved clients can self-throttle before hitting 429.
		arcjet.SetRateLimitHeaders(c.Writer, decision)

		// ERROR conclusions mean Arcjet couldn't evaluate the rules (bad
		// config, transient upstream issue, etc.). Log loudly and fail open
		// rather than swallow it like a normal allow.
		if decision.IsErrored() {
			log.Printf("arcjet decision errored (fail-open): id=%s reason=%s", decision.ID, decision.Reason.Message)
			c.Next()
			return
		}

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
