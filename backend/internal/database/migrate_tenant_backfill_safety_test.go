package database

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/aiproducttext"
	"github.com/trademind-ai/trademind/backend/internal/modules/collect"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/gorm"
)

func TestBackfillTenantIDsStopsOnSQLFailure(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:tenant_backfill_error_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	for _, statement := range []string{
		`CREATE TABLE inventory_sync_tasks (id INTEGER PRIMARY KEY, tenant_id INTEGER NOT NULL DEFAULT 0, shop_id TEXT)`,
		`CREATE TABLE shops (id TEXT PRIMARY KEY, tenant_id INTEGER NOT NULL DEFAULT 0)`,
		`CREATE TABLE products (id TEXT PRIMARY KEY, tenant_id INTEGER NOT NULL DEFAULT 0, created_by TEXT)`,
		`CREATE TABLE admin_users (id TEXT PRIMARY KEY, tenant_id INTEGER NOT NULL DEFAULT 0)`,
		`CREATE TABLE collect_batches (id TEXT PRIMARY KEY, tenant_id INTEGER NOT NULL DEFAULT 0, created_by TEXT)`,
		`CREATE TABLE collect_tasks (id TEXT PRIMARY KEY, tenant_id INTEGER NOT NULL DEFAULT 0, created_by TEXT, batch_id TEXT, result_product_id TEXT)`,
		`INSERT INTO shops (id, tenant_id) VALUES ('shop-a', 21)`,
		`INSERT INTO inventory_sync_tasks (id, tenant_id, shop_id) VALUES (1, 0, 'shop-a')`,
		`CREATE TRIGGER fail_inventory_tenant_backfill BEFORE UPDATE ON inventory_sync_tasks BEGIN SELECT RAISE(ABORT, 'forced backfill failure'); END`,
	} {
		require.NoError(t, db.Exec(statement).Error)
	}

	err = backfillTenantIDs(db)
	require.Error(t, err)
	require.Contains(t, err.Error(), "inventory_sync_tasks")
}

func TestBackfillTenantMembershipsPreservesDisabledIdentities(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:tenant_membership_backfill_disabled_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&admin.AdminUser{}, &admin.Tenant{}, &admin.TenantMembership{}))

	disabled := admin.AdminUser{TenantID: 42, Username: admin.NewInternalUsername(), Role: admin.RoleAdmin, Status: admin.StatusDisabled}
	inactive := admin.AdminUser{TenantID: 42, Username: admin.NewInternalUsername(), Role: "operator", Status: "inactive"}
	require.NoError(t, db.Create(&disabled).Error)
	require.NoError(t, db.Create(&inactive).Error)

	require.NoError(t, backfillTenantMemberships(db))
	var memberships []admin.TenantMembership
	require.NoError(t, db.Where("tenant_id = ?", 42).Order("user_id").Find(&memberships).Error)
	require.Len(t, memberships, 2)
	for _, membership := range memberships {
		require.Equal(t, admin.MembershipDisabled, membership.Status)
	}

	require.NoError(t, backfillTenantMemberships(db))
	var count int64
	require.NoError(t, db.Model(&admin.TenantMembership{}).Where("tenant_id = ?", 42).Count(&count).Error)
	require.EqualValues(t, 2, count)
}

func TestTenantSecurityMigrationLeavesMixedAIBatchUnassigned(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:tenant_backfill_ai_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&admin.AdminUser{}, &product.Product{}, &shop.Shop{},
		&aiproducttext.AIProductTextBatch{}, &aiproducttext.AIProductTextItem{},
	))

	productA := product.Product{Base: model.Base{ID: uuid.New()}, TenantID: 21, Source: "manual", Title: "A", Status: product.StatusDraft}
	productB := product.Product{Base: model.Base{ID: uuid.New()}, TenantID: 22, Source: "manual", Title: "B", Status: product.StatusDraft}
	productSameTenant := product.Product{Base: model.Base{ID: uuid.New()}, TenantID: 21, Source: "manual", Title: "Same tenant", Status: product.StatusDraft}
	require.NoError(t, db.Create(&productA).Error)
	require.NoError(t, db.Create(&productB).Error)
	require.NoError(t, db.Create(&productSameTenant).Error)
	mixedBatch := aiproducttext.AIProductTextBatch{BatchNo: "mixed-tenant-batch", IdempotencyKey: "mixed-tenant-key", Status: aiproducttext.BatchFailed}
	sameTenantBatch := aiproducttext.AIProductTextBatch{BatchNo: "same-tenant-batch", IdempotencyKey: "same-tenant-key", Status: aiproducttext.BatchFailed}
	require.NoError(t, db.Create(&mixedBatch).Error)
	require.NoError(t, db.Create(&sameTenantBatch).Error)
	for _, productID := range []uuid.UUID{productA.ID, productB.ID} {
		item := aiproducttext.AIProductTextItem{
			BatchID: mixedBatch.ID, ProductID: productID, OperationType: aiproducttext.OpTitle,
			Status: aiproducttext.ItemFailed,
		}
		require.NoError(t, db.Create(&item).Error)
	}
	require.NoError(t, db.Create(&aiproducttext.AIProductTextItem{
		BatchID: sameTenantBatch.ID, ProductID: productSameTenant.ID, OperationType: aiproducttext.OpTitle,
		Status: aiproducttext.ItemFailed,
	}).Error)

	require.NoError(t, migrateTenantSecurity(db))
	var storedMixed, storedSameTenant aiproducttext.AIProductTextBatch
	require.NoError(t, db.First(&storedMixed, "id = ?", mixedBatch.ID).Error)
	require.NoError(t, db.First(&storedSameTenant, "id = ?", sameTenantBatch.ID).Error)
	require.Zero(t, storedMixed.TenantID)
	require.EqualValues(t, 21, storedSameTenant.TenantID)
}

func TestBackfillCollectTenantIDsLeavesConflictingEvidenceUnassigned(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:collect_backfill_evidence_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&admin.AdminUser{}, &product.Product{}, &collect.CollectBatch{}, &collect.CollectTask{}))

	owner21 := admin.AdminUser{TenantID: 21, Username: admin.NewInternalUsername(), Role: admin.RoleAdmin, Status: admin.StatusActive}
	owner22 := admin.AdminUser{TenantID: 22, Username: admin.NewInternalUsername(), Role: admin.RoleAdmin, Status: admin.StatusActive}
	require.NoError(t, db.Create(&owner21).Error)
	require.NoError(t, db.Create(&owner22).Error)
	product21 := product.Product{Base: model.Base{ID: uuid.New()}, TenantID: 21, Source: "manual", Title: "Tenant 21", Status: product.StatusDraft}
	product22 := product.Product{Base: model.Base{ID: uuid.New()}, TenantID: 22, Source: "manual", Title: "Tenant 22", Status: product.StatusDraft}
	require.NoError(t, db.Create(&product21).Error)
	require.NoError(t, db.Create(&product22).Error)
	batch21 := collect.CollectBatch{
		TenantID: 21, Source: "manual", TotalCount: 3, PendingCount: 3, Status: collect.BatchStatusRunning,
		CreatedBy: &owner21.ID,
	}
	require.NoError(t, db.Create(&batch21).Error)
	consistent := collect.CollectTask{
		Source: "manual", SourceURL: "https://example.test/consistent", Status: collect.StatusSuccess,
		CreatedBy: &owner21.ID, BatchID: &batch21.ID, ResultProductID: &product21.ID,
	}
	conflicting := collect.CollectTask{
		Source: "manual", SourceURL: "https://example.test/conflicting", Status: collect.StatusSuccess,
		CreatedBy: &owner21.ID, BatchID: &batch21.ID, ResultProductID: &product22.ID,
	}
	creatorConflict := collect.CollectTask{
		Source: "manual", SourceURL: "https://example.test/creator-conflict", Status: collect.StatusSuccess,
		CreatedBy: &owner22.ID, BatchID: &batch21.ID, ResultProductID: &product21.ID,
	}
	require.NoError(t, db.Create(&consistent).Error)
	require.NoError(t, db.Create(&conflicting).Error)
	require.NoError(t, db.Create(&creatorConflict).Error)

	require.NoError(t, backfillCollectTenantIDs(db))
	var stored []collect.CollectTask
	require.NoError(t, db.Order("source_url").Find(&stored).Error)
	require.Len(t, stored, 3)
	byURL := make(map[string]collect.CollectTask, len(stored))
	for _, task := range stored {
		byURL[task.SourceURL] = task
	}
	require.EqualValues(t, 21, byURL[consistent.SourceURL].TenantID)
	require.Zero(t, byURL[conflicting.SourceURL].TenantID)
	require.Zero(t, byURL[creatorConflict.SourceURL].TenantID)
}
