package productpublish

const (
	TaskTypeProductPublish = "product_publish"

	ModeManual = "manual"

	TaskPending   = "pending"
	TaskRunning   = "running"
	TaskSuccess   = "success"
	TaskFailed    = "failed"
	TaskCancelled = "cancelled"
)

const (
	StatusDraft           = "draft"
	StatusPublishing      = "publishing"
	StatusPublishedRecord = "published"
	StatusPubFailed       = "failed"
	StatusRejected        = "rejected"
	StatusOffline         = "offline"
)
