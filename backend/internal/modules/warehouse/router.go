package warehouse

import "github.com/gin-gonic/gin"

func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	g.GET("/warehouses", h.List)
	g.POST("/warehouses", h.Create)
	g.GET("/warehouses/:id/locations", h.ListLocations)
	g.POST("/warehouses/:id/locations", h.CreateLocation)
	g.PUT("/warehouses/:id/locations/:locationId", h.UpdateLocation)
	g.PUT("/warehouses/:id", h.Update)
}
