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

// ListQuery binds GET /collect/tasks.
type ListQuery struct {
	Page     int
	PageSize int
	Status   string
	Source   string
	Keyword  string
}

// TaskDTO is API-facing task shape.
type TaskDTO struct {
	ID              uuid.UUID       `json:"id"`
	Source          string          `json:"source"`
	SourceURL       string          `json:"sourceUrl"`
	Status          string          `json:"status"`
	ResultProductID *uuid.UUID      `json:"resultProductId,omitempty"`
	RawResult       json.RawMessage `json:"rawResult,omitempty"`
	ErrorMessage    string          `json:"errorMessage,omitempty"`
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
		Source:          t.Source,
		SourceURL:       t.SourceURL,
		Status:          t.Status,
		ResultProductID: t.ResultProductID,
		RawResult:       raw,
		ErrorMessage:    t.ErrorMessage,
		CreatedBy:       t.CreatedBy,
		StartedAt:       t.StartedAt,
		FinishedAt:      t.FinishedAt,
		CreatedAt:       t.CreatedAt,
		UpdatedAt:       t.UpdatedAt,
	}
}
