package advertisingfee

import (
	"time"

	"github.com/google/uuid"
)

const (
	PolicyVersion    = "shop_day_equal_paid_order_v1"
	DispositionNew   = "new"
	DispositionDup   = "duplicate"
	MaxCSVRows       = 1000
	MaxFileBytes     = 2 * 1024 * 1024
	MaxPageSize      = 100
	MaxJSONSafeInt64 = int64(9007199254740991)
)

var CSVHeaders = []string{"spend_date", "currency", "spend_minor", "settlement_coverage"}

type Scope struct {
	RestrictStoreScope bool
	AllowedShopIDs     []uuid.UUID
}

type CSVRow struct {
	Line               int    `json:"line"`
	SpendDate          string `json:"spendDate"`
	Currency           string `json:"currency"`
	SpendMinor         int64  `json:"spendMinor"`
	SettlementCoverage string `json:"settlementCoverage"`
	RowHash            string `json:"-"`
	Disposition        string `json:"disposition"`
}

type ValidationIssue struct {
	Line    int    `json:"line"`
	Field   string `json:"field,omitempty"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type PreviewOrder struct {
	Line        int        `json:"line"`
	SpendDate   string     `json:"spendDate"`
	OrderID     uuid.UUID  `json:"orderId"`
	OrderNo     string     `json:"orderNo"`
	PaidAt      *time.Time `json:"paidAt,omitempty"`
	Currency    string     `json:"currency"`
	AmountMinor *int64     `json:"amountMinor"`
	Included    bool       `json:"included"`
	ReasonCode  string     `json:"reasonCode,omitempty"`
	Reason      string     `json:"reason,omitempty"`
}

type SpendPreview struct {
	CSVRow
	EligibleOrderCount int    `json:"eligibleOrderCount"`
	ExcludedOrderCount int    `json:"excludedOrderCount"`
	CalculationHash    string `json:"calculationHash,omitempty"`
}

type PreviewResult struct {
	FileName        string            `json:"fileName"`
	FileHash        string            `json:"fileHash"`
	CalculationHash string            `json:"calculationHash"`
	PolicyVersion   string            `json:"policyVersion"`
	ShopID          uuid.UUID         `json:"shopId"`
	ShopName        string            `json:"shopName"`
	Platform        string            `json:"platform"`
	ShopTimezone    string            `json:"shopTimezone"`
	Valid           bool              `json:"valid"`
	SourceRows      int               `json:"sourceRows"`
	NewSpends       int               `json:"newSpends"`
	DuplicateSpends int               `json:"duplicateSpends"`
	AllocationCount int               `json:"allocationCount"`
	Rows            []SpendPreview    `json:"rows"`
	Orders          []PreviewOrder    `json:"orders"`
	Issues          []ValidationIssue `json:"issues"`
	Warnings        []ValidationIssue `json:"warnings"`
}

type ImportResult struct {
	Import   Import `json:"import"`
	Replayed bool   `json:"replayed"`
}

type ListQuery struct {
	Page               int
	PageSize           int
	OrderNo            string
	ShopID             *uuid.UUID
	Currency           string
	SettlementCoverage string
}

type AllocationSummary struct {
	Allocation
	SpendDate          string `json:"spendDate"`
	ShopName           string `json:"shopName"`
	Platform           string `json:"platform"`
	ShopTimezone       string `json:"shopTimezone"`
	SettlementCoverage string `json:"settlementCoverage"`
	BaseSpendMinor     int64  `json:"baseSpendMinor"`
	AdjustmentCount    int    `json:"adjustmentCount"`
	AdjustmentMinor    int64  `json:"adjustmentMinor"`
	NetAmountMinor     int64  `json:"netAmountMinor"`
	ProfitStatus       string `json:"profitStatus"`
	ReasonCode         string `json:"reasonCode,omitempty"`
	Reason             string `json:"reason,omitempty"`
}

type AllocationList struct {
	List       []AllocationSummary `json:"list"`
	Page       int                 `json:"page"`
	PageSize   int                 `json:"pageSize"`
	Total      int64               `json:"total"`
	TotalPages int                 `json:"totalPages"`
}

type AllocationDetail struct {
	AllocationSummary
	Adjustments []Adjustment `json:"adjustments"`
}

type ImportList struct {
	List       []Import `json:"list"`
	Page       int      `json:"page"`
	PageSize   int      `json:"pageSize"`
	Total      int64    `json:"total"`
	TotalPages int      `json:"totalPages"`
}

type CreateAdjustmentInput struct {
	AmountMinor    int64  `json:"amountMinor"`
	Reason         string `json:"reason"`
	IdempotencyKey string `json:"idempotencyKey"`
}

type ReverseAdjustmentInput struct {
	Reason         string `json:"reason"`
	IdempotencyKey string `json:"idempotencyKey"`
}

type AdjustmentResult struct {
	Adjustment Adjustment `json:"adjustment"`
	Replayed   bool       `json:"replayed"`
}

type ProfitabilityFact struct {
	OrderID       uuid.UUID
	ImportID      uuid.UUID
	SpendID       uuid.UUID
	AllocationID  uuid.UUID
	AdjustmentIDs []uuid.UUID
	AmountMinor   int64
	Currency      string
	Status        string
	SourceAt      time.Time
	ReasonCode    string
	Reason        string
}
