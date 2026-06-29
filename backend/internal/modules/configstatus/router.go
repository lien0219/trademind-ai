package configstatus

import "github.com/gin-gonic/gin"

// Register mounts config status routes.
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	g.GET("/settings/config-status", h.GetOverview)
}
