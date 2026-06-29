package adminperm

import (
	"testing"

	"github.com/google/uuid"
)

func TestPermissionsForRole(t *testing.T) {
	if !HasPermission(RoleAdmin, PermUserManage) {
		t.Fatal("admin should manage users")
	}
	if len(PermissionsForRole(RoleAdmin)) < 10 {
		t.Fatal("admin perms too short")
	}
	if HasPermission(RoleOperator, PermSettingsManage) {
		t.Fatal("operator must not manage settings")
	}
	if !HasPermission(RoleOperator, PermOrderOperate) {
		t.Fatal("operator should operate orders")
	}
	if HasPermission(RoleReadonly, PermProductWrite) {
		t.Fatal("readonly must not write products")
	}
	if !HasPermission(RoleReadonly, PermOrderView) {
		t.Fatal("readonly should view orders")
	}
}

func TestPrincipalStoreAccess(t *testing.T) {
	sid := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	other := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	p := &Principal{
		Role: RoleOperator,
		StoreGrants: []StoreGrant{
			{StoreID: sid, PermissionScope: "operate"},
		},
	}
	if !p.CanViewStore(sid) {
		t.Fatal("should view granted store")
	}
	if !p.CanOperateStore(sid) {
		t.Fatal("should operate granted store")
	}
	if p.CanViewStore(other) {
		t.Fatal("must not view other store")
	}
	if p.CanOperateStore(other) {
		t.Fatal("must not operate other store")
	}
}
