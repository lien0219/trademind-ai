package operationdashboard

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/aioperationbatch"
	"github.com/trademind-ai/trademind/backend/internal/modules/aitask"
	"github.com/trademind-ai/trademind/backend/internal/modules/imagetask"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestProductOperationDashboardScopesAITasksAndImageTasksToTenant(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:operation_dashboard_tenant_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&product.Product{},
		&aitask.AITask{},
		&imagetask.ImageTask{},
		&aioperationbatch.AIOperationBatch{},
	))

	now := time.Now().UTC().Truncate(time.Microsecond)
	productA := product.Product{
		Base:     model.Base{ID: uuid.New(), CreatedAt: now, UpdatedAt: now},
		TenantID: 41,
		Source:   "manual",
		Title:    "tenant-a",
		Status:   product.StatusDraft,
	}
	productB := product.Product{
		Base:     model.Base{ID: uuid.New(), CreatedAt: now, UpdatedAt: now},
		TenantID: 42,
		Source:   "manual",
		Title:    "tenant-b",
		Status:   product.StatusDraft,
	}
	require.NoError(t, db.Create(&productA).Error)
	require.NoError(t, db.Create(&productB).Error)

	require.NoError(t, db.Create(&aitask.AITask{
		HardDeleteBase: model.HardDeleteBase{ID: uuid.New(), CreatedAt: now, UpdatedAt: now},
		TaskType:       "title_optimize",
		Status:         aitask.StatusFailed,
		ProductID:      &productA.ID,
	}).Error)
	require.NoError(t, db.Create(&aitask.AITask{
		HardDeleteBase: model.HardDeleteBase{ID: uuid.New(), CreatedAt: now, UpdatedAt: now},
		TaskType:       "title_optimize",
		Status:         aitask.StatusFailed,
		ProductID:      &productB.ID,
	}).Error)
	require.NoError(t, db.Create(&imagetask.ImageTask{
		HardDeleteBase: model.HardDeleteBase{ID: uuid.New(), CreatedAt: now, UpdatedAt: now},
		TenantID:       41,
		TaskType:       imagetask.TaskTypeGenerateScene,
		Provider:       "test",
		Status:         imagetask.StatusFailed,
	}).Error)
	require.NoError(t, db.Create(&imagetask.ImageTask{
		HardDeleteBase: model.HardDeleteBase{ID: uuid.New(), CreatedAt: now, UpdatedAt: now},
		TenantID:       42,
		TaskType:       imagetask.TaskTypeGenerateScene,
		Provider:       "test",
		Status:         imagetask.StatusFailed,
	}).Error)

	tenantID := int64(41)
	svc := &Service{DB: db}
	out, err := svc.GetProductOperationDashboard(context.Background(), Query{}, Scope{
		IsAdmin:  true,
		TenantID: &tenantID,
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), out.Summary.AiTaskFailedCount)
	require.Equal(t, int64(1), out.Summary.ImageTaskFailed)
	require.Len(t, out.Recent.AiTasks, 1)
	require.Len(t, out.Recent.ImageTasks, 1)
}
