package warehouse

import "github.com/gin-gonic/gin"

func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	g.GET("/warehouses", h.List)
	g.POST("/warehouses", h.Create)
}
