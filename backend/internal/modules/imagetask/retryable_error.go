package imagetask

import (
	"context"
	"errors"
	"net"
	"strconv"
	"strings"
)

// ErrWorkerLeaseExpired is used when a DB lease times out before work completes.
var ErrWorkerLeaseExpired = errors.New("worker lease expired")

// IsRetryableImageTaskError classifies provider / transport failures for automatic backoff.
// Unknown errors default to non-retryable to avoid infinite loops on configuration mistakes.
func IsRetryableImageTaskError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrWorkerLeaseExpired) {
		return true
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return true
	}

	msg := strings.ToLower(err.Error())
	if msg == "" {
		return false
	}
	// Prefer transient HTTP signals before broad "not implemented" wording (e.g. HTTP 501).
	if imageTaskErrHTTPRetryable(msg) {
		return true
	}
	if imageTaskErrDefinitelyNonRetryable(msg) {
		return false
	}
	if imageTaskErrTransportRetryable(msg) {
		return true
	}
	return false
}

func imageTaskErrDefinitelyNonRetryable(msg string) bool {
	patterns := []string{
		"api key not configured",
		"api_key is not configured",
		"requires settings service",
		"unsupported image provider",
		"invalid tasktype",
		"invalid task type",
		"unknown task type",
		"sourceimageid or sourceimageurl required",
		"source image exceeds maximum",
		"sourceimageid not found",
		"source image is not readable and not publicly accessible",
		"unsupported storage provider",
		"invalid object key",
		"file not found",
		"file has no public url",
		"product image has no url",
		"not publicly accessible",
		"workflow_json",
		"workflow json",
		"invalid json",
		"not valid json",
		"parsed to an empty object",
		"still contains unreplaced placeholders",
		"workflow validation failed",
		"node_errors",
		"not found in workflow",
		"is not an object",
		"assembled prompt required",
		"empty prompt",
		"no image data returned",
		"empty image data",
		"neither url nor b64_json",
		"invalid base64",
		"decode response json",
		"assembled prompt required for openai_image replace_background",
		"source image is required for replace_background",
		"source image is required and must be readable for openai replace_background",
		"remove_background is not implemented",
		"noop:",
		"illegal",
		"invalid parameter",
		"moderation",
		"content policy",
		"policy",
		"blocked",
		"unsupported image",
		"image format",
		"invalid image",
		"status 400",
		"status 401",
		"status 403",
		"status 404",
		"status 422",
		"comfyui execution failed",
		"not implemented",
	}
	for _, p := range patterns {
		if strings.Contains(msg, p) {
			return true
		}
	}
	// Misconfiguration: unset endpoints / workflow (avoid infinite retry)
	if strings.Contains(msg, "is not configured") {
		return true
	}
	if strings.Contains(msg, "comfyui_output_node_id") || strings.Contains(msg, "comfyui_image_node_id") || strings.Contains(msg, "comfyui_prompt_node_id") {
		return true
	}
	return false
}

func imageTaskErrHTTPRetryable(msg string) bool {
	if strings.Contains(msg, "429") || strings.Contains(msg, "rate limit") || strings.Contains(msg, "too many requests") {
		return true
	}
	for code := 500; code <= 599; code++ {
		s := strconv.Itoa(code)
		if strings.Contains(msg, "status "+s) {
			return true
		}
		if strings.Contains(msg, "http "+s) {
			return true
		}
		if strings.Contains(msg, "http: "+s) {
			return true
		}
	}
	return false
}

func imageTaskErrTransportRetryable(msg string) bool {
	patterns := []string{
		"timeout",
		"deadline exceeded",
		"connection refused",
		"connection reset",
		"broken pipe",
		"wsarecv",
		"eof",
		"tls handshake",
		"temporary failure",
		"no such host",
		"i/o timeout",
		"client timeout",
		"server closed",
		"unexpected eof",
		"timed out waiting for prompt",
		"service unavailable",
	}
	for _, p := range patterns {
		if strings.Contains(msg, p) {
			return true
		}
	}
	return false
}
