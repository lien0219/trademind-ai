package integration

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/warehouse"
	"github.com/trademind-ai/trademind/backend/internal/modules/warehousefee"
	"github.com/trademind-ai/trademind/backend/internal/testing/postgrestest"
	"github.com/trademind-ai/trademind/backend/internal/testing/safeenv"
	"gorm.io/gorm"
)

func TestWarehouseFeePostgresConcurrencyAndConstraints(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	_, ok, err := safeenv.TestDatabaseURLFromEnv()
	require.NoError(t, err)
	if !ok {
		t.Skip("TEST_DATABASE_URL is not set; skipping warehouse fee PostgreSQL concurrency test")
	}

	harness := postgrestest.Require(t)
	harness.EmitMetadata(t)
	db := harness.DB
	require.NoError(t, db.AutoMigrate(
		&warehouse.Warehouse{},
		&warehousefee.RateCard{},
		&warehousefee.RateCardRevision{},
		&warehousefee.Snapshot{},
		&warehousefee.Adjustment{},
	))

	tenantID := time.Now().UnixNano()
	warehouseRow := warehouse.Warehouse{TenantID: tenantID, Code: "FEE-PG", Name: "Warehouse fee PG", Status: warehouse.StatusActive}
	require.NoError(t, db.Create(&warehouseRow).Error)
	service := &warehousefee.Service{DB: db}
	card, err := service.CreateRateCard(t.Context(), tenantID, nil, warehousefee.CreateRateCardInput{
		WarehouseID: warehouseRow.ID, Code: "PG-RATE", Name: "PostgreSQL rate", Currency: "CNY",
		OutboundBaseFeeMinor: 100, PickingFeePerItemMinor: 10, PackingFeePerPackageMinor: 20,
	})
	require.NoError(t, err)

	startRate := make(chan struct{})
	rateErrors := make(chan error, 2)
	var rateWait sync.WaitGroup
	for index := 0; index < 2; index++ {
		rateWait.Add(1)
		go func() {
			defer rateWait.Done()
			<-startRate
			svc := &warehousefee.Service{DB: db.Session(&gorm.Session{NewDB: true})}
			_, updateErr := svc.UpdateRateCard(context.Background(), tenantID, nil, card.ID, warehousefee.UpdateRateCardInput{
				ExpectedRevision: 1, Name: "Concurrent revision", Currency: "CNY",
				OutboundBaseFeeMinor: 110, PickingFeePerItemMinor: 10, PackingFeePerPackageMinor: 20,
				Status: warehousefee.StatusActive,
			})
			rateErrors <- updateErr
		}()
	}
	close(startRate)
	rateWait.Wait()
	close(rateErrors)
	var rateSuccesses, rateConflicts int
	for updateErr := range rateErrors {
		switch {
		case updateErr == nil:
			rateSuccesses++
		case errors.Is(updateErr, warehousefee.ErrConflict):
			rateConflicts++
		default:
			t.Fatalf("unexpected concurrent rate update error: %v", updateErr)
		}
	}
	require.Equal(t, 1, rateSuccesses)
	require.Equal(t, 1, rateConflicts)
	var revisionCount int64
	require.NoError(t, db.Model(&warehousefee.RateCardRevision{}).Where("tenant_id = ? AND rate_card_id = ?", tenantID, card.ID).Count(&revisionCount).Error)
	require.EqualValues(t, 2, revisionCount)

	snapshot := warehousefee.Snapshot{
		TenantID: tenantID, OrderID: uuid.New(), OrderNo: "FEE-PG-ORDER", WarehouseID: warehouseRow.ID,
		WarehouseCode: warehouseRow.Code, WarehouseName: warehouseRow.Name, WaveID: uuid.New(), WaveNo: "FEE-PG-WAVE",
		WaveRevision: 1, WaveOrderID: uuid.New(), PackVerificationID: uuid.New(), PackageCode: "FEE-PG-PACKAGE",
		ItemQuantity: 1, PackageQuantity: 1, RateCardID: card.ID, RateCardRevision: 1, RateCardCode: card.Code,
		RateCardName: card.Name, OutboundBaseFeeMinor: 100, PickingFeePerItemMinor: 0, PickingFeeMinor: 0,
		PackingFeePerPackageMinor: 0, PackingFeeMinor: 0, AmountMinor: 100, Currency: "CNY",
		CalculationHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		IdempotencyKey:  "warehouse-fee-pg-confirm", RequestHash: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		ConfirmedAt: time.Now().UTC(),
	}
	require.NoError(t, db.Create(&snapshot).Error)

	type adjustmentResult struct {
		result *warehousefee.AdjustmentResult
		err    error
	}
	startAdjustment := make(chan struct{})
	adjustmentResults := make(chan adjustmentResult, 2)
	var adjustmentWait sync.WaitGroup
	for index := 0; index < 2; index++ {
		adjustmentWait.Add(1)
		go func() {
			defer adjustmentWait.Done()
			<-startAdjustment
			svc := &warehousefee.Service{DB: db.Session(&gorm.Session{NewDB: true})}
			result, createErr := svc.CreateAdjustment(context.Background(), tenantID, order.WarehouseFeeScope{}, nil, snapshot.ID, warehousefee.CreateAdjustmentInput{
				AmountMinor: 25, Reason: "concurrent idempotent adjustment", IdempotencyKey: "warehouse-fee-pg-adjustment",
			})
			adjustmentResults <- adjustmentResult{result: result, err: createErr}
		}()
	}
	close(startAdjustment)
	adjustmentWait.Wait()
	close(adjustmentResults)
	var adjustmentID uuid.UUID
	var replayed int
	for row := range adjustmentResults {
		require.NoError(t, row.err)
		require.NotNil(t, row.result)
		if adjustmentID == uuid.Nil {
			adjustmentID = row.result.Adjustment.ID
		}
		require.Equal(t, adjustmentID, row.result.Adjustment.ID)
		if row.result.Replayed {
			replayed++
		}
	}
	require.Equal(t, 1, replayed)

	startReversal := make(chan struct{})
	reversalErrors := make(chan error, 2)
	var reversalWait sync.WaitGroup
	for index := 0; index < 2; index++ {
		index := index
		reversalWait.Add(1)
		go func() {
			defer reversalWait.Done()
			<-startReversal
			svc := &warehousefee.Service{DB: db.Session(&gorm.Session{NewDB: true})}
			_, reverseErr := svc.ReverseAdjustment(context.Background(), tenantID, order.WarehouseFeeScope{}, nil, snapshot.ID, adjustmentID, warehousefee.ReverseAdjustmentInput{
				Reason: "concurrent one-time reversal", IdempotencyKey: "warehouse-fee-pg-reversal-" + string(rune('a'+index)),
			})
			reversalErrors <- reverseErr
		}()
	}
	close(startReversal)
	reversalWait.Wait()
	close(reversalErrors)
	var reversalSuccesses, reversalConflicts int
	for reverseErr := range reversalErrors {
		switch {
		case reverseErr == nil:
			reversalSuccesses++
		case errors.Is(reverseErr, warehousefee.ErrConflict):
			reversalConflicts++
		default:
			t.Fatalf("unexpected concurrent reversal error: %v", reverseErr)
		}
	}
	require.Equal(t, 1, reversalSuccesses)
	require.Equal(t, 1, reversalConflicts)

	startFloor := make(chan struct{})
	floorErrors := make(chan error, 2)
	var floorWait sync.WaitGroup
	for index := 0; index < 2; index++ {
		index := index
		floorWait.Add(1)
		go func() {
			defer floorWait.Done()
			<-startFloor
			svc := &warehousefee.Service{DB: db.Session(&gorm.Session{NewDB: true})}
			_, createErr := svc.CreateAdjustment(context.Background(), tenantID, order.WarehouseFeeScope{}, nil, snapshot.ID, warehousefee.CreateAdjustmentInput{
				AmountMinor: -80, Reason: "concurrent net floor", IdempotencyKey: "warehouse-fee-pg-floor-" + string(rune('a'+index)),
			})
			floorErrors <- createErr
		}()
	}
	close(startFloor)
	floorWait.Wait()
	close(floorErrors)
	var floorSuccesses, floorRejections int
	for createErr := range floorErrors {
		switch {
		case createErr == nil:
			floorSuccesses++
		case errors.Is(createErr, warehousefee.ErrInvalidInput):
			floorRejections++
		default:
			t.Fatalf("unexpected concurrent floor error: %v", createErr)
		}
	}
	require.Equal(t, 1, floorSuccesses)
	require.Equal(t, 1, floorRejections)

	detail, err := service.GetSnapshot(t.Context(), tenantID, order.WarehouseFeeScope{}, snapshot.ID)
	require.NoError(t, err)
	require.Equal(t, 3, detail.AdjustmentCount)
	require.EqualValues(t, -80, detail.AdjustmentMinor)
	require.EqualValues(t, 20, detail.NetAmountMinor)
}
