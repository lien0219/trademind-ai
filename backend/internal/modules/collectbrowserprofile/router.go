package collectbrowserprofile

import (
	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
)

// Register mounts JWT-protected browser profile routes under /api/v1.
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	view := func(c *gin.Context) {
		if !adminperm.RequirePermission(c, h.Svc.DB, adminperm.PermProductView) {
			c.Abort()
			return
		}
		c.Next()
	}
	write := func(c *gin.Context) {
		if !adminperm.RequireWrite(c, h.Svc.DB, adminperm.PermProductWrite) {
			c.Abort()
			return
		}
		c.Next()
	}
	g.GET("/collect/browser-profiles", view, h.List)
	g.POST("/collect/browser-profiles", write, h.Create)
	g.POST("/collect/browser-profiles/:id/open-login", write, h.OpenLogin)
	g.POST("/collect/browser-profiles/:id/check", write, h.Check)
	g.POST("/collect/browser-profiles/:id/disable", write, h.Disable)
	g.POST("/collect/browser-profiles/:id/enable", write, h.Enable)
	g.DELETE("/collect/browser-profiles/:id", write, h.Delete)
}
