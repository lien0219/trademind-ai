package inventory

import "github.com/gin-gonic/gin"

// Register mounts inventory + inventory sync REST routes under authenticated /api/v1.
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	g.POST("/products/:id/skus/:skuId/adjust-stock", h.AdjustStock)
	g.GET("/products/:id/skus/:skuId/warehouse-balances", h.ListWarehouseBalances)
	g.GET("/products/:id/skus/:skuId/inventory-logs", h.ListSKULogs)
	g.GET("/products/:id/publication-skus", h.ListPublicationSKURows)

	g.POST("/product-publication-skus/:id/sync-inventory", h.SyncPublicationSKU)
	g.POST("/products/:id/sync-inventory", h.BatchSyncProduct)

	g.GET("/inventory", h.ListCenter)
	g.GET("/inventory/logs", h.ListGlobalLogs)
	g.GET("/inventory/effects", h.ListGlobalOrderEffects)
	g.GET("/inventory/alerts", h.ListAlerts)
	g.GET("/inventory/warehouse-ledger/reconciliation", h.ReconcileWarehouseLedger)
	g.POST("/inventory/warehouse-ledger/migrate-legacy", h.MigrateLegacyStock)
	g.GET("/inventory/warehouse-transfers", h.ListWarehouseTransfers)
	g.GET("/inventory/warehouse-transfers/:id", h.GetWarehouseTransfer)
	g.POST("/inventory/warehouse-transfers", h.CreateWarehouseTransfer)
	g.POST("/inventory/warehouse-transfers/:id/submit", h.SubmitWarehouseTransfer)
	g.POST("/inventory/warehouse-transfers/:id/approve", h.ApproveWarehouseTransfer)
	g.POST("/inventory/warehouse-transfers/:id/dispatch", h.DispatchWarehouseTransfer)
	g.POST("/inventory/warehouse-transfers/:id/receive", h.ReceiveWarehouseTransfer)
	g.POST("/inventory/warehouse-transfers/:id/cancel", h.CancelWarehouseTransfer)
	g.GET("/inventory/stocktakes", h.ListInventoryStocktakes)
	g.GET("/inventory/stocktakes/:id", h.GetInventoryStocktake)
	g.POST("/inventory/stocktakes", h.CreateInventoryStocktake)
	g.PATCH("/inventory/stocktakes/:id/items/:itemId", h.UpdateInventoryStocktakeItem)
	g.POST("/inventory/stocktakes/:id/submit", h.SubmitInventoryStocktake)
	g.POST("/inventory/stocktakes/:id/approve", h.ApproveInventoryStocktake)
	g.POST("/inventory/stocktakes/:id/post", h.PostInventoryStocktake)
	g.POST("/inventory/stocktakes/:id/cancel", h.CancelInventoryStocktake)
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
