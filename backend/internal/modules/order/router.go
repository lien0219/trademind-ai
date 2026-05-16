package order

import "github.com/gin-gonic/gin"

// Register mounts authenticated routes (already under Bearer /api/v1).
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	o := g.Group("/orders")
	o.GET("", h.List)
	o.POST("", h.Create)

	o.POST("/:id/items", h.PostItem)
	o.PUT("/:id/items/:itemId", h.PutItem)
	o.DELETE("/:id/items/:itemId", h.DeleteItem)

	o.POST("/:id/shipments", h.PostShipment)
	o.PUT("/:id/shipments/:shipmentId", h.PutShipment)
	o.DELETE("/:id/shipments/:shipmentId", h.DeleteShipment)

	o.GET("/:id", h.Get)
	o.PUT("/:id", h.Update)
	o.DELETE("/:id", h.Delete)
}
