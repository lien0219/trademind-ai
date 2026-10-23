package collectrule

import (
	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/gorm"
)

// Register mounts JWT-protected collect rule routes under /api/v1 (group g).
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	db := h.Svc.DB
	view := requireSettingsManage(db, false)
	write := requireSettingsManage(db, true)
	g.GET("/collect/rules", view, h.List)
	g.POST("/collect/rules", write, h.Create)
	g.GET("/collect/rules/:id", view, h.Get)
	g.PUT("/collect/rules/:id", write, h.Update)
	g.DELETE("/collect/rules/:id", write, h.Delete)
	g.POST("/collect/rules/:id/enable", write, h.Enable)
	g.POST("/collect/rules/:id/disable", write, h.Disable)
	g.POST("/collect/rules/:id/test", write, h.Test)
}

func requireSettingsManage(db *gorm.DB, write bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if db == nil {
			response.Fail(c, 500, response.CodeInternalError, "collect rules unavailable")
			c.Abort()
			return
		}
		tenantID, err := adminperm.TenantIDFromGin(c)
		if err != nil {
			response.HandleError(c, err)
			c.Abort()
			return
		}
		if tenantID != 0 {
			adminperm.DenySettings(c)
			c.Abort()
			return
		}
		allowed := adminperm.RequirePermission(c, db, adminperm.PermSettingsManage)
		if write {
			allowed = adminperm.RequireWrite(c, db, adminperm.PermSettingsManage)
		}
		if !allowed {
			c.Abort()
			return
		}
		c.Next()
	}
}
