package taskcenter

import (
	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
)

// Register mounts task center routes under authenticated /api/v1.
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	tc := g.Group("/task-center")
	taskPermission := h.requirePermission(adminperm.PermTaskRetry)
	alertReadPermission := h.requireAnyPermission(adminperm.PermAlertsRead, adminperm.PermTaskRetry)
	tc.GET("/failure-categories", taskPermission, h.FailureCategories)
	tc.GET("/alert-notifications", alertReadPermission, h.ListAlertNotifications)
	tc.GET("/alerts", alertReadPermission, h.ListAlerts)
	tc.POST("/alerts/scan", taskPermission, h.ScanAlerts)
	tc.POST("/alerts/:id/notify", taskPermission, h.NotifyAlert)
	tc.POST("/alerts/:id/handle", taskPermission, h.HandleAlert)
	tc.POST("/alerts/:id/ignore", taskPermission, h.IgnoreAlert)
	tc.DELETE("/alerts/:id/mark", taskPermission, h.UnmarkAlert)
	tc.GET("/failures", taskPermission, h.ListFailures)
	tc.GET("/summary", taskPermission, h.Summary)
	tc.POST("/failures/batch-retry", taskPermission, h.BatchRetry)
	tc.POST("/failures/batch-ignore", taskPermission, h.BatchIgnore)
	tc.POST("/failures/batch-handle", taskPermission, h.BatchHandle)
	tc.GET("/failures/:taskType/:id", taskPermission, h.GetFailure)
	tc.POST("/failures/:taskType/:id/generate-alert", taskPermission, h.GenerateAlertFromFailure)
	tc.POST("/failures/:taskType/:id/retry", taskPermission, h.Retry)
	tc.POST("/failures/:taskType/:id/ignore", taskPermission, h.Ignore)
	tc.POST("/failures/:taskType/:id/handle", taskPermission, h.Handle)
	tc.DELETE("/failures/:taskType/:id/mark", taskPermission, h.Unmark)
}
