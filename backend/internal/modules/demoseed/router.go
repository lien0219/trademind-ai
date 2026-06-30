package demoseed

import "github.com/gin-gonic/gin"

// Register mounts dev/demo-only seed routes (must not be registered in production).
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	dev := g.Group("/dev/demo-seed")
	dev.POST("/full-project-edge-cases", h.SeedFullProjectEdgeCases)
}
