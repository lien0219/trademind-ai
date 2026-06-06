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
	g.POST("/platform/settings/:platform/test-connection", h.TestPlatformAppSettings)
	g.GET("/platform/publish-settings/:platform", h.GetPlatformPublishSettings)
	g.PUT("/platform/publish-settings/:platform", h.PutPlatformPublishSettings)
	g.GET("/platform/douyin/categories", h.ListDouyinCategories)
	g.POST("/platform/douyin/categories/sync", h.SyncDouyinCategories)
	g.GET("/platform/douyin/categories/stats", h.DouyinCategoryStats)
	g.GET("/platform/douyin/categories/:categoryId/attributes", h.ListDouyinCategoryAttributes)
	g.POST("/platform/douyin/categories/:categoryId/attributes/sync", h.SyncDouyinCategoryAttributes)

	s := g.Group("/shops")
	s.GET("", h.List)
	s.POST("", h.Create)
	s.GET("/:id", h.Get)
	s.PUT("/:id", h.Update)
	s.DELETE("/:id", h.Delete)
	s.PUT("/:id/auth", h.PutAuth)
	s.POST("/:id/test-connection", h.TestConnection)
	s.GET("/oauth/douyin/start", h.DouyinOAuthStart)
	s.GET("/:id/oauth/douyin/authorize-url", h.DouyinOAuthAuthorizeURL)
	s.POST("/:id/oauth/douyin/refresh", h.DouyinOAuthRefresh)
	s.POST("/:id/oauth/douyin/revoke", h.DouyinOAuthRevoke)
	s.POST("/:id/oauth/douyin/test", h.DouyinOAuthTest)
	s.POST("/:id/oauth/douyin/sync-shop-info", h.DouyinSyncShopInfo)
	s.GET("/:id/oauth/tiktok/authorize-url", h.TikTokOAuthAuthorizeURL)
	s.POST("/:id/oauth/tiktok/callback", h.TikTokOAuthCallback)
	s.GET("/:id/oauth/shopee/authorize-url", h.ShopeeOAuthAuthorizeURL)
	s.POST("/:id/oauth/shopee/callback", h.ShopeeOAuthCallback)
	s.GET("/:id/oauth/lazada/authorize-url", h.LazadaOAuthAuthorizeURL)
	s.POST("/:id/oauth/lazada/callback", h.LazadaOAuthCallback)
	s.GET("/:id/oauth/amazon/authorize-url", h.AmazonOAuthAuthorizeURL)
	s.POST("/:id/oauth/amazon/callback", h.AmazonOAuthCallback)
}

// RegisterPublic mounts OAuth callbacks that external platforms call directly.
func RegisterPublic(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	g.GET("/shops/oauth/douyin/callback", h.DouyinOAuthCallback)
}
