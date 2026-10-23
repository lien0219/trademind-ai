package adminuser

import (
	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
)

// Register mounts admin user management routes.
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	var logs *operationlog.Service
	if h.Svc != nil {
		logs = h.Svc.OpLog
	}
	rg := g.Group("/admin/users")
	rg.GET("", h.List)
	rg.POST("", logs.AuditFailures("user.create", "admin_user", ""), h.Create)
	rg.GET("/:id", h.Get)
	rg.PATCH("/:id", logs.AuditFailures("user.update", "admin_user", "id"), h.Update)
	rg.PUT("/:id/store-permissions", logs.AuditFailures("user.store_permissions.set", "admin_user", "id"), h.SetStorePermissions)
	rg.POST("/:id/reset-password", logs.AuditFailures("user.password.reset", "admin_user", "id"), h.ResetPassword)
	roles := g.Group("/admin/roles")
	roles.GET("", h.ListRoles)
	roles.POST("", logs.AuditFailures("role.create", "admin_role", ""), h.CreateRole)
	roles.PATCH("/:code", logs.AuditFailures("role.update", "admin_role", "code"), h.UpdateRole)
	g.GET("/admin/permissions", h.PermissionCatalog)
}
