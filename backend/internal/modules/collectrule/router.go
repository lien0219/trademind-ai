package collectrule

import "github.com/gin-gonic/gin"

// Register mounts JWT-protected collect rule routes under /api/v1 (group g).
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	g.GET("/collect/rules", h.List)
	g.POST("/collect/rules", h.Create)
	g.GET("/collect/rules/:id", h.Get)
	g.PUT("/collect/rules/:id", h.Update)
	g.DELETE("/collect/rules/:id", h.Delete)
	g.POST("/collect/rules/:id/enable", h.Enable)
	g.POST("/collect/rules/:id/disable", h.Disable)
	g.POST("/collect/rules/:id/test", h.Test)
}
