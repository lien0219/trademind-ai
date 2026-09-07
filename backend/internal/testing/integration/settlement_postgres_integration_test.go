package integration

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/database"
	"github.com/trademind-ai/trademind/backend/internal/modules/settlement"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/testing/postgrestest"
	"github.com/trademind-ai/trademind/backend/internal/testing/safeenv"
	"gorm.io/gorm"
)

func TestSettlementPostgresConcurrentImportConstraints(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	_, ok, err := safeenv.TestDatabaseURLFromEnv()
	require.NoError(t, err)
	if !ok {
		t.Skip("TEST_DATABASE_URL is not set; skipping settlement PostgreSQL constraints")
	}

	harness := postgrestest.Require(t)
	harness.EmitMetadata(t)
	db := harness.DB
	require.NoError(t, database.AutoMigrate(db))
	require.NoError(t, database.AutoMigrate(db))
	require.True(t, db.Migrator().HasIndex(&settlement.Import{}, "ux_settlement_import_idempotency"))
	require.True(t, db.Migrator().HasIndex(&settlement.Import{}, "ux_settlement_import_file"))
	require.True(t, db.Migrator().HasIndex(&settlement.Transaction{}, "ux_settlement_external_transaction"))

	tenantID := time.Now().UnixNano()
	shopRow := shop.Shop{
		TenantID: tenantID, Platform: "manual", ShopName: "Settlement PostgreSQL",
		ExternalShopID: "settlement-postgres-test", Status: "active", AuthStatus: "mock",
	}
	require.NoError(t, db.Create(&shopRow).Error)

	header := "external_transaction_id,order_no,currency,order_gross_minor,platform_fee_minor,settlement_amount_minor,settled_at\n"
	dataA := []byte(header +
		"SETTLEMENT-PG-SHARED,SO-PG-1,CNY,10000,1000,9000,2026-09-07T01:00:00Z\n" +
		"SETTLEMENT-PG-A,SO-PG-1,CNY,0,-100,100,2026-09-07T02:00:00Z\n")
	dataB := []byte(header +
		"SETTLEMENT-PG-SHARED,SO-PG-1,CNY,10000,1000,9000,2026-09-07T01:00:00Z\n" +
		"SETTLEMENT-PG-B,SO-PG-1,CNY,0,-200,200,2026-09-07T03:00:00Z\n")
	previewA, err := (&settlement.Service{DB: db}).Preview(t.Context(), tenantID, settlement.Scope{}, shopRow.ID, "a.csv", dataA)
	require.NoError(t, err)
	require.True(t, previewA.Valid)
	previewB, err := (&settlement.Service{DB: db}).Preview(t.Context(), tenantID, settlement.Scope{}, shopRow.ID, "b.csv", dataB)
	require.NoError(t, err)
	require.True(t, previewB.Valid)

	type request struct {
		name string
		data []byte
		hash string
		key  string
	}
	requests := []request{
		{name: "a.csv", data: dataA, hash: previewA.FileHash, key: "settlement-postgres-import-a"},
		{name: "b.csv", data: dataB, hash: previewB.FileHash, key: "settlement-postgres-import-b"},
	}
	start := make(chan struct{})
	errs := make(chan error, len(requests))
	var wg sync.WaitGroup
	for _, input := range requests {
		input := input
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			svc := &settlement.Service{DB: db.Session(&gorm.Session{NewDB: true})}
			_, confirmErr := svc.Confirm(context.Background(), tenantID, settlement.Scope{}, shopRow.ID, input.name, input.data, input.hash, input.key, nil)
			errs <- confirmErr
		}()
	}
	close(start)
	wg.Wait()
	close(errs)

	var successes, conflicts int
	for confirmErr := range errs {
		switch {
		case confirmErr == nil:
			successes++
		case errors.Is(confirmErr, settlement.ErrConflict):
			conflicts++
		default:
			require.NoError(t, confirmErr)
		}
	}
	require.Equal(t, 1, successes)
	require.Equal(t, 1, conflicts)

	var importCount, transactionCount int64
	require.NoError(t, db.Model(&settlement.Import{}).Where("tenant_id = ?", tenantID).Count(&importCount).Error)
	require.NoError(t, db.Model(&settlement.Transaction{}).Where("tenant_id = ?", tenantID).Count(&transactionCount).Error)
	require.EqualValues(t, 1, importCount)
	require.EqualValues(t, 2, transactionCount)
}
