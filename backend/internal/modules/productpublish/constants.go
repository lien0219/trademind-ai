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
	StatusChecking        = "checking"
	StatusReady           = "ready"
	StatusPublishing      = "publishing"
	StatusPublishedRecord = "published"
	StatusSuccess         = "success"
	StatusPubFailed       = "failed"
	StatusRejected        = "rejected"
	StatusOffline         = "offline"
)
