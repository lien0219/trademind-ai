package freightfee

import (
	"time"

	"github.com/google/uuid"
)

const (
	PolicyVersion    = "carrier_shipment_final_bill_v1"
	DispositionNew   = "new"
	DispositionDup   = "duplicate"
	MaxCSVRows       = 1000
	MaxFileBytes     = 2 * 1024 * 1024
	MaxPageSize      = 100
	MaxJSONSafeInt64 = int64(9007199254740991)
)

var CSVHeaders = []string{"external_line_id", "carrier", "tracking_no", "amount_minor", "currency", "billed_at"}

type Scope struct {
	RestrictStoreScope bool
	AllowedShopIDs     []uuid.UUID
}

type CSVRow struct {
	Line           int       `json:"line"`
	ExternalLineID string    `json:"externalLineId"`
	Carrier        string    `json:"carrier"`
	TrackingNo     string    `json:"trackingNo"`
	AmountMinor    int64     `json:"amountMinor"`
	Currency       string    `json:"currency"`
	BilledAt       time.Time `json:"billedAt"`
	RowHash        string    `json:"-"`
	Disposition    string    `json:"disposition"`
}

type ValidationIssue struct {
	Line    int    `json:"line"`
	Field   string `json:"field,omitempty"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ChargePreview struct {
	CSVRow
	OrderID       *uuid.UUID `json:"orderId,omitempty"`
	ShipmentID    *uuid.UUID `json:"shipmentId,omitempty"`
	OrderNo       string     `json:"orderNo,omitempty"`
	OrderCurrency string     `json:"orderCurrency,omitempty"`
	Status        string     `json:"status"`
	ReasonCode    string     `json:"reasonCode,omitempty"`
	Reason        string     `json:"reason,omitempty"`
}

type PreviewResult struct {
	FileName         string            `json:"fileName"`
	FileHash         string            `json:"fileHash"`
	CalculationHash  string            `json:"calculationHash"`
	PolicyVersion    string            `json:"policyVersion"`
	ShopID           uuid.UUID         `json:"shopId"`
	ShopName         string            `json:"shopName"`
	Valid            bool              `json:"valid"`
	SourceRows       int               `json:"sourceRows"`
	NewCharges       int               `json:"newCharges"`
	DuplicateCharges int               `json:"duplicateCharges"`
	Rows             []ChargePreview   `json:"rows"`
	Issues           []ValidationIssue `json:"issues"`
	Warnings         []ValidationIssue `json:"warnings"`
}

type ImportResult struct {
	Import   Import `json:"import"`
	Replayed bool   `json:"replayed"`
}

type ListQuery struct {
	Page       int
	PageSize   int
	OrderNo    string
	TrackingNo string
	ShopID     *uuid.UUID
	Currency   string
	Status     string
}

type ChargeSummary struct {
	Charge
	AdjustmentCount int    `json:"adjustmentCount"`
	AdjustmentMinor int64  `json:"adjustmentMinor"`
	NetAmountMinor  int64  `json:"netAmountMinor"`
	Status          string `json:"status"`
	ReasonCode      string `json:"reasonCode,omitempty"`
	Reason          string `json:"reason,omitempty"`
}

type ChargeList struct {
	List       []ChargeSummary `json:"list"`
	Page       int             `json:"page"`
	PageSize   int             `json:"pageSize"`
	Total      int64           `json:"total"`
	TotalPages int             `json:"totalPages"`
}

type ChargeDetail struct {
	ChargeSummary
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
	OrderID             uuid.UUID
	ChargeIDs           []uuid.UUID
	ImportIDs           []uuid.UUID
	AdjustmentIDs       []uuid.UUID
	AmountMinor         int64
	Currency            string
	Status              string
	ShipmentCount       int
	BilledShipmentCount int
	SourceAt            time.Time
	ReasonCode          string
	Reason              string
}
