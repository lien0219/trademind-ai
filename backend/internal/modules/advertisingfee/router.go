package advertisingfee

import "github.com/gin-gonic/gin"

func Register(group *gin.RouterGroup, handler *Handler) {
	if group == nil || handler == nil {
		return
	}
	group.POST("/advertising-fee-imports/preview", handler.Preview)
	group.POST("/advertising-fee-imports", handler.Confirm)
	group.GET("/advertising-fee-imports", handler.ListImports)
	group.GET("/advertising-fees", handler.ListAllocations)
	group.GET("/advertising-fees/:id", handler.GetAllocation)
	group.POST("/advertising-fees/:id/adjustments", handler.CreateAdjustment)
	group.POST("/advertising-fees/:id/adjustments/:adjustmentId/reverse", handler.ReverseAdjustment)
}
