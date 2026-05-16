package shop

import "github.com/gin-gonic/gin"

// Register mounts shop routes under authenticated /api/v1.
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	g.GET("/platform/providers", h.ListProviders)

	s := g.Group("/shops")
	s.GET("", h.List)
	s.POST("", h.Create)
	s.GET("/:id", h.Get)
	s.PUT("/:id", h.Update)
	s.DELETE("/:id", h.Delete)
	s.PUT("/:id/auth", h.PutAuth)
	s.POST("/:id/test-connection", h.TestConnection)
}
