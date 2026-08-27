package order

import (
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
)

const (
	FulfillmentWaveDraft      = "draft"
	FulfillmentWavePicking    = "picking"
	FulfillmentWavePacking    = "packing"
	FulfillmentWaveCompleting = "completing"
	FulfillmentWavePartial    = "partial"
	FulfillmentWaveCompleted  = "completed"
	FulfillmentWaveCancelled  = "cancelled"

	FulfillmentWaveOrderPending     = "pending"
	FulfillmentWaveOrderBlocked     = "blocked"
	FulfillmentWaveOrderReadyToPack = "ready_to_pack"
	FulfillmentWaveOrderPacked      = "packed"
	FulfillmentWaveOrderFulfilled   = "fulfilled"
	FulfillmentWaveOrderFailed      = "failed"

	FulfillmentWaveLinePending  = "pending"
	FulfillmentWaveLinePicked   = "picked"
	FulfillmentWaveLineShortage = "shortage"
)

// FulfillmentWave is the persistent warehouse picking and packing aggregate.
// It never contains provider credentials and never represents an external
// carrier write; completion delegates to the existing local order fulfillment
// transaction only after an operator has recorded packing information.
type FulfillmentWave struct {
	model.Base
	TenantID       int64     `gorm:"not null;default:0;uniqueIndex:ux_fulfillment_wave_no,priority:1;index" json:"tenantId"`
	WaveNo         string    `gorm:"size:64;not null;uniqueIndex:ux_fulfillment_wave_no,priority:2" json:"waveNo"`
	WarehouseID    uuid.UUID `gorm:"type:char(36);not null;index" json:"warehouseId"`
	Status         string    `gorm:"size:32;not null;index" json:"status"`
	Revision       int       `gorm:"not null;default:1" json:"revision"`
	Remark         string    `gorm:"size:500" json:"remark,omitempty"`
	OrderCount     int       `gorm:"not null;default:0" json:"orderCount"`
	LineCount      int       `gorm:"not null;default:0" json:"lineCount"`
	RequiredQty    int       `gorm:"not null;default:0" json:"requiredQuantity"`
	PickedQty      int       `gorm:"not null;default:0" json:"pickedQuantity"`
	ShortageQty    int       `gorm:"not null;default:0" json:"shortageQuantity"`
	FulfilledCount int       `gorm:"not null;default:0" json:"fulfilledCount"`
	FailedCount    int       `gorm:"not null;default:0" json:"failedCount"`
	// PackingVerificationRequired is enabled for newly created waves. Keeping
	// the database default false lets waves created before this capability use
	// the legacy manual packing action without an unsafe migration backfill.
	PackingVerificationRequired bool `gorm:"not null;default:false" json:"packingVerificationRequired"`
	// CompletionBaseRevision pins all retries of one explicit completion run to
	// a server-owned idempotency identity. It is intentionally not exposed to
	// clients and is cleared when the run is finalized.
	CompletionBaseRevision int                               `gorm:"not null;default:0" json:"-"`
	CreatedBy              *uuid.UUID                        `gorm:"type:char(36);index" json:"createdBy,omitempty"`
	StartedBy              *uuid.UUID                        `gorm:"type:char(36);index" json:"startedBy,omitempty"`
	StartedAt              *time.Time                        `json:"startedAt,omitempty"`
	CompletedBy            *uuid.UUID                        `gorm:"type:char(36);index" json:"completedBy,omitempty"`
	CompletedAt            *time.Time                        `json:"completedAt,omitempty"`
	CancelledBy            *uuid.UUID                        `gorm:"type:char(36);index" json:"cancelledBy,omitempty"`
	CancelledAt            *time.Time                        `json:"cancelledAt,omitempty"`
	WarehouseCode          string                            `gorm:"-" json:"warehouseCode,omitempty"`
	WarehouseName          string                            `gorm:"-" json:"warehouseName,omitempty"`
	Orders                 []FulfillmentWaveOrder            `gorm:"foreignKey:WaveID" json:"orders,omitempty"`
	Lines                  []FulfillmentWaveLine             `gorm:"foreignKey:WaveID" json:"lines,omitempty"`
	PickScans              []FulfillmentWavePickScan         `gorm:"foreignKey:WaveID" json:"pickScans,omitempty"`
	PackVerifications      []FulfillmentWavePackVerification `gorm:"foreignKey:WaveID" json:"packVerifications,omitempty"`
	PackScans              []FulfillmentWavePackScan         `gorm:"foreignKey:WaveID" json:"packScans,omitempty"`
}

func (FulfillmentWave) TableName() string { return "fulfillment_waves" }

// FulfillmentWaveOrder stores an immutable order header snapshot plus the
// mutable packing/fulfillment result for that order.
type FulfillmentWaveOrder struct {
	model.HardDeleteBase
	TenantID          int64      `gorm:"not null;default:0;index" json:"tenantId"`
	WaveID            uuid.UUID  `gorm:"type:char(36);not null;uniqueIndex:ux_fulfillment_wave_order,priority:1;index" json:"waveId"`
	OrderID           uuid.UUID  `gorm:"type:char(36);not null;uniqueIndex:ux_fulfillment_wave_order,priority:2;index" json:"orderId"`
	ShopID            *uuid.UUID `gorm:"type:char(36);index" json:"shopId,omitempty"`
	OrderNo           string     `gorm:"size:128;not null" json:"orderNo"`
	Status            string     `gorm:"size:32;not null;index" json:"status"`
	Carrier           string     `gorm:"size:128" json:"carrier,omitempty"`
	TrackingNo        string     `gorm:"size:255" json:"trackingNo,omitempty"`
	TrackingURL       string     `gorm:"size:2048" json:"trackingUrl,omitempty"`
	PackageCode       string     `gorm:"size:255" json:"packageCode,omitempty"`
	ActualWeightGrams *int       `json:"actualWeightGrams,omitempty"`
	FailureCode       string     `gorm:"size:64" json:"failureCode,omitempty"`
	FailureReason     string     `gorm:"size:500" json:"failureReason,omitempty"`
	ShipmentID        *uuid.UUID `gorm:"type:char(36);index" json:"shipmentId,omitempty"`
	PackedBy          *uuid.UUID `gorm:"type:char(36);index" json:"packedBy,omitempty"`
	PackedAt          *time.Time `json:"packedAt,omitempty"`
	FulfilledAt       *time.Time `json:"fulfilledAt,omitempty"`
}

func (FulfillmentWaveOrder) TableName() string { return "fulfillment_wave_orders" }

// FulfillmentWaveLine is the immutable order/SKU snapshot and the operator's
// final pick result. Quantity corrections replace the same line rather than
// creating hidden stock movements.
type FulfillmentWaveLine struct {
	model.HardDeleteBase
	TenantID     int64      `gorm:"not null;default:0;index" json:"tenantId"`
	WaveID       uuid.UUID  `gorm:"type:char(36);not null;uniqueIndex:ux_fulfillment_wave_line,priority:1;index" json:"waveId"`
	WaveOrderID  uuid.UUID  `gorm:"type:char(36);not null;index" json:"waveOrderId"`
	OrderID      uuid.UUID  `gorm:"type:char(36);not null;index" json:"orderId"`
	OrderItemID  uuid.UUID  `gorm:"type:char(36);not null;uniqueIndex:ux_fulfillment_wave_line,priority:2" json:"orderItemId"`
	ProductID    *uuid.UUID `gorm:"type:char(36);index" json:"productId,omitempty"`
	ProductSKUID uuid.UUID  `gorm:"column:product_sku_id;type:char(36);not null;index" json:"productSkuId"`
	ProductTitle string     `gorm:"size:512" json:"productTitle,omitempty"`
	SKUCode      string     `gorm:"size:128" json:"skuCode,omitempty"`
	SKUName      string     `gorm:"size:512" json:"skuName,omitempty"`
	Barcode      string     `gorm:"size:128;index" json:"barcode,omitempty"`
	LocationID   *uuid.UUID `gorm:"type:char(36);index" json:"locationId,omitempty"`
	LocationCode string     `gorm:"size:64" json:"locationCode,omitempty"`
	LocationName string     `gorm:"size:160" json:"locationName,omitempty"`
	RequiredQty  int        `gorm:"not null" json:"requiredQuantity"`
	PickedQty    int        `gorm:"not null;default:0" json:"pickedQuantity"`
	ShortageQty  int        `gorm:"not null;default:0" json:"shortageQuantity"`
	Status       string     `gorm:"size:32;not null;index" json:"status"`
}

func (FulfillmentWaveLine) TableName() string { return "fulfillment_wave_lines" }

// FulfillmentWaveAssignment serializes active wave ownership per order. It is
// deleted only when the wave is cancelled or that order is fulfilled.
type FulfillmentWaveAssignment struct {
	model.HardDeleteBase
	TenantID int64     `gorm:"not null;default:0;uniqueIndex:ux_fulfillment_wave_assignment,priority:1" json:"tenantId"`
	OrderID  uuid.UUID `gorm:"type:char(36);not null;uniqueIndex:ux_fulfillment_wave_assignment,priority:2" json:"orderId"`
	WaveID   uuid.UUID `gorm:"type:char(36);not null;index" json:"waveId"`
}

func (FulfillmentWaveAssignment) TableName() string { return "fulfillment_wave_assignments" }

// FulfillmentWaveAction is the immutable audit/idempotency fact for atomic
// wave transitions other than create/complete, which use the shared service.
type FulfillmentWaveAction struct {
	model.HardDeleteBase
	TenantID         int64      `gorm:"not null;default:0;uniqueIndex:ux_fulfillment_wave_action,priority:1;index" json:"tenantId"`
	WaveID           uuid.UUID  `gorm:"type:char(36);not null;uniqueIndex:ux_fulfillment_wave_action,priority:2;index" json:"waveId"`
	Action           string     `gorm:"size:32;not null;index" json:"action"`
	IdempotencyKey   string     `gorm:"size:128;not null;uniqueIndex:ux_fulfillment_wave_action,priority:3" json:"idempotencyKey"`
	RequestHash      string     `gorm:"size:64;not null" json:"requestHash"`
	ExpectedRevision int        `gorm:"not null" json:"expectedRevision"`
	ResultRevision   int        `gorm:"not null" json:"resultRevision"`
	ActorID          *uuid.UUID `gorm:"type:char(36);index" json:"actorId,omitempty"`
}

func (FulfillmentWaveAction) TableName() string { return "fulfillment_wave_actions" }

// FulfillmentWavePickScan is an immutable validation fact for one recorded
// pick action. It preserves expected and operator-provided values for audit.
type FulfillmentWavePickScan struct {
	model.HardDeleteBase
	TenantID         int64      `gorm:"not null;index" json:"tenantId"`
	WaveID           uuid.UUID  `gorm:"type:char(36);not null;index" json:"waveId"`
	WaveLineID       uuid.UUID  `gorm:"type:char(36);not null;index" json:"waveLineId"`
	ActionID         uuid.UUID  `gorm:"type:char(36);not null;index" json:"actionId"`
	ExpectedBarcode  string     `gorm:"size:128" json:"expectedBarcode,omitempty"`
	ScannedBarcode   string     `gorm:"size:128" json:"scannedBarcode,omitempty"`
	ExpectedLocation string     `gorm:"size:64" json:"expectedLocationCode,omitempty"`
	ScannedLocation  string     `gorm:"size:64" json:"scannedLocationCode,omitempty"`
	PickedQty        int        `gorm:"not null" json:"pickedQuantity"`
	ShortageQty      int        `gorm:"not null" json:"shortageQuantity"`
	Validated        bool       `gorm:"not null;default:true" json:"validated"`
	ActorID          *uuid.UUID `gorm:"type:char(36);index" json:"actorId,omitempty"`
}

func (FulfillmentWavePickScan) TableName() string { return "fulfillment_wave_pick_scans" }

// FulfillmentWavePackVerification is an immutable header audit for one
// successful order/package verification. Re-verification is intentionally not
// supported after the order becomes packed.
type FulfillmentWavePackVerification struct {
	model.HardDeleteBase
	TenantID          int64      `gorm:"not null;index;uniqueIndex:ux_fulfillment_wave_pack_verification_order,priority:1" json:"tenantId"`
	WaveID            uuid.UUID  `gorm:"type:char(36);not null;index" json:"waveId"`
	WaveOrderID       uuid.UUID  `gorm:"type:char(36);not null;index;uniqueIndex:ux_fulfillment_wave_pack_verification_order,priority:2" json:"waveOrderId"`
	OrderID           uuid.UUID  `gorm:"type:char(36);not null;index" json:"orderId"`
	ActionID          uuid.UUID  `gorm:"type:char(36);not null;uniqueIndex" json:"actionId"`
	PackageCode       string     `gorm:"size:255;not null" json:"packageCode"`
	ScannedOrderNo    string     `gorm:"size:128;not null" json:"scannedOrderNo"`
	Carrier           string     `gorm:"size:128;not null" json:"carrier"`
	TrackingNo        string     `gorm:"size:255;not null" json:"trackingNo"`
	TrackingURL       string     `gorm:"size:2048" json:"trackingUrl,omitempty"`
	ActualWeightGrams *int       `json:"actualWeightGrams,omitempty"`
	LineCount         int        `gorm:"not null" json:"lineCount"`
	VerifiedQty       int        `gorm:"not null" json:"verifiedQuantity"`
	ActorID           *uuid.UUID `gorm:"type:char(36);index" json:"actorId,omitempty"`
}

func (FulfillmentWavePackVerification) TableName() string {
	return "fulfillment_wave_pack_verifications"
}

// FulfillmentWavePackScan preserves the expected frozen SKU/barcode and the
// operator-provided value used by one successful packing verification.
type FulfillmentWavePackScan struct {
	model.HardDeleteBase
	TenantID       int64      `gorm:"not null;index" json:"tenantId"`
	WaveID         uuid.UUID  `gorm:"type:char(36);not null;index" json:"waveId"`
	WaveOrderID    uuid.UUID  `gorm:"type:char(36);not null;index" json:"waveOrderId"`
	WaveLineID     uuid.UUID  `gorm:"type:char(36);not null;index;uniqueIndex:ux_fulfillment_wave_pack_scan_line,priority:2" json:"waveLineId"`
	VerificationID uuid.UUID  `gorm:"type:char(36);not null;index;uniqueIndex:ux_fulfillment_wave_pack_scan_line,priority:1" json:"verificationId"`
	ExpectedCode   string     `gorm:"size:128;not null" json:"expectedCode"`
	ScannedCode    string     `gorm:"size:128;not null" json:"scannedCode"`
	ExpectedQty    int        `gorm:"not null" json:"expectedQuantity"`
	VerifiedQty    int        `gorm:"not null" json:"verifiedQuantity"`
	Validated      bool       `gorm:"not null;default:true" json:"validated"`
	ActorID        *uuid.UUID `gorm:"type:char(36);index" json:"actorId,omitempty"`
}

func (FulfillmentWavePackScan) TableName() string { return "fulfillment_wave_pack_scans" }
