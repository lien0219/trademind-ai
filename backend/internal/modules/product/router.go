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
	g.POST("/products/:id/platform-configs/douyin_shop/build-mapping", h.BuildDouyinDraftMapping)
	g.GET("/products/:id/platform-configs/douyin_shop/mapping", h.GetDouyinDraftMapping)
	g.PUT("/products/:id/platform-configs/douyin_shop/mapping", h.PutDouyinDraftMapping)
	g.POST("/products/:id/platform-configs/douyin_shop/validate", h.ValidateDouyinDraftMapping)
	g.POST("/products/:id/platform-configs/douyin_shop/images/upload", h.UploadDouyinImages)
	g.POST("/products/:id/platform-configs/douyin_shop/images/:imageKey/retry", h.RetryDouyinImage)
	g.GET("/products/:id/platform-configs/douyin_shop/images/status", h.GetDouyinImageStatus)
	g.GET("/products/:id/platform-configs/:platform", h.GetPlatformPublishConfig)
	g.PUT("/products/:id/platform-configs/:platform", h.PutPlatformPublishConfig)

	g.GET("/product-skus/search", h.SearchSKUs)

	g.POST("/products/:id/skus", h.PostSKU)
	g.PUT("/products/:id/skus/:skuId", h.PutSKU)
	g.PUT("/products/:id/skus/:skuId/stock-settings", h.PutSKUStockSettings)
	g.DELETE("/products/:id/skus/:skuId", h.DeleteSKU)

	g.POST("/products/:id/images/reorder", h.PostImagesReorder)
	g.POST("/products/:id/sync-images", h.SyncImages)
	g.POST("/products/:id/images", h.PostImage)
	g.PUT("/products/:id/images/:imageId", h.PutImage)
	g.DELETE("/products/:id/images/:imageId", h.DeleteImage)

	g.POST("/products/:id/ai/optimize-title", h.OptimizeTitle)
	g.POST("/products/:id/ai/generate-description", h.GenerateDescription)
	g.POST("/products/:id/apply-ai-title", h.ApplyAITitle)
	g.POST("/products/:id/apply-ai-description", h.ApplyAIDescription)
	g.GET("/products/:id/ai/tasks", h.ListAITasks)
}
