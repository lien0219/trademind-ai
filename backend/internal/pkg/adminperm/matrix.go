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
	PermCustomerView       = "customer.view"
	PermCustomerOperate    = "customer.operate"
	PermTaskRetry          = "task.retry"
	PermSettingsManage     = "settings.manage"
	PermUserManage         = "user.manage"
	PermOperationLogView   = "operationlog.view"
	PermStoreView          = "store.view"
	PermStoreOperate       = "store.operate"
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
	PermCustomerView,
	PermCustomerOperate,
	PermTaskRetry,
	PermSettingsManage,
	PermUserManage,
	PermOperationLogView,
	PermStoreView,
	PermStoreOperate,
}

var adminPermissions = append([]string(nil), allPermissions...)

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
}

var readonlyPermissions = []string{
	PermProductView,
	PermOrderView,
	PermInventoryView,
	PermCustomerView,
	PermOperationLogView,
	PermStoreView,
}

// PermissionsForRole returns granted permission keys for a role.
func PermissionsForRole(role string) []string {
	switch normalizeRole(role) {
	case RoleReadonly:
		out := make([]string, len(readonlyPermissions))
		copy(out, readonlyPermissions)
		return out
	case RoleOperator:
		out := make([]string, len(operatorPermissions))
		copy(out, operatorPermissions)
		return out
	default:
		out := make([]string, len(adminPermissions))
		copy(out, adminPermissions)
		return out
	}
}

// HasPermission checks whether role grants a permission key.
func HasPermission(role, perm string) bool {
	perm = strings.TrimSpace(perm)
	if perm == "" {
		return false
	}
	for _, p := range PermissionsForRole(role) {
		if p == perm {
			return true
		}
	}
	return false
}
