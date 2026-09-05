package profitability

import "github.com/gin-gonic/gin"

func Register(group *gin.RouterGroup, handler *Handler) {
	if group == nil || handler == nil {
		return
	}
	group.GET("/order-profits", handler.List)
	group.GET("/order-profits/:orderId", handler.Get)
}
