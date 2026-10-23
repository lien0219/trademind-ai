package collect

import (
	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"gorm.io/gorm"
)

// Register mounts authenticated collect routes on g (already under /api/v1).
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	requirePermission := func(permission string, write bool) gin.HandlerFunc {
		return func(c *gin.Context) {
			allowed := false
			if write {
				allowed = adminperm.RequireWrite(c, h.Svc.DB, permission)
			} else {
				allowed = adminperm.RequirePermission(c, h.Svc.DB, permission)
			}
			if !allowed {
				c.Abort()
				return
			}
			c.Next()
		}
	}
	viewProduct := requirePermission(adminperm.PermProductView, false)
	writeProduct := requirePermission(adminperm.PermProductWrite, true)
	retryTask := requirePermission(adminperm.PermTaskRetry, true)

	g.GET("/collect/providers", viewProduct, h.ListProviders)
	g.POST("/collect/tasks", writeProduct, h.Create)
	g.GET("/collect/tasks", viewProduct, h.List)
	g.GET("/collect/monitor", viewProduct, h.Monitor)
	g.GET("/collect/tasks/:id/events", viewProduct, h.ListTaskEvents)
	g.GET("/collect/tasks/:id", viewProduct, h.Get)
	g.POST("/collect/tasks/:id/retry", retryTask, h.Retry)

	g.POST("/collect/batches", writeProduct, h.CreateBatch)
	g.GET("/collect/batches", viewProduct, h.ListBatches)
	g.GET("/collect/batches/:id/tasks", viewProduct, h.ListBatchTasks)
	g.GET("/collect/batches/:id", viewProduct, h.GetBatch)
	g.POST("/collect/batches/:id/retry-failed", retryTask, h.RetryBatchFailed)

	g.GET("/collector/providers/1688/auth-status", viewProduct, h.Get1688AuthStatus)
	g.POST("/collector/providers/1688/open-login-browser", writeProduct, h.Open1688LoginBrowser)
	g.GET("/collector/providers/pinduoduo/auth-status", viewProduct, h.GetPinduoduoAuthStatus)
	g.POST("/collector/providers/pinduoduo/check-login", viewProduct, h.CheckPinduoduoLogin)
	g.POST("/collect/providers/pinduoduo/check-login", viewProduct, h.CheckPinduoduoLogin)
	g.POST("/collector/providers/pinduoduo/open-login-browser", writeProduct, h.OpenPinduoduoLoginBrowser)

	g.POST("/collector/providers/taobao_tmall/check-login", viewProduct, h.CheckTaobaoTmallLogin)
	g.POST("/collect/providers/taobao_tmall/check-login", viewProduct, h.CheckTaobaoTmallLogin)
	g.POST("/collector/providers/taobao_tmall/open-login-browser", writeProduct, h.OpenTaobaoTmallLoginBrowser)
	g.POST("/collect/providers/taobao_tmall/open-login-browser", writeProduct, h.OpenTaobaoTmallLoginBrowser)
}

// RegisterAliases mounts legacy /api/collector paths with the same permission
// gates as their canonical /api/v1/collector counterparts.
func RegisterAliases(r gin.IRouter, h *Handler, db *gorm.DB, auth gin.HandlerFunc) {
	if r == nil || h == nil || db == nil || auth == nil {
		return
	}
	g := r.Group("/api/collector")
	g.Use(auth)
	view := func(c *gin.Context) {
		if !adminperm.RequirePermission(c, db, adminperm.PermProductView) {
			c.Abort()
			return
		}
		c.Next()
	}
	write := func(c *gin.Context) {
		if !adminperm.RequireWrite(c, db, adminperm.PermProductWrite) {
			c.Abort()
			return
		}
		c.Next()
	}
	g.GET("/providers/1688/auth-status", view, h.Get1688AuthStatus)
	g.POST("/providers/1688/open-login-browser", write, h.Open1688LoginBrowser)
	g.GET("/providers/pinduoduo/auth-status", view, h.GetPinduoduoAuthStatus)
	g.POST("/providers/pinduoduo/check-login", view, h.CheckPinduoduoLogin)
	g.POST("/providers/pinduoduo/open-login-browser", write, h.OpenPinduoduoLoginBrowser)
	g.POST("/providers/taobao_tmall/check-login", view, h.CheckTaobaoTmallLogin)
	g.POST("/providers/taobao_tmall/open-login-browser", write, h.OpenTaobaoTmallLoginBrowser)
}
