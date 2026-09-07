package integration

import (
	"context"
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

	var successes int
	for confirmErr := range errs {
		require.NoError(t, confirmErr)
		successes++
	}
	require.Equal(t, 2, successes)

	var importCount, transactionCount, sharedTransactionCount, duplicateRowCount int64
	require.NoError(t, db.Model(&settlement.Import{}).Where("tenant_id = ?", tenantID).Count(&importCount).Error)
	require.NoError(t, db.Model(&settlement.Import{}).
		Where("tenant_id = ?", tenantID).
		Select("COALESCE(SUM(duplicate_rows), 0)").
		Scan(&duplicateRowCount).Error)
	require.NoError(t, db.Model(&settlement.Transaction{}).Where("tenant_id = ?", tenantID).Count(&transactionCount).Error)
	require.NoError(t, db.Model(&settlement.Transaction{}).
		Where("tenant_id = ? AND external_transaction_id = ?", tenantID, "SETTLEMENT-PG-SHARED").
		Count(&sharedTransactionCount).Error)
	require.EqualValues(t, 2, importCount)
	require.EqualValues(t, 3, transactionCount)
	require.EqualValues(t, 1, sharedTransactionCount)
	require.EqualValues(t, 1, duplicateRowCount)

	list, err := (&settlement.Service{DB: db}).List(t.Context(), tenantID, settlement.Scope{}, settlement.ListQuery{Page: 1, PageSize: 20})
	require.NoError(t, err)
	require.EqualValues(t, 1, list.Total)
	require.Len(t, list.List, 1)
}
