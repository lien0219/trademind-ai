package collect

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// CreateTaskBody binds POST /collect/tasks.
type CreateTaskBody struct {
	Source string  `json:"source"`
	URL    string  `json:"url"`
	RuleID *string `json:"ruleId,omitempty"`
}

// CreateBatchBody binds POST /collect/batches.
type CreateBatchBody struct {
	Source string   `json:"source"`
	URLs   []string `json:"urls"`
}

// BatchListQuery binds GET /collect/batches.
type BatchListQuery struct {
	Page     int
	PageSize int
	Status   string
	Source   string
	StartRFC string
	EndRFC   string
}

// BatchDTO is API-facing batch shape.
type BatchDTO struct {
	ID               uuid.UUID      `json:"id"`
	Source           string         `json:"source"`
	TotalCount       int            `json:"totalCount"`
	PendingCount     int            `json:"pendingCount"`
	RunningCount     int            `json:"runningCount"`
	SuccessCount     int            `json:"successCount"`
	FailedCount      int            `json:"failedCount"`
	CancelledCount   int            `json:"cancelledCount"`
	RetryingCount    int            `json:"retryingCount,omitempty"`
	BlockedCount     int            `json:"blockedCount,omitempty"`
	TimeoutCount     int            `json:"timeoutCount,omitempty"`
	ParseFailedCount int            `json:"parseFailedCount,omitempty"`
	ErrorSummary     map[string]int `json:"errorSummary,omitempty"`
	Status           string         `json:"status"`
	CreatedBy        *uuid.UUID     `json:"createdBy,omitempty"`
	CreatedAt        time.Time      `json:"createdAt"`
	UpdatedAt        time.Time      `json:"updatedAt"`
	FinishedAt       *time.Time     `json:"finishedAt,omitempty"`
}

// BatchListResult paginates batches.
type BatchListResult struct {
	Items      []BatchDTO `json:"list"`
	Total      int64      `json:"total"`
	Page       int        `json:"page"`
	PageSize   int        `json:"pageSize"`
	TotalPages int        `json:"totalPages"`
}

// CreateBatchResult is returned after creating a batch and tasks.
type CreateBatchResult struct {
	Batch     BatchDTO `json:"batch"`
	TaskCount int      `json:"taskCount"`
}

// RetryBatchFailedResult is returned from retry-failed.
type RetryBatchFailedResult struct {
	Retried int `json:"retried"`
}

// ListQuery binds GET /collect/tasks.
type ListQuery struct {
	Page     int
	PageSize int
	Status   string
	Source   string
	Keyword  string
	BatchID  string
}

// TaskDTO is API-facing task shape.
type TaskDTO struct {
	ID                        uuid.UUID       `json:"id"`
	BatchID                   *uuid.UUID      `json:"batchId,omitempty"`
	Source                    string          `json:"source"`
	SourceURL                 string          `json:"sourceUrl"`
	Status                    string          `json:"status"`
	ResultProductID           *uuid.UUID      `json:"resultProductId,omitempty"`
	RawResult                 json.RawMessage `json:"rawResult,omitempty"`
	ErrorMessage              string          `json:"errorMessage,omitempty"`
	CollectorErrorCode        string          `json:"collectorErrorCode,omitempty"`
	Retryable                 *bool           `json:"retryable,omitempty"`
	FailureHint               string          `json:"failureHint,omitempty"`
	SameUrlSucceededElsewhere bool            `json:"sameUrlSucceededElsewhere,omitempty"`
	RetryCount                int             `json:"retryCount"`
	MaxRetries                int             `json:"maxRetries"`
	RequestOptions            json.RawMessage `json:"requestOptions,omitempty"`
	NextRetryAt               *time.Time      `json:"nextRetryAt,omitempty"`
	RetryEnqueuedAt           *time.Time      `json:"retryEnqueuedAt,omitempty"`
	CreatedBy                 *uuid.UUID      `json:"createdBy,omitempty"`
	StartedAt                 *time.Time      `json:"startedAt,omitempty"`
	FinishedAt                *time.Time      `json:"finishedAt,omitempty"`
	CreatedAt                 time.Time       `json:"createdAt"`
	UpdatedAt                 time.Time       `json:"updatedAt"`
}

// ListResult paginates tasks.
type ListResult struct {
	Items      []TaskDTO `json:"list"`
	Total      int64     `json:"total"`
	Page       int       `json:"page"`
	PageSize   int       `json:"pageSize"`
	TotalPages int       `json:"totalPages"`
}

func taskToDTO(t *CollectTask) TaskDTO {
	var raw json.RawMessage
	if len(t.RawResult) > 0 {
		raw = json.RawMessage(t.RawResult)
	}
	var reqOpts json.RawMessage
	if len(t.RequestOptions) > 0 {
		reqOpts = json.RawMessage(t.RequestOptions)
	}
	return TaskDTO{
		ID:              t.ID,
		BatchID:         t.BatchID,
		Source:          t.Source,
		SourceURL:       t.SourceURL,
		Status:          t.Status,
		ResultProductID: t.ResultProductID,
		RawResult:       raw,
		ErrorMessage:    t.ErrorMessage,
		RetryCount:      t.RetryCount,
		MaxRetries:      t.MaxRetries,
		RequestOptions:  reqOpts,
		NextRetryAt:     t.NextRetryAt,
		RetryEnqueuedAt: t.RetryEnqueuedAt,
		CreatedBy:       t.CreatedBy,
		StartedAt:       t.StartedAt,
		FinishedAt:      t.FinishedAt,
		CreatedAt:       t.CreatedAt,
		UpdatedAt:       t.UpdatedAt,
	}
}

func batchToDTO(b *CollectBatch) BatchDTO {
	if b == nil {
		return BatchDTO{}
	}
	return BatchDTO{
		ID:             b.ID,
		Source:         b.Source,
		TotalCount:     b.TotalCount,
		PendingCount:   b.PendingCount,
		RunningCount:   b.RunningCount,
		SuccessCount:   b.SuccessCount,
		FailedCount:    b.FailedCount,
		CancelledCount: b.CancelledCount,
		Status:         b.Status,
		CreatedBy:      b.CreatedBy,
		CreatedAt:      b.CreatedAt,
		UpdatedAt:      b.UpdatedAt,
		FinishedAt:     b.FinishedAt,
	}
}

func batchToDetailDTO(b *CollectBatch, stats BatchStatsDTO) BatchDTO {
	dto := batchToDTO(b)
	dto.RetryingCount = stats.RetryingCount
	dto.BlockedCount = stats.BlockedCount
	dto.TimeoutCount = stats.TimeoutCount
	dto.ParseFailedCount = stats.ParseFailedCount
	if len(stats.ErrorSummary) > 0 {
		dto.ErrorSummary = stats.ErrorSummary
	}
	return dto
}
