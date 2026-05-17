package inventory

import (
	"time"

	"github.com/google/uuid"
)

// QueueMessage is Redis LIST payload for workers.
type QueueMessage struct {
	TaskID string `json:"taskId"`
}

// AdjustStockBody POST /products/:id/skus/:skuId/adjust-stock
type AdjustStockBody struct {
	Stock  int    `json:"stock"`
	Reason string `json:"reason"`
	Remark string `json:"remark"`
	Sync   bool   `json:"sync"`
}

// PublicationSkuSyncBody POST /product-publication-skus/:id/sync-inventory
type PublicationSkuSyncBody struct {
	Stock   int            `json:"stock"`
	Options map[string]any `json:"options"`
}

// ProductBatchInventoryBody POST /products/:id/sync-inventory
type ProductBatchInventoryBody struct {
	ShopID   string         `json:"shopId"`
	SKUIDs   []string       `json:"skuIds"`
	Options  map[string]any `json:"options"`
	UseLocal bool           `json:"useLocal"` // reserved; MVP always uses local snapshot
}

// ListQuery filters task list APIs.
type ListQuery struct {
	Page         int
	PageSize     int
	ProductID    *uuid.UUID
	ProductSKUID *uuid.UUID
	ShopID       *uuid.UUID
	Platform     string
	Status       string
	Start        *time.Time
	End          *time.Time
}

// TaskDTO is outward projection with joined labels for admin UI.
type TaskDTO struct {
	ID               uuid.UUID  `json:"id"`
	ProductID        uuid.UUID  `json:"productId"`
	ProductTitle     string     `json:"productTitle,omitempty"`
	ProductSKUID     *uuid.UUID `json:"productSkuId,omitempty"`
	SKUCode          string     `json:"skuCode,omitempty"`
	PublicationID    *uuid.UUID `json:"publicationId,omitempty"`
	PublicationSkuID *uuid.UUID `json:"publicationSkuId,omitempty"`
	ShopID           uuid.UUID  `json:"shopId"`
	ShopName         string     `json:"shopName,omitempty"`
	Platform         string     `json:"platform"`
	TaskType         string     `json:"taskType"`
	Status           string     `json:"status"`
	Mode             string     `json:"mode"`
	TargetStock      int        `json:"targetStock"`
	StartedAt        *time.Time `json:"startedAt,omitempty"`
	FinishedAt       *time.Time `json:"finishedAt,omitempty"`
	ErrorMessage     string     `json:"errorMessage,omitempty"`
	Input            any        `json:"input,omitempty"`
	Output           any        `json:"output,omitempty"`
	CreatedBy        *uuid.UUID `json:"createdBy,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
}

type ListTasksResult struct {
	Items      []TaskDTO `json:"items"`
	Total      int64     `json:"total"`
	Page       int       `json:"page"`
	PageSize   int       `json:"pageSize"`
	TotalPages int       `json:"totalPages"`
}

// ChangeLogDTO is one inventory_change_logs row projection.
type ChangeLogDTO struct {
	ID          uuid.UUID  `json:"id"`
	CreatedAt   time.Time  `json:"createdAt"`
	ChangeType  string     `json:"changeType"`
	BeforeStock int        `json:"beforeStock"`
	AfterStock  int        `json:"afterStock"`
	Delta       int        `json:"delta"`
	Reason      string     `json:"reason,omitempty"`
	Remark      string     `json:"remark,omitempty"`
	CreatedBy   *uuid.UUID `json:"createdBy,omitempty"`
}

// PublicationSkuListingRow lists platform mapping rows for SKU inventory UI.
type PublicationSkuListingRow struct {
	PublicationSKUID  uuid.UUID  `json:"publicationSkuId"`
	PublicationID     uuid.UUID  `json:"publicationId"`
	ProductSKUID      *uuid.UUID `json:"productSkuId,omitempty"`
	ShopID            uuid.UUID  `json:"shopId"`
	ShopName          string     `json:"shopName,omitempty"`
	Platform          string     `json:"platform"`
	ExternalProductID string     `json:"externalProductId,omitempty"`
	ExternalSKUID     string     `json:"externalSkuId,omitempty"`
	SKUCode           string     `json:"skuCode,omitempty"`
	PlatformStock     *int       `json:"platformStock,omitempty"`
	InventoryCap      string     `json:"inventorySyncCapability,omitempty"`
}

// GlobalLogsQuery optional filters for audit feed.
type GlobalLogsQuery struct {
	Page         int
	PageSize     int
	ProductID    *uuid.UUID
	ProductSKUID *uuid.UUID
	ChangeType   string
	Start        *time.Time
	End          *time.Time
}

// PaginatedLogs common pagination shell for changelog APIs.
type PaginatedLogs struct {
	Items      []ChangeLogDTO `json:"list"`
	Total      int64          `json:"total"`
	Page       int            `json:"page"`
	PageSize   int            `json:"pageSize"`
	TotalPages int            `json:"totalPages"`
}
