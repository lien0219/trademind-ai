package inventory

import (
	"context"
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestRetryInventorySyncBatchFailedRejectsForeignOrMissingBatch(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:inventory_batch_retry_scope?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.AutoMigrate(&InventorySyncBatch{}))

	service := &Service{DB: db}
	_, err = service.RetryInventorySyncBatchFailed(context.Background(), 42, uuid.New(), nil)
	require.Error(t, err)
	require.True(t, errors.Is(err, gorm.ErrRecordNotFound))
}
