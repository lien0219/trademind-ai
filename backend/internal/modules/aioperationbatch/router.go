package aioperationbatch

import "github.com/gin-gonic/gin"

// Register mounts JWT-protected ai batch routes (caller wraps with BearerAuth).
func Register(r gin.IRouter, h *Handler) {
	if h == nil {
		return
	}
	g := r.Group("/ai/batches")
	g.POST("/product-text", h.CreateProductText)
	g.POST("/product-images", h.CreateProductImages)
	g.GET("", h.List)
	g.GET("/:id", h.Get)
	g.GET("/:id/tasks", h.Tasks)
	g.POST("/:id/retry-failed", h.RetryFailed)
	g.POST("/:id/apply-results", h.ApplyResults)
}
