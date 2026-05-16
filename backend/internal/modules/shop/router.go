package shop

import "github.com/gin-gonic/gin"

// Register mounts shop routes under authenticated /api/v1.
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	g.GET("/platform/providers", h.ListProviders)
	g.GET("/platform/settings/:platform", h.GetPlatformAppSettings)
	g.PUT("/platform/settings/:platform", h.PutPlatformAppSettings)

	s := g.Group("/shops")
	s.GET("", h.List)
	s.POST("", h.Create)
	s.GET("/:id", h.Get)
	s.PUT("/:id", h.Update)
	s.DELETE("/:id", h.Delete)
	s.PUT("/:id/auth", h.PutAuth)
	s.POST("/:id/test-connection", h.TestConnection)
	s.GET("/:id/oauth/tiktok/authorize-url", h.TikTokOAuthAuthorizeURL)
	s.POST("/:id/oauth/tiktok/callback", h.TikTokOAuthCallback)
	s.GET("/:id/oauth/shopee/authorize-url", h.ShopeeOAuthAuthorizeURL)
	s.POST("/:id/oauth/shopee/callback", h.ShopeeOAuthCallback)
}
