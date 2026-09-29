package warehousefee

import "github.com/gin-gonic/gin"

func Register(group *gin.RouterGroup, handler *Handler) {
	if group == nil || handler == nil {
		return
	}
	group.GET("/warehouse-fee-rate-cards", handler.ListRateCards)
	group.GET("/warehouse-fee-rate-cards/:id", handler.GetRateCard)
	group.POST("/warehouse-fee-rate-cards", handler.CreateRateCard)
	group.PUT("/warehouse-fee-rate-cards/:id", handler.UpdateRateCard)
	group.GET("/warehouse-operation-fees/candidates", handler.ListCandidates)
	group.POST("/warehouse-operation-fees/preview", handler.Preview)
	group.POST("/warehouse-operation-fees", handler.Confirm)
	group.GET("/warehouse-operation-fees", handler.ListSnapshots)
	group.GET("/warehouse-operation-fees/:id", handler.GetSnapshot)
	group.POST("/warehouse-operation-fees/:id/adjustments", handler.CreateAdjustment)
	group.POST("/warehouse-operation-fees/:id/adjustments/:adjustmentId/reverse", handler.ReverseAdjustment)
}
