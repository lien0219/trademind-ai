package adminperm

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
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
		// No auth context (unit tests / internal calls): preserve legacy admin scope behavior.
		p := &Principal{Role: RoleAdmin, Permissions: PermissionsForRole(RoleAdmin)}
		c.Set(ctxPrincipalKey, p)
		return p, nil
	}
	s, _ := idStr.(string)
	uid, err := uuid.Parse(strings.TrimSpace(s))
	if err != nil || uid == uuid.Nil {
		p := &Principal{Role: RoleReadonly, Permissions: PermissionsForRole(RoleReadonly)}
		c.Set(ctxPrincipalKey, p)
		return p, nil
	}

	var row admin.AdminUser
	if err := db.WithContext(c.Request.Context()).Select("id", "role", "status", "tenant_id", "token_version").First(&row, "id = ?", uid).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			p := &Principal{UserID: uid, Role: RoleReadonly, Disabled: true}
			c.Set(ctxPrincipalKey, p)
			return p, nil
		}
		return nil, err
	}
	platformOperator := false
	if db.Migrator().HasTable(&admin.PlatformOperator{}) {
		var operator admin.PlatformOperator
		err := db.WithContext(c.Request.Context()).Where("user_id = ? AND enabled = ?", uid, true).First(&operator).Error
		if err == nil {
			platformOperator = true
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}
	requestTenant := row.TenantID
	if rawTenant, ok := c.Get(ctxkey.TenantID); ok {
		switch v := rawTenant.(type) {
		case int64:
			requestTenant = v
		case int:
			requestTenant = int64(v)
		default:
			requestTenant = -1
		}
	}
	registry, memberships := TenantSchemaState(db)
	root, err := IsRoot(c.Request.Context(), db, uid)
	if err != nil {
		return nil, err
	}
	if root && requestTenant > 0 && registry && memberships {
		var target admin.Tenant
		if err := db.WithContext(c.Request.Context()).First(&target, "id = ?", requestTenant).Error; err != nil || target.Status != admin.TenantActive {
			return &Principal{UserID: uid, TenantID: requestTenant, Disabled: true}, nil
		}
		principal := &Principal{UserID: uid, TenantID: requestTenant, Role: RoleAdmin, PlatformOperator: true, Permissions: PermissionsForRole(RoleAdmin)}
		c.Set(ctxPrincipalKey, principal)
		return principal, nil
	}
	if registry && memberships && requestTenant == 0 && IsSystemAdministrator(&row) {
		// Do not put system capabilities in the tenant permission cache.
		p := &Principal{UserID: uid, TenantID: 0, Role: RoleAdmin, SystemContext: true,
			PlatformOperator: platformOperator, Permissions: systemPermissions()}
		c.Set(ctxPrincipalKey, p)
		return p, nil
	}
	if registry || memberships {
		// Once migration has created either tenant table, legacy AdminUser.Role
		// cannot authorize a positive tenant. Partial migration also fails closed.
		p := &Principal{UserID: uid, TenantID: requestTenant, Role: RoleReadonly, Disabled: true, PlatformOperator: platformOperator}
		if requestTenant > 0 && registry && memberships {
			access, err := ResolveTenantAccess(c.Request.Context(), db, uid, requestTenant)
			if err != nil {
				return nil, err
			}
			if access != nil && access.Allowed {
				row.Role = access.Membership.Role
				p.Disabled = false
			}
		}
		if p.Disabled {
			c.Set(ctxPrincipalKey, p)
			return p, nil
		}
	} else if requestTenant < 0 || row.TenantID != requestTenant {
		p := &Principal{UserID: uid, TenantID: requestTenant, Role: RoleReadonly, Disabled: true, PlatformOperator: platformOperator}
		c.Set(ctxPrincipalKey, p)
		return p, nil
	}
	// Every authenticated request reads the user security version from SQL.
	// Role changes atomically bump it for all affected users.
	cacheKey := permissionCacheKey(requestTenant, uid, row.TokenVersion, row.Status, row.Role, "")
	if cached, ok := getCachedPrincipal(cacheKey); ok {
		c.Set(ctxPrincipalKey, cached)
		return cached, nil
	}
	role := normalizeRole(row.Role)
	p := &Principal{
		UserID:           uid,
		TenantID:         requestTenant,
		Role:             role,
		PlatformOperator: platformOperator,
		Permissions:      PermissionsForRole(role),
	}
	if IsCustomRole(role) {
		var definition admin.RoleDefinition
		roleErr := db.WithContext(c.Request.Context()).Where("tenant_id = ? AND code = ? AND enabled = ?", requestTenant, role, true).First(&definition).Error
		if errors.Is(roleErr, gorm.ErrRecordNotFound) {
			p.Disabled = true
			p.Permissions = nil
		} else if roleErr != nil {
			return nil, fmt.Errorf("resolve custom role: %w", roleErr)
		} else {
			var decoded []string
			if err := json.Unmarshal([]byte(definition.PermissionsJSON), &decoded); err != nil {
				return nil, fmt.Errorf("invalid role permissions: %w", err)
			}
			grants, err := ValidateCustomPermissions(decoded)
			if err != nil {
				p.Disabled = true
				p.Permissions = nil
			} else {
				p.Permissions = grants
			}
		}
	}
	if !strings.EqualFold(strings.TrimSpace(row.Status), admin.StatusActive) {
		p.Disabled = true
		p.Permissions = nil
		c.Set(ctxPrincipalKey, p)
		putCachedPrincipal(cacheKey, p)
		return p, nil
	}
	if !p.Disabled && !p.IsAdmin() {
		var grants []admin.UserStorePermission
		if err := db.WithContext(c.Request.Context()).
			Where("user_id = ? AND store_id IN (SELECT id FROM shops WHERE tenant_id = ?)", uid, requestTenant).
			Order("created_at ASC").
			Find(&grants).Error; err != nil {
			return nil, fmt.Errorf("load store grants: %w", err)
		}
		p.StoreGrants = make([]StoreGrant, 0, len(grants))
		for _, g := range grants {
			p.StoreGrants = append(p.StoreGrants, StoreGrant{
				StoreID:         g.StoreID,
				Platform:        strings.TrimSpace(g.Platform),
				PermissionScope: admin.NormalizeStorePermScope(g.PermissionScope),
			})
		}
	}
	cacheKey = permissionCacheKey(requestTenant, uid, row.TokenVersion, row.Status, row.Role, storeGrantsFingerprint(p.StoreGrants))
	if cached, ok := getCachedPrincipal(cacheKey); ok {
		c.Set(ctxPrincipalKey, cached)
		return cached, nil
	}
	c.Set(ctxPrincipalKey, p)
	putCachedPrincipal(cacheKey, p)
	return p, nil
}

func storeGrantsFingerprint(grants []StoreGrant) string {
	if len(grants) == 0 {
		return ""
	}
	type row struct {
		StoreID string `json:"storeId"`
		Scope   string `json:"scope"`
		Plat    string `json:"platform"`
	}
	rows := make([]row, 0, len(grants))
	for _, g := range grants {
		rows = append(rows, row{
			StoreID: g.StoreID.String(),
			Scope:   strings.TrimSpace(strings.ToLower(g.PermissionScope)),
			Plat:    strings.TrimSpace(strings.ToLower(g.Platform)),
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].StoreID == rows[j].StoreID {
			return rows[i].Scope < rows[j].Scope
		}
		return rows[i].StoreID < rows[j].StoreID
	})
	raw, _ := json.Marshal(rows)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:8])
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
	if p.CanViewStore(storeID) && storeBelongsToRequestTenant(c, db, storeID) {
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
	if p.CanViewStore(*shopID) && storeBelongsToRequestTenant(c, db, *shopID) {
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
	if p.CanOperateStore(storeID) && storeBelongsToRequestTenant(c, db, storeID) {
		return true
	}
	if storeID != uuid.Nil && !p.CanViewStore(storeID) {
		response.Fail(c, 404, response.CodeNotFound, "资源不存在")
		return false
	}
	DenyStorePermission(c)
	return false
}

// storeBelongsToRequestTenant is a defense-in-depth check for grant rows. The
// grant table predates tenant_id and therefore cannot be trusted by itself.
func storeBelongsToRequestTenant(c *gin.Context, db *gorm.DB, storeID uuid.UUID) bool {
	if db == nil || storeID == uuid.Nil {
		return false
	}
	tid, err := TenantIDFromGin(c)
	if err != nil {
		return false
	}
	var tenant struct{ TenantID int64 }
	if err := db.WithContext(c.Request.Context()).Table("shops").Select("tenant_id").Where("id = ?", storeID).Take(&tenant).Error; err != nil {
		return false
	}
	return tenant.TenantID == tid
}
