package operationdashboard

import "github.com/gin-gonic/gin"

// Register mounts dashboard routes under an authenticated group.
func Register(g *gin.RouterGroup, h *Handler) {
	g.GET("/dashboard/product-operations", h.ProductOperations)
}
