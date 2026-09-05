package logistics

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/warehouse"
	"gorm.io/gorm"
)

func newLogisticsService(t *testing.T) (*Service, warehouse.Warehouse) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:logistics-%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&warehouse.Warehouse{}, &ShippingChannel{}, &ShippingRateTemplate{}); err != nil {
		t.Fatal(err)
	}
	w := warehouse.Warehouse{TenantID: 7, Code: "WH-A", Name: "A warehouse", Status: warehouse.StatusActive}
	if err := db.Create(&w).Error; err != nil {
		t.Fatal(err)
	}
	return &Service{DB: db}, w
}

func TestQuoteUsesTenantDestinationWarehouseAndStartedKilograms(t *testing.T) {
	svc, w := newLogisticsService(t)
	channel, err := svc.CreateChannel(context.Background(), 7, nil, CreateChannelInput{Code: "local", Name: "Local", Carrier: "Carrier A"})
	if err != nil {
		t.Fatal(err)
	}
	rate, err := svc.CreateRate(context.Background(), 7, nil, CreateRateInput{RateInput: RateInput{ChannelID: channel.ID, WarehouseID: &w.ID, Code: "cn-main", Name: "CN main", CountryCode: "cn", MinWeightGrams: 500, MaxWeightGrams: 5000, BaseFeeMinor: 800, PerKilogramFeeMinor: 300, Currency: "cny", Priority: 10}})
	if err != nil {
		t.Fatal(err)
	}
	quotes, err := svc.Quote(context.Background(), 7, QuoteInput{WarehouseID: w.ID, CountryCode: "CN", WeightGrams: 1501})
	if err != nil {
		t.Fatal(err)
	}
	if len(quotes) != 1 || quotes[0].RateTemplateID != rate.ID || quotes[0].AmountMinor != 1400 {
		t.Fatalf("unexpected deterministic quote: %#v", quotes)
	}
	if _, err := svc.Quote(context.Background(), 8, QuoteInput{WarehouseID: w.ID, CountryCode: "CN", WeightGrams: 1501}); !errors.Is(err, ErrNoQuote) {
		t.Fatalf("tenant isolation must fail closed, got %v", err)
	}
	if _, err := svc.Quote(context.Background(), 7, QuoteInput{WarehouseID: w.ID, WeightGrams: 1501}); !errors.Is(err, ErrDestination) {
		t.Fatalf("missing destination must fail closed, got %v", err)
	}
}

func TestOptimisticRevisionProtectsChannelUpdates(t *testing.T) {
	svc, _ := newLogisticsService(t)
	row, err := svc.CreateChannel(context.Background(), 7, nil, CreateChannelInput{Code: "CH-1", Name: "One", Carrier: "Carrier"})
	if err != nil {
		t.Fatal(err)
	}
	updated, err := svc.UpdateChannel(context.Background(), 7, row.ID, UpdateChannelInput{ExpectedRevision: 1, Name: "Two", Carrier: "Carrier", Status: StatusActive})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Revision != 2 {
		t.Fatalf("expected revision 2, got %d", updated.Revision)
	}
	if _, err := svc.UpdateChannel(context.Background(), 7, row.ID, UpdateChannelInput{ExpectedRevision: 1, Name: "Stale", Carrier: "Carrier", Status: StatusActive}); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale update must conflict, got %v", err)
	}
}
