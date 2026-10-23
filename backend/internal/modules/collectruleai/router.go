package collectruleai

import (
	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/gorm"
)

// Register mounts AI collect rule routes (must register before /collect/rules/:id).
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	db := (*gorm.DB)(nil)
	if h.Svc != nil && h.Svc.Settings != nil {
		db = h.Svc.Settings.DB
	}
	guard := requireSettingsManage(db)
	g.POST("/collect/rules/ai-generate", guard, h.Generate)
	g.POST("/collect/rules/ai-generate-and-save", guard, h.GenerateAndSave)
}

func requireSettingsManage(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if db == nil {
			response.Fail(c, 500, response.CodeInternalError, "collect rule ai unavailable")
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
		if !adminperm.RequireWrite(c, db, adminperm.PermSettingsManage) {
			c.Abort()
			return
		}
		c.Next()
	}
}
