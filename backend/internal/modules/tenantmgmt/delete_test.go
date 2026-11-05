package tenantmgmt

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
)

func TestDeleteTenantRetainsBusinessAndAudit(t *testing.T) {
	svc, actor := provisionFixture(t)
	tenant := admin.Tenant{Code: "delete-company", Name: "Deleted", Status: admin.TenantSuspended, SettingsJSON: "{}", Version: 3}
	require.NoError(t, svc.DB.Create(&tenant).Error)
	member := admin.AdminUser{Username: admin.NewInternalUsername(), Email: "member-delete@example.test", PasswordHash: "test-only", Role: admin.RoleAdmin, Status: admin.StatusActive}
	require.NoError(t, svc.DB.Create(&member).Error)
	require.NoError(t, svc.DB.Create(&admin.TenantMembership{TenantID: tenant.ID, UserID: member.ID, Role: admin.RoleAdmin, Status: admin.MembershipActive}).Error)
	require.NoError(t, svc.DB.Create(&shop.Shop{TenantID: tenant.ID, ShopName: "Kept shop", Platform: "shopee", Status: "active"}).Error)
	other := admin.Tenant{Code: "untouched", Name: "Untouched", Status: admin.TenantActive, SettingsJSON: "{}", Version: 1}
	require.NoError(t, svc.DB.Create(&other).Error)
	require.NoError(t, svc.DeleteTenant(context.Background(), actor.ID, tenant.ID, tenant.Code, 3))
	var actual admin.Tenant
	require.NoError(t, svc.DB.First(&actual, "id = ?", tenant.ID).Error)
	require.Equal(t, admin.TenantDeleted, actual.Status)
	require.NotNil(t, actual.DeletedAt)
	require.EqualValues(t, 4, actual.Version)
	var count int64
	require.NoError(t, svc.DB.Model(&admin.TenantMembership{}).Where("tenant_id = ?", tenant.ID).Count(&count).Error)
	require.EqualValues(t, 1, count)
	require.NoError(t, svc.DB.Model(&shop.Shop{}).Where("tenant_id = ?", tenant.ID).Count(&count).Error)
	require.EqualValues(t, 1, count)
	require.NoError(t, svc.DB.Model(&operationlog.OperationLog{}).Where("tenant_id = ? AND action = ?", tenant.ID, "tenant.delete").Count(&count).Error)
	require.EqualValues(t, 1, count)
	rows, total, err := svc.ListTenants(context.Background(), "", "", 1, 20)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Equal(t, other.ID, rows[0].ID)
	rows, total, err = svc.ListTenants(context.Background(), "", admin.TenantDeleted, 1, 20)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Equal(t, tenant.ID, rows[0].ID)
	require.ErrorIs(t, svc.DeleteTenant(context.Background(), actor.ID, tenant.ID, tenant.Code, 4), ErrTenantAlreadyDeleted)
	_, err = svc.SetStatus(context.Background(), actor.ID, tenant.ID, admin.TenantActive, 4)
	require.Error(t, err)
	_, err = svc.UpdateTenant(context.Background(), actor.ID, tenant.ID, TenantUpdateInput{Name: "Renamed", Version: 4})
	require.ErrorIs(t, err, ErrTenantAlreadyDeleted)
}

func TestDeleteTenantValidatesStatusVersionCodeAndOperator(t *testing.T) {
	svc, actor := provisionFixture(t)
	tenant := admin.Tenant{Code: "delete-guard", Name: "Delete guard", Status: admin.TenantActive, SettingsJSON: "{}", Version: 2}
	require.NoError(t, svc.DB.Create(&tenant).Error)
	require.ErrorIs(t, svc.DeleteTenant(context.Background(), actor.ID, tenant.ID, tenant.Code, 2), ErrTenantMustBeInactive)
	require.ErrorIs(t, svc.DeleteTenant(context.Background(), actor.ID, tenant.ID, tenant.Code, 1), ErrTenantVersionConflict)
	require.ErrorIs(t, svc.DeleteTenant(context.Background(), actor.ID, tenant.ID, "wrong", 2), ErrTenantCodeMismatch)
	require.Error(t, svc.DeleteTenant(context.Background(), uuid.New(), tenant.ID, tenant.Code, 2))
	_, err := svc.SetStatus(context.Background(), actor.ID, tenant.ID, admin.TenantSuspended, 2)
	require.NoError(t, err)
	svc.Audit = nil
	require.Error(t, svc.DeleteTenant(context.Background(), actor.ID, tenant.ID, tenant.Code, 3))
	var stored admin.Tenant
	require.NoError(t, svc.DB.First(&stored, "id = ?", tenant.ID).Error)
	require.Equal(t, admin.TenantSuspended, stored.Status)
}
