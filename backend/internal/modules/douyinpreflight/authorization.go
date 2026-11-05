package douyinpreflight

import (
	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"gorm.io/gorm"
)

// Global configuration is tenant-zero, while store and product probes
// are scoped to the trusted, selected business tenant. The singleton Root
// is allowed from any active workspace, not by faking tenant ID zero.
func authorizePreflight(c *gin.Context, db *gorm.DB, write bool) bool {
	permission := adminperm.PermStoreView
	if write {
		permission = adminperm.PermSettingsManage
	}
	return adminperm.RequirePlatformPermission(c, db, permission, write)
}
