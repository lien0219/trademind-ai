package imagetask

import "github.com/gin-gonic/gin"

// Register mounts /api/v1/image/tasks routes on an authenticated group.
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	g.GET("/image/providers", h.ListProviders)

	rg := g.Group("/image/tasks")
	rg.POST("", h.Create)
	rg.GET("", h.List)
	rg.GET("/monitor", h.Monitor)
	rg.GET("/:id", h.Get)
	rg.GET("/:id/items", h.ListItems)
	rg.POST("/:id/apply", h.Apply)
	rg.DELETE("/:id/items/:itemId", h.DeleteItem)
	rg.POST("/:id/retry", h.Retry)
}
