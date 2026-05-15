package middleware

import (
	"log/slog"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/gin-gonic/gin"
)

// AccessLog emits one structured line per request (after handlers run).
func AccessLog(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		c.Next()
		if log == nil {
			return
		}
		tid, _ := c.Get(ctxkey.TraceID)
		log.Info("http_request",
			"trace_id", tid,
			"method", c.Request.Method,
			"path", path,
			"status", c.Writer.Status(),
			"latency_ms", time.Since(start).Milliseconds(),
			"client_ip", c.ClientIP(),
		)
	}
}
