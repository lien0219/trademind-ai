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
