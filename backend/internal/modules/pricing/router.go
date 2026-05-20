package pricing

import "github.com/gin-gonic/gin"

// Register mounts pricing routes on g (already under /api/v1, authenticated).
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	g.POST("/pricing/calculate", h.Calculate)
	g.POST("/products/:id/pricing/apply", h.ApplyProduct)
	g.POST("/products/pricing/batch-apply", h.BatchApply)
}
