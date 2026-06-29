package adminperm

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/gorm"
)

const ctxPrincipalKey = "adminperm.principal"

// LoadPrincipal resolves role, permissions and store grants for the current admin.
func LoadPrincipal(c *gin.Context, db *gorm.DB) (*Principal, error) {
	if c == nil {
		return &Principal{Role: RoleAdmin, Permissions: PermissionsForRole(RoleAdmin)}, nil
	}
	if cached, ok := c.Get(ctxPrincipalKey); ok {
		if p, ok := cached.(*Principal); ok && p != nil {
			return p, nil
		}
	}
	if db == nil {
		p := &Principal{Role: RoleAdmin, Permissions: PermissionsForRole(RoleAdmin)}
		c.Set(ctxPrincipalKey, p)
		return p, nil
	}
	idStr, ok := c.Get(ctxkey.AdminID)
	if !ok {
		p := &Principal{Role: RoleAdmin, Permissions: PermissionsForRole(RoleAdmin)}
		c.Set(ctxPrincipalKey, p)
		return p, nil
	}
	s, _ := idStr.(string)
	uid, err := uuid.Parse(strings.TrimSpace(s))
	if err != nil || uid == uuid.Nil {
		p := &Principal{Role: RoleAdmin, Permissions: PermissionsForRole(RoleAdmin)}
		c.Set(ctxPrincipalKey, p)
		return p, nil
	}

	var row admin.AdminUser
	if err := db.WithContext(c.Request.Context()).Select("id", "role", "status").First(&row, "id = ?", uid).Error; err != nil {
		p := &Principal{UserID: uid, Role: RoleAdmin, Permissions: PermissionsForRole(RoleAdmin)}
		c.Set(ctxPrincipalKey, p)
		return p, nil
	}
	role := normalizeRole(row.Role)
	p := &Principal{
		UserID:      uid,
		Role:        role,
		Permissions: PermissionsForRole(role),
	}
	if !p.IsAdmin() {
		var grants []admin.UserStorePermission
		_ = db.WithContext(c.Request.Context()).
			Where("user_id = ?", uid).
			Order("created_at ASC").
			Find(&grants).Error
		p.StoreGrants = make([]StoreGrant, 0, len(grants))
		for _, g := range grants {
			p.StoreGrants = append(p.StoreGrants, StoreGrant{
				StoreID:         g.StoreID,
				Platform:        strings.TrimSpace(g.Platform),
				PermissionScope: admin.NormalizeStorePermScope(g.PermissionScope),
			})
		}
	}
	c.Set(ctxPrincipalKey, p)
	return p, nil
}

// RoleFromContext loads admin_users.role for the authenticated admin (defaults to admin when missing).
func RoleFromContext(c *gin.Context, db *gorm.DB) string {
	p, _ := LoadPrincipal(c, db)
	if p == nil {
		return RoleAdmin
	}
	return p.Role
}

// ApplyStoreScope restricts query to allowed stores for non-admin principals.
// column is the SQL column name, e.g. "shop_id".
func ApplyStoreScope(c *gin.Context, db *gorm.DB, tx *gorm.DB, column string) (*gorm.DB, error) {
	if tx == nil {
		return tx, nil
	}
	p, err := LoadPrincipal(c, db)
	if err != nil {
		return nil, err
	}
	if p.IsAdmin() {
		return tx, nil
	}
	ids := p.AllowedStoreIDs()
	col := strings.TrimSpace(column)
	if col == "" {
		col = "shop_id"
	}
	if len(ids) == 0 {
		return tx.Where("1 = 0"), nil
	}
	return tx.Where(col+" IN ?", ids), nil
}

// RequireStoreView denies with 404 when store is inaccessible (no existence leak).
func RequireStoreView(c *gin.Context, db *gorm.DB, storeID uuid.UUID) bool {
	p, err := LoadPrincipal(c, db)
	if err != nil {
		response.HandleError(c, err)
		return false
	}
	if p.CanViewStore(storeID) {
		return true
	}
	response.Fail(c, 404, response.CodeNotFound, "资源不存在")
	return false
}

// EnsureStoreVisible returns gorm.ErrRecordNotFound when shop is out of scope.
func EnsureStoreVisible(c *gin.Context, db *gorm.DB, shopID *uuid.UUID) error {
	if shopID == nil || *shopID == uuid.Nil {
		return nil
	}
	p, err := LoadPrincipal(c, db)
	if err != nil {
		return err
	}
	if p.CanViewStore(*shopID) {
		return nil
	}
	return gorm.ErrRecordNotFound
}

// RequireStoreOperate denies when store write is not allowed.
func RequireStoreOperate(c *gin.Context, db *gorm.DB, storeID uuid.UUID) bool {
	p, err := LoadPrincipal(c, db)
	if err != nil {
		response.HandleError(c, err)
		return false
	}
	if p.CanOperateStore(storeID) {
		return true
	}
	if storeID != uuid.Nil && !p.CanViewStore(storeID) {
		response.Fail(c, 404, response.CodeNotFound, "资源不存在")
		return false
	}
	DenyStorePermission(c)
	return false
}
