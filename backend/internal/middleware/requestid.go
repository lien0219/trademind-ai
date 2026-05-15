package middleware

import (
	"strings"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/gin-gonic/gin"
)

// RequestID ensures each request has a trace id (reuses inbound header when present).
// Values are always canonical UUID strings to match backend ID conventions.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := normalizeTraceID(c.GetHeader(TraceIDHeader))
		c.Writer.Header().Set(TraceIDHeader, id)
		c.Set(ctxkey.TraceID, id)
		c.Next()
	}
}

func normalizeTraceID(header string) string {
	s := strings.TrimSpace(header)
	if s == "" {
		return uuid.New().String()
	}
	if _, err := uuid.Parse(s); err == nil {
		return s
	}
	return uuid.New().String()
}
