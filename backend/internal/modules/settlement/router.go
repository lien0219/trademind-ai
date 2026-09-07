package settlement

import "github.com/gin-gonic/gin"

func Register(group *gin.RouterGroup, handler *Handler) {
	if group == nil || handler == nil {
		return
	}
	group.POST("/settlement-imports/preview", handler.Preview)
	group.POST("/settlement-imports", handler.Confirm)
	group.GET("/settlement-reconciliation", handler.List)
	group.GET("/settlement-reconciliation/:id", handler.Get)
}
