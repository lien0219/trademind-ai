package middleware

import (
	"log/slog"
	"runtime/debug"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

// Recovery catches panics and returns the unified JSON error body.
func Recovery(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if rec := recover(); rec != nil {
				if log != nil {
					log.Error("panic recovered",
						"recover", rec,
						"path", c.Request.URL.Path,
						"stack", string(debug.Stack()),
					)
				}
				response.Fail(c, 500, response.CodeInternalError, "internal error")
				c.Abort()
			}
		}()
		c.Next()
	}
}
