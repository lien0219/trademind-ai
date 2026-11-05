package operationlog

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

// AuditFailures records rejected JSON mutation requests after any business
// transaction has rolled back. Successful mutations must use WriteTx. Buffering
// the response lets an audit failure return a clear error before sending data.
// Request bodies and handler error messages are never copied into audit rows.
func (s *Service) AuditFailures(action, resource, resourceParam string) gin.HandlerFunc {
	return func(c *gin.Context) {
		original := c.Writer
		captured := &auditResponse{ResponseWriter: original, status: http.StatusOK}
		c.Writer = captured
		defer func() { c.Writer = original }()
		c.Next()
		c.Writer = original
		if captured.status >= http.StatusBadRequest {
			status := "failed"
			if captured.status == http.StatusForbidden || captured.status == http.StatusUnauthorized {
				status = "denied"
			}
			var err error
			if s == nil || s.DB == nil {
				err = fmt.Errorf("operationlog: failure audit unavailable")
			} else {
				err = s.Write(c, WriteOpts{
					Action: action, Resource: resource, ResourceID: truncateRunes(strings.TrimSpace(c.Param(resourceParam)), 128),
					Status: status, Message: fmt.Sprintf("http_status=%d", captured.status),
				})
			}
			if err != nil {
				c.Abort()
				c.Header("Content-Length", "")
				response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "审计记录暂时无法保存，请稍后重试")
				return
			}
		}
		original.WriteHeader(captured.status)
		if captured.written {
			original.WriteHeaderNow()
			// Network write errors are recorded by the HTTP server. Mutation and
			// audit transactions have already completed and cannot be retried here.
			_, _ = original.Write(captured.body.Bytes())
		}
	}
}

// auditResponse is limited to the JSON mutation routes using AuditFailures;
// streaming and upgraded connections must not use this middleware.
type auditResponse struct {
	gin.ResponseWriter
	body    bytes.Buffer
	status  int
	written bool
}

func (w *auditResponse) WriteHeader(status int) {
	if !w.written {
		w.status = status
	}
}
func (w *auditResponse) WriteHeaderNow() { w.written = true }
func (w *auditResponse) Write(body []byte) (int, error) {
	w.written = true
	return w.body.Write(body)
}
func (w *auditResponse) WriteString(body string) (int, error) {
	w.written = true
	return w.body.WriteString(body)
}
func (w *auditResponse) Status() int { return w.status }
func (w *auditResponse) Size() int {
	if !w.written {
		return -1
	}
	return w.body.Len()
}
func (w *auditResponse) Written() bool { return w.written }
func (w *auditResponse) Flush()        { w.WriteHeaderNow() }
