package adminperm

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"gorm.io/gorm"
)

const (
	RoleAdmin    = "admin"
	RoleOperator = "operator"
	RoleReadonly = "readonly"
)

// RoleFromContext loads admin_users.role for the authenticated admin (defaults to admin when missing).
func RoleFromContext(c *gin.Context, db *gorm.DB) string {
	if c == nil || db == nil {
		return RoleAdmin
	}
	idStr, ok := c.Get(ctxkey.AdminID)
	if !ok {
		return RoleAdmin
	}
	s, _ := idStr.(string)
	uid, err := uuid.Parse(strings.TrimSpace(s))
	if err != nil || uid == uuid.Nil {
		return RoleAdmin
	}
	var row admin.AdminUser
	if err := db.WithContext(c.Request.Context()).Select("role").First(&row, "id = ?", uid).Error; err != nil {
		return RoleAdmin
	}
	r := strings.TrimSpace(strings.ToLower(row.Role))
	switch r {
	case RoleReadonly, RoleOperator, RoleAdmin:
		return r
	default:
		return RoleAdmin
	}
}

// CanWriteOrders returns false for readonly admins (F2 lightweight RBAC until F5).
func CanWriteOrders(c *gin.Context, db *gorm.DB) bool {
	return RoleFromContext(c, db) != RoleReadonly
}

// CanWriteInventory returns false for readonly admins (F3 lightweight RBAC until F5).
func CanWriteInventory(c *gin.Context, db *gorm.DB) bool {
	return RoleFromContext(c, db) != RoleReadonly
}
