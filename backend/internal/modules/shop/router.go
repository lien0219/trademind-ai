package shop

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
)

// Register mounts shop routes under authenticated /api/v1.
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	var logs *operationlog.Service
	if h.Svc != nil {
		logs = h.Svc.OpLog
	}
	g = g.Group("", func(c *gin.Context) {
		switch c.Request.Method {
		case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
			resource := "shop"
			if strings.Contains(c.FullPath(), "/platform/") {
				resource = "platform_settings"
			}
			logs.AuditFailures(resource+".mutation", resource, "id")(c)
		default:
			c.Next()
		}
	})
	g.GET("/platform/providers", h.ListProviders)
	platformSettings := g.Group("/platform", requirePlatformSettingsManage(h))
	platformSettings.GET("/settings/:platform", h.GetPlatformAppSettings)
	platformSettings.PUT("/settings/:platform", h.PutPlatformAppSettings)
	platformSettings.POST("/settings/:platform/test-connection", h.TestPlatformAppSettings)
	platformSettings.GET("/publish-settings/:platform", h.GetPlatformPublishSettings)
	platformSettings.PUT("/publish-settings/:platform", h.PutPlatformPublishSettings)
	g.GET("/platform/douyin/categories", requireShopPermission(h, adminperm.PermStoreView, "view"), h.ListDouyinCategories)
	g.POST("/platform/douyin/categories/sync", requireShopPermission(h, adminperm.PermStoreOperate, "operate"), h.SyncDouyinCategories)
	g.GET("/platform/douyin/categories/stats", requireShopPermission(h, adminperm.PermStoreView, ""), h.DouyinCategoryStats)
	g.GET("/platform/douyin/categories/:categoryId/attributes", requireShopPermission(h, adminperm.PermStoreView, ""), h.ListDouyinCategoryAttributes)
	g.POST("/platform/douyin/categories/:categoryId/attributes/sync", requireShopPermission(h, adminperm.PermStoreOperate, "operate"), h.SyncDouyinCategoryAttributes)

	s := g.Group("/shops")
	s.GET("", requireShopPermission(h, adminperm.PermStoreView, ""), h.List)
	s.POST("", requireGlobalAdmin(h), h.Create)
	s.GET("/:id", requireShopPermission(h, adminperm.PermStoreView, "view"), h.Get)
	s.PUT("/:id", requireShopPermission(h, adminperm.PermStoreOperate, "operate"), h.Update)
	s.DELETE("/:id", requireGlobalAdmin(h), h.Delete)
	s.PUT("/:id/auth", requireShopPermission(h, adminperm.PermStoreOperate, "operate"), h.PutAuth)
	s.POST("/:id/test-connection", requireShopPermission(h, adminperm.PermStoreOperate, "operate"), h.TestConnection)
	s.GET("/oauth/douyin/start", requireDouyinOAuthStart(h), h.DouyinOAuthStart)
	s.GET("/:id/oauth/douyin/authorize-url", requireShopPermission(h, adminperm.PermStoreOperate, "operate"), h.DouyinOAuthAuthorizeURL)
	s.POST("/:id/oauth/douyin/refresh", requireShopPermission(h, adminperm.PermStoreOperate, "operate"), h.DouyinOAuthRefresh)
	s.POST("/:id/oauth/douyin/revoke", requireShopPermission(h, adminperm.PermStoreOperate, "operate"), h.DouyinOAuthRevoke)
	s.POST("/:id/oauth/douyin/test", requireShopPermission(h, adminperm.PermStoreOperate, "operate"), h.DouyinOAuthTest)
	s.POST("/:id/oauth/douyin/sync-shop-info", requireShopPermission(h, adminperm.PermStoreOperate, "operate"), h.DouyinSyncShopInfo)
	s.GET("/:id/oauth/tiktok/authorize-url", requireShopPermission(h, adminperm.PermStoreOperate, "operate"), h.TikTokOAuthAuthorizeURL)
	s.POST("/:id/oauth/tiktok/callback", requireShopPermission(h, adminperm.PermStoreOperate, "operate"), h.TikTokOAuthCallback)
	s.GET("/:id/oauth/shopee/authorize-url", requireShopPermission(h, adminperm.PermStoreOperate, "operate"), h.ShopeeOAuthAuthorizeURL)
	s.POST("/:id/oauth/shopee/callback", requireShopPermission(h, adminperm.PermStoreOperate, "operate"), h.ShopeeOAuthCallback)
	s.GET("/:id/oauth/lazada/authorize-url", requireShopPermission(h, adminperm.PermStoreOperate, "operate"), h.LazadaOAuthAuthorizeURL)
	s.POST("/:id/oauth/lazada/callback", requireShopPermission(h, adminperm.PermStoreOperate, "operate"), h.LazadaOAuthCallback)
	s.GET("/:id/oauth/amazon/authorize-url", requireShopPermission(h, adminperm.PermStoreOperate, "operate"), h.AmazonOAuthAuthorizeURL)
	s.POST("/:id/oauth/amazon/callback", requireShopPermission(h, adminperm.PermStoreOperate, "operate"), h.AmazonOAuthCallback)
}

// RegisterPublic mounts OAuth callbacks that external platforms call directly.
func RegisterPublic(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	g.GET("/shops/oauth/douyin/callback", h.DouyinOAuthCallback)
}
