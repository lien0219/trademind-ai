package aiopsworkbench

const (
	TodoTypeAITextReview           = "ai_text_review"
	TodoTypeAITextConflict         = "ai_text_conflict"
	TodoTypeAIImageReview          = "ai_image_review"
	TodoTypeAIImageConflict        = "ai_image_conflict"
	TodoTypePublishCheckFailed     = "publish_check_failed"
	TodoTypePublishCheckWarning    = "publish_check_warning"
	TodoTypePublishBatchFailed     = "publish_batch_failed"
	TodoTypePublishBatchPartial    = "publish_batch_partial_success"
	TodoTypeTaskCenterFailure      = "taskcenter_failure"
	TodoIssuePendingReview         = "pending_review"
	TodoIssueQualityWarning        = "quality_warning"
	TodoIssueConflict              = "conflict"
	TodoIssueFailed                = "failed"
	TodoIssuePartialSuccess        = "partial_success"
	TodoIssueBatchFailed           = "batch_failed"

	PriorityP0 = "P0"
	PriorityP1 = "P1"
	PriorityP2 = "P2"
	PriorityP3 = "P3"

	SourceAIText        = "ai_text"
	SourceAIImage       = "ai_image"
	SourcePublishCheck  = "publish_check"
	SourcePublishBatch  = "publish_batch"
	SourceTaskCenter    = "taskcenter"

	defaultPageSize     = 50
	maxPageSize         = 50
	maxMergePerSource   = 500
	maxPublishCheckScan = 200
)

var priorityRank = map[string]int{
	PriorityP0: 0,
	PriorityP1: 1,
	PriorityP2: 2,
	PriorityP3: 3,
}
