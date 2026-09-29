package advertisingfee

import (
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
)

const (
	CoverageExcluded = "excluded"
	CoverageIncluded = "included"
	CoverageUnknown  = "unknown"

	FactAdjustment = "adjustment"
	FactReversal   = "reversal"
)

// Import records one explicitly confirmed advertising spend CSV import.
type Import struct {
	model.HardDeleteBase
	TenantID        int64      `gorm:"not null;index;uniqueIndex:ux_ad_fee_import_key,priority:1;uniqueIndex:ux_ad_fee_import_file,priority:1" json:"tenantId"`
	ShopID          uuid.UUID  `gorm:"type:char(36);not null;index;uniqueIndex:ux_ad_fee_import_file,priority:2" json:"shopId"`
	Platform        string     `gorm:"size:64;not null;index" json:"platform"`
	ShopTimezone    string     `gorm:"size:128;not null" json:"shopTimezone"`
	FileName        string     `gorm:"size:255;not null" json:"fileName"`
	FileHash        string     `gorm:"size:64;not null;uniqueIndex:ux_ad_fee_import_file,priority:3" json:"fileHash"`
	PolicyVersion   string     `gorm:"size:64;not null" json:"policyVersion"`
	CalculationHash string     `gorm:"size:64;not null" json:"calculationHash"`
	IdempotencyKey  string     `gorm:"size:128;not null;uniqueIndex:ux_ad_fee_import_key,priority:2" json:"idempotencyKey"`
	RequestHash     string     `gorm:"size:64;not null" json:"-"`
	SourceRowCount  int        `gorm:"not null" json:"sourceRowCount"`
	ImportedSpends  int        `gorm:"not null" json:"importedSpends"`
	DuplicateSpends int        `gorm:"not null" json:"duplicateSpends"`
	AllocationCount int        `gorm:"not null" json:"allocationCount"`
	ImportedBy      *uuid.UUID `gorm:"type:char(36);index" json:"importedBy,omitempty"`
	ConfirmedAt     time.Time  `gorm:"not null;index" json:"confirmedAt"`
}

func (Import) TableName() string { return "advertising_fee_imports" }

// Spend is one immutable shop-local-day advertising source fact.
type Spend struct {
	model.HardDeleteBase
	TenantID           int64      `gorm:"not null;index;uniqueIndex:ux_ad_fee_spend_day,priority:1" json:"tenantId"`
	ImportID           uuid.UUID  `gorm:"type:char(36);not null;index" json:"importId"`
	ShopID             uuid.UUID  `gorm:"type:char(36);not null;index;uniqueIndex:ux_ad_fee_spend_day,priority:2" json:"shopId"`
	Platform           string     `gorm:"size:64;not null;index" json:"platform"`
	ShopName           string     `gorm:"size:255;not null" json:"shopName"`
	ShopTimezone       string     `gorm:"size:128;not null" json:"shopTimezone"`
	SpendDate          string     `gorm:"type:date;not null;index;uniqueIndex:ux_ad_fee_spend_day,priority:3" json:"spendDate"`
	Currency           string     `gorm:"size:3;not null;index;uniqueIndex:ux_ad_fee_spend_day,priority:4" json:"currency"`
	AmountMinor        int64      `gorm:"not null" json:"amountMinor"`
	SettlementCoverage string     `gorm:"size:16;not null;index" json:"settlementCoverage"`
	PolicyVersion      string     `gorm:"size:64;not null" json:"policyVersion"`
	CalculationHash    string     `gorm:"size:64;not null" json:"calculationHash"`
	RowHash            string     `gorm:"size:64;not null" json:"-"`
	EligibleOrderCount int        `gorm:"not null" json:"eligibleOrderCount"`
	ExcludedOrderCount int        `gorm:"not null" json:"excludedOrderCount"`
	AttributedBy       *uuid.UUID `gorm:"type:char(36);index" json:"attributedBy,omitempty"`
	AttributedAt       time.Time  `gorm:"not null;index" json:"attributedAt"`
}

func (Spend) TableName() string { return "advertising_fee_spends" }

// Allocation is an immutable order attribution produced by a confirmed spend.
type Allocation struct {
	model.HardDeleteBase
	TenantID        int64      `gorm:"not null;index;uniqueIndex:ux_ad_fee_allocation_spend_order,priority:1;uniqueIndex:ux_ad_fee_allocation_order,priority:1" json:"tenantId"`
	ImportID        uuid.UUID  `gorm:"type:char(36);not null;index" json:"importId"`
	SpendID         uuid.UUID  `gorm:"type:char(36);not null;index;uniqueIndex:ux_ad_fee_allocation_spend_order,priority:2" json:"spendId"`
	ShopID          uuid.UUID  `gorm:"type:char(36);not null;index" json:"shopId"`
	OrderID         uuid.UUID  `gorm:"type:char(36);not null;index;uniqueIndex:ux_ad_fee_allocation_spend_order,priority:3;uniqueIndex:ux_ad_fee_allocation_order,priority:2" json:"orderId"`
	OrderNo         string     `gorm:"size:128;not null;index" json:"orderNo"`
	PaidAt          time.Time  `gorm:"not null;index" json:"paidAt"`
	AmountMinor     int64      `gorm:"not null" json:"amountMinor"`
	Currency        string     `gorm:"size:3;not null" json:"currency"`
	PolicyVersion   string     `gorm:"size:64;not null" json:"policyVersion"`
	CalculationHash string     `gorm:"size:64;not null" json:"calculationHash"`
	AttributedBy    *uuid.UUID `gorm:"type:char(36);index" json:"attributedBy,omitempty"`
	AttributedAt    time.Time  `gorm:"not null;index" json:"attributedAt"`
}

func (Allocation) TableName() string { return "advertising_fee_allocations" }

// Adjustment is an append-only signed correction. A reversal negates exactly
// one earlier adjustment and is protected by a unique reference.
type Adjustment struct {
	model.HardDeleteBase
	TenantID             int64      `gorm:"not null;index;uniqueIndex:ux_ad_fee_adjustment_key,priority:1" json:"tenantId"`
	AllocationID         uuid.UUID  `gorm:"type:char(36);not null;index" json:"allocationId"`
	OrderID              uuid.UUID  `gorm:"type:char(36);not null;index" json:"orderId"`
	FactType             string     `gorm:"size:24;not null;index" json:"factType"`
	AmountMinor          int64      `gorm:"not null" json:"amountMinor"`
	Currency             string     `gorm:"size:3;not null" json:"currency"`
	Reason               string     `gorm:"size:500;not null" json:"reason"`
	ReversesAdjustmentID *uuid.UUID `gorm:"type:char(36);uniqueIndex:ux_ad_fee_adjustment_reversal;index" json:"reversesAdjustmentId,omitempty"`
	IdempotencyKey       string     `gorm:"size:128;not null;uniqueIndex:ux_ad_fee_adjustment_key,priority:2" json:"idempotencyKey"`
	RequestHash          string     `gorm:"size:64;not null" json:"-"`
	ActorID              *uuid.UUID `gorm:"type:char(36);index" json:"actorId,omitempty"`
}

func (Adjustment) TableName() string { return "advertising_fee_adjustments" }
