package integration

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/database"
	"github.com/trademind-ai/trademind/backend/internal/modules/advertisingfee"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/testing/postgrestest"
	"github.com/trademind-ai/trademind/backend/internal/testing/safeenv"
	"gorm.io/gorm"
)

func TestAdvertisingFeePostgresConcurrencyAndConstraints(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	_, ok, err := safeenv.TestDatabaseURLFromEnv()
	require.NoError(t, err)
	if !ok {
		t.Skip("TEST_DATABASE_URL is not set; skipping advertising fee PostgreSQL constraints")
	}

	harness := postgrestest.Require(t)
	harness.EmitMetadata(t)
	db := harness.DB
	require.NoError(t, database.AutoMigrate(db))
	require.True(t, db.Migrator().HasIndex(&advertisingfee.Import{}, "ux_ad_fee_import_key"))
	require.True(t, db.Migrator().HasIndex(&advertisingfee.Spend{}, "ux_ad_fee_spend_day"))
	require.True(t, db.Migrator().HasIndex(&advertisingfee.Allocation{}, "ux_ad_fee_allocation_order"))
	require.True(t, db.Migrator().HasIndex(&advertisingfee.Adjustment{}, "ux_ad_fee_adjustment_reversal"))

	tenantID := time.Now().UnixNano()
	shopRow := shop.Shop{TenantID: tenantID, Platform: "manual", ShopName: "Advertising PostgreSQL", ExternalShopID: "advertising-postgres-test", Status: "active", AuthStatus: "mock", Currency: "CNY", Timezone: "UTC"}
	require.NoError(t, db.Create(&shopRow).Error)
	paidAt := time.Date(2026, 9, 8, 1, 0, 0, 0, time.UTC)
	orderRow := order.Order{TenantID: tenantID, Platform: "manual", ShopID: &shopRow.ID, OrderNo: "ADVERTISING-PG-ORDER", CustomerName: "PG", Status: order.StatusPaid, PaymentStatus: order.PaymentPaid, FulfillmentStatus: order.FulfillmentUnfulfilled, Currency: "CNY", TotalAmount: 100, PaidAt: &paidAt, OrderedAt: &paidAt}
	require.NoError(t, db.Create(&orderRow).Error)

	data := []byte("spend_date,currency,spend_minor,settlement_coverage\n2026-09-08,CNY,101,excluded\n")
	preview, err := (&advertisingfee.Service{DB: db}).Preview(t.Context(), tenantID, advertisingfee.Scope{}, shopRow.ID, "advertising.csv", data)
	require.NoError(t, err)
	require.True(t, preview.Valid)

	type confirmation struct {
		result *advertisingfee.ImportResult
		err    error
	}
	start := make(chan struct{})
	results := make(chan confirmation, 2)
	var wait sync.WaitGroup
	for index := 0; index < 2; index++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			svc := &advertisingfee.Service{DB: db.Session(&gorm.Session{NewDB: true})}
			result, confirmErr := svc.Confirm(context.Background(), tenantID, advertisingfee.Scope{}, shopRow.ID, "advertising.csv", data, preview.FileHash, preview.CalculationHash, "advertising-postgres-confirm", nil)
			results <- confirmation{result: result, err: confirmErr}
		}()
	}
	close(start)
	wait.Wait()
	close(results)
	var importID string
	var replayed int
	for row := range results {
		require.NoError(t, row.err)
		require.NotNil(t, row.result)
		if importID == "" {
			importID = row.result.Import.ID.String()
		}
		require.Equal(t, importID, row.result.Import.ID.String())
		if row.result.Replayed {
			replayed++
		}
	}
	require.Equal(t, 1, replayed)

	var allocation advertisingfee.Allocation
	require.NoError(t, db.Where("tenant_id = ? AND order_id = ?", tenantID, orderRow.ID).Take(&allocation).Error)
	adjustmentStart := make(chan struct{})
	adjustmentResults := make(chan *advertisingfee.AdjustmentResult, 2)
	adjustmentErrors := make(chan error, 2)
	for index := 0; index < 2; index++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-adjustmentStart
			svc := &advertisingfee.Service{DB: db.Session(&gorm.Session{NewDB: true})}
			result, createErr := svc.CreateAdjustment(context.Background(), tenantID, advertisingfee.Scope{}, nil, allocation.ID, advertisingfee.CreateAdjustmentInput{AmountMinor: 25, Reason: "PostgreSQL concurrent correction", IdempotencyKey: "advertising-postgres-adjustment"})
			adjustmentResults <- result
			adjustmentErrors <- createErr
		}()
	}
	close(adjustmentStart)
	wait.Wait()
	close(adjustmentResults)
	close(adjustmentErrors)
	for createErr := range adjustmentErrors {
		require.NoError(t, createErr)
	}
	var adjustmentID string
	replayed = 0
	for result := range adjustmentResults {
		require.NotNil(t, result)
		if adjustmentID == "" {
			adjustmentID = result.Adjustment.ID.String()
		}
		require.Equal(t, adjustmentID, result.Adjustment.ID.String())
		if result.Replayed {
			replayed++
		}
	}
	require.Equal(t, 1, replayed)
}
