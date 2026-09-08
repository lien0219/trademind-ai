package warehousefee

import (
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
)

const (
	StatusActive   = "active"
	StatusInactive = "inactive"

	FactAdjustment = "adjustment"
	FactReversal   = "reversal"
)

// RateCard is the current tenant and warehouse scoped warehouse operation fee rule.
// Every update also appends an immutable RateCardRevision before this projection moves.
type RateCard struct {
	model.Base
	TenantID                  int64      `gorm:"not null;uniqueIndex:ux_warehouse_fee_rate_card_code,priority:1;index" json:"tenantId"`
	WarehouseID               uuid.UUID  `gorm:"type:char(36);not null;uniqueIndex:ux_warehouse_fee_rate_card_code,priority:2;index" json:"warehouseId"`
	Code                      string     `gorm:"size:64;not null;uniqueIndex:ux_warehouse_fee_rate_card_code,priority:3" json:"code"`
	Name                      string     `gorm:"size:160;not null" json:"name"`
	Currency                  string     `gorm:"size:8;not null" json:"currency"`
	OutboundBaseFeeMinor      int64      `gorm:"not null;default:0" json:"outboundBaseFeeMinor"`
	PickingFeePerItemMinor    int64      `gorm:"not null;default:0" json:"pickingFeePerItemMinor"`
	PackingFeePerPackageMinor int64      `gorm:"not null;default:0" json:"packingFeePerPackageMinor"`
	Status                    string     `gorm:"size:24;not null;default:active;index" json:"status"`
	Revision                  int        `gorm:"not null;default:1" json:"revision"`
	CreatedBy                 *uuid.UUID `gorm:"type:char(36);index" json:"createdBy,omitempty"`
	UpdatedBy                 *uuid.UUID `gorm:"type:char(36);index" json:"updatedBy,omitempty"`
	WarehouseCode             string     `gorm:"-" json:"warehouseCode,omitempty"`
	WarehouseName             string     `gorm:"-" json:"warehouseName,omitempty"`
}

func (RateCard) TableName() string { return "warehouse_fee_rate_cards" }

// RateCardRevision preserves every rate used by previews and confirmed snapshots.
type RateCardRevision struct {
	model.HardDeleteBase
	TenantID                  int64      `gorm:"not null;uniqueIndex:ux_warehouse_fee_rate_revision,priority:1;index" json:"tenantId"`
	RateCardID                uuid.UUID  `gorm:"type:char(36);not null;uniqueIndex:ux_warehouse_fee_rate_revision,priority:2;index" json:"rateCardId"`
	Revision                  int        `gorm:"not null;uniqueIndex:ux_warehouse_fee_rate_revision,priority:3" json:"revision"`
	Name                      string     `gorm:"size:160;not null" json:"name"`
	Currency                  string     `gorm:"size:8;not null" json:"currency"`
	OutboundBaseFeeMinor      int64      `gorm:"not null" json:"outboundBaseFeeMinor"`
	PickingFeePerItemMinor    int64      `gorm:"not null" json:"pickingFeePerItemMinor"`
	PackingFeePerPackageMinor int64      `gorm:"not null" json:"packingFeePerPackageMinor"`
	Status                    string     `gorm:"size:24;not null" json:"status"`
	CreatedBy                 *uuid.UUID `gorm:"type:char(36);index" json:"createdBy,omitempty"`
}

func (RateCardRevision) TableName() string { return "warehouse_fee_rate_card_revisions" }

// Snapshot is the immutable result of an explicit warehouse fee confirmation.
type Snapshot struct {
	model.HardDeleteBase
	TenantID                  int64      `gorm:"not null;uniqueIndex:ux_warehouse_fee_snapshot_order,priority:1;uniqueIndex:ux_warehouse_fee_snapshot_key,priority:1;index" json:"tenantId"`
	OrderID                   uuid.UUID  `gorm:"type:char(36);not null;uniqueIndex:ux_warehouse_fee_snapshot_order,priority:2;index" json:"orderId"`
	ShopID                    *uuid.UUID `gorm:"type:char(36);index" json:"shopId,omitempty"`
	OrderNo                   string     `gorm:"size:128;not null;index" json:"orderNo"`
	WarehouseID               uuid.UUID  `gorm:"type:char(36);not null;index" json:"warehouseId"`
	WarehouseCode             string     `gorm:"size:64;not null" json:"warehouseCode"`
	WarehouseName             string     `gorm:"size:160;not null" json:"warehouseName"`
	WaveID                    uuid.UUID  `gorm:"type:char(36);not null;index" json:"waveId"`
	WaveNo                    string     `gorm:"size:64;not null" json:"waveNo"`
	WaveRevision              int        `gorm:"not null" json:"waveRevision"`
	WaveOrderID               uuid.UUID  `gorm:"type:char(36);not null;index" json:"waveOrderId"`
	PackVerificationID        uuid.UUID  `gorm:"type:char(36);not null;index" json:"packVerificationId"`
	PackageCode               string     `gorm:"size:255;not null" json:"packageCode"`
	ItemQuantity              int        `gorm:"not null" json:"itemQuantity"`
	PackageQuantity           int        `gorm:"not null" json:"packageQuantity"`
	RateCardID                uuid.UUID  `gorm:"type:char(36);not null;index" json:"rateCardId"`
	RateCardRevision          int        `gorm:"not null" json:"rateCardRevision"`
	RateCardCode              string     `gorm:"size:64;not null" json:"rateCardCode"`
	RateCardName              string     `gorm:"size:160;not null" json:"rateCardName"`
	OutboundBaseFeeMinor      int64      `gorm:"not null" json:"outboundBaseFeeMinor"`
	PickingFeePerItemMinor    int64      `gorm:"not null" json:"pickingFeePerItemMinor"`
	PickingFeeMinor           int64      `gorm:"not null" json:"pickingFeeMinor"`
	PackingFeePerPackageMinor int64      `gorm:"not null" json:"packingFeePerPackageMinor"`
	PackingFeeMinor           int64      `gorm:"not null" json:"packingFeeMinor"`
	AmountMinor               int64      `gorm:"not null" json:"amountMinor"`
	Currency                  string     `gorm:"size:8;not null" json:"currency"`
	CalculationHash           string     `gorm:"size:64;not null" json:"calculationHash"`
	IdempotencyKey            string     `gorm:"size:128;not null;uniqueIndex:ux_warehouse_fee_snapshot_key,priority:2" json:"idempotencyKey"`
	RequestHash               string     `gorm:"size:64;not null" json:"-"`
	ConfirmedBy               *uuid.UUID `gorm:"type:char(36);index" json:"confirmedBy,omitempty"`
	ConfirmedAt               time.Time  `gorm:"not null;index" json:"confirmedAt"`
}

func (Snapshot) TableName() string { return "warehouse_fee_snapshots" }

// Adjustment is an append-only signed fee fact. A reversal references and
// negates exactly one earlier adjustment; confirmed snapshots are never edited.
type Adjustment struct {
	model.HardDeleteBase
	TenantID             int64      `gorm:"not null;uniqueIndex:ux_warehouse_fee_adjustment_key,priority:1;index" json:"tenantId"`
	SnapshotID           uuid.UUID  `gorm:"type:char(36);not null;index" json:"snapshotId"`
	OrderID              uuid.UUID  `gorm:"type:char(36);not null;index" json:"orderId"`
	FactType             string     `gorm:"size:24;not null;index" json:"factType"`
	AmountMinor          int64      `gorm:"not null" json:"amountMinor"`
	Currency             string     `gorm:"size:8;not null" json:"currency"`
	Reason               string     `gorm:"size:500;not null" json:"reason"`
	ReversesAdjustmentID *uuid.UUID `gorm:"type:char(36);uniqueIndex:ux_warehouse_fee_adjustment_reversal;index" json:"reversesAdjustmentId,omitempty"`
	IdempotencyKey       string     `gorm:"size:128;not null;uniqueIndex:ux_warehouse_fee_adjustment_key,priority:2" json:"idempotencyKey"`
	RequestHash          string     `gorm:"size:64;not null" json:"-"`
	ActorID              *uuid.UUID `gorm:"type:char(36);index" json:"actorId,omitempty"`
}

func (Adjustment) TableName() string { return "warehouse_fee_adjustments" }
