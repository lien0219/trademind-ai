package warehousefee

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"gorm.io/gorm"
)

const (
	MaxPageSize        = 100
	MaxJSONSafeInteger = int64(9007199254740991)
)

type FulfillmentReader interface {
	ListWarehouseFeeFacts(context.Context, *gorm.DB, int64, order.WarehouseFeeScope, order.WarehouseFeeFactQuery) (*order.WarehouseFeeFactList, error)
	GetWarehouseFeeFact(context.Context, *gorm.DB, int64, order.WarehouseFeeScope, uuid.UUID, bool) (*order.WarehouseFeeFact, error)
}

type CreateRateCardInput struct {
	WarehouseID               uuid.UUID `json:"warehouseId"`
	Code                      string    `json:"code"`
	Name                      string    `json:"name"`
	Currency                  string    `json:"currency"`
	OutboundBaseFeeMinor      int64     `json:"outboundBaseFeeMinor"`
	PickingFeePerItemMinor    int64     `json:"pickingFeePerItemMinor"`
	PackingFeePerPackageMinor int64     `json:"packingFeePerPackageMinor"`
}

type UpdateRateCardInput struct {
	ExpectedRevision          int    `json:"expectedRevision"`
	Name                      string `json:"name"`
	Currency                  string `json:"currency"`
	OutboundBaseFeeMinor      int64  `json:"outboundBaseFeeMinor"`
	PickingFeePerItemMinor    int64  `json:"pickingFeePerItemMinor"`
	PackingFeePerPackageMinor int64  `json:"packingFeePerPackageMinor"`
	Status                    string `json:"status"`
}

type RateCardDetail struct {
	RateCard
	Revisions []RateCardRevision `json:"revisions"`
}

type PreviewInput struct {
	OrderID          uuid.UUID `json:"orderId"`
	RateCardID       uuid.UUID `json:"rateCardId"`
	RateCardRevision int       `json:"rateCardRevision"`
}

type Preview struct {
	OrderID                   uuid.UUID  `json:"orderId"`
	OrderNo                   string     `json:"orderNo"`
	ShopID                    *uuid.UUID `json:"shopId,omitempty"`
	WarehouseID               uuid.UUID  `json:"warehouseId"`
	WarehouseCode             string     `json:"warehouseCode"`
	WarehouseName             string     `json:"warehouseName"`
	WaveID                    uuid.UUID  `json:"waveId"`
	WaveNo                    string     `json:"waveNo"`
	WaveRevision              int        `json:"waveRevision"`
	WaveOrderID               uuid.UUID  `json:"waveOrderId"`
	PackVerificationID        uuid.UUID  `json:"packVerificationId"`
	PackageCode               string     `json:"packageCode"`
	ItemQuantity              int        `json:"itemQuantity"`
	PackageQuantity           int        `json:"packageQuantity"`
	RateCardID                uuid.UUID  `json:"rateCardId"`
	RateCardCode              string     `json:"rateCardCode"`
	RateCardName              string     `json:"rateCardName"`
	RateCardRevision          int        `json:"rateCardRevision"`
	OutboundBaseFeeMinor      int64      `json:"outboundBaseFeeMinor"`
	PickingFeePerItemMinor    int64      `json:"pickingFeePerItemMinor"`
	PickingFeeMinor           int64      `json:"pickingFeeMinor"`
	PackingFeePerPackageMinor int64      `json:"packingFeePerPackageMinor"`
	PackingFeeMinor           int64      `json:"packingFeeMinor"`
	AmountMinor               int64      `json:"amountMinor"`
	Currency                  string     `json:"currency"`
	CalculationHash           string     `json:"calculationHash"`
}

type ConfirmInput struct {
	OrderID          uuid.UUID `json:"orderId"`
	RateCardID       uuid.UUID `json:"rateCardId"`
	RateCardRevision int       `json:"rateCardRevision"`
	WaveRevision     int       `json:"waveRevision"`
	CalculationHash  string    `json:"calculationHash"`
	IdempotencyKey   string    `json:"idempotencyKey"`
}

type ConfirmResult struct {
	Snapshot Snapshot `json:"snapshot"`
	Replayed bool     `json:"replayed"`
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

type Candidate struct {
	order.WarehouseFeeFact
	SnapshotID *uuid.UUID `json:"snapshotId,omitempty"`
	Confirmed  bool       `json:"confirmed"`
}

type CandidateList struct {
	List       []Candidate `json:"list"`
	Page       int         `json:"page"`
	PageSize   int         `json:"pageSize"`
	Total      int64       `json:"total"`
	TotalPages int         `json:"totalPages"`
}

type SnapshotListQuery struct {
	Page        int
	PageSize    int
	OrderNo     string
	WarehouseID *uuid.UUID
	Currency    string
}

type SnapshotSummary struct {
	Snapshot
	AdjustmentCount int   `json:"adjustmentCount"`
	AdjustmentMinor int64 `json:"adjustmentMinor"`
	NetAmountMinor  int64 `json:"netAmountMinor"`
}

type SnapshotList struct {
	List       []SnapshotSummary `json:"list"`
	Page       int               `json:"page"`
	PageSize   int               `json:"pageSize"`
	Total      int64             `json:"total"`
	TotalPages int               `json:"totalPages"`
}

type SnapshotDetail struct {
	SnapshotSummary
	Adjustments []Adjustment `json:"adjustments"`
}

type ProfitabilityFact struct {
	OrderID       uuid.UUID
	SnapshotID    uuid.UUID
	AdjustmentIDs []uuid.UUID
	AmountMinor   int64
	Currency      string
	Status        string
	SourceAt      time.Time
	ReasonCode    string
	Reason        string
}
