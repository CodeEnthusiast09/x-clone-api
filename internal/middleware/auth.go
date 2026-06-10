package middleware

import (
	"net/http"
	"strings"

	"github.com/CodeEnthusiast09/x-clone-api/internal/common"
	"github.com/clerk/clerk-sdk-go/v2/jwt"
	"github.com/gin-gonic/gin"
)

const (
	ContextClerkID = "clerkID"
)

// RequireAuth verifies the Clerk-issued JWT.
// Primary: Authorization: Bearer <token> header.
// Fallback: ?token= query parameter — used by WebSocket clients that cannot
// set request headers during the HTTP upgrade handshake (e.g. Android).
// On success, the verified clerk_id is stored on the Gin context under ContextClerkID.
func RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractBearerToken(c.GetHeader("Authorization"))
		if token == "" && c.GetHeader("Upgrade") == "websocket" {
			// WebSocket upgrade requests cannot set Authorization headers on
			// some Android RN versions — accept the token as a query param
			// only for WS handshakes, never for plain HTTP requests.
			token = c.Query("token")
		}
		if token == "" {
			common.Error(c, http.StatusUnauthorized, "missing or malformed authorization header")
			c.Abort()
			return
		}

		claims, err := jwt.Verify(c.Request.Context(), &jwt.VerifyParams{
			Token: token,
		})
		if err != nil {
			common.Error(c, http.StatusUnauthorized, "invalid or expired token")
			c.Abort()
			return
		}

		c.Set(ContextClerkID, claims.Subject)
		c.Next()
	}
}

func extractBearerToken(header string) string {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return ""
	}
	return strings.TrimSpace(header[len(prefix):])
}
