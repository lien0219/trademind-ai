package salesreturn

import (
	"time"

	"github.com/google/uuid"
)

type CreateInput struct {
	IdempotencyKey string            `json:"idempotencyKey"`
	OrderID        uuid.UUID         `json:"orderId"`
	Type           string            `json:"type"`
	Reason         string            `json:"reason"`
	Remark         string            `json:"remark"`
	Items          []CreateItemInput `json:"items"`
}

type CreateItemInput struct {
	OrderItemID       uuid.UUID `json:"orderItemId"`
	Quantity          int       `json:"quantity"`
	Disposition       string    `json:"disposition"`
	RefundAmountMinor int64     `json:"refundAmountMinor"`
}

type ActionInput struct {
	ExpectedRevision int    `json:"expectedRevision"`
	IdempotencyKey   string `json:"idempotencyKey"`
	Reason           string `json:"reason"`
}

type ListRow struct {
	SalesReturn
	ItemCount int `json:"itemCount"`
}

type ListResult struct {
	List       []ListRow `json:"list"`
	Page       int       `json:"page"`
	PageSize   int       `json:"pageSize"`
	Total      int64     `json:"total"`
	TotalPages int       `json:"totalPages"`
}

type ReturnableItem struct {
	OrderItemID             uuid.UUID `json:"orderItemId"`
	ProductSKUID            uuid.UUID `json:"productSkuId"`
	ProductTitle            string    `json:"productTitle"`
	SKUCode                 string    `json:"skuCode"`
	SKUName                 string    `json:"skuName"`
	UnitPrice               float64   `json:"unitPrice"`
	DeductedQuantity        int       `json:"deductedQuantity"`
	RestoredQuantity        int       `json:"restoredQuantity"`
	AllocatedReturnQuantity int       `json:"allocatedReturnQuantity"`
	RemainingQuantity       int       `json:"remainingQuantity"`
}

type ReturnableItemResult struct {
	OrderID     uuid.UUID        `json:"orderId"`
	OrderNo     string           `json:"orderNo"`
	WarehouseID uuid.UUID        `json:"warehouseId"`
	Currency    string           `json:"currency"`
	List        []ReturnableItem `json:"list"`
}

type PlatformAfterSaleListQuery struct {
	TenantID             int64
	Platform             string
	PlatformShopID       string
	InternalShopID       *uuid.UUID
	AllowedShopIDs       []uuid.UUID
	RestrictStoreScope   bool
	ExternalOrderID      string
	PlatformStatus       string
	ReconciliationStatus string
	Start                *time.Time
	End                  *time.Time
	Page                 int
	PageSize             int
}

type PlatformAfterSaleListResult struct {
	List       []PlatformAfterSale `json:"list"`
	Page       int                 `json:"page"`
	PageSize   int                 `json:"pageSize"`
	Total      int64               `json:"total"`
	TotalPages int                 `json:"totalPages"`
}
