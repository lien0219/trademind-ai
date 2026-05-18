package orderexception

// Exception types surfaced by the workbench.
const (
	TypeSKUUnmatched           = "sku_unmatched"
	TypeSKUAmbiguous           = "sku_ambiguous"
	TypeInsufficientStock      = "insufficient_stock"
	TypeInventoryDeductFailed  = "inventory_deduct_failed"
	TypeInventoryRestoreFailed = "inventory_restore_failed"
	TypeInventorySyncFailed    = "inventory_sync_failed"
	TypeOrderSyncPartialFailed = "order_sync_partial_failed"
	TypeMissingOrderItem       = "missing_order_item"
	TypeUnknown                = "unknown"
)

// Source references (maps to underlying rows).
const (
	SourceOrderItemSKUMatch    = "order_item_sku_match"
	SourceOrderItem            = "order_item"
	SourceOrderInventoryEffect = "order_inventory_effect"
	SourceInventorySyncTask    = "inventory_sync_task"
	SourceOrderSyncTask        = "order_sync_task"
	SourceOrder                = "order"
)

const (
	MarkHandled = "handled"
	MarkIgnored = "ignored"
)

const (
	StatusOpen    = "open"
	StatusHandled = "handled"
	StatusIgnored = "ignored"
)

const (
	SeverityLow      = "low"
	SeverityMedium   = "medium"
	SeverityHigh     = "high"
	SeverityCritical = "critical"
)
