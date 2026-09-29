package supplier

import "github.com/gin-gonic/gin"

func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	g.GET("/suppliers", h.List)
	g.POST("/suppliers", h.Create)
	g.PUT("/suppliers/:id", h.Update)
	g.GET("/suppliers/:id/skus", h.ListSKUs)
	g.POST("/suppliers/:id/skus", h.BindSKU)
}
