package freightfee

import (
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
)

const (
	FactAdjustment = "adjustment"
	FactReversal   = "reversal"
)

// Import records an explicitly confirmed carrier invoice upload.
type Import struct {
	model.HardDeleteBase
	TenantID        int64      `gorm:"not null;index;uniqueIndex:ux_freight_fee_import_key,priority:1;uniqueIndex:ux_freight_fee_import_file,priority:1" json:"tenantId"`
	ShopID          uuid.UUID  `gorm:"type:char(36);not null;index;uniqueIndex:ux_freight_fee_import_file,priority:2" json:"shopId"`
	FileName        string     `gorm:"size:255;not null" json:"fileName"`
	FileHash        string     `gorm:"size:64;not null;uniqueIndex:ux_freight_fee_import_file,priority:3" json:"fileHash"`
	PolicyVersion   string     `gorm:"size:64;not null" json:"policyVersion"`
	CalculationHash string     `gorm:"size:64;not null" json:"calculationHash"`
	IdempotencyKey  string     `gorm:"size:128;not null;uniqueIndex:ux_freight_fee_import_key,priority:2" json:"idempotencyKey"`
	RequestHash     string     `gorm:"size:64;not null" json:"-"`
	SourceRowCount  int        `gorm:"not null" json:"sourceRowCount"`
	ImportedRows    int        `gorm:"not null" json:"importedRows"`
	DuplicateRows   int        `gorm:"not null" json:"duplicateRows"`
	ImportedBy      *uuid.UUID `gorm:"type:char(36);index" json:"importedBy,omitempty"`
	ConfirmedAt     time.Time  `gorm:"not null;index" json:"confirmedAt"`
}

func (Import) TableName() string { return "freight_fee_imports" }

// Charge is one immutable final carrier charge for a shipment.
type Charge struct {
	model.HardDeleteBase
	TenantID       int64      `gorm:"not null;index;uniqueIndex:ux_freight_fee_external_line,priority:1;uniqueIndex:ux_freight_fee_shipment,priority:1" json:"tenantId"`
	ImportID       uuid.UUID  `gorm:"type:char(36);not null;index" json:"importId"`
	ShopID         uuid.UUID  `gorm:"type:char(36);not null;index;uniqueIndex:ux_freight_fee_external_line,priority:2" json:"shopId"`
	OrderID        uuid.UUID  `gorm:"type:char(36);not null;index" json:"orderId"`
	ShipmentID     uuid.UUID  `gorm:"type:char(36);not null;uniqueIndex:ux_freight_fee_shipment,priority:2" json:"shipmentId"`
	OrderNo        string     `gorm:"size:128;not null;index" json:"orderNo"`
	OrderCurrency  string     `gorm:"size:3;not null" json:"orderCurrency"`
	Carrier        string     `gorm:"size:128;not null;index;uniqueIndex:ux_freight_fee_external_line,priority:3" json:"carrier"`
	TrackingNo     string     `gorm:"size:255;not null;index" json:"trackingNo"`
	ExternalLineID string     `gorm:"size:128;not null;uniqueIndex:ux_freight_fee_external_line,priority:4" json:"externalLineId"`
	BilledAt       time.Time  `gorm:"not null;index" json:"billedAt"`
	AmountMinor    int64      `gorm:"not null" json:"amountMinor"`
	Currency       string     `gorm:"size:3;not null;index" json:"currency"`
	RowHash        string     `gorm:"size:64;not null" json:"-"`
	ImportedBy     *uuid.UUID `gorm:"type:char(36);index" json:"importedBy,omitempty"`
	ImportedAt     time.Time  `gorm:"not null;index" json:"importedAt"`
}

func (Charge) TableName() string { return "freight_fee_charges" }

// Adjustment appends a signed correction. A reversal can negate one
// adjustment exactly once.
type Adjustment struct {
	model.HardDeleteBase
	TenantID             int64      `gorm:"not null;index;uniqueIndex:ux_freight_fee_adjustment_key,priority:1" json:"tenantId"`
	ChargeID             uuid.UUID  `gorm:"type:char(36);not null;index" json:"chargeId"`
	ShopID               uuid.UUID  `gorm:"type:char(36);not null;index" json:"shopId"`
	OrderID              uuid.UUID  `gorm:"type:char(36);not null;index" json:"orderId"`
	FactType             string     `gorm:"size:24;not null;index" json:"factType"`
	AmountMinor          int64      `gorm:"not null" json:"amountMinor"`
	Currency             string     `gorm:"size:3;not null" json:"currency"`
	Reason               string     `gorm:"size:500;not null" json:"reason"`
	ReversesAdjustmentID *uuid.UUID `gorm:"type:char(36);uniqueIndex:ux_freight_fee_adjustment_reversal;index" json:"reversesAdjustmentId,omitempty"`
	IdempotencyKey       string     `gorm:"size:128;not null;uniqueIndex:ux_freight_fee_adjustment_key,priority:2" json:"idempotencyKey"`
	RequestHash          string     `gorm:"size:64;not null" json:"-"`
	ActorID              *uuid.UUID `gorm:"type:char(36);index" json:"actorId,omitempty"`
}

func (Adjustment) TableName() string { return "freight_fee_adjustments" }
