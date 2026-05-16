package collect

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// CreateTaskBody binds POST /collect/tasks.
type CreateTaskBody struct {
	Source string `json:"source"`
	URL    string `json:"url"`
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
	ID             uuid.UUID  `json:"id"`
	Source         string     `json:"source"`
	TotalCount     int        `json:"totalCount"`
	PendingCount   int        `json:"pendingCount"`
	RunningCount   int        `json:"runningCount"`
	SuccessCount   int        `json:"successCount"`
	FailedCount    int        `json:"failedCount"`
	CancelledCount int        `json:"cancelledCount"`
	Status         string     `json:"status"`
	CreatedBy      *uuid.UUID `json:"createdBy,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
	FinishedAt     *time.Time `json:"finishedAt,omitempty"`
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
	ID              uuid.UUID       `json:"id"`
	BatchID         *uuid.UUID      `json:"batchId,omitempty"`
	Source          string          `json:"source"`
	SourceURL       string          `json:"sourceUrl"`
	Status          string          `json:"status"`
	ResultProductID *uuid.UUID      `json:"resultProductId,omitempty"`
	RawResult       json.RawMessage `json:"rawResult,omitempty"`
	ErrorMessage    string          `json:"errorMessage,omitempty"`
	RetryCount      int             `json:"retryCount"`
	MaxRetries      int             `json:"maxRetries"`
	NextRetryAt     *time.Time      `json:"nextRetryAt,omitempty"`
	RetryEnqueuedAt *time.Time      `json:"retryEnqueuedAt,omitempty"`
	CreatedBy       *uuid.UUID      `json:"createdBy,omitempty"`
	StartedAt       *time.Time      `json:"startedAt,omitempty"`
	FinishedAt      *time.Time      `json:"finishedAt,omitempty"`
	CreatedAt       time.Time       `json:"createdAt"`
	UpdatedAt       time.Time       `json:"updatedAt"`
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
