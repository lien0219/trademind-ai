package integration

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/database"
	"github.com/trademind-ai/trademind/backend/internal/modules/freightfee"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"github.com/trademind-ai/trademind/backend/internal/testing/postgrestest"
	"github.com/trademind-ai/trademind/backend/internal/testing/safeenv"
	"gorm.io/gorm"
)

func TestFreightFeePostgresImportAndAdjustmentIdempotency(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	_, ok, err := safeenv.TestDatabaseURLFromEnv()
	require.NoError(t, err)
	if !ok {
		t.Skip("TEST_DATABASE_URL is not set; skipping freight fee PostgreSQL concurrency test")
	}

	harness := postgrestest.Require(t)
	harness.EmitMetadata(t)
	db := harness.DB
	require.NoError(t, database.AutoMigrate(db))
	require.True(t, db.Migrator().HasIndex(&freightfee.Import{}, "ux_freight_fee_import_key"))
	require.True(t, db.Migrator().HasIndex(&freightfee.Import{}, "ux_freight_fee_import_file"))
	require.True(t, db.Migrator().HasIndex(&freightfee.Charge{}, "ux_freight_fee_shipment"))
	require.True(t, db.Migrator().HasIndex(&freightfee.Adjustment{}, "ux_freight_fee_adjustment_reversal"))

	tenantID := time.Now().UnixNano()
	shopRow := shop.Shop{
		TenantID: tenantID, Platform: "manual", ShopName: "Freight fee PostgreSQL",
		ExternalShopID: "freight-fee-postgres-test", Status: "active", AuthStatus: "mock", Currency: "CNY",
	}
	require.NoError(t, db.Create(&shopRow).Error)
	orderID, shipmentID := uuid.New(), uuid.New()
	orderRow := order.Order{
		Base: model.Base{ID: orderID}, TenantID: tenantID, Platform: "manual", ShopID: &shopRow.ID,
		OrderNo: "SO-FREIGHT-PG-1", CustomerName: "Test buyer", Status: order.StatusRefunded,
		PaymentStatus: order.PaymentRefunded, FulfillmentStatus: order.FulfillmentFulfilled, Currency: "CNY",
	}
	require.NoError(t, db.Create(&orderRow).Error)
	shipment := order.OrderShipment{
		HardDeleteBase: model.HardDeleteBase{ID: shipmentID}, OrderID: orderID, Carrier: "Carrier A",
		TrackingNo: "TRACK-FREIGHT-PG-1", Status: order.ShipmentDelivered,
	}
	require.NoError(t, db.Create(&shipment).Error)

	data := []byte("external_line_id,carrier,tracking_no,amount_minor,currency,billed_at\n" +
		"FREIGHT-PG-1,Carrier A,TRACK-FREIGHT-PG-1,500,CNY,2026-09-28T01:00:00Z\n")
	service := &freightfee.Service{DB: db}
	preview, err := service.Preview(t.Context(), tenantID, freightfee.Scope{}, shopRow.ID, "freight.csv", data)
	require.NoError(t, err)
	require.True(t, preview.Valid)
	require.Equal(t, 1, preview.NewCharges)

	type confirmResult struct {
		result *freightfee.ImportResult
		err    error
	}
	start := make(chan struct{})
	results := make(chan confirmResult, 2)
	var wait sync.WaitGroup
	for _, key := range []string{"freight-fee-postgres-import-a", "freight-fee-postgres-import-b"} {
		key := key
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			svc := &freightfee.Service{DB: db.Session(&gorm.Session{NewDB: true})}
			result, confirmErr := svc.Confirm(context.Background(), tenantID, freightfee.Scope{}, shopRow.ID, "freight.csv", data,
				preview.FileHash, preview.CalculationHash, key, nil)
			results <- confirmResult{result: result, err: confirmErr}
		}()
	}
	close(start)
	wait.Wait()
	close(results)
	var importID uuid.UUID
	var replayCount int
	for result := range results {
		require.NoError(t, result.err)
		require.NotNil(t, result.result)
		if importID == uuid.Nil {
			importID = result.result.Import.ID
		}
		require.Equal(t, importID, result.result.Import.ID)
		if result.result.Replayed {
			replayCount++
		}
	}
	require.Equal(t, 1, replayCount)
	var imports, charges int64
	require.NoError(t, db.Model(&freightfee.Import{}).Where("tenant_id = ?", tenantID).Count(&imports).Error)
	require.NoError(t, db.Model(&freightfee.Charge{}).Where("tenant_id = ?", tenantID).Count(&charges).Error)
	require.EqualValues(t, 1, imports)
	require.EqualValues(t, 1, charges)

	var charge freightfee.Charge
	require.NoError(t, db.Where("tenant_id = ? AND order_id = ?", tenantID, orderID).Take(&charge).Error)
	type adjustmentResult struct {
		result *freightfee.AdjustmentResult
		err    error
	}
	adjustStart := make(chan struct{})
	adjustments := make(chan adjustmentResult, 2)
	var adjustmentWait sync.WaitGroup
	for range 2 {
		adjustmentWait.Add(1)
		go func() {
			defer adjustmentWait.Done()
			<-adjustStart
			svc := &freightfee.Service{DB: db.Session(&gorm.Session{NewDB: true})}
			result, adjustErr := svc.CreateAdjustment(context.Background(), tenantID, freightfee.Scope{}, nil, charge.ID,
				freightfee.CreateAdjustmentInput{AmountMinor: 25, Reason: "Concurrent correction", IdempotencyKey: "freight-fee-postgres-adjustment"})
			adjustments <- adjustmentResult{result: result, err: adjustErr}
		}()
	}
	close(adjustStart)
	adjustmentWait.Wait()
	close(adjustments)
	var adjustmentID uuid.UUID
	replayCount = 0
	for result := range adjustments {
		require.NoError(t, result.err)
		require.NotNil(t, result.result)
		if adjustmentID == uuid.Nil {
			adjustmentID = result.result.Adjustment.ID
		}
		require.Equal(t, adjustmentID, result.result.Adjustment.ID)
		if result.result.Replayed {
			replayCount++
		}
	}
	require.Equal(t, 1, replayCount)

	facts, err := service.ProfitabilityFeesForOrders(t.Context(), tenantID, []uuid.UUID{orderID})
	require.NoError(t, err)
	require.Equal(t, "confirmed", facts[orderID].Status, "a shipped order remains financially billable after refund")
	require.EqualValues(t, 1, facts[orderID].ShipmentCount)
	require.EqualValues(t, 1, facts[orderID].BilledShipmentCount)
	require.EqualValues(t, 525, facts[orderID].AmountMinor)

	require.NoError(t, db.Model(&order.OrderShipment{}).Where("id = ?", shipmentID).Update("tracking_no", "TRACK-FREIGHT-CHANGED").Error)
	facts, err = service.ProfitabilityFeesForOrders(t.Context(), tenantID, []uuid.UUID{orderID})
	require.NoError(t, err)
	require.Equal(t, "blocked", facts[orderID].Status)
	require.Equal(t, "freight_fact_scope_invalid", facts[orderID].ReasonCode)

	detail, err := service.GetCharge(t.Context(), tenantID, freightfee.Scope{}, charge.ID)
	require.NoError(t, err)
	require.Len(t, detail.Adjustments, 1)
	_, err = service.CreateAdjustment(t.Context(), tenantID, freightfee.Scope{}, nil, charge.ID,
		freightfee.CreateAdjustmentInput{AmountMinor: -1000, Reason: "Negative net", IdempotencyKey: "freight-fee-negative-net"})
	require.True(t, errors.Is(err, freightfee.ErrInvalidInput))
}
