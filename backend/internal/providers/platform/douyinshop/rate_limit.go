package douyinshop

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ParseRetryAfter parses HTTP Retry-After header (seconds or HTTP-date).
func ParseRetryAfter(header string) time.Duration {
	header = strings.TrimSpace(header)
	if header == "" {
		return 0
	}
	if sec, err := strconv.Atoi(header); err == nil && sec > 0 {
		return time.Duration(sec) * time.Second
	}
	if t, err := http.ParseTime(header); err == nil {
		d := time.Until(t)
		if d > 0 {
			return d
		}
	}
	return 0
}

// RateLimitDelayFromError extracts retry delay from a Douyin error if rate-limited.
func RateLimitDelayFromError(err error) (time.Duration, bool) {
	var de *Error
	if AsError(err, &de) && de != nil {
		if de.RateLimited {
			return 2 * time.Second, true
		}
	}
	return 0, false
}
