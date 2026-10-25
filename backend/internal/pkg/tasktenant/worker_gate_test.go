package tasktenant

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/pkg/security"
	"gorm.io/gorm"
)

type tenantRegistryFixture struct {
	ID     int64  `gorm:"primaryKey"`
	Status string `gorm:"not null"`
}

func (tenantRegistryFixture) TableName() string { return "tenants" }

type tenantShopFixture struct {
	ID       uuid.UUID `gorm:"type:char(36);primaryKey"`
	TenantID int64     `gorm:"not null"`
}

func (tenantShopFixture) TableName() string { return "shops" }

func openTaskTenantDB(t *testing.T, withRegistry bool) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:task_tenant_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	if withRegistry {
		require.NoError(t, db.AutoMigrate(&tenantRegistryFixture{}, &tenantShopFixture{}))
	}
	return db
}

type rejectionTaskFixture struct {
	ID           uuid.UUID `gorm:"type:char(36);primaryKey"`
	TenantID     int64
	Status       string
	ErrorMessage string
	FinishedAt   *time.Time
}

func TestRejectedDequeuedWorkPreservesOtherTenantsAndStartedTasks(t *testing.T) {
	db := openTaskTenantDB(t, false)
	require.NoError(t, db.AutoMigrate(&rejectionTaskFixture{}))
	for _, entry := range []struct {
		status   string
		tenantID int64
		reject   bool
	}{
		{"pending", 31, true}, {"retrying", 31, true},
		{"running", 31, false}, {"success", 31, false}, {"failed", 31, false},
		{"pending", 32, false},
	} {
		row := rejectionTaskFixture{ID: uuid.New(), TenantID: entry.tenantID, Status: entry.status, ErrorMessage: "existing-evidence"}
		require.NoError(t, db.Create(&row).Error)
		query := db.Model(&rejectionTaskFixture{}).Where("id = ? AND tenant_id = ?", row.ID, 31)
		require.NoError(t, RejectUnstartedTask(context.Background(), query, ErrTenantNotActive, nil))
		var actual rejectionTaskFixture
		require.NoError(t, db.First(&actual, "id = ?", row.ID).Error)
		if entry.reject {
			require.Equal(t, "failed", actual.Status)
			require.NotNil(t, actual.FinishedAt)
			require.NotEqual(t, row.ErrorMessage, actual.ErrorMessage)
		} else {
			require.Equal(t, row.Status, actual.Status)
			require.Equal(t, row.ErrorMessage, actual.ErrorMessage)
			require.Nil(t, actual.FinishedAt)
		}
	}
}

func TestBeginWorkerWithLegacyZeroRequiresExplicitAuthorization(t *testing.T) {
	_, _, err := BeginWorker(context.Background(), nil, 0, uuid.Nil, "collect")
	require.True(t, errors.Is(err, security.ErrTaskTenantMissing))

	ctx, scope, err := BeginWorkerWithLegacyZero(context.Background(), nil, 0, uuid.Nil, "collect", true)
	require.NoError(t, err)
	require.Zero(t, scope.TenantID)
	require.True(t, scope.AllowLegacyTenantZero)

	tenant, err := security.RequireTenantContext(ctx)
	require.NoError(t, err)
	require.Zero(t, tenant.TenantID)
	require.Equal(t, security.AuthSourceLegacyDevZero, tenant.AuthSource)
}

func TestBeginWorkerWithLegacyZeroStillRejectsNegativeTenant(t *testing.T) {
	_, _, err := BeginWorkerWithLegacyZero(context.Background(), nil, -1, uuid.Nil, "collect", true)
	require.True(t, errors.Is(err, security.ErrTaskTenantMissing))
}

func TestBeginWorkerRejectsMissingOrInactiveTenant(t *testing.T) {
	for _, test := range []struct {
		name   string
		status string
		seed   bool
	}{
		{name: "missing"},
		{name: "suspended", status: "suspended", seed: true},
		{name: "pending", status: "pending", seed: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			db := openTaskTenantDB(t, true)
			if test.seed {
				require.NoError(t, db.Create(&tenantRegistryFixture{ID: 31, Status: test.status}).Error)
			}
			_, _, err := BeginWorker(context.Background(), db, 31, uuid.Nil, "test_worker")
			require.Error(t, err)
			require.True(t, errors.Is(err, ErrTenantNotActive))
		})
	}
}

func TestBeginWorkerValidatesShopAgainstTenant(t *testing.T) {
	db := openTaskTenantDB(t, true)
	require.NoError(t, db.Create(&tenantRegistryFixture{ID: 31, Status: "active"}).Error)
	shopID := uuid.New()
	require.NoError(t, db.Create(&tenantShopFixture{ID: shopID, TenantID: 32}).Error)

	_, _, err := BeginWorker(context.Background(), db, 31, shopID, "test_worker")
	require.Error(t, err)
	require.True(t, errors.Is(err, security.ErrTaskShopScopeMismatch))

	_, _, err = BeginWorker(context.Background(), db, 31, uuid.New(), "test_worker")
	require.Error(t, err)
	require.True(t, errors.Is(err, security.ErrTaskShopScopeMismatch))
}

func TestBeginWorkerPreservesCallerCancellation(t *testing.T) {
	db := openTaskTenantDB(t, true)
	require.NoError(t, db.Create(&tenantRegistryFixture{ID: 31, Status: "active"}).Error)
	cancelled, cancel := context.WithCancel(context.Background())
	wctx, _, err := BeginWorker(cancelled, db, 31, uuid.Nil, "test_worker")
	require.NoError(t, err)
	cancel()
	require.ErrorIs(t, wctx.Err(), context.Canceled)
}

func TestBeginWorkerAllowsLegacySQLiteWithoutTenantRegistry(t *testing.T) {
	db := openTaskTenantDB(t, false)
	ctx, scope, err := BeginWorker(context.Background(), db, 31, uuid.Nil, "test_worker")
	require.NoError(t, err)
	require.EqualValues(t, 31, scope.TenantID)
	_, err = security.RequireTenantContext(ctx)
	require.NoError(t, err)
}

func TestIsPermanentWorkerScopeErrorIncludesResourceAndTenantMismatches(t *testing.T) {
	for _, err := range []error{
		security.ErrTaskTenantMismatch,
		security.ErrTaskResourceMismatch,
	} {
		if !IsPermanentWorkerScopeError(err) {
			t.Errorf("IsPermanentWorkerScopeError(%v) = false, want true", err)
		}
	}
}
