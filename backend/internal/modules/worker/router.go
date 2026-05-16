package worker

import "github.com/gin-gonic/gin"

// Register mounts worker routes on authenticated /api/v1 group.
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	g.GET("/workers/monitor", h.Monitor)
}
