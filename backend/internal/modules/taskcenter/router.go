package taskcenter

import "github.com/gin-gonic/gin"

// Register mounts task center routes under authenticated /api/v1.
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	tc := g.Group("/task-center")
	tc.GET("/failures", h.ListFailures)
	tc.GET("/summary", h.Summary)
	tc.POST("/failures/batch-retry", h.BatchRetry)
	tc.POST("/failures/batch-ignore", h.BatchIgnore)
	tc.POST("/failures/batch-handle", h.BatchHandle)
	tc.GET("/failures/:taskType/:id", h.GetFailure)
	tc.POST("/failures/:taskType/:id/retry", h.Retry)
	tc.POST("/failures/:taskType/:id/ignore", h.Ignore)
	tc.POST("/failures/:taskType/:id/handle", h.Handle)
	tc.DELETE("/failures/:taskType/:id/mark", h.Unmark)
}
