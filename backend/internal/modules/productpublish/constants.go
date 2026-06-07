package productpublish

const (
	TaskTypeProductPublish    = "product_publish"
	TaskTypeDouyinDraftCreate = "douyin_draft_create"

	ModeManual = "manual"

	TaskPending   = "pending"
	TaskRunning   = "running"
	TaskSuccess   = "success"
	TaskFailed    = "failed"
	TaskCancelled = "cancelled"
)

const (
	StatusDraft                 = "draft"
	StatusChecking              = "checking"
	StatusMappingFields         = "mapping_fields"
	StatusCreatingPlatformDraft = "creating_platform_draft"
	StatusDraftCreated          = "draft_created"
	StatusReady                 = "ready"
	StatusPublishing            = "publishing"
	StatusPublishedRecord       = "published"
	StatusSuccess               = "success"
	StatusPubFailed             = "failed"
	StatusRejected              = "rejected"
	StatusOffline               = "offline"
)

const (
	BindStatusBound     = "bound"
	BindStatusUnmatched = "unmatched"
	BindStatusAmbiguous = "ambiguous"
	BindStatusSkipped   = "skipped"
	BindStatusFailed    = "failed"
)
