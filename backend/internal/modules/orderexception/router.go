package orderexception

import "github.com/gin-gonic/gin"

// Register mounts routes under /orders/exceptions (parent group already covers /api/v1).
func Register(parent *gin.RouterGroup, h *Handler) {
	if parent == nil || h == nil {
		return
	}
	g := parent.Group("/orders/exceptions")
	g.GET("", h.List)
	g.GET("/:sourceType/:sourceId", h.Detail)
	g.POST("/:sourceType/:sourceId/handle", h.Handle)
	g.POST("/:sourceType/:sourceId/ignore", h.Ignore)
	g.DELETE("/:sourceType/:sourceId/mark", h.Unmark)
	g.POST("/:sourceType/:sourceId/bind-sku", h.BindSKU)
	g.POST("/:sourceType/:sourceId/retry-deduct", h.RetryDeduct)
	g.POST("/:sourceType/:sourceId/retry-inventory-sync", h.RetryInventorySync)
}
