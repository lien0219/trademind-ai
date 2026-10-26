package collect

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"github.com/trademind-ai/trademind/backend/internal/modules/tenantconfig"
	"github.com/trademind-ai/trademind/backend/internal/pkg/security"
	"github.com/trademind-ai/trademind/backend/internal/pkg/tasklease"
	"gorm.io/gorm"
)

func openCollectImportTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:collect_import_%s?mode=memory&cache=shared", uuid.New())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&CollectTask{},
		&product.Product{},
		&product.ProductImage{},
		&product.ProductSKU{},
		&tenantconfig.Setting{},
	))
	return db
}

func claimCollectImportTask(t *testing.T, db *gorm.DB, svc *Service) (*CollectTask, string, *tasklease.ClaimResult) {
	t.Helper()
	task := &CollectTask{
		HardDeleteBase: model.HardDeleteBase{ID: uuid.New()},
		TenantID:       91,
		Source:         "custom",
		SourceURL:      "https://example.com/product/1",
		Status:         StatusPending,
	}
	require.NoError(t, db.Create(task).Error)
	workerID := "collect-import-test-worker"
	claimed, claim, ok, err := svc.tryClaimCollectTask(context.Background(), task.ID, workerID, time.Minute)
	require.NoError(t, err)
	require.True(t, ok)
	require.NotNil(t, claimed)
	require.NotNil(t, claim)
	return claimed, workerID, claim
}

func collectImportContext() context.Context {
	return security.WithTenantContext(context.Background(), &security.TenantContext{TenantID: 91, AuthSource: security.AuthSourceWorker})
}

func TestImportDraftAndFinishTaskCommitsOneProduct(t *testing.T) {
	db := openCollectImportTestDB(t)
	svc := &Service{DB: db, Products: &product.Service{DB: db}}
	task, workerID, claim := claimCollectImportTask(t, db, svc)

	created, _, err := svc.importDraftAndFinishTask(
		collectImportContext(),
		task,
		workerID,
		claim,
		product.ImportDraftParams{Source: "custom", SourceURL: task.SourceURL, Title: "Imported product"},
		json.RawMessage(`{"title":"Imported product"}`),
	)
	require.NoError(t, err)
	require.NotNil(t, created)

	var productCount int64
	require.NoError(t, db.Model(&product.Product{}).Where("tenant_id = ?", 91).Count(&productCount).Error)
	require.Equal(t, int64(1), productCount)
	var stored CollectTask
	require.NoError(t, db.First(&stored, "id = ?", task.ID).Error)
	require.Equal(t, StatusSuccess, stored.Status)
	require.NotNil(t, stored.ResultProductID)
	require.Equal(t, created.ID, *stored.ResultProductID)
}

func TestImportDraftAndFinishTaskRollsBackWhenLeaseChangesBeforeFinish(t *testing.T) {
	db := openCollectImportTestDB(t)
	svc := &Service{DB: db, Products: &product.Service{DB: db}}
	task, workerID, claim := claimCollectImportTask(t, db, svc)
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register("test:steal_collect_lease", func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Name == "CollectTask" {
			tx.Exec("UPDATE collect_tasks SET lock_version = lock_version + 1 WHERE id = ?", task.ID)
		}
	}))

	created, _, err := svc.importDraftAndFinishTask(
		collectImportContext(),
		task,
		workerID,
		claim,
		product.ImportDraftParams{Source: "custom", SourceURL: task.SourceURL, Title: "Must roll back"},
		json.RawMessage(`{"title":"Must roll back"}`),
	)
	require.ErrorIs(t, err, tasklease.ErrLeaseLost)
	require.Nil(t, created)

	var productCount int64
	require.NoError(t, db.Model(&product.Product{}).Where("tenant_id = ?", 91).Count(&productCount).Error)
	require.Zero(t, productCount)
	var stored CollectTask
	require.NoError(t, db.First(&stored, "id = ?", task.ID).Error)
	require.Equal(t, StatusRunning, stored.Status)
	require.Equal(t, claim.LeaseVersion, stored.LockVersion)
}
