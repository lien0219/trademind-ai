package inventory

const (
	TaskTypeInventorySync = "inventory_sync"
)

const (
	StatusPending   = "pending"
	StatusRunning   = "running"
	StatusSuccess   = "success"
	StatusFailed    = "failed"
	StatusCancelled = "cancelled"
)

const (
	ModeManual       = "manual"
	ModePublication  = "publication"
	ModeSKU          = "sku"
	ModeProductBatch = "product_batch"
)

const (
	ChangeManualAdjust = "manual_adjust"
	ChangeSyncSuccess  = "sync_success"
	ChangeSyncFailed   = "sync_failed"
	ChangeOrderDeduct  = "order_deduct"
	ChangeOrderCancel  = "order_cancel_restore"
	ChangeImport       = "import"
)

// Platform-side snapshot status vs local SKU stock (alerts only).
const (
	PlatformStockUnknown  = "platform_stock_unknown"
	PlatformStockMismatch = "platform_stock_mismatch"
	PlatformStockSynced   = "platform_stock_synced"
)

// SKU-level alert type tags returned by GET /inventory/alerts.
const (
	AlertTypeOutOfStock            = "out_of_stock"
	AlertTypeLowStock              = "low_stock"
	AlertTypeBelowSafetyStock      = "below_safety_stock"
	AlertTypePlatformStockMismatch = "platform_stock_mismatch"
	AlertTypePlatformStockUnknown  = "platform_stock_unknown"
	AlertTypeInventorySyncFailed   = "inventory_sync_failed"
)
