package order

import "github.com/gin-gonic/gin"

// Register mounts authenticated routes (already under Bearer /api/v1).
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	g.GET("/order-item-sku-matches", h.ListGlobalSKUMatches)
	g.POST("/order-items/:itemId/bind-sku", h.PostBindOrderItemSKU)

	w := g.Group("/fulfillment-waves")
	w.GET("", h.ListFulfillmentWaves)
	w.POST("", h.PostFulfillmentWave)
	w.GET("/:id", h.GetFulfillmentWave)
	w.GET("/:id/documents", h.ListFulfillmentWaveDocuments)
	w.POST("/:id/documents", h.PostFulfillmentWaveDocument)
	w.GET("/:id/documents/:documentId", h.GetFulfillmentWaveDocument)
	w.POST("/:id/documents/:documentId/print-events", h.PostFulfillmentWaveDocumentPrintEvent)
	w.POST("/:id/start", h.PostStartFulfillmentWave)
	w.POST("/:id/picks", h.PostRecordFulfillmentWavePick)
	w.POST("/:id/orders/:orderId/pack", h.PostPackFulfillmentWaveOrder)
	w.POST("/:id/orders/:orderId/verify-pack", h.PostVerifyFulfillmentWavePack)
	w.POST("/:id/complete", h.PostCompleteFulfillmentWave)
	w.POST("/:id/cancel", h.PostCancelFulfillmentWave)

	o := g.Group("/orders")
	o.GET("", h.List)
	o.POST("", h.Create)
	o.GET("/warehouse-allocations", h.ListWarehouseAllocations)
	o.GET("/:id/warehouse-allocation", h.GetWarehouseAllocation)
	o.POST("/:id/warehouse-allocation", h.PostWarehouseAllocation)

	o.POST("/:id/items", h.PostItem)
	o.PUT("/:id/items/:itemId", h.PutItem)
	o.DELETE("/:id/items/:itemId", h.DeleteItem)

	o.POST("/:id/deduct-inventory", h.PostDeductInventory)
	o.POST("/:id/restore-inventory", h.PostRestoreInventory)
	o.POST("/fulfillment-batch", h.PostFulfillBatch)
	o.POST("/:id/fulfill", h.PostFulfill)
	o.GET("/fulfillment-reconciliation", h.ListFulfillmentReconciliation)
	o.GET("/:id/inventory-effects", h.GetOrderInventoryEffects)
	o.GET("/:id/fulfillment-reconciliation", h.GetFulfillmentReconciliation)
	o.GET("/:id/sku-matches", h.GetOrderSKUMatches)
	o.POST("/:id/match-skus", h.PostMatchOrderSKUs)

	o.POST("/:id/shipments", h.PostShipment)
	o.PUT("/:id/shipments/:shipmentId", h.PutShipment)
	o.DELETE("/:id/shipments/:shipmentId", h.DeleteShipment)
	o.GET("/:id/shipments", h.GetShipments)
	o.GET("/:id/shipments/:shipmentId/events", h.GetShipmentEvents)
	o.POST("/:id/shipments/:shipmentId/events", h.PostShipmentEvent)

	o.GET("/:id", h.Get)
	o.PUT("/:id", h.Update)
	o.DELETE("/:id", h.Delete)
}
