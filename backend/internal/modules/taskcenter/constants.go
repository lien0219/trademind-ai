package taskcenter

// Unified task kinds (aggregation key).
const (
	TaskTypeCollect             = "collect"
	TaskTypeImage               = "image"
	TaskTypeOrderSync           = "order_sync"
	TaskTypeCustomerMessageSync = "customer_message_sync"
	TaskTypeProductPublish      = "product_publish"
	TaskTypeInventorySync       = "inventory_sync"
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
)

const (
	maxErrorMessageLen   = 500
	maxRawSummaryLen     = 400
	maxBatchItems        = 50
	maxMergeFetchPerTbl  = 500
	staleRetryAfterDrift = 30 // minutes past next_retry_at before we label stale

	detailEventsCollectLimit = 10
)
