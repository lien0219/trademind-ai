package collect

import "github.com/gin-gonic/gin"

// Register mounts authenticated collect routes on g (already under /api/v1).
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	g.GET("/collect/providers", h.ListProviders)
	g.POST("/collect/tasks", h.Create)
	g.GET("/collect/tasks", h.List)
	g.GET("/collect/monitor", h.Monitor)
	g.GET("/collect/tasks/:id/events", h.ListTaskEvents)
	g.GET("/collect/tasks/:id", h.Get)
	g.POST("/collect/tasks/:id/retry", h.Retry)

	g.POST("/collect/batches", h.CreateBatch)
	g.GET("/collect/batches", h.ListBatches)
	g.GET("/collect/batches/:id/tasks", h.ListBatchTasks)
	g.GET("/collect/batches/:id", h.GetBatch)
	g.POST("/collect/batches/:id/retry-failed", h.RetryBatchFailed)

	g.GET("/collector/providers/1688/auth-status", h.Get1688AuthStatus)
	g.POST("/collector/providers/1688/open-login-browser", h.Open1688LoginBrowser)
}
