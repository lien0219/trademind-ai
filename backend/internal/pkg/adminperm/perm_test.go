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
	if !StrictHasPermission(RoleReviewer, PermOperationTaskReview) {
		t.Fatal("reviewer should review operation tasks")
	}
	if !StrictHasPermission(RoleReviewer, PermOperationTaskExecute) || !StrictHasPermission(RoleReviewer, PermOperationTaskRetry) {
		t.Fatal("reviewer should execute and retry operation tasks")
	}
	if StrictHasPermission(RoleReviewer, PermOperationTaskEdit) {
		t.Fatal("reviewer must not edit operation tasks")
	}
	if !StrictHasPermission(RoleOperator, PermOperationTaskEdit) {
		t.Fatal("operator should edit operation tasks")
	}
	if StrictHasPermission(RoleOperator, PermOperationTaskReview) || StrictHasPermission(RoleOperator, PermOperationTaskExecute) || StrictHasPermission(RoleOperator, PermOperationTaskRetry) {
		t.Fatal("operator must not review, execute, or retry operation tasks")
	}
	if !StrictHasPermission(RoleOperator, PermInventorySyncRun) || !StrictHasPermission(RoleOperator, PermInventorySyncRerun) || !StrictHasPermission(RoleOperator, PermSKUBindingManage) {
		t.Fatal("operator should run fixture inventory sync and manage SKU bindings")
	}
	if StrictHasPermission(RoleOperator, PermSKUBindingResolveManual) || StrictHasPermission(RoleOperator, PermInventorySyncAuditRead) {
		t.Fatal("operator must not resolve manual bindings or read inventory sync audit")
	}
	if !StrictHasPermission(RoleReviewer, PermSKUBindingResolveManual) || !StrictHasPermission(RoleReviewer, PermInventorySyncAuditRead) {
		t.Fatal("reviewer should resolve manual bindings and read inventory sync audit")
	}
	if StrictHasPermission(RoleReviewer, PermInventorySyncRun) || StrictHasPermission(RoleReviewer, PermInventorySyncRerun) || StrictHasPermission(RoleReadonly, PermInventorySyncRun) {
		t.Fatal("reviewer and readonly must not run inventory sync")
	}
	if !StrictHasPermission(RoleReadonly, PermInventorySyncRead) || !StrictHasPermission(RoleReadonly, PermInventorySnapshotRead) || !StrictHasPermission(RoleReadonly, PermSKUBindingRead) {
		t.Fatal("readonly should read inventory sync, snapshots, and SKU bindings")
	}
	if !StrictHasPermission(RoleOperator, PermProcurementManage) || !StrictHasPermission(RoleOperator, PermProcurementReceive) || StrictHasPermission(RoleOperator, PermProcurementApprove) {
		t.Fatal("operator should manage and receive purchase orders but must not approve them")
	}
	if !StrictHasPermission(RoleReviewer, PermProcurementApprove) || StrictHasPermission(RoleReviewer, PermProcurementManage) || StrictHasPermission(RoleReviewer, PermProcurementReceive) {
		t.Fatal("reviewer should approve purchase orders without editing or receiving them")
	}
	if !StrictHasPermission(RoleReadonly, PermWarehouseView) || !StrictHasPermission(RoleReadonly, PermSupplierView) || !StrictHasPermission(RoleReadonly, PermProcurementView) {
		t.Fatal("readonly should view ERP master data and purchase orders")
	}
	if StrictHasPermission(RoleReadonly, PermWarehouseManage) || StrictHasPermission(RoleReadonly, PermSupplierManage) || StrictHasPermission(RoleReadonly, PermProcurementManage) {
		t.Fatal("readonly must not mutate ERP resources")
	}
	if StrictHasPermission("surprise", PermOperationTaskReview) || StrictHasPermission("surprise", PermUserManage) || StrictHasPermission("surprise", PermInventorySyncRun) || StrictHasPermission(RoleAdmin, "inventory.run") {
		t.Fatal("unknown roles and synonymous permissions must not inherit permissions on strict path")
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
