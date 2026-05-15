package middleware

import "github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"

const (
	// TraceIDHeader is the inbound/outbound request correlation header.
	TraceIDHeader = "X-Request-ID"
	// TraceIDContextKey matches ctxkey.TraceID; keep alias for handler ergonomics.
	TraceIDContextKey = ctxkey.TraceID
)
