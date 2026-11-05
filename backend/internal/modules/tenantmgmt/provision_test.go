package tenantmgmt

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/auth"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"gorm.io/gorm"
)

func provisionFixture(t *testing.T) (*Service, admin.AdminUser) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:provision_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	conn, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })
	require.NoError(t, db.AutoMigrate(&admin.AdminUser{}, &admin.AdminMutationLock{}, &admin.Tenant{}, &admin.TenantMembership{}, &admin.TenantInvitation{}, &admin.PlatformOperator{}, &admin.UserStorePermission{}, &auth.AuthSession{}, &auth.AuthRefreshToken{}, &operationlog.OperationLog{}, &shop.Shop{}))
	actor := admin.AdminUser{Username: admin.NewInternalUsername(), Email: "operator@example.test", Role: "readonly", Status: "active", PasswordHash: "test-only"}
	require.NoError(t, db.Create(&actor).Error)
	require.NoError(t, db.Create(&admin.PlatformOperator{UserID: actor.ID, Enabled: true}).Error)
	return &Service{DB: db, Audit: &operationlog.Service{DB: db}}, actor
}

func TestProvisionRejectsOrphanTenant(t *testing.T) {
	svc, actor := provisionFixture(t)
	_, err := svc.Create(context.Background(), actor.ID, CreateTenantInput{Code: "orphan-company", Name: "Orphan company"})
	require.Error(t, err, "enterprise provisioning must require an effective administrator")
	var count int64
	require.NoError(t, svc.DB.Model(&admin.Tenant{}).Count(&count).Error)
	require.Zero(t, count)
}

func TestProvisionCreatesOwnerIdentityAndMembershipAtomically(t *testing.T) {
	svc, actor := provisionFixture(t)
	row, err := svc.Create(context.Background(), actor.ID, CreateTenantInput{
		Code: "company-owner", Name: "Owner company", OwnerEmail: "owner@example.test",
		OwnerPassword: "ValidPassword123!", OwnerDisplayName: "Owner",
		IdempotencyKey: "provision-key-0001",
	})
	require.NoError(t, err)
	require.Equal(t, admin.TenantActive, row.Status)
	var owner admin.AdminUser
	require.NoError(t, svc.DB.Where("email = ?", "owner@example.test").First(&owner).Error)
	require.Equal(t, row.ID, owner.TenantID, "a provisioned enterprise owner must not become a tenant-zero system administrator")
	var membership admin.TenantMembership
	require.NoError(t, svc.DB.Where("tenant_id = ? AND user_id = ?", row.ID, owner.ID).First(&membership).Error)
	require.Equal(t, admin.MembershipActive, membership.Status)
	require.Equal(t, "admin", membership.Role)
	var audit operationlog.OperationLog
	require.NoError(t, svc.DB.Where("tenant_id = ? AND action = ?", row.ID, "tenant.create").First(&audit).Error)
}

func TestProvisionRejectsInvalidSettingsJSON(t *testing.T) {
	svc, actor := provisionFixture(t)
	_, err := svc.Create(context.Background(), actor.ID, CreateTenantInput{
		Code: "invalid-settings", Name: "Invalid settings", OwnerEmail: "owner@example.test",
		OwnerPassword: "ValidPassword123!", SettingsJSON: "[]",
	})
	require.Error(t, err)
	var count int64
	require.NoError(t, svc.DB.Model(&admin.Tenant{}).Count(&count).Error)
	require.Zero(t, count)
}

func TestProvisionRejectsDisabledExistingOwner(t *testing.T) {
	svc, actor := provisionFixture(t)
	owner := admin.AdminUser{Username: admin.NewInternalUsername(), Email: "disabled-owner@example.test", PasswordHash: "test-only", Role: "admin", Status: admin.StatusDisabled}
	require.NoError(t, svc.DB.Create(&owner).Error)
	_, err := svc.Create(context.Background(), actor.ID, CreateTenantInput{
		Code: "disabled-owner", Name: "Disabled owner", OwnerEmail: owner.Email,
	})
	require.Error(t, err)
	var count int64
	require.NoError(t, svc.DB.Model(&admin.Tenant{}).Count(&count).Error)
	require.Zero(t, count)
}

func TestUpdateTenantRejectsOversizedSettingsJSON(t *testing.T) {
	svc, actor := provisionFixture(t)
	tenant := admin.Tenant{Code: "settings-bound", Name: "Settings bound", Status: admin.TenantActive, SettingsJSON: "{}", Version: 1}
	require.NoError(t, svc.DB.Create(&tenant).Error)
	updated, err := svc.UpdateTenant(context.Background(), actor.ID, tenant.ID, TenantUpdateInput{
		Name: tenant.Name, Version: tenant.Version, SettingsJSON: strings.Repeat("x", 64*1024+1),
	})
	require.Error(t, err)
	require.Nil(t, updated)
	var stored admin.Tenant
	require.NoError(t, svc.DB.First(&stored, "id = ?", tenant.ID).Error)
	require.Equal(t, int64(1), stored.Version)
	require.Equal(t, "{}", stored.SettingsJSON)
}

func TestTenantDetailWorksWithoutShopsTable(t *testing.T) {
	svc, _ := provisionFixture(t)
	tenant := admin.Tenant{Code: "no-shops-table", Name: "No shops table", Status: admin.TenantActive, SettingsJSON: "{}", Version: 1}
	require.NoError(t, svc.DB.Create(&tenant).Error)
	detail, err := svc.TenantDetail(context.Background(), tenant.ID)
	require.NoError(t, err)
	require.Equal(t, tenant.ID, detail.Tenant.ID)
	require.Zero(t, detail.ShopCount)
}

func TestCreateInvitationRejectsExistingTenantMember(t *testing.T) {
	svc, actor := provisionFixture(t)
	tenant := admin.Tenant{Code: "existing-member", Name: "Existing member", Status: admin.TenantActive, SettingsJSON: "{}", Version: 1}
	require.NoError(t, svc.DB.Create(&tenant).Error)
	require.NoError(t, svc.DB.Create(&admin.TenantMembership{TenantID: tenant.ID, UserID: actor.ID, Role: "admin", Status: admin.MembershipActive}).Error)
	member := admin.AdminUser{Username: admin.NewInternalUsername(), Email: "member@example.test", PasswordHash: "test-only", Role: "readonly", Status: admin.StatusActive}
	require.NoError(t, svc.DB.Create(&member).Error)
	require.NoError(t, svc.DB.Create(&admin.TenantMembership{TenantID: tenant.ID, UserID: member.ID, Role: "readonly", Status: admin.MembershipActive}).Error)
	_, token, err := svc.CreateInvitation(context.Background(), actor.ID, tenant.ID, InvitationInput{Email: member.Email, Role: "readonly", ExpiresInHours: 24})
	require.Error(t, err)
	require.Empty(t, token)
	var count int64
	require.NoError(t, svc.DB.Model(&admin.TenantInvitation{}).Where("tenant_id = ?", tenant.ID).Count(&count).Error)
	require.Zero(t, count)
}

func TestLeaveWritesTenantAuditInMembershipTransaction(t *testing.T) {
	svc, _ := provisionFixture(t)
	tenant := admin.Tenant{Code: "member-leave", Name: "Member leave", Status: admin.TenantActive, SettingsJSON: "{}", Version: 1}
	require.NoError(t, svc.DB.Create(&tenant).Error)
	member := admin.AdminUser{Username: admin.NewInternalUsername(), Email: "leaving@example.test", PasswordHash: "test-only", Role: "operator", Status: admin.StatusActive}
	require.NoError(t, svc.DB.Create(&member).Error)
	require.NoError(t, svc.DB.Create(&admin.TenantMembership{TenantID: tenant.ID, UserID: member.ID, Role: "operator", Status: admin.MembershipActive}).Error)
	require.NoError(t, svc.Leave(context.Background(), member.ID, tenant.ID))
	var audit operationlog.OperationLog
	require.NoError(t, svc.DB.Where("tenant_id = ? AND admin_user_id = ? AND action = ?", tenant.ID, member.ID, "tenant.membership.leave").First(&audit).Error)
}

func TestAcceptInvitationReplacesOldTenantStoreGrants(t *testing.T) {
	svc, _ := provisionFixture(t)
	tenant := admin.Tenant{Code: "invite-grants", Name: "Invite grants", Status: admin.TenantActive, SettingsJSON: "{}", Version: 1}
	require.NoError(t, svc.DB.Create(&tenant).Error)
	member := admin.AdminUser{Username: admin.NewInternalUsername(), Email: "rejoin@example.test", PasswordHash: "test-only", Role: "admin", Status: admin.StatusActive}
	require.NoError(t, svc.DB.Create(&member).Error)
	require.NoError(t, svc.DB.Create(&admin.TenantMembership{TenantID: tenant.ID, UserID: member.ID, Role: "readonly", Status: admin.MembershipDisabled}).Error)
	store := shop.Shop{TenantID: tenant.ID, Platform: "shopee", ShopName: "Invite shop", Status: "active", AuthStatus: "ok"}
	require.NoError(t, svc.DB.Create(&store).Error)
	require.NoError(t, svc.DB.Create(&admin.UserStorePermission{UserID: member.ID, StoreID: store.ID, Platform: store.Platform, PermissionScope: admin.StorePermScopeManage}).Error)
	grants, err := json.Marshal([]admin.UserStorePermission{{StoreID: store.ID, Platform: store.Platform, PermissionScope: admin.StorePermScopeView}})
	require.NoError(t, err)
	token := "rejoin-invitation-token"
	inv := admin.TenantInvitation{
		TenantID: tenant.ID, Email: member.Email, Role: "readonly", StoreGrantsJSON: string(grants),
		TokenHash: invitationHash(token), Status: admin.InvitationPending, ExpiresAt: time.Now().UTC().Add(time.Hour), CreatedBy: member.ID,
	}
	require.NoError(t, svc.DB.Create(&inv).Error)

	gotTenantID, err := svc.AcceptInvitation(context.Background(), member.ID, token, member.Email)
	require.NoError(t, err)
	require.Equal(t, tenant.ID, gotTenantID)
	var stored []admin.UserStorePermission
	require.NoError(t, svc.DB.Where("user_id = ? AND store_id = ?", member.ID, store.ID).Find(&stored).Error)
	require.Len(t, stored, 1)
	require.Equal(t, admin.StorePermScopeView, stored[0].PermissionScope)
}
