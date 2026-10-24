package customersync

import (
	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
)

// Register mounts customer message sync routes on authenticated /api/v1.
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	view := func(c *gin.Context) {
		if !adminperm.RequirePermission(c, h.Svc.DB, adminperm.PermCustomerView) {
			c.Abort()
			return
		}
		c.Next()
	}
	operate := func(c *gin.Context) {
		if !adminperm.RequireWrite(c, h.Svc.DB, adminperm.PermCustomerOperate) {
			c.Abort()
			return
		}
		c.Next()
	}

	g.POST("/shops/:id/sync-customer-messages", operate, h.SyncShopCustomerMessages)
	cg := g.Group("/customer/message-sync")
	cg.GET("/tasks", view, h.ListTasks)
	cg.GET("/tasks/:id", view, h.GetTask)
	cg.POST("/tasks/:id/retry", operate, h.RetryTask)
}
