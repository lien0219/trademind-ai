package settlement

import (
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
)

// Import records one explicitly confirmed CSV import. It is append-only.
type Import struct {
	model.HardDeleteBase
	TenantID       int64      `gorm:"not null;index;uniqueIndex:ux_settlement_import_idempotency,priority:1;uniqueIndex:ux_settlement_import_file,priority:1" json:"tenantId"`
	ShopID         uuid.UUID  `gorm:"type:char(36);not null;index;uniqueIndex:ux_settlement_import_file,priority:2" json:"shopId"`
	Platform       string     `gorm:"size:64;not null;index" json:"platform"`
	FileName       string     `gorm:"size:255;not null" json:"fileName"`
	FileHash       string     `gorm:"size:64;not null;uniqueIndex:ux_settlement_import_file,priority:3" json:"fileHash"`
	IdempotencyKey string     `gorm:"size:128;not null;uniqueIndex:ux_settlement_import_idempotency,priority:2" json:"idempotencyKey"`
	SourceRowCount int        `gorm:"not null" json:"sourceRowCount"`
	ImportedRows   int        `gorm:"not null" json:"importedRows"`
	DuplicateRows  int        `gorm:"not null" json:"duplicateRows"`
	ImportedBy     *uuid.UUID `gorm:"type:char(36);index" json:"importedBy,omitempty"`
	ConfirmedAt    time.Time  `gorm:"not null;index" json:"confirmedAt"`
}

func (Import) TableName() string { return "settlement_imports" }

// Transaction is an immutable platform settlement fact. Corrections are new
// rows with their own external transaction id; existing rows are never edited.
type Transaction struct {
	model.HardDeleteBase
	TenantID              int64      `gorm:"not null;index;uniqueIndex:ux_settlement_external_transaction,priority:1" json:"tenantId"`
	ImportID              uuid.UUID  `gorm:"type:char(36);not null;index" json:"importId"`
	ReconciliationID      uuid.UUID  `gorm:"type:char(36);not null;index" json:"reconciliationId"`
	ShopID                uuid.UUID  `gorm:"type:char(36);not null;index;uniqueIndex:ux_settlement_external_transaction,priority:2" json:"shopId"`
	Platform              string     `gorm:"size:64;not null;index;uniqueIndex:ux_settlement_external_transaction,priority:3" json:"platform"`
	ExternalTransactionID string     `gorm:"size:255;not null;uniqueIndex:ux_settlement_external_transaction,priority:4" json:"externalTransactionId"`
	OrderNo               string     `gorm:"size:128;not null;index" json:"orderNo"`
	Currency              string     `gorm:"size:3;not null;index" json:"currency"`
	OrderGrossMinor       int64      `gorm:"not null" json:"orderGrossMinor"`
	PlatformFeeMinor      int64      `gorm:"not null" json:"platformFeeMinor"`
	SettlementAmountMinor int64      `gorm:"not null" json:"settlementAmountMinor"`
	SettledAt             time.Time  `gorm:"not null;index" json:"settledAt"`
	RowHash               string     `gorm:"size:64;not null" json:"rowHash"`
	ImportedBy            *uuid.UUID `gorm:"type:char(36);index" json:"importedBy,omitempty"`
}

func (Transaction) TableName() string { return "settlement_transactions" }
