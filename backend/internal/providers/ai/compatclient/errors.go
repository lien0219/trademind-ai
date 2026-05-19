package compatclient

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
)

// HTTPError is returned when the remote API responds with a non-2xx status.
type HTTPError struct {
	StatusCode int
	Body       []byte
}

func (e *HTTPError) Error() string {
	if e == nil {
		return "chat completions HTTP error"
	}
	msg := strings.TrimSpace(APIErrorMessage(e.Body))
	if msg != "" {
		return fmt.Sprintf("HTTP %d: %s", e.StatusCode, msg)
	}
	return fmt.Sprintf("HTTP %d", e.StatusCode)
}

// APIErrorMessage extracts a human-readable message from an OpenAI-style error body.
func APIErrorMessage(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	var wrap struct {
		Error struct {
			Message string `json:"message"`
			Code    string `json:"code"`
			Type    string `json:"type"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &wrap); err == nil {
		if m := strings.TrimSpace(wrap.Error.Message); m != "" {
			return m
		}
	}
	var flat struct {
		Message string `json:"message"`
		Code    string `json:"code"`
	}
	if err := json.Unmarshal(body, &flat); err == nil {
		return strings.TrimSpace(flat.Message)
	}
	return ""
}

// IsInvalidModel reports whether the error body or message suggests an unknown/forbidden model.
func IsInvalidModel(body []byte, errMsg string) bool {
	joined := strings.ToLower(APIErrorMessage(body) + " " + errMsg)
	for _, kw := range []string{
		"model", "model_not_found", "invalid_model", "does not exist",
		"not found", "not exist", "no permission", "permission",
		"model.access", "model_access",
	} {
		if strings.Contains(joined, kw) {
			if strings.Contains(joined, "model") || strings.Contains(joined, "permission") {
				return true
			}
		}
	}
	return strings.Contains(joined, "model_not_found") ||
		strings.Contains(joined, "invalid_model") ||
		strings.Contains(joined, "model.access")
}

// IsTimeout reports network/deadline timeouts.
func IsTimeout(err error) bool {
	if err == nil {
		return false
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline exceeded")
}
