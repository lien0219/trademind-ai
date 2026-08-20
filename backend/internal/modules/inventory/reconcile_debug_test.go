package inventory

import (
	"context"
	"os"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestDebugReconcileAgainstConfiguredPostgres(t *testing.T) {
	dsn := os.Getenv("RECON_DSN")
	if dsn == "" {
		t.Skip("RECON_DSN is not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Info)})
	if err != nil {
		t.Fatal(err)
	}
	result, err := (&Service{DB: db}).ReconcileWarehouseLedger(context.Background(), 1, 1, 20, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("result=%+v", result)
}
