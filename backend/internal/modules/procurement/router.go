package procurement

import "github.com/gin-gonic/gin"

func Register(group *gin.RouterGroup, handler *Handler) {
	if group == nil || handler == nil {
		return
	}
	group.GET("/purchase-orders", handler.List)
	group.POST("/purchase-orders", handler.Create)
	group.GET("/purchase-orders/:id", handler.Get)
	group.POST("/purchase-orders/:id/submit", handler.Submit)
	group.POST("/purchase-orders/:id/approve", handler.Approve)
	group.POST("/purchase-orders/:id/cancel", handler.Cancel)
	group.POST("/purchase-orders/:id/close", handler.Close)
	group.POST("/purchase-orders/:id/receipts", handler.Receive)
	group.GET("/purchase-orders/:id/returnable-receipt-items", handler.ListReturnableReceiptItems)
	group.GET("/purchase-returns", handler.ListPurchaseReturns)
	group.POST("/purchase-returns", handler.CreatePurchaseReturn)
	group.GET("/purchase-returns/:id", handler.GetPurchaseReturn)
	group.POST("/purchase-returns/:id/submit", handler.SubmitPurchaseReturn)
	group.POST("/purchase-returns/:id/approve", handler.ApprovePurchaseReturn)
	group.POST("/purchase-returns/:id/complete", handler.CompletePurchaseReturn)
	group.POST("/purchase-returns/:id/cancel", handler.CancelPurchaseReturn)
}
