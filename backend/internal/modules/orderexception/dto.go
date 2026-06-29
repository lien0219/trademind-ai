package orderexception

import "time"

// ListOrderExceptionsRequest binds GET /orders/exceptions.
type ListOrderExceptionsRequest struct {
	ExceptionType string
	Severity      string
	Status        string // open | handled | ignored (derived client-side filter hint; server applies marks)
	Platform      string
	ShopID        string
	OrderID       string
	Keyword       string
	Ignored       *bool // explicit filter
	Handled       *bool
	Start         *time.Time
	End           *time.Time
	Page          int
	PageSize      int
}

// ExceptionSummaryDTO is returned alongside paginated rows.
type ExceptionSummaryDTO struct {
	TotalOpen              int64 `json:"totalOpen"`
	SKUUnmatched           int64 `json:"skuUnmatched"`
	SKUAmbiguous           int64 `json:"skuAmbiguous"`
	InsufficientStock      int64 `json:"insufficientStock"`
	InventoryDeductFailed  int64 `json:"inventoryDeductFailed"`
	InventoryRestoreFailed int64 `json:"inventoryRestoreFailed"`
	InventorySyncFailed    int64 `json:"inventorySyncFailed"`
	OrderSyncPartial       int64 `json:"orderSyncPartialFailed"`
}

// ListOrderExceptionsResult is the list payload.
type ListOrderExceptionsResult struct {
	List    []OrderExceptionDTO `json:"list"`
	Total   int64               `json:"total"`
	Summary ExceptionSummaryDTO `json:"summary"`
}

// OrderExceptionDTO is the unified workbench row.
type OrderExceptionDTO struct {
	ID              string    `json:"id"`
	ExceptionType   string    `json:"exceptionType"`
	Severity        string    `json:"severity"`
	Status          string    `json:"status"`
	SourceType      string    `json:"sourceType"`
	SourceID        string    `json:"sourceId"`
	OrderID         string    `json:"orderId,omitempty"`
	OrderNo         string    `json:"orderNo,omitempty"`
	ExternalOrderID string    `json:"externalOrderId,omitempty"`
	Platform        string    `json:"platform,omitempty"`
	ShopID          string    `json:"shopId,omitempty"`
	ShopName        string    `json:"shopName,omitempty"`
	OrderItemID     string    `json:"orderItemId,omitempty"`
	ExternalItemID  string    `json:"externalItemId,omitempty"`
	ExternalSkuID   string    `json:"externalSkuId,omitempty"`
	SKUCode         string    `json:"skuCode,omitempty"`
	SKUName         string    `json:"skuName,omitempty"`
	ProductID       string    `json:"productId,omitempty"`
	ProductSkuID    string    `json:"productSkuId,omitempty"`
	ProductTitle    string    `json:"productTitle,omitempty"`
	LocalSkuCode    string    `json:"localSkuCode,omitempty"`
	Quantity        int       `json:"quantity,omitempty"`
	ErrorMessage    string    `json:"errorMessage,omitempty"`
	SuggestedAction string    `json:"suggestedAction,omitempty"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
	DetailURL       string    `json:"detailUrl"`
	OrderURL        string    `json:"orderUrl,omitempty"`
	TaskCenterURL   string    `json:"taskCenterUrl,omitempty"`
	SyncTaskID      string    `json:"syncTaskId,omitempty"`
	Handled         bool      `json:"handled"`
	Ignored         bool      `json:"ignored"`
}

// BindSKURequest binds POST .../bind-sku.
type BindSKURequest struct {
	ExceptionType       string `json:"exceptionType"`
	ProductSKUID        string `json:"productSkuId"`
	DeductInventory     *bool  `json:"deductInventory"` // nil => true (workbench default)
	SyncInventory       *bool  `json:"syncInventory"`   // nil => false
	AutoMarkHandled     *bool  `json:"autoMarkHandled"` // nil => true
	CandidateConfidence *int   `json:"candidateConfidence"`
	CandidateSource     string `json:"candidateSource"`
}

// HandleBody binds POST .../handle.
type HandleBody struct {
	ExceptionType string `json:"exceptionType"`
	Remark        string `json:"remark"`
}
