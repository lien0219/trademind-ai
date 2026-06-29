package adminperm

import (
	"strings"

	"github.com/google/uuid"
)

// StoreGrant is one authorized shop binding for scoped roles.
type StoreGrant struct {
	StoreID         uuid.UUID `json:"storeId"`
	Platform        string    `json:"platform,omitempty"`
	PermissionScope string    `json:"permissionScope"`
}

// Principal is the resolved admin authorization context.
type Principal struct {
	UserID      uuid.UUID
	Role        string
	Permissions []string
	StoreGrants []StoreGrant
}

// IsAdmin returns true for global admin role.
func (p *Principal) IsAdmin() bool {
	return p != nil && normalizeRole(p.Role) == RoleAdmin
}

// IsReadonly returns true for readonly role.
func (p *Principal) IsReadonly() bool {
	return p != nil && normalizeRole(p.Role) == RoleReadonly
}

// Can returns true when principal has a permission key.
func (p *Principal) Can(perm string) bool {
	if p == nil {
		return false
	}
	return HasPermission(p.Role, perm)
}

// AllowedStoreIDs returns nil for admin (all stores), otherwise explicit store ids.
func (p *Principal) AllowedStoreIDs() []uuid.UUID {
	if p == nil || p.IsAdmin() {
		return nil
	}
	if len(p.StoreGrants) == 0 {
		return []uuid.UUID{}
	}
	seen := make(map[uuid.UUID]struct{}, len(p.StoreGrants))
	out := make([]uuid.UUID, 0, len(p.StoreGrants))
	for _, g := range p.StoreGrants {
		if g.StoreID == uuid.Nil {
			continue
		}
		if _, ok := seen[g.StoreID]; ok {
			continue
		}
		seen[g.StoreID] = struct{}{}
		out = append(out, g.StoreID)
	}
	return out
}

// CanViewStore checks read access to a store.
func (p *Principal) CanViewStore(storeID uuid.UUID) bool {
	if p == nil || storeID == uuid.Nil {
		return false
	}
	if p.IsAdmin() {
		return true
	}
	for _, g := range p.StoreGrants {
		if g.StoreID == storeID {
			return true
		}
	}
	return false
}

// CanOperateStore checks write access to a store-scoped resource.
func (p *Principal) CanOperateStore(storeID uuid.UUID) bool {
	if p == nil || storeID == uuid.Nil {
		return false
	}
	if p.IsReadonly() {
		return false
	}
	if p.IsAdmin() {
		return true
	}
	for _, g := range p.StoreGrants {
		if g.StoreID != storeID {
			continue
		}
		scope := strings.TrimSpace(strings.ToLower(g.PermissionScope))
		return scope == "operate" || scope == "manage"
	}
	return false
}

func normalizeRole(role string) string {
	r := strings.TrimSpace(strings.ToLower(role))
	switch r {
	case RoleReadonly, RoleOperator, RoleAdmin:
		return r
	default:
		return RoleAdmin
	}
}
