package taskcenter

import "github.com/gin-gonic/gin"

// Register mounts task center routes under authenticated /api/v1.
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	tc := g.Group("/task-center")
	tc.GET("/failure-categories", h.FailureCategories)
	tc.GET("/alert-notifications", h.ListAlertNotifications)
	tc.GET("/alerts", h.ListAlerts)
	tc.POST("/alerts/scan", h.ScanAlerts)
	tc.POST("/alerts/:id/notify", h.NotifyAlert)
	tc.POST("/alerts/:id/handle", h.HandleAlert)
	tc.POST("/alerts/:id/ignore", h.IgnoreAlert)
	tc.DELETE("/alerts/:id/mark", h.Unmark)
	tc.GET("/failures", h.ListFailures)
	tc.GET("/summary", h.Summary)
	tc.POST("/failures/batch-retry", h.BatchRetry)
	tc.POST("/failures/batch-ignore", h.BatchIgnore)
	tc.POST("/failures/batch-handle", h.BatchHandle)
	tc.GET("/failures/:taskType/:id", h.GetFailure)
	tc.POST("/failures/:taskType/:id/generate-alert", h.GenerateAlertFromFailure)
	tc.POST("/failures/:taskType/:id/retry", h.Retry)
	tc.POST("/failures/:taskType/:id/ignore", h.Ignore)
	tc.POST("/failures/:taskType/:id/handle", h.Handle)
	tc.DELETE("/failures/:taskType/:id/mark", h.Unmark)
}
