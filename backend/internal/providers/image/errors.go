package image

import (
	"errors"
	"fmt"
	"strings"
)

// Sentinel errors for consistent Chinese messages (wrap underlying cause when needed).
var (
	ErrConfigIncomplete = errors.New("image provider config incomplete: please configure settings.image first")
	ErrAPIKeyMissing    = errors.New("image provider api key missing")
	ErrRequestTimeout   = errors.New("image provider request timeout")
	ErrPermissionDenied = errors.New("image provider permission denied or quota exceeded")
	ErrResponseInvalid  = errors.New("image provider response invalid")
	ErrTaskNotSupported = errors.New("image provider task not supported")
)

// WrapProviderError maps HTTP/SDK errors to user-facing messages without leaking keys.
func WrapProviderError(err error) error {
	if err == nil {
		return nil
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline"):
		return fmt.Errorf("%w: %v", ErrRequestTimeout, err)
	case strings.Contains(msg, "401") || strings.Contains(msg, "403") ||
		strings.Contains(msg, "permission") || strings.Contains(msg, "quota") ||
		strings.Contains(msg, "insufficient"):
		return fmt.Errorf("%w: %v", ErrPermissionDenied, err)
	case strings.Contains(msg, "not configured") || strings.Contains(msg, "api key") || strings.Contains(msg, "api_key"):
		return fmt.Errorf("%w: %v", ErrAPIKeyMissing, err)
	case strings.Contains(msg, "not implemented") || strings.Contains(msg, "not supported"):
		return fmt.Errorf("%w: %v", ErrTaskNotSupported, err)
	case strings.Contains(msg, "decode") || strings.Contains(msg, "invalid") || strings.Contains(msg, "empty image"):
		return fmt.Errorf("%w: %v", ErrResponseInvalid, err)
	default:
		return err
	}
}
