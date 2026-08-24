package salesreturn

import "github.com/gin-gonic/gin"

func Register(group *gin.RouterGroup, handler *Handler) {
	if group == nil || handler == nil {
		return
	}
	group.GET("/orders/:id/sales-returnable-items", handler.ListReturnableItems)
	group.GET("/sales-returns", handler.List)
	group.GET("/sales-return-reconciliation", handler.ListPlatformReconciliation)
	group.GET("/sales-return-reconciliation/:id", handler.GetPlatformReconciliation)
	group.GET("/refund-executions", handler.ListRefundExecutions)
	group.GET("/refund-executions/:id", handler.GetRefundExecution)
	group.POST("/sales-returns", handler.Create)
	group.GET("/sales-returns/:id", handler.Get)
	group.POST("/sales-returns/:id/refund-execution", handler.CreateRefundExecution)
	group.POST("/sales-returns/:id/submit", handler.Submit)
	group.POST("/sales-returns/:id/approve", handler.Approve)
	group.POST("/sales-returns/:id/complete", handler.Complete)
	group.POST("/sales-returns/:id/cancel", handler.Cancel)
	group.POST("/refund-executions/:id/result", handler.RecordRefundResult)
	group.POST("/refund-executions/:id/confirm-from-platform", handler.ConfirmRefundFromPlatform)
	group.POST("/refund-executions/:id/cancel", handler.CancelRefundExecution)
}
