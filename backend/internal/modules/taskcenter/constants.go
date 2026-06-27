package taskcenter

// Unified task kinds (aggregation key).
const (
	TaskTypeCollect             = "collect"
	TaskTypeImage               = "image"
	TaskTypeOrderSync           = "order_sync"
	TaskTypeCustomerMessageSync = "customer_message_sync"
	TaskTypeProductPublish      = "product_publish"
	TaskTypeInventorySync       = "inventory_sync"
	TaskTypeAIText              = "ai_text"
	TaskTypeAIImage             = "ai_image"
)

// NormalizedStatus is a coarse status for operations views.
const (
	NormFailed       = "failed"
	NormRetrying     = "retrying"
	NormStale        = "stale"
	NormLeaseExpired = "lease_expired"
	NormCancelled    = "cancelled"
	NormSuccess      = "success"
	NormRunning      = "running"
	NormPending      = "pending"
)

const (
	MarkIgnored = "ignored"
	MarkHandled = "handled"
)

const (
	SourceTableCollectTasks             = "collect_tasks"
	SourceTableImageTasks               = "image_tasks"
	SourceTableOrderSyncTasks           = "order_sync_tasks"
	SourceTableCustomerMessageSyncTasks = "customer_message_sync_tasks"
	SourceTableProductPublishTasks      = "product_publish_tasks"
	SourceTableInventorySyncTasks       = "inventory_sync_tasks"
	SourceTableAIProductTextItems       = "ai_product_text_items"
	SourceTableAIProductImageItems      = "ai_product_image_items"
)

// AI product text failure categories (taskcenter dedup: task_type + source_id + failure_category).
const (
	CategoryAITextGenerationFailed = "ai_text_generation_failed"
	CategoryAITextApplyConflict    = "ai_text_apply_conflict"
	CategoryAITextApplyFailed      = "ai_text_apply_failed"
	CategoryAITextUndoFailed       = "ai_text_undo_failed"
	CategoryAITextQualityWarning   = "ai_text_quality_warning"
)

const (
	CategoryAIImageProcessFailed = "ai_image_process_failed"
	CategoryAIImageApplyConflict = "ai_image_apply_conflict"
	CategoryAIImageApplyFailed   = "ai_image_apply_failed"
	CategoryAIImageUndoFailed    = "ai_image_undo_failed"
	CategoryAIImageQualityWarn   = "ai_image_quality_warning"
)

const (
	maxErrorMessageLen   = 500
	maxRawSummaryLen     = 400
	maxBatchItems        = 50
	maxMergeFetchPerTbl  = 500
	staleRetryAfterDrift = 30 // minutes past next_retry_at before we label stale

	detailEventsCollectLimit = 10
)
