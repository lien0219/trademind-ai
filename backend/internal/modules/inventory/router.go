package inventory

import "github.com/gin-gonic/gin"

// Register mounts inventory + inventory sync REST routes under authenticated /api/v1.
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	g.POST("/products/:id/skus/:skuId/adjust-stock", h.AdjustStock)
	g.GET("/products/:id/skus/:skuId/inventory-logs", h.ListSKULogs)
	g.GET("/products/:id/publication-skus", h.ListPublicationSkuRows)

	g.POST("/product-publication-skus/:id/sync-inventory", h.SyncPublicationSku)
	g.POST("/products/:id/sync-inventory", h.BatchSyncProduct)

	g.GET("/inventory/logs", h.ListGlobalLogs)
	g.GET("/inventory/effects", h.ListGlobalOrderEffects)
	g.GET("/inventory/alerts", h.ListAlerts)
	g.POST("/inventory/stock-settings/batch-preview", h.BatchPreviewStockSettings)
	g.POST("/inventory/stock-settings/batch-update", h.BatchUpdateStockSettings)

	g.GET("/inventory-sync/tasks", h.ListTasks)
	g.GET("/inventory-sync/tasks/:id", h.GetTask)
	g.POST("/inventory-sync/tasks/:id/retry", h.RetryTask)

	g.POST("/inventory-sync/batches/retry-failed-tasks", h.RetryInventorySyncTasksBatch)
	g.POST("/inventory-sync/batches", h.CreateInventorySyncBatch)
	g.GET("/inventory-sync/batches", h.ListInventorySyncBatches)
	g.GET("/inventory-sync/batches/:id/tasks", h.ListInventorySyncBatchTasks)
	g.GET("/inventory-sync/batches/:id", h.GetInventorySyncBatch)
	g.POST("/inventory-sync/batches/:id/retry-failed", h.RetryInventorySyncBatchFailed)
}
