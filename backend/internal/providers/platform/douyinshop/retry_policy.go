package douyinshop

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"net/http"
	"strings"
	"time"
)

// RetryPolicy controls automatic HTTP retry for Douyin OpenAPI calls.
type RetryPolicy struct {
	MaxAttempts    int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
	JitterRatio    float64
}

// RetryDecision describes whether and how long to wait before another attempt.
type RetryDecision struct {
	Retryable bool
	Reason    string
	Delay     time.Duration
}

// DefaultRetryPolicy returns the standard Douyin retry policy (max 3 attempts).
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxAttempts:    3,
		InitialBackoff: 500 * time.Millisecond,
		MaxBackoff:     8 * time.Second,
		JitterRatio:    0.2,
	}
}

// EvaluateRetry decides if err is retryable at the given attempt number.
func EvaluateRetry(err error, attempt int, policy RetryPolicy, retryAfter time.Duration) RetryDecision {
	if err == nil {
		return RetryDecision{}
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		if errors.Is(err, context.Canceled) {
			return RetryDecision{Reason: "context cancelled"}
		}
	}
	max := policy.MaxAttempts
	if max <= 0 {
		max = 3
	}
	if attempt >= max {
		return RetryDecision{Reason: "max attempts reached"}
	}

	var de *Error
	if errors.As(err, &de) {
		if de.PermissionDenied || de.AuthExpired {
			return RetryDecision{Reason: "permission or auth error"}
		}
		if !de.Retryable && !de.RateLimited {
			return RetryDecision{Reason: "non-retryable platform error: " + de.Code}
		}
		if de.RateLimited || de.Retryable {
			delay := retryAfter
			if delay <= 0 {
				delay = backoffDelay(attempt, policy)
			}
			reason := "rate limited"
			if !de.RateLimited {
				reason = "retryable platform error"
			}
			return RetryDecision{Retryable: true, Reason: reason, Delay: delay}
		}
	}

	// Transport / unknown errors: retry only timeouts and connection failures.
	if isTransportRetryable(err) {
		return RetryDecision{
			Retryable: true,
			Reason:    "transport error",
			Delay:     backoffDelay(attempt, policy),
		}
	}
	return RetryDecision{Reason: "non-retryable error"}
}

func isTransportRetryable(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var de *Error
	if errors.As(err, &de) {
		switch de.Code {
		case CodeDouyinRequestTimeout, CodeDouyinRateLimited,
			CodeDouyinOrderRateLimited, CodeDouyinInventoryRateLimited:
			return true
		}
		if de.Retryable || de.RateLimited {
			return true
		}
		return false
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "timeout") || strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "connection reset") || strings.Contains(msg, "temporary failure") ||
		strings.Contains(msg, "no such host") && strings.Contains(msg, "i/o") {
		return true
	}
	return false
}

func backoffDelay(attempt int, policy RetryPolicy) time.Duration {
	initial := policy.InitialBackoff
	if initial <= 0 {
		initial = 500 * time.Millisecond
	}
	maxB := policy.MaxBackoff
	if maxB <= 0 {
		maxB = 8 * time.Second
	}
	exp := float64(initial) * math.Pow(2, float64(attempt-1))
	if exp > float64(maxB) {
		exp = float64(maxB)
	}
	delay := time.Duration(exp)
	jitter := policy.JitterRatio
	if jitter > 0 {
		spread := float64(delay) * jitter
		delay = time.Duration(float64(delay) + (rand.Float64()*2-1)*spread)
		if delay < 0 {
			delay = initial
		}
	}
	return delay
}

// ExecuteWithRetry runs operation up to policy.MaxAttempts times.
// operation receives the 1-based attempt number. Returns final attempt count and error.
func ExecuteWithRetry(ctx context.Context, policy RetryPolicy, operation func(context.Context, int) error) (int, error) {
	max := policy.MaxAttempts
	if max <= 0 {
		max = 3
	}
	var lastErr error
	for attempt := 1; attempt <= max; attempt++ {
		if ctx.Err() != nil {
			return attempt, ctx.Err()
		}
		lastErr = operation(ctx, attempt)
		if lastErr == nil {
			return attempt, nil
		}
		var retryAfter time.Duration
		decision := EvaluateRetry(lastErr, attempt, policy, retryAfter)
		if !decision.Retryable || attempt >= max {
			return attempt, lastErr
		}
		if decision.Delay > 0 {
			timer := time.NewTimer(decision.Delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return attempt, ctx.Err()
			case <-timer.C:
			}
		}
	}
	return max, lastErr
}

// HTTPStatusRetryable reports whether an HTTP status should trigger retry.
func HTTPStatusRetryable(status int) bool {
	switch status {
	case http.StatusTooManyRequests,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout,
		http.StatusRequestTimeout:
		return true
	default:
		return false
	}
}
