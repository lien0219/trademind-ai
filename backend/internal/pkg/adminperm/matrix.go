package adminperm

import "strings"

// Permission keys for role matrix and profile export.
const (
	PermProductView        = "product.view"
	PermProductWrite       = "product.write"
	PermAITextApply        = "ai_text.apply"
	PermAIImageApply       = "ai_image.apply"
	PermPublishCreateDraft = "publish.create_draft"
	PermOrderView          = "order.view"
	PermOrderOperate       = "order.operate"
	PermSKUBind            = "sku.bind"
	PermInventoryView      = "inventory.view"
	PermInventoryOperate   = "inventory.operate"
	PermInventoryApprove   = "inventory.approve"
	PermCustomerView       = "customer.view"
	PermCustomerOperate    = "customer.operate"
	PermTaskRetry          = "task.retry"
	PermSettingsManage     = "settings.manage"
	PermUserManage         = "user.manage"
	PermOperationLogView   = "operationlog.view"
	PermStoreView          = "store.view"
	PermStoreOperate       = "store.operate"
	// ERP foundation permissions
	PermWarehouseView      = "warehouse.view"
	PermWarehouseManage    = "warehouse.manage"
	PermSupplierView       = "supplier.view"
	PermSupplierManage     = "supplier.manage"
	PermProcurementView    = "procurement.view"
	PermProcurementManage  = "procurement.manage"
	PermProcurementApprove = "procurement.approve"
	PermProcurementReceive = "procurement.receive"
	PermProcurementReturn  = "procurement.return"
	// Security permissions
	PermSecuritySessionManage = "security.session.manage"
	PermSecurityKeyRotate     = "security.key.rotate"
	PermAuditRead             = "audit.read"
	PermAuditExport           = "audit.export"
	PermPIIReadMasked         = "pii.read_masked"
	PermPIIReadFull           = "pii.read_full"
	PermPIIExport             = "pii.export"
	PermConfigRead            = "config.read"
	PermConfigManage          = "config.manage"
	PermExportRead            = "export.read"
	PermExportCreate          = "export.create"
	// Observability permissions
	PermObservabilityRead   = "observability.read"
	PermObservabilityManage = "observability.manage"
	PermAlertsRead          = "alerts.read"
	PermAlertsAck           = "alerts.ack"
	PermAlertsSilence       = "alerts.silence"
	PermSLORead             = "slo.read"
	PermSLOManage           = "slo.manage"
	// Operation task permissions
	PermOperationTaskEdit      = "operationtask.edit"
	PermOperationTaskReview    = "operationtask.review"
	PermOperationTaskExecute   = "operationtask.execute"
	PermOperationTaskRetry     = "operationtask.retry"
	PermOperationTaskAuditRead = "operationtask.audit.read"
	// Inventory sync permissions
	PermInventorySyncRead       = "inventory_sync.read"
	PermInventorySyncRun        = "inventory_sync.run"
	PermInventorySyncRerun      = "inventory_sync.rerun"
	PermInventorySnapshotRead   = "inventory_snapshot.read"
	PermSKUBindingRead          = "sku_binding.read"
	PermSKUBindingManage        = "sku_binding.manage"
	PermSKUBindingResolveManual = "sku_binding.resolve_manual"
	PermInventorySyncAuditRead  = "inventory_sync.audit.read"
)

var allPermissions = []string{
	PermProductView,
	PermProductWrite,
	PermAITextApply,
	PermAIImageApply,
	PermPublishCreateDraft,
	PermOrderView,
	PermOrderOperate,
	PermSKUBind,
	PermInventoryView,
	PermInventoryOperate,
	PermInventoryApprove,
	PermCustomerView,
	PermCustomerOperate,
	PermTaskRetry,
	PermSettingsManage,
	PermUserManage,
	PermOperationLogView,
	PermStoreView,
	PermStoreOperate,
	PermWarehouseView,
	PermWarehouseManage,
	PermSupplierView,
	PermSupplierManage,
	PermProcurementView,
	PermProcurementManage,
	PermProcurementApprove,
	PermProcurementReceive,
	PermProcurementReturn,
	PermSecuritySessionManage,
	PermSecurityKeyRotate,
	PermAuditRead,
	PermAuditExport,
	PermPIIReadMasked,
	PermPIIReadFull,
	PermPIIExport,
	PermConfigRead,
	PermConfigManage,
	PermExportRead,
	PermExportCreate,
	PermObservabilityRead,
	PermObservabilityManage,
	PermAlertsRead,
	PermAlertsAck,
	PermAlertsSilence,
	PermSLORead,
	PermSLOManage,
	PermOperationTaskEdit,
	PermOperationTaskReview,
	PermOperationTaskExecute,
	PermOperationTaskRetry,
	PermOperationTaskAuditRead,
	PermInventorySyncRead,
	PermInventorySyncRun,
	PermInventorySyncRerun,
	PermInventorySnapshotRead,
	PermSKUBindingRead,
	PermSKUBindingManage,
	PermSKUBindingResolveManual,
	PermInventorySyncAuditRead,
}

var adminPermissions = append([]string(nil), allPermissions...)

var reviewerPermissions = []string{
	PermOperationLogView,
	PermAuditRead,
	PermOperationTaskReview,
	PermOperationTaskExecute,
	PermOperationTaskRetry,
	PermOperationTaskAuditRead,
	PermInventorySyncRead,
	PermInventoryView,
	PermInventoryApprove,
	PermInventorySnapshotRead,
	PermSKUBindingRead,
	PermSKUBindingResolveManual,
	PermInventorySyncAuditRead,
	PermWarehouseView,
	PermSupplierView,
	PermProcurementView,
	PermProcurementApprove,
}

var operatorPermissions = []string{
	PermProductView,
	PermProductWrite,
	PermAITextApply,
	PermAIImageApply,
	PermPublishCreateDraft,
	PermOrderView,
	PermOrderOperate,
	PermSKUBind,
	PermInventoryView,
	PermInventoryOperate,
	PermCustomerView,
	PermCustomerOperate,
	PermTaskRetry,
	PermOperationLogView,
	PermStoreView,
	PermStoreOperate,
	PermSecuritySessionManage,
	PermPIIReadMasked,
	PermAuditRead,
	PermConfigRead,
	PermObservabilityRead,
	PermAlertsRead,
	PermSLORead,
	PermOperationTaskEdit,
	PermOperationTaskAuditRead,
	PermInventorySyncRead,
	PermInventorySyncRun,
	PermInventorySyncRerun,
	PermInventorySnapshotRead,
	PermSKUBindingRead,
	PermSKUBindingManage,
	PermWarehouseView,
	PermWarehouseManage,
	PermSupplierView,
	PermSupplierManage,
	PermProcurementView,
	PermProcurementManage,
	PermProcurementReceive,
	PermProcurementReturn,
}

var readonlyPermissions = []string{
	PermProductView,
	PermOrderView,
	PermInventoryView,
	PermCustomerView,
	PermOperationLogView,
	PermStoreView,
	PermPIIReadMasked,
	PermAuditRead,
	PermConfigRead,
	PermObservabilityRead,
	PermAlertsRead,
	PermSLORead,
	PermOperationTaskAuditRead,
	PermInventorySyncRead,
	PermInventorySnapshotRead,
	PermSKUBindingRead,
	PermWarehouseView,
	PermSupplierView,
	PermProcurementView,
}

// PermissionsForRole returns granted permission keys for a role.
func PermissionsForRole(role string) []string {
	switch normalizeRole(role) {
	case RoleReadonly:
		return copyPermissions(readonlyPermissions)
	case RoleOperator:
		return copyPermissions(operatorPermissions)
	case RoleReviewer:
		return copyPermissions(reviewerPermissions)
	default:
		return copyPermissions(adminPermissions)
	}
}

func StrictPermissionsForRole(role string) []string {
	switch strictRole(role) {
	case RoleAdmin:
		return copyPermissions(adminPermissions)
	case RoleOperator:
		return copyPermissions(operatorPermissions)
	case RoleReadonly:
		return copyPermissions(readonlyPermissions)
	case RoleReviewer:
		return copyPermissions(reviewerPermissions)
	default:
		return []string{}
	}
}

// HasPermission checks whether role grants a permission key.
func HasPermission(role, perm string) bool {
	return permissionIn(PermissionsForRole(role), perm)
}

func StrictHasPermission(role, perm string) bool {
	return permissionIn(StrictPermissionsForRole(role), perm)
}

func copyPermissions(in []string) []string {
	out := make([]string, len(in))
	copy(out, in)
	return out
}

func permissionIn(perms []string, perm string) bool {
	perm = strings.TrimSpace(perm)
	if perm == "" {
		return false
	}
	for _, p := range perms {
		if p == perm {
			return true
		}
	}
	return false
}
