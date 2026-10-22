package adminuser

import "github.com/gin-gonic/gin"

// Register mounts admin user management routes.
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	rg := g.Group("/admin/users")
	rg.GET("", h.List)
	rg.POST("", h.Create)
	rg.GET("/:id", h.Get)
	rg.PATCH("/:id", h.Update)
	rg.PUT("/:id/store-permissions", h.SetStorePermissions)
	roles := g.Group("/admin/roles")
	roles.GET("", h.ListRoles)
	roles.POST("", h.CreateRole)
	roles.PATCH("/:code", h.UpdateRole)
	g.GET("/admin/permissions", h.PermissionCatalog)
}
