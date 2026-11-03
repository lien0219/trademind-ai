package profitability

import "github.com/gin-gonic/gin"

func Register(group *gin.RouterGroup, handler *Handler) {
	if group == nil || handler == nil {
		return
	}
	group.GET("/order-profits", handler.List)
	group.GET("/finance/fx-rates", handler.ListFXRates)
	group.POST("/finance/fx-rates", handler.CreateFXRate)
	group.GET("/finance/fx-report", handler.FXReport)
	group.GET("/order-profits/:orderId", handler.Get)
}
