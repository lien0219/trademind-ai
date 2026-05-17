package productpublish

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// PublishRequestBody POST /products/:id/publish
type PublishRequestBody struct {
	ShopID  string                 `json:"shopId"`
	Options map[string]any `json:"options"`
}

// TaskDTO API shape for CRUD endpoints.
type TaskDTO struct {
	ID           uuid.UUID  `json:"id"`
	ProductID    uuid.UUID  `json:"productId"`
	ShopID       uuid.UUID  `json:"shopId"`
	ShopName     string     `json:"shopName,omitempty"`
	ProductTitle string     `json:"productTitle,omitempty"`
	Platform     string     `json:"platform"`
	TaskType     string     `json:"taskType"`
	Status       string     `json:"status"`
	Mode         string     `json:"mode"`
	StartedAt    *time.Time `json:"startedAt,omitempty"`
	FinishedAt   *time.Time `json:"finishedAt,omitempty"`
	ErrorMessage string     `json:"errorMessage,omitempty"`
	Input        any        `json:"input,omitempty"`
	Output       any        `json:"output,omitempty"`
	CreatedBy    *uuid.UUID `json:"createdBy,omitempty"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}

type ListTasksQuery struct {
	Page       int
	PageSize   int
	ProductID  *uuid.UUID
	ShopID     *uuid.UUID
	Platform   string
	Status     string
	Start      *time.Time
	End        *time.Time
}

type ListTasksResult struct {
	Items      []TaskDTO
	Total      int64
	Page       int
	PageSize   int
	TotalPages int
}

// PublicationDTO list row for drafts tab.
type PublicationDTO struct {
	ID                uuid.UUID  `json:"id"`
	ProductID         uuid.UUID  `json:"productId"`
	ShopID            uuid.UUID  `json:"shopId"`
	ShopName          string     `json:"shopName,omitempty"`
	Platform          string     `json:"platform"`
	PublishTaskID     *uuid.UUID `json:"publishTaskId,omitempty"`
	ExternalProductID string     `json:"externalProductId,omitempty"`
	ExternalURL       string     `json:"externalUrl,omitempty"`
	Status            string     `json:"status"`
	PublishStatus     string     `json:"publishStatus"`
	PublishedAt       *time.Time `json:"publishedAt,omitempty"`
	LastSyncedAt      *time.Time `json:"lastSyncedAt,omitempty"`
	SKUMappingSummary []string   `json:"skuMappingsSummary,omitempty"`
}

func pagesOf(total int64, ps int) int {
	if ps < 1 {
		ps = 20
	}
	pages := int(total) / ps
	if int(total)%ps != 0 {
		pages++
	}
	if pages == 0 && total > 0 {
		pages = 1
	}
	return pages
}

func dtoTrimJSON(raw datatypes.JSON) any {
	var v any
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	_ = json.Unmarshal(raw, &v)
	return v
}
