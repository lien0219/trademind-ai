package aiprompt

import "github.com/gin-gonic/gin"

// Register mounts /api/v1/ai/prompts routes on an authenticated group.
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	rg := g.Group("/ai/prompts")
	rg.GET("", h.List)
	rg.POST("", h.Create)
	rg.GET("/:id", h.Get)
	rg.PUT("/:id", h.Put)
	rg.DELETE("/:id", h.Delete)
	rg.POST("/:id/enable", h.Enable)
	rg.POST("/:id/disable", h.Disable)
}
