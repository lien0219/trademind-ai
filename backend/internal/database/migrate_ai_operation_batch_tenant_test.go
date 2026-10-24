package database

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/aioperationbatch"
	"github.com/trademind-ai/trademind/backend/internal/modules/aitask"
	"github.com/trademind-ai/trademind/backend/internal/modules/imagetask"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func TestMigrateAIOperationBatchTenantBackfillsOnlyUnambiguousOwnership(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:ai_batch_tenant_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&admin.AdminUser{}, &product.Product{}, &aioperationbatch.AIOperationBatch{}, &aitask.AITask{}, &imagetask.ImageTask{}))

	owner := admin.AdminUser{TenantID: 21, Username: admin.NewInternalUsername(), Role: admin.RoleAdmin, Status: admin.StatusActive}
	require.NoError(t, db.Create(&owner).Error)
	sameTenantProduct := product.Product{Base: model.Base{ID: uuid.New()}, TenantID: 21, Source: "manual", Title: "A", Status: product.StatusDraft}
	crossTenantProduct := product.Product{Base: model.Base{ID: uuid.New()}, TenantID: 22, Source: "manual", Title: "B", Status: product.StatusDraft}
	require.NoError(t, db.Create(&sameTenantProduct).Error)
	require.NoError(t, db.Create(&crossTenantProduct).Error)
	clearBatch := func() aioperationbatch.AIOperationBatch {
		return aioperationbatch.AIOperationBatch{Base: model.Base{ID: uuid.New()}, BatchNo: uuid.NewString(), OperationType: aioperationbatch.OperationTitleOptimize, Status: aioperationbatch.StatusFailed, CreatedBy: &owner.ID, Input: datatypes.JSON(`{}`)}
	}
	owned, mixed, orphan := clearBatch(), clearBatch(), clearBatch()
	require.NoError(t, db.Create(&owned).Error)
	require.NoError(t, db.Create(&mixed).Error)
	require.NoError(t, db.Create(&orphan).Error)
	require.NoError(t, db.Create(&aitask.AITask{TaskType: "title_optimize", Status: aitask.StatusSuccess, ProductID: &sameTenantProduct.ID, BatchID: &owned.ID}).Error)
	require.NoError(t, db.Create(&aitask.AITask{TaskType: "title_optimize", Status: aitask.StatusSuccess, ProductID: &sameTenantProduct.ID, BatchID: &mixed.ID}).Error)
	require.NoError(t, db.Create(&aitask.AITask{TaskType: "title_optimize", Status: aitask.StatusSuccess, ProductID: &crossTenantProduct.ID, BatchID: &mixed.ID}).Error)

	require.NoError(t, migrateAIOperationBatchTenant(db))
	require.NoError(t, migrateAIOperationBatchTenant(db))
	var got []aioperationbatch.AIOperationBatch
	require.NoError(t, db.Find(&got).Error)
	byID := make(map[uuid.UUID]aioperationbatch.AIOperationBatch, len(got))
	for _, row := range got {
		byID[row.ID] = row
	}
	require.EqualValues(t, 21, byID[owned.ID].TenantID)
	require.Zero(t, byID[mixed.ID].TenantID)
	require.EqualValues(t, 21, byID[orphan.ID].TenantID)
}

func TestMigrateAIOperationBatchTenantRejectsConflictingOrIncompleteChildEvidence(t *testing.T) {
	for _, tc := range []struct {
		name          string
		textTenants   []int64
		imageTenants  []int64
		missingSource bool
		wantTenant    int64
	}{
		{name: "conflicting text and image tenants", textTenants: []int64{21}, imageTenants: []int64{22}},
		{name: "mixed images cannot inherit text owner", textTenants: []int64{21}, imageTenants: []int64{21, 22}},
		{name: "unknown text owner", textTenants: []int64{21, 0}},
		{name: "unknown image owner", imageTenants: []int64{21, 0}},
		{name: "missing text source", textTenants: []int64{21}, missingSource: true},
		{name: "consistent mixed children", textTenants: []int64{21}, imageTenants: []int64{21}, wantTenant: 21},
		{name: "consistent image children", imageTenants: []int64{21, 21}, wantTenant: 21},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, err := gorm.Open(sqlite.Open("file:ai_batch_child_evidence_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
			require.NoError(t, err)
			require.NoError(t, db.AutoMigrate(&admin.AdminUser{}, &product.Product{}, &aioperationbatch.AIOperationBatch{}, &aitask.AITask{}, &imagetask.ImageTask{}))
			creator := admin.AdminUser{TenantID: 21, Username: admin.NewInternalUsername(), Role: admin.RoleAdmin, Status: admin.StatusActive}
			require.NoError(t, db.Create(&creator).Error)
			batch := aioperationbatch.AIOperationBatch{BatchNo: uuid.NewString(), OperationType: aioperationbatch.OperationTitleOptimize, Status: aioperationbatch.StatusFailed, CreatedBy: &creator.ID}
			require.NoError(t, db.Create(&batch).Error)
			for _, tenantID := range tc.textTenants {
				p := product.Product{TenantID: tenantID, Source: "manual", Title: "Migration source", Status: product.StatusDraft}
				require.NoError(t, db.Create(&p).Error)
				require.NoError(t, db.Create(&aitask.AITask{TaskType: "title_optimize", Status: aitask.StatusSuccess, ProductID: &p.ID, BatchID: &batch.ID}).Error)
			}
			if tc.missingSource {
				missingID := uuid.New()
				require.NoError(t, db.Create(&aitask.AITask{TaskType: "title_optimize", Status: aitask.StatusSuccess, ProductID: &missingID, BatchID: &batch.ID}).Error)
			}
			for _, tenantID := range tc.imageTenants {
				require.NoError(t, db.Create(&imagetask.ImageTask{TenantID: tenantID, BatchID: &batch.ID, TaskType: imagetask.TaskTypeGenerateScene, Provider: "local", Status: imagetask.StatusSuccess}).Error)
			}
			require.NoError(t, migrateAIOperationBatchTenant(db))
			require.NoError(t, migrateAIOperationBatchTenant(db))
			var stored aioperationbatch.AIOperationBatch
			require.NoError(t, db.First(&stored, "id = ?", batch.ID).Error)
			require.Equal(t, tc.wantTenant, stored.TenantID)
		})
	}
}
