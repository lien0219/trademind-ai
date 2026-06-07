package imagetask

import "github.com/gin-gonic/gin"

// Register mounts /api/v1/image/tasks and /api/v1/ai/image/* routes on an authenticated group.
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
	rg.GET("/:id/translate-edit-state", h.GetTranslateEditState)
	rg.POST("/:id/manual-render", h.ManualRenderTranslate)

	ai := g.Group("/ai/image")
	ai.POST("/tasks", h.Create)
	ai.GET("/tasks", h.List)
	ai.GET("/tasks/:id", h.Get)
	ai.GET("/tasks/:id/translate-edit-state", h.GetTranslateEditState)
	ai.POST("/tasks/:id/manual-render", h.ManualRenderTranslate)
	ai.POST("/task-items/:id/save-to-product", h.SaveItemToProduct)
	ai.POST("/task-items/:id/set-as-main", h.SetItemAsMain)
	ai.POST("/score", h.ScoreImage)

	g.POST("/products/:id/images/select-best-main", h.SelectBestMainForProduct)
}
