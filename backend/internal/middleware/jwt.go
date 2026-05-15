package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/modules/auth"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

// BearerAuth enforces a valid JWT and stores admin id in context.
func BearerAuth(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if cfg == nil {
			response.Fail(c, 500, response.CodeInternalError, "misconfigured")
			c.Abort()
			return
		}
		h := strings.TrimSpace(c.GetHeader("Authorization"))
		parts := strings.SplitN(h, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
			response.Fail(c, 401, response.CodeUnauthorized, "missing bearer token")
			c.Abort()
			return
		}
		raw := strings.TrimSpace(parts[1])
		claims, err := auth.ParseToken(cfg, raw)
		if err != nil {
			response.Fail(c, 401, response.CodeUnauthorized, "invalid token")
			c.Abort()
			return
		}
		c.Set(ctxkey.AdminID, claims.Subject)
		c.Next()
	}
}
