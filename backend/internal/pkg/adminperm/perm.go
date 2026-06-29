package adminperm

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	RoleAdmin    = "admin"
	RoleOperator = "operator"
	RoleReadonly = "readonly"
)

// CanViewProduct returns true when admin can view product drafts.
func CanViewProduct(c *gin.Context, db *gorm.DB) bool {
	p, _ := LoadPrincipal(c, db)
	return p != nil && p.Can(PermProductView)
}

// CanWriteProduct returns true when admin can mutate product drafts.
func CanWriteProduct(c *gin.Context, db *gorm.DB) bool {
	p, _ := LoadPrincipal(c, db)
	return p != nil && !p.IsReadonly() && p.Can(PermProductWrite)
}

// CanApplyAIText returns true when admin can apply AI text results.
func CanApplyAIText(c *gin.Context, db *gorm.DB) bool {
	p, _ := LoadPrincipal(c, db)
	return p != nil && !p.IsReadonly() && p.Can(PermAITextApply)
}

// CanApplyAIImage returns true when admin can apply AI image results.
func CanApplyAIImage(c *gin.Context, db *gorm.DB) bool {
	p, _ := LoadPrincipal(c, db)
	return p != nil && !p.IsReadonly() && p.Can(PermAIImageApply)
}

// CanCreatePublishDraft returns true when admin can create platform publish drafts.
func CanCreatePublishDraft(c *gin.Context, db *gorm.DB) bool {
	p, _ := LoadPrincipal(c, db)
	return p != nil && !p.IsReadonly() && p.Can(PermPublishCreateDraft)
}

// CanViewOrder returns true when admin can view orders.
func CanViewOrder(c *gin.Context, db *gorm.DB) bool {
	p, _ := LoadPrincipal(c, db)
	return p != nil && p.Can(PermOrderView)
}

// CanOperateOrder returns true when admin can mutate orders.
func CanOperateOrder(c *gin.Context, db *gorm.DB) bool {
	p, _ := LoadPrincipal(c, db)
	return p != nil && !p.IsReadonly() && p.Can(PermOrderOperate)
}

// CanWriteOrders is an alias for CanOperateOrder (F2 compat).
func CanWriteOrders(c *gin.Context, db *gorm.DB) bool {
	return CanOperateOrder(c, db)
}

// CanBindSKU returns true when admin can bind SKU matches.
func CanBindSKU(c *gin.Context, db *gorm.DB) bool {
	p, _ := LoadPrincipal(c, db)
	return p != nil && !p.IsReadonly() && p.Can(PermSKUBind)
}

// CanViewInventory returns true when admin can view inventory data.
func CanViewInventory(c *gin.Context, db *gorm.DB) bool {
	p, _ := LoadPrincipal(c, db)
	return p != nil && p.Can(PermInventoryView)
}

// CanOperateInventory returns true when admin can mutate inventory.
func CanOperateInventory(c *gin.Context, db *gorm.DB) bool {
	p, _ := LoadPrincipal(c, db)
	return p != nil && !p.IsReadonly() && p.Can(PermInventoryOperate)
}

// CanWriteInventory is an alias for CanOperateInventory (F3 compat).
func CanWriteInventory(c *gin.Context, db *gorm.DB) bool {
	return CanOperateInventory(c, db)
}

// CanViewCustomer returns true when admin can view customer conversations.
func CanViewCustomer(c *gin.Context, db *gorm.DB) bool {
	p, _ := LoadPrincipal(c, db)
	return p != nil && p.Can(PermCustomerView)
}

// CanOperateCustomer returns true when admin can send customer replies.
func CanOperateCustomer(c *gin.Context, db *gorm.DB) bool {
	p, _ := LoadPrincipal(c, db)
	return p != nil && !p.IsReadonly() && p.Can(PermCustomerOperate)
}

// CanWriteCustomer is an alias for CanOperateCustomer (F4 compat).
func CanWriteCustomer(c *gin.Context, db *gorm.DB) bool {
	return CanOperateCustomer(c, db)
}

// CanRetryTask returns true when admin can retry failed tasks.
func CanRetryTask(c *gin.Context, db *gorm.DB) bool {
	p, _ := LoadPrincipal(c, db)
	return p != nil && !p.IsReadonly() && p.Can(PermTaskRetry)
}

// CanManageSettings returns true when admin can change system settings.
func CanManageSettings(c *gin.Context, db *gorm.DB) bool {
	p, _ := LoadPrincipal(c, db)
	return p != nil && !p.IsReadonly() && p.Can(PermSettingsManage)
}

// CanManageUsers returns true when admin can manage users.
func CanManageUsers(c *gin.Context, db *gorm.DB) bool {
	p, _ := LoadPrincipal(c, db)
	return p != nil && !p.IsReadonly() && p.Can(PermUserManage)
}

// CanViewOperationLog returns true when admin can view audit logs.
func CanViewOperationLog(c *gin.Context, db *gorm.DB) bool {
	p, _ := LoadPrincipal(c, db)
	return p != nil && p.Can(PermOperationLogView)
}

// CanViewStore returns true when admin can view shop list/detail.
func CanViewStore(c *gin.Context, db *gorm.DB) bool {
	p, _ := LoadPrincipal(c, db)
	return p != nil && p.Can(PermStoreView)
}

// CanOperateStoreAuth returns true when admin can authorize shops.
func CanOperateStore(c *gin.Context, db *gorm.DB) bool {
	p, _ := LoadPrincipal(c, db)
	return p != nil && !p.IsReadonly() && p.Can(PermStoreOperate)
}
