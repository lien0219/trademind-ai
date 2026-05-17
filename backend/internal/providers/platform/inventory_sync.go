package platform

import (
	"context"
	"strings"

	"github.com/google/uuid"
)

// SyncInventoryRequest is passed to adapters (never log Auth secrets).
type SyncInventoryRequest struct {
	ShopID            uuid.UUID
	Platform          string
	Auth              TestConnectionRequest
	PublicationID     uuid.UUID
	PublicationSKUID  uuid.UUID
	ExternalProductID string
	ExternalSKUID     string
	SKUCode           string
	Stock             int
	Options           map[string]any
}

// SyncInventoryResult is persisted as trimmed output (no tokens).
type SyncInventoryResult struct {
	ExternalProductID string
	ExternalSKUID     string
	Stock             int
	Status            string
	RawSummary        map[string]any
}

// InventorySyncProvider applies one stock snapshot to one platform listing SKU.
type InventorySyncProvider interface {
	SyncInventory(ctx context.Context, req SyncInventoryRequest) (*SyncInventoryResult, error)
}

// AsInventorySync performs a narrowed type assertion without requiring Provider embedding.
func AsInventorySync(p Provider) (InventorySyncProvider, bool) {
	if p == nil {
		return nil, false
	}
	ip, ok := p.(InventorySyncProvider)
	return ip, ok
}

// InventorySyncImplementationStatus is rollout granularity for capability inventory_sync on /platform/providers.
func InventorySyncImplementationStatus(p Provider) string {
	if p == nil {
		return StatusDisabled
	}
	switch strings.TrimSpace(strings.ToLower(p.Platform())) {
	case "mock":
		return StatusAvailable
	case "manual":
		return StatusDisabled
	case "tiktok", "shopee", "lazada", "amazon":
		return StatusPlanned
	default:
		st := p.Status()
		if st == StatusPlanned || st == StatusDisabled {
			return st
		}
		if !HasCapability(p, CapInventorySync) {
			return StatusDisabled
		}
		return StatusPlanned
	}
}

// IsInventorySyncRunnable is true only when enqueue / worker path may call the provider safely (not planned-only placeholders).
func IsInventorySyncRunnable(p Provider) bool {
	switch InventorySyncImplementationStatus(p) {
	case StatusAvailable, StatusBeta:
		return true
	default:
		return false
	}
}
