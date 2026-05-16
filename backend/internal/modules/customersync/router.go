package customersync

import "github.com/gin-gonic/gin"

// Register mounts customer message sync routes on authenticated /api/v1.
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	g.POST("/shops/:id/sync-customer-messages", h.SyncShopCustomerMessages)
	cg := g.Group("/customer/message-sync")
	cg.GET("/tasks", h.ListTasks)
	cg.GET("/tasks/:id", h.GetTask)
	cg.POST("/tasks/:id/retry", h.RetryTask)
}
