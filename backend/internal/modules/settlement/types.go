package settlement

import (
	"time"

	"github.com/google/uuid"
)

const (
	StatusMatched  = "matched"
	StatusPending  = "pending"
	StatusMismatch = "mismatch"
	StatusBlocked  = "blocked"

	DispositionNew       = "new"
	DispositionDuplicate = "duplicate"

	MaxCSVRows    = 1000
	MaxFileBytes  = 2 * 1024 * 1024
	MaxPageSize   = 100
	MaxExportRows = 5000
)

var CSVHeaders = []string{
	"external_transaction_id",
	"order_no",
	"currency",
	"order_gross_minor",
	"platform_fee_minor",
	"settlement_amount_minor",
	"settled_at",
}

type Scope struct {
	RestrictStoreScope bool
	AllowedShopIDs     []uuid.UUID
}

type CSVRow struct {
	Line                  int       `json:"line"`
	ExternalTransactionID string    `json:"externalTransactionId"`
	OrderNo               string    `json:"orderNo"`
	Currency              string    `json:"currency"`
	OrderGrossMinor       int64     `json:"orderGrossMinor"`
	PlatformFeeMinor      int64     `json:"platformFeeMinor"`
	SettlementAmountMinor int64     `json:"settlementAmountMinor"`
	SettledAt             time.Time `json:"settledAt"`
	RowHash               string    `json:"-"`
	Disposition           string    `json:"disposition"`
}

type ValidationIssue struct {
	Line    int    `json:"line"`
	Field   string `json:"field,omitempty"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type PreviewResult struct {
	FileName      string            `json:"fileName"`
	FileHash      string            `json:"fileHash"`
	ShopID        uuid.UUID         `json:"shopId"`
	ShopName      string            `json:"shopName"`
	Platform      string            `json:"platform"`
	Valid         bool              `json:"valid"`
	SourceRows    int               `json:"sourceRows"`
	NewRows       int               `json:"newRows"`
	DuplicateRows int               `json:"duplicateRows"`
	Rows          []CSVRow          `json:"rows"`
	Issues        []ValidationIssue `json:"issues"`
}

type ImportResult struct {
	Import   Import `json:"import"`
	Replayed bool   `json:"replayed"`
}

type ListQuery struct {
	Page     int
	PageSize int
	OrderNo  string
	Platform string
	ShopID   *uuid.UUID
	Currency string
	Status   string
	Start    *time.Time
	End      *time.Time
	Export   bool
}

type Issue struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ReconciliationRow struct {
	ID                    uuid.UUID  `json:"id"`
	ShopID                uuid.UUID  `json:"shopId"`
	ShopName              string     `json:"shopName"`
	Platform              string     `json:"platform"`
	OrderID               *uuid.UUID `json:"orderId,omitempty"`
	OrderNo               string     `json:"orderNo"`
	Currency              string     `json:"currency"`
	OrderAmountMinor      *int64     `json:"orderAmountMinor"`
	OrderGrossMinor       *int64     `json:"orderGrossMinor"`
	PlatformFeeMinor      *int64     `json:"platformFeeMinor"`
	SettlementAmountMinor *int64     `json:"settlementAmountMinor"`
	TransactionCount      int        `json:"transactionCount"`
	Status                string     `json:"status"`
	Issues                []Issue    `json:"issues"`
	FirstSettledAt        time.Time  `json:"firstSettledAt"`
	LastSettledAt         time.Time  `json:"lastSettledAt"`
	LastImportedAt        time.Time  `json:"lastImportedAt"`
}

type ReconciliationDetail struct {
	ReconciliationRow
	Transactions []Transaction `json:"transactions"`
}

type ListResult struct {
	List       []ReconciliationRow `json:"list"`
	Page       int                 `json:"page"`
	PageSize   int                 `json:"pageSize"`
	Total      int64               `json:"total"`
	TotalPages int                 `json:"totalPages"`
}

type PlatformFeeFact struct {
	OrderID          uuid.UUID
	ReconciliationID uuid.UUID
	TransactionIDs   []uuid.UUID
	AmountMinor      int64
	Currency         string
	Status           string
	SourceAt         time.Time
	ReasonCode       string
	Reason           string
}

type shopFact struct {
	ID       uuid.UUID `gorm:"column:id"`
	Name     string    `gorm:"column:shop_name"`
	Platform string    `gorm:"column:platform"`
}

type groupFact struct {
	ID       uuid.UUID `gorm:"column:reconciliation_id"`
	ShopID   uuid.UUID `gorm:"column:shop_id"`
	ShopName string    `gorm:"column:shop_name"`
	Platform string    `gorm:"column:platform"`
	OrderNo  string    `gorm:"column:order_no"`
}

type orderFact struct {
	ID               uuid.UUID `gorm:"column:id"`
	ReconciliationID uuid.UUID `gorm:"column:reconciliation_id"`
	Platform         string    `gorm:"column:platform"`
	Currency         string    `gorm:"column:currency"`
	TotalAmountText  string    `gorm:"column:total_amount_text"`
}
