package collect

import "github.com/gin-gonic/gin"

// Register mounts authenticated collect routes on g (already under /api/v1).
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	g.POST("/collect/tasks", h.Create)
	g.GET("/collect/tasks", h.List)
	g.GET("/collect/tasks/:id", h.Get)
	g.POST("/collect/tasks/:id/retry", h.Retry)
}
