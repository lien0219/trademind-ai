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
	WarehouseID    uuid.UUID `json:"warehouseId" binding:"required"`
	Stock          int       `json:"stock"`
	Reason         string    `json:"reason"`
	Remark         string    `json:"remark"`
	IdempotencyKey string    `json:"idempotencyKey" binding:"required"`
	Sync           bool      `json:"sync"`
}

// WarehouseBalanceDTO is one SKU balance at one warehouse.
type WarehouseBalanceDTO struct {
	WarehouseID   uuid.UUID `json:"warehouseId"`
	WarehouseCode string    `json:"warehouseCode"`
	WarehouseName string    `json:"warehouseName"`
	IsDefault     bool      `json:"isDefault"`
	OnHand        int       `json:"onHand"`
	Reserved      int       `json:"reserved"`
	InTransit     int       `json:"inTransit"`
	Damaged       int       `json:"damaged"`
	Available     int       `json:"available"`
	Version       int       `json:"version"`
}

// ManualAdjustmentResult reports both the warehouse fact and aggregate projection.
type ManualAdjustmentResult struct {
	ProductSKUID     uuid.UUID `json:"productSkuId"`
	WarehouseID      uuid.UUID `json:"warehouseId"`
	WarehouseOnHand  int       `json:"warehouseOnHand"`
	AggregateStock   int       `json:"aggregateStock"`
	MovementID       uuid.UUID `json:"movementId"`
	IdempotentReplay bool      `json:"idempotentReplay"`
}

// LegacyStockMigrationBody controls one bounded, repeatable migration batch.
type LegacyStockMigrationBody struct {
	Limit int `json:"limit"`
}

type LegacyStockMigrationResult struct {
	WarehouseID    uuid.UUID `json:"warehouseId"`
	WarehouseCode  string    `json:"warehouseCode"`
	MigratedCount  int       `json:"migratedCount"`
	RemainingCount int64     `json:"remainingCount"`
}

type WarehouseLedgerReconciliationRow struct {
	ProductID       uuid.UUID `json:"productId"`
	ProductTitle    string    `json:"productTitle"`
	ProductSKUID    uuid.UUID `json:"productSkuId"`
	SKUCode         string    `json:"skuCode"`
	SKUName         string    `json:"skuName"`
	AggregateStock  int       `json:"aggregateStock"`
	WarehouseOnHand int       `json:"warehouseOnHand"`
	Difference      int       `json:"difference"`
	BalanceCount    int       `json:"balanceCount"`
	Status          string    `json:"status"`
}

type WarehouseLedgerReconciliationResult struct {
	Items      []WarehouseLedgerReconciliationRow `json:"list"`
	Total      int64                              `json:"total"`
	Page       int                                `json:"page"`
	PageSize   int                                `json:"pageSize"`
	TotalPages int                                `json:"totalPages"`
	Matched    int64                              `json:"matched"`
	Unmigrated int64                              `json:"unmigrated"`
	Mismatch   int64                              `json:"mismatch"`
}

// PublicationSKUSyncBody POST /product-publication-skus/:id/sync-inventory
type PublicationSKUSyncBody struct {
	Stock              int            `json:"stock"`
	Options            map[string]any `json:"options"`
	FromInventoryAlert bool           `json:"fromInventoryAlert"`
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
	BatchID      *uuid.UUID
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
	PublicationSKUID *uuid.UUID `json:"publicationSkuId,omitempty"`
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
	BatchID          *uuid.UUID `json:"batchId,omitempty"`
	BatchNo          string     `json:"batchNo,omitempty"`
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
	ID             uuid.UUID  `json:"id"`
	CreatedAt      time.Time  `json:"createdAt"`
	ChangeType     string     `json:"changeType"`
	BeforeStock    int        `json:"beforeStock"`
	AfterStock     int        `json:"afterStock"`
	Delta          int        `json:"delta"`
	Reason         string     `json:"reason,omitempty"`
	Remark         string     `json:"remark,omitempty"`
	CreatedBy      *uuid.UUID `json:"createdBy,omitempty"`
	RefOrderID     *uuid.UUID `json:"refOrderId,omitempty"`
	RefOrderItemID *uuid.UUID `json:"refOrderItemId,omitempty"`
}

// PublicationSKUListingRow lists platform mapping rows for SKU inventory UI.
type PublicationSKUListingRow struct {
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
	BindStatus        string     `json:"bindStatus,omitempty"`
	BindConfidence    int        `json:"bindConfidence,omitempty"`
	BindMessage       string     `json:"bindMessage,omitempty"`
	LastSyncedAt      *time.Time `json:"lastSyncedAt,omitempty"`
}

// GlobalLogsQuery optional filters for audit feed.
type GlobalLogsQuery struct {
	TenantID     int64
	Page         int
	PageSize     int
	ProductID    *uuid.UUID
	ProductSKUID *uuid.UUID
	RefOrderID   *uuid.UUID
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

// PlatformStockAlertEntry is one mapped listing SKU line in an alert row.
type PlatformStockAlertEntry struct {
	PublicationSKUID    uuid.UUID  `json:"publicationSkuId"`
	ShopID              uuid.UUID  `json:"shopId"`
	ShopName            string     `json:"shopName,omitempty"`
	Platform            string     `json:"platform"`
	ExternalProductID   string     `json:"externalProductId,omitempty"`
	ExternalSKUID       string     `json:"externalSkuId,omitempty"`
	PlatformStock       *int       `json:"platformStock,omitempty"`
	PlatformStockStatus string     `json:"platformStockStatus"`
	LastSyncedAt        *time.Time `json:"lastSyncedAt,omitempty"`
	LastSyncTaskID      *uuid.UUID `json:"lastSyncTaskId,omitempty"`
	LastSyncStatus      string     `json:"lastSyncStatus,omitempty"`
	LastSyncError       string     `json:"lastSyncError,omitempty"`
	LastSyncAt          *time.Time `json:"lastSyncAt,omitempty"`
}

// InventoryAlertEntry is one local SKU row in the inventory alerts list.
type InventoryAlertEntry struct {
	ProductID             uuid.UUID                 `json:"productId"`
	ProductTitle          string                    `json:"productTitle"`
	ProductSKUID          uuid.UUID                 `json:"productSkuId"`
	SKUCode               string                    `json:"skuCode"`
	SKUName               string                    `json:"skuName"`
	Stock                 int                       `json:"stock"`
	WarningStock          int                       `json:"warningStock"`
	SafetyStock           int                       `json:"safetyStock"`
	StockStatus           string                    `json:"stockStatus"`
	AlertTypes            []string                  `json:"alertTypes"`
	PublicationCount      int                       `json:"publicationCount"`
	PlatformStocks        []PlatformStockAlertEntry `json:"platformStocks"`
	LastInventoryChangeAt *time.Time                `json:"lastInventoryChangeAt,omitempty"`
	LastSyncTaskID        *uuid.UUID                `json:"lastSyncTaskId,omitempty"`
	LastSyncStatus        string                    `json:"lastSyncStatus,omitempty"`
	LastSyncError         string                    `json:"lastSyncError,omitempty"`
	LastSyncAt            *time.Time                `json:"lastSyncAt,omitempty"`
}

// AlertsListQuery filters GET /inventory/alerts.
type AlertsListQuery struct {
	TenantID      int64
	Keyword       string
	ProductID     *uuid.UUID
	ProductSKUID  *uuid.UUID
	Platform      string
	ShopID        *uuid.UUID
	AlertType     string
	StockStatus   string
	OnlyPublished bool
	IncludeNormal bool
	Page          int
	PageSize      int
}

// AlertsListResult paginates alert rows.
type AlertsListResult struct {
	Items      []InventoryAlertEntry `json:"list"`
	Total      int64                 `json:"total"`
	Page       int                   `json:"page"`
	PageSize   int                   `json:"pageSize"`
	TotalPages int                   `json:"totalPages"`
}

// CreateInventorySyncBatchBody POST /inventory-sync/batches
type CreateInventorySyncBatchBody struct {
	Source            string         `json:"source"`
	Platform          string         `json:"platform"`
	ShopID            string         `json:"shopId"`
	ProductID         string         `json:"productId"`
	ProductSKUIDs     []string       `json:"productSkuIds"`
	PublicationSKUIDs []string       `json:"publicationSkuIds"`
	OnlyAlerts        bool           `json:"onlyAlerts"`
	AlertTypes        []string       `json:"alertTypes"`
	OnlyPublished     *bool          `json:"onlyPublished"`
	ConfirmAll        bool           `json:"confirmAll"`
	Force             bool           `json:"force"`
	Options           map[string]any `json:"options"`
}

func (b CreateInventorySyncBatchBody) effectiveOnlyPublished() bool {
	if b.OnlyPublished == nil {
		return true
	}
	return *b.OnlyPublished
}

// InventorySyncBatchDTO lists batch rows for admin APIs.
type InventorySyncBatchDTO struct {
	ID            uuid.UUID  `json:"id"`
	BatchNo       string     `json:"batchNo"`
	Source        string     `json:"source"`
	Status        string     `json:"status"`
	Platform      string     `json:"platform,omitempty"`
	ShopID        *uuid.UUID `json:"shopId,omitempty"`
	ShopName      string     `json:"shopName,omitempty"`
	ProductID     *uuid.UUID `json:"productId,omitempty"`
	TotalCount    int        `json:"totalCount"`
	PendingCount  int        `json:"pendingCount"`
	RunningCount  int        `json:"runningCount"`
	SuccessCount  int        `json:"successCount"`
	FailedCount   int        `json:"failedCount"`
	SkippedCount  int        `json:"skippedCount"`
	SkippedReason string     `json:"skippedReason,omitempty"`
	Input         any        `json:"input,omitempty"`
	Output        any        `json:"output,omitempty"`
	CreatedBy     *uuid.UUID `json:"createdBy,omitempty"`
	StartedAt     *time.Time `json:"startedAt,omitempty"`
	FinishedAt    *time.Time `json:"finishedAt,omitempty"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
	RecentTasks   []TaskDTO  `json:"recentTasks,omitempty"`
}

// InventorySyncBatchListQuery filters GET /inventory-sync/batches.
type InventorySyncBatchListQuery struct {
	TenantID  int64
	Source    string
	Status    string
	Platform  string
	ShopID    *uuid.UUID
	ProductID *uuid.UUID
	Start     *time.Time
	End       *time.Time
	Page      int
	PageSize  int
}

// InventorySyncBatchListResult paginates batches.
type InventorySyncBatchListResult struct {
	Items      []InventorySyncBatchDTO `json:"items"`
	Total      int64                   `json:"total"`
	Page       int                     `json:"page"`
	PageSize   int                     `json:"pageSize"`
	TotalPages int                     `json:"totalPages"`
}

// RetryInventorySyncTasksBatchBody POST batch retry-from-task-ids (≤100).
type RetryInventorySyncTasksBatchBody struct {
	TaskIDs []string `json:"taskIds"`
}

// StockSettingsBatchPreviewBody POST /inventory/stock-settings/batch-preview
type StockSettingsBatchPreviewBody struct {
	ProductID     string   `json:"productId"`
	ProductSKUIDs []string `json:"productSkuIds"`
	Platform      string   `json:"platform"`
	ShopID        string   `json:"shopId"`
	StockStatus   string   `json:"stockStatus"`
	AlertTypes    []string `json:"alertTypes"`
	Keyword       string   `json:"keyword"`
	OnlyPublished bool     `json:"onlyPublished"`
	IncludeNormal bool     `json:"includeNormal"`
	Page          int      `json:"page"`
	PageSize      int      `json:"pageSize"`
}

// StockSettingsBatchUpdateBody POST /inventory/stock-settings/batch-update
type StockSettingsBatchUpdateBody struct {
	StockSettingsBatchPreviewBody
	WarningStock int  `json:"warningStock"`
	SafetyStock  int  `json:"safetyStock"`
	Confirm      bool `json:"confirm"`
	ConfirmLarge bool `json:"confirmLarge"`
	ConfirmAll   bool `json:"confirmAll"`
}

// StockSettingsSampleSKU is a preview row without platform payloads.
type StockSettingsSampleSKU struct {
	ProductID    uuid.UUID `json:"productId"`
	ProductSKUID uuid.UUID `json:"productSkuId"`
	SKUCode      string    `json:"skuCode"`
	ProductTitle string    `json:"productTitle,omitempty"`
}

// StockSettingsBatchPreviewResult matches POST /inventory/stock-settings/batch-preview data.
type StockSettingsBatchPreviewResult struct {
	MatchedCount int64                    `json:"matchedCount"`
	SampleSKUs   []StockSettingsSampleSKU `json:"sampleSkus"`
	Page         int                      `json:"page"`
	PageSize     int                      `json:"pageSize"`
	TotalPages   int                      `json:"totalPages"`
}

// StockSettingsBatchUpdateResult matches POST /inventory/stock-settings/batch-update data.
type StockSettingsBatchUpdateResult struct {
	MatchedCount int64  `json:"matchedCount"`
	UpdatedCount int64  `json:"updatedCount"`
	Summary      string `json:"summary"`
}
