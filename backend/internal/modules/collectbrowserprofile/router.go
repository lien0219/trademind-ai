package collectbrowserprofile

import "github.com/gin-gonic/gin"

// Register mounts JWT-protected browser profile routes under /api/v1.
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	g.GET("/collect/browser-profiles", h.List)
	g.POST("/collect/browser-profiles", h.Create)
	g.POST("/collect/browser-profiles/:id/open-login", h.OpenLogin)
	g.POST("/collect/browser-profiles/:id/check", h.Check)
	g.DELETE("/collect/browser-profiles/:id", h.Delete)
}
