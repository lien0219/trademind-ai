package ocrerror

import (
	"fmt"
	"strings"
)

const (
	CodeSecretMissing        = "SECRET_MISSING"
	CodeAuthFailed           = "AUTH_FAILED"
	CodeServiceNotOpen       = "SERVICE_NOT_OPEN"
	CodePermissionDenied     = "PERMISSION_DENIED"
	CodeImageURLInaccessible = "IMAGE_URL_NOT_ACCESSIBLE"
	CodeTimeout              = "TIMEOUT"
	CodeEmptyBlocks          = "EMPTY_BLOCKS"
	CodeRateLimited          = "RATE_LIMITED"
	CodeUnknown              = "UNKNOWN"
)

type Error struct {
	Code      string
	Message   string
	RequestID string
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	code := strings.TrimSpace(e.Code)
	msg := strings.TrimSpace(e.Message)
	if code == "" {
		return msg
	}
	if e.RequestID != "" {
		return fmt.Sprintf("%s: %s (requestId=%s)", code, msg, e.RequestID)
	}
	return fmt.Sprintf("%s: %s", code, msg)
}

func New(code, message string) *Error {
	return &Error{Code: strings.TrimSpace(code), Message: strings.TrimSpace(message)}
}

func NewWithRequestID(code, message, requestID string) *Error {
	return &Error{Code: strings.TrimSpace(code), Message: strings.TrimSpace(message), RequestID: strings.TrimSpace(requestID)}
}
