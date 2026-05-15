package product

import "github.com/gin-gonic/gin"

// Register mounts authenticated product routes on g (already under /api/v1).
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	g.GET("/products", h.List)
	g.POST("/products", h.Create)
	g.GET("/products/:id", h.Get)
	g.PUT("/products/:id", h.Put)
	g.DELETE("/products/:id", h.Delete)

	g.POST("/products/:id/skus", h.PostSKU)
	g.PUT("/products/:id/skus/:skuId", h.PutSKU)
	g.DELETE("/products/:id/skus/:skuId", h.DeleteSKU)

	g.POST("/products/:id/images/reorder", h.PostImagesReorder)
	g.POST("/products/:id/images", h.PostImage)
	g.PUT("/products/:id/images/:imageId", h.PutImage)
	g.DELETE("/products/:id/images/:imageId", h.DeleteImage)

	g.POST("/products/:id/ai/optimize-title", h.OptimizeTitle)
	g.POST("/products/:id/ai/generate-description", h.GenerateDescription)
	g.POST("/products/:id/apply-ai-title", h.ApplyAITitle)
	g.POST("/products/:id/apply-ai-description", h.ApplyAIDescription)
	g.GET("/products/:id/ai/tasks", h.ListAITasks)
}
