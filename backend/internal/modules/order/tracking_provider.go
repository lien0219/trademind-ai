package order

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// TrackingProvider is the read-only boundary for carrier tracking adapters.
// Implementations must not mutate orders, shipments, or provider state.
type TrackingProvider interface {
	Name() string
	ListEvents(ctx context.Context, shipmentID uuid.UUID) ([]OrderShipmentEvent, error)
}

// StoredTrackingProvider exposes locally persisted events. It is the default
// provider while external carrier integrations remain disabled at L0.
type StoredTrackingProvider struct {
	DB *gorm.DB
}

func (p *StoredTrackingProvider) Name() string { return "local" }

func (p *StoredTrackingProvider) ListEvents(ctx context.Context, shipmentID uuid.UUID) ([]OrderShipmentEvent, error) {
	if p == nil || p.DB == nil {
		return nil, fmt.Errorf("tracking provider unavailable")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	var rows []OrderShipmentEvent
	err := p.DB.WithContext(ctx).
		Where("shipment_id = ?", shipmentID).
		Order("occurred_at DESC, id DESC").
		Find(&rows).Error
	return rows, err
}

func trackingContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithTimeout(ctx, 3*time.Second)
}
