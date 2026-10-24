package tenantmgmt

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"gorm.io/gorm"
)

func TestTenantMutationsNeverAuthorizeTenantAdministrators(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:tenant_admin_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&admin.Tenant{}, &admin.PlatformOperator{}))
	actor := uuid.New()
	svc := &Service{DB: db}
	_, err = svc.Create(context.Background(), actor, CreateTenantInput{
		Code: "company-a", Name: "Company A",
	})
	require.Error(t, err)
	var count int64
	require.NoError(t, db.Model(&admin.Tenant{}).Count(&count).Error)
	require.Zero(t, count)
}

func TestTenantMetadataRejectsInvalidInputsEvenForOperators(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:tenant_input_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&admin.Tenant{}, &admin.PlatformOperator{}))
	actor := uuid.New()
	require.NoError(t, db.Create(&admin.PlatformOperator{UserID: actor, Enabled: true}).Error)
	svc := &Service{DB: db}
	for _, code := range []string{"", "../tenant", "admin:all", "a", "TWO WORDS"} {
		_, err := svc.Create(context.Background(), actor, CreateTenantInput{Code: code, Name: "Example"})
		require.Error(t, err, code)
	}
	var count int64
	require.NoError(t, db.Model(&admin.Tenant{}).Count(&count).Error)
	require.Zero(t, count)
}

func TestTenantStatusFailsClosedWhenAuditUnavailable(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:tenant_status_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&admin.Tenant{}, &admin.PlatformOperator{}))
	actor := uuid.New()
	require.NoError(t, db.Create(&admin.PlatformOperator{UserID: actor, Enabled: true}).Error)
	row := admin.Tenant{Code: "tenant-a", Name: "Tenant A", Status: admin.TenantActive}
	require.NoError(t, db.Create(&row).Error)
	svc := &Service{DB: db}
	require.Error(t, svc.SetStatus(context.Background(), actor, row.ID, admin.TenantSuspended))
	var reloaded admin.Tenant
	require.NoError(t, db.First(&reloaded, "id = ?", row.ID).Error)
	require.Equal(t, admin.TenantActive, reloaded.Status)
}
