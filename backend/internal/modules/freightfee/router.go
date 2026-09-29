package freightfee

import "github.com/gin-gonic/gin"

func Register(group *gin.RouterGroup, handler *Handler) {
	if group == nil || handler == nil {
		return
	}
	group.POST("/freight-fee-imports/preview", handler.Preview)
	group.POST("/freight-fee-imports", handler.Confirm)
	group.GET("/freight-fee-imports", handler.ListImports)
	group.GET("/freight-fees", handler.ListCharges)
	group.GET("/freight-fees/:id", handler.GetCharge)
	group.POST("/freight-fees/:id/adjustments", handler.CreateAdjustment)
	group.POST("/freight-fees/:id/adjustments/:adjustmentId/reverse", handler.ReverseAdjustment)
}
