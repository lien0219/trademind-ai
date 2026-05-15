package aitask

import "github.com/gin-gonic/gin"

// Register mounts /api/v1/ai/tasks routes on an authenticated group.
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	rg := g.Group("/ai/tasks")
	rg.GET("", h.List)
	rg.GET("/:id", h.Get)
}
