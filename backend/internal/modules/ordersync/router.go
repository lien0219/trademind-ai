package ordersync

import "github.com/gin-gonic/gin"

// Register mounts order-sync routes on authenticated /api/v1.
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	g.POST("/shops/:id/sync-orders", h.SyncShopOrders)

	og := g.Group("/order-sync")
	og.GET("/tasks", h.ListTasks)
	og.GET("/tasks/:id", h.GetTask)
	og.POST("/tasks/:id/retry", h.RetryTask)
}
