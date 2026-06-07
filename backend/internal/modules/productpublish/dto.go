package productpublish

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/productcheck"
	"gorm.io/datatypes"
)

// PublishRequestBody POST /products/:id/publish
type PublishRequestBody struct {
	ShopID  string         `json:"shopId"`
	Options map[string]any `json:"options"`
	// Force is reserved for future use; errors from readiness checks cannot be bypassed in v1.
	Force bool `json:"force"`
}

// TaskDTO API shape for CRUD endpoints.
type TaskDTO struct {
	ID                uuid.UUID  `json:"id"`
	ProductID         uuid.UUID  `json:"productId"`
	ShopID            uuid.UUID  `json:"shopId"`
	TargetStoreID     uuid.UUID  `json:"targetStoreId"`
	ShopName          string     `json:"shopName,omitempty"`
	ProductTitle      string     `json:"productTitle,omitempty"`
	Platform          string     `json:"platform"`
	TargetPlatform    string     `json:"targetPlatform"`
	TaskType          string     `json:"taskType"`
	Status            string     `json:"status"`
	PublishStatus     string     `json:"publishStatus,omitempty"`
	Mode              string     `json:"mode"`
	PublishMode       string     `json:"publishMode,omitempty"`
	Title             string     `json:"title,omitempty"`
	Description       string     `json:"description,omitempty"`
	Images            any        `json:"images,omitempty"`
	SKUs              any        `json:"skus,omitempty"`
	Price             *float64   `json:"price,omitempty"`
	Currency          string     `json:"currency,omitempty"`
	CheckResult       any        `json:"checkResult,omitempty"`
	PlatformPayload   any        `json:"platformPayload,omitempty"`
	PlatformResult    any        `json:"platformResult,omitempty"`
	PlatformProductID string     `json:"platformProductId,omitempty"`
	PlatformRawError  any        `json:"platformRawError,omitempty"`
	Retryable         bool       `json:"retryable,omitempty"`
	RequestID         string     `json:"requestId,omitempty"`
	MappingSnapshot   any        `json:"mappingSnapshot,omitempty"`
	StartedAt         *time.Time `json:"startedAt,omitempty"`
	FinishedAt        *time.Time `json:"finishedAt,omitempty"`
	ErrorCode         string     `json:"errorCode,omitempty"`
	ErrorMessage      string     `json:"errorMessage,omitempty"`
	Input             any        `json:"input,omitempty"`
	Output            any        `json:"output,omitempty"`
	CreatedBy         *uuid.UUID `json:"createdBy,omitempty"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`
	// Readiness is present when the publish path ran a pre-check and there were warnings (no errors).
	Readiness *productcheck.CheckProductReadinessResult `json:"readiness,omitempty"`
}

type ListTasksQuery struct {
	Page      int
	PageSize  int
	ProductID *uuid.UUID
	ShopID    *uuid.UUID
	Platform  string
	Status    string
	Start     *time.Time
	End       *time.Time
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
