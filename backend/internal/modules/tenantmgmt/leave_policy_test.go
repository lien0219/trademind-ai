package tenantmgmt

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
)

func leaveTestUser(t *testing.T, svc *Service, role string, tenantID int64) admin.AdminUser {
	t.Helper()
	user := admin.AdminUser{
		Username: admin.NewInternalUsername(),
		Role: role, TenantID: tenantID, LastTenantID: tenantID,
		Status: admin.StatusActive, PasswordHash: "test-only",
	}
	require.NoError(t, svc.DB.Create(&user).Error)
	return user
}

func TestLeaveTenantForbidsRootOwnerAndAdminsWithoutAnyMutation(t *testing.T) {
	cases := []struct {
		name string
		root bool
		owner bool
		role string
	}{
		{name: "Root even with operator membership", root: true, role: adminperm.RoleOperator},
		{name: "enterprise owner even after role downgrade", owner: true, role: adminperm.RoleOperator},
		{name: "administrator with another administrator still present", role: adminperm.RoleAdmin},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, _ := provisionFixture(t)
			require.NoError(t, svc.DB.AutoMigrate(&admin.RootAccount{}))
			tenant := admin.Tenant{Code: "exit-guard", Name: "Exit guard", Status: admin.TenantActive, SettingsJSON: "{}", Version: 1}
			require.NoError(t, svc.DB.Create(&tenant).Error)

			globalTenant := tenant.ID
			globalRole := tc.role
			if tc.root {
				globalTenant = 0
				globalRole = adminperm.RoleAdmin
			}
			target := leaveTestUser(t, svc, globalRole, globalTenant)
			if tc.root {
				require.NoError(t, svc.DB.Create(&admin.RootAccount{ID: 1, UserID: target.ID}).Error)
			}
			if tc.owner {
				require.NoError(t, svc.DB.Model(&tenant).Update("owner_user_id", target.ID).Error)
			}
			member := admin.TenantMembership{TenantID: tenant.ID, UserID: target.ID, Role: tc.role, Status: admin.MembershipActive}
			require.NoError(t, svc.DB.Create(&member).Error)
			// A second administrator explicitly demonstrates that the previous
			// "last admin only" check did not satisfy the requested policy.
			another := leaveTestUser(t, svc, adminperm.RoleAdmin, tenant.ID)
			require.NoError(t, svc.DB.Create(&admin.TenantMembership{
				TenantID: tenant.ID, UserID: another.ID,
				Role: adminperm.RoleAdmin, Status: admin.MembershipActive,
			}).Error)

			err := svc.Leave(context.Background(), target.ID, tenant.ID)
			require.ErrorIs(t, err, ErrTenantLeaveForbidden)
			var persisted admin.TenantMembership
			require.NoError(t, svc.DB.First(&persisted, "id = ?", member.ID).Error)
			require.Equal(t, admin.MembershipActive, persisted.Status)
			var auditCount int64
			require.NoError(t, svc.DB.Model(&operationlog.OperationLog{}).
				Where("tenant_id = ? AND action = ?", tenant.ID, "tenant.membership.leave").Count(&auditCount).Error)
			require.Zero(t, auditCount, "forbidden leaves must have no success audit")
		})
	}
}

func TestLeaveTenantAllowsOrdinaryMemberAndRevokesMembership(t *testing.T) {
	for _, role := range []string{adminperm.RoleOperator, adminperm.RoleReadonly, adminperm.RoleReviewer} {
		t.Run(role, func(t *testing.T) {
			svc, _ := provisionFixture(t)
			tenant := admin.Tenant{Code: "exit-ordinary", Name: "Exit ordinary", Status: admin.TenantActive, SettingsJSON: "{}", Version: 1}
			require.NoError(t, svc.DB.Create(&tenant).Error)
			adminUser := leaveTestUser(t, svc, adminperm.RoleAdmin, tenant.ID)
			require.NoError(t, svc.DB.Create(&admin.TenantMembership{
				TenantID: tenant.ID, UserID: adminUser.ID, Role: adminperm.RoleAdmin, Status: admin.MembershipActive,
			}).Error)
			user := leaveTestUser(t, svc, role, tenant.ID)
			member := admin.TenantMembership{TenantID: tenant.ID, UserID: user.ID, Role: role, Status: admin.MembershipActive}
			require.NoError(t, svc.DB.Create(&member).Error)

			require.NoError(t, svc.Leave(context.Background(), user.ID, tenant.ID))
			var persisted admin.TenantMembership
			require.NoError(t, svc.DB.First(&persisted, "id = ?", member.ID).Error)
			require.Equal(t, admin.MembershipRemoved, persisted.Status)
			require.Error(t, svc.Leave(context.Background(), user.ID, tenant.ID))
			var auditCount int64
			require.NoError(t, svc.DB.Model(&operationlog.OperationLog{}).
				Where("tenant_id = ? AND action = ?", tenant.ID, "tenant.membership.leave").Count(&auditCount).Error)
			require.EqualValues(t, 1, auditCount)
		})
	}
}

func TestLeaveTenantDoesNotMaskUnexpectedRootLookupErrors(t *testing.T) {
	svc, _ := provisionFixture(t)
	require.NoError(t, svc.DB.AutoMigrate(&admin.RootAccount{}))
	tenant := admin.Tenant{Code:"root-lookup",Name:"Root lookup",Status:admin.TenantActive,SettingsJSON:"{}",Version:1}
	require.NoError(t,svc.DB.Create(&tenant).Error)
	user := leaveTestUser(t,svc,adminperm.RoleOperator,tenant.ID)
	require.NoError(t,svc.DB.Create(&admin.TenantMembership{TenantID:tenant.ID,UserID:user.ID,Role:adminperm.RoleOperator,Status:admin.MembershipActive}).Error)
	require.False(t,errors.Is(svc.Leave(context.Background(),uuid.Nil,tenant.ID),ErrTenantLeaveForbidden))
}
