package taskcenter

import "time"

// UnifiedTaskDTO is the cross-module task projection for failure center APIs.
type UnifiedTaskDTO struct {
	ID                   string     `json:"id"`
	TaskType             string     `json:"taskType"`
	SourceTable          string     `json:"sourceTable"`
	SourceID             string     `json:"sourceId"`
	Title                string     `json:"title"`
	Platform             string     `json:"platform,omitempty"`
	ShopID               string     `json:"shopId,omitempty"`
	ShopName             string     `json:"shopName,omitempty"`
	RelatedResourceType  string     `json:"relatedResourceType,omitempty"`
	RelatedResourceID    string     `json:"relatedResourceId,omitempty"`
	RelatedResourceTitle string     `json:"relatedResourceTitle,omitempty"`
	Status               string     `json:"status"`
	NormalizedStatus     string     `json:"normalizedStatus"`
	Retryable            bool       `json:"retryable"`
	Ignored              bool       `json:"ignored"`
	Handled              bool       `json:"handled"`
	ErrorMessage         string     `json:"errorMessage,omitempty"`
	ErrorCode            string     `json:"errorCode,omitempty"`
	RetryCount           int        `json:"retryCount"`
	MaxRetries           int        `json:"maxRetries,omitempty"`
	LockedBy             string     `json:"lockedBy,omitempty"`
	LockedUntil          *time.Time `json:"lockedUntil,omitempty"`
	CreatedAt            time.Time  `json:"createdAt"`
	UpdatedAt            time.Time  `json:"updatedAt"`
	StartedAt            *time.Time `json:"startedAt,omitempty"`
	FinishedAt           *time.Time `json:"finishedAt,omitempty"`
	DetailURL            string     `json:"detailUrl,omitempty"`
	RetryAction          string     `json:"retryAction,omitempty"`
	RawSummary           string     `json:"rawSummary,omitempty"`
	SortKey              time.Time  `json:"-"`
	FailureCategory      string     `json:"failureCategory,omitempty"`
	Severity             string     `json:"severity,omitempty"`
	ClassificationReason string     `json:"classificationReason,omitempty"`
	MatchedRule          string     `json:"matchedRule,omitempty"`
	SuggestedAction      string     `json:"suggestedAction,omitempty"`
	AlertStatus          string     `json:"alertStatus,omitempty"`
	RelatedAlertID       string     `json:"relatedAlertId,omitempty"`
}

// FailuresSummary is returned with list and summary-only endpoints.
type FailuresSummary struct {
	TotalFailed       int64            `json:"totalFailed"`
	RetryingTotal     int64            `json:"retryingTotal"`
	StaleTotal        int64            `json:"staleTotal"`
	LeaseExpiredTotal int64            `json:"leaseExpiredTotal"`
	ByType            map[string]int64 `json:"byType"`
	ByPlatform        map[string]int64 `json:"byPlatform"`
	RetryableCount    int64            `json:"retryableCount"`
	IgnoredCount      int64            `json:"ignoredCount"`
	HandledCount      int64            `json:"handledCount"`
	LatestFailedAt    *time.Time       `json:"latestFailedAt,omitempty"`
}

// ListFailuresResult is paged list + summary for the current filter.
type ListFailuresResult struct {
	List    []UnifiedTaskDTO `json:"list"`
	Total   int64            `json:"total"`
	Summary FailuresSummary  `json:"summary"`
}

// FailureDetailDTO extends the list row with optional type-specific snippets.
type FailureDetailDTO struct {
	UnifiedTaskDTO
	Extra map[string]any `json:"extra,omitempty"`
}

// BatchRetryItem is one entry in batch retry.
type BatchRetryItem struct {
	TaskType string `json:"taskType"`
	ID       string `json:"id"`
}

// BatchRetryRequest is POST /failures/batch-retry body.
type BatchRetryRequest struct {
	Items []BatchRetryItem `json:"items"`
}

// BatchRetryOneResult records per-item outcome.
type BatchRetryOneResult struct {
	TaskType string `json:"taskType"`
	ID       string `json:"id"`
	OK       bool   `json:"ok"`
	Error    string `json:"error,omitempty"`
}

// BatchRetryResponse is POST /failures/batch-retry response.
type BatchRetryResponse struct {
	SuccessCount int                   `json:"successCount"`
	FailedCount  int                   `json:"failedCount"`
	Results      []BatchRetryOneResult `json:"results"`
}

// BatchMarkRequest is batch ignore / handle body.
type BatchMarkRequest struct {
	Items  []BatchRetryItem `json:"items"`
	Remark string           `json:"remark,omitempty"`
}

// MarkRemarkBody is ignore / handle single body.
type MarkRemarkBody struct {
	Remark string `json:"remark,omitempty"`
}
