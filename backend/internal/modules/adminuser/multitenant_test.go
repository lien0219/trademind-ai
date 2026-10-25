package adminuser

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/auth"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"gorm.io/gorm"
)

func TestTenantMutationsRecheckActorAfterAuthorization(t *testing.T) {
	for _, action := range []string{"create", "promote-self", "delete", "stores", "reset", "create-role", "update-role", "delete-role"} {
		t.Run(action, func(t *testing.T) {
			svc, actor, target := tenantMembershipFixture(t)
			role := admin.RoleDefinition{TenantID: actor.TenantID, Code: "r_0123456789abcdef01234567", Name: "Original", PermissionsJSON: "[]", Enabled: true}
			require.NoError(t, svc.DB.Create(&role).Error)
			c := securityContext(actor)
			require.True(t, (&Handler{Svc: svc}).requireManage(c))
			// The request has passed its HTTP permission check. A different
			// committed request revokes the actor before its mutation starts.
			require.NoError(t, svc.DB.Model(&admin.TenantMembership{}).
				Where("tenant_id = ? AND user_id = ?", actor.TenantID, actor.ID).
				Update("role", adminperm.RoleReadonly).Error)
			var err error
			switch action {
			case "create":
				_, err = svc.Create(c, CreateBody{Email: "new@example.test", Password: "test-password-123", Role: adminperm.RoleAdmin}, &actor.ID)
			case "promote-self":
				adminRole := adminperm.RoleAdmin
				_, err = svc.Update(c, actor.ID, UpdateBody{Role: &adminRole}, &actor.ID)
			case "delete":
				err = svc.Delete(c, target.ID, &actor.ID)
			case "stores":
				_, err = svc.SetStorePermissions(c, target.ID, SetStorePermissionsBody{}, &actor.ID)
			case "reset":
				var secret string
				secret, err = svc.ResetPassword(c, target.ID, &actor.ID)
				require.Empty(t, secret)
			case "delete-role":
				err = svc.DeleteRole(c, role.Code, &actor.ID)
			default:
				c.Params = gin.Params{{Key: "code", Value: role.Code}}
				c.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/admin/roles/"+role.Code, bytes.NewBufferString(`{"name":"Changed","permissions":[]}`))
				c.Request.Header.Set("Content-Type", "application/json")
				if action == "create-role" {
					(&Handler{Svc: svc}).CreateRole(c)
				} else {
					(&Handler{Svc: svc}).UpdateRole(c)
				}
				require.Equal(t, http.StatusForbidden, c.Writer.Status())
				return
			}
			require.Error(t, err, "a revoked actor must not mutate tenant security state")
			var member admin.TenantMembership
			require.NoError(t, svc.DB.First(&member, "tenant_id = ? AND user_id = ?", actor.TenantID, actor.ID).Error)
			require.Equal(t, adminperm.RoleReadonly, member.Role)
			var unchanged admin.AdminUser
			require.NoError(t, svc.DB.First(&unchanged, "id = ?", target.ID).Error)
			require.Equal(t, target.PasswordHash, unchanged.PasswordHash)
		})
	}
}

func TestTenantAdminCannotResetUnownedOrPlatformIdentity(t *testing.T) {
	for _, platform := range []bool{false, true} {
		t.Run(map[bool]string{false: "foreign-origin", true: "platform-operator"}[platform], func(t *testing.T) {
			svc, actor, target := tenantMembershipFixture(t)
			if platform {
				require.NoError(t, svc.DB.AutoMigrate(&admin.PlatformOperator{}))
				require.NoError(t, svc.DB.Create(&admin.PlatformOperator{UserID: target.ID, Enabled: true}).Error)
			} else {
				require.NoError(t, svc.DB.Model(&target).Update("tenant_id", 99).Error)
			}
			secret, err := svc.ResetPassword(securityContext(actor), target.ID, &actor.ID)
			require.Error(t, err)
			require.Empty(t, secret)
			var unchanged admin.AdminUser
			require.NoError(t, svc.DB.First(&unchanged, "id = ?", target.ID).Error)
			require.Equal(t, target.PasswordHash, unchanged.PasswordHash)
			require.Equal(t, target.TokenVersion, unchanged.TokenVersion)
		})
	}
}

func tenantMembershipFixture(t *testing.T) (*Service, admin.AdminUser, admin.AdminUser) {
	t.Helper()
	svc, actor, target := securityFixture(t)
	require.NoError(t, svc.DB.AutoMigrate(&admin.Tenant{}, &admin.TenantMembership{}))
	require.NoError(t, svc.DB.Create(&admin.Tenant{ID: 17, Code: "tenant-17", Name: "Tenant 17", Status: admin.TenantActive}).Error)
	require.NoError(t, svc.DB.Create(&admin.Tenant{ID: 99, Code: "tenant-99", Name: "Tenant 99", Status: admin.TenantActive}).Error)
	require.NoError(t, svc.DB.Create(&admin.TenantMembership{
		TenantID: 17, UserID: actor.ID, Role: adminperm.RoleAdmin, Status: admin.MembershipActive,
	}).Error)
	require.NoError(t, svc.DB.Create(&admin.TenantMembership{
		TenantID: 17, UserID: target.ID, Role: adminperm.RoleOperator, Status: admin.MembershipActive,
	}).Error)
	return svc, actor, target
}

func TestAdminUserServiceUsesTenantMembershipForListGetAndUpdate(t *testing.T) {
	svc, actor, target := tenantMembershipFixture(t)
	require.NoError(t, svc.DB.Model(&target).Updates(map[string]any{
		"tenant_id": 99, "role": adminperm.RoleAdmin,
	}).Error)
	c := securityContext(actor)

	listed, err := svc.List(c, ListQuery{Role: adminperm.RoleOperator})
	require.NoError(t, err)
	require.EqualValues(t, 1, listed.Total)
	require.Len(t, listed.Items, 1)
	require.Equal(t, target.ID.String(), listed.Items[0].ID)
	require.Equal(t, adminperm.RoleOperator, listed.Items[0].Role)

	got, err := svc.Get(c, target.ID)
	require.NoError(t, err)
	require.Equal(t, adminperm.RoleOperator, got.Role)

	newRole := adminperm.RoleReviewer
	newStatus := admin.MembershipDisabled
	updated, err := svc.Update(c, target.ID, UpdateBody{Role: &newRole, Status: &newStatus}, &actor.ID)
	require.NoError(t, err)
	require.Equal(t, newRole, updated.Role)
	require.Equal(t, newStatus, updated.Status)

	var membership admin.TenantMembership
	require.NoError(t, svc.DB.First(&membership, "tenant_id = ? AND user_id = ?", 17, target.ID).Error)
	require.Equal(t, newRole, membership.Role)
	require.Equal(t, newStatus, membership.Status)
	var identity admin.AdminUser
	require.NoError(t, svc.DB.First(&identity, "id = ?", target.ID).Error)
	require.EqualValues(t, 99, identity.TenantID)
	require.Equal(t, adminperm.RoleAdmin, identity.Role)
}

func TestCreateTenantMemberRequiresExistingIdentityConsent(t *testing.T) {
	svc, actor, _ := tenantMembershipFixture(t)
	require.NoError(t, svc.DB.AutoMigrate(&auth.AuthSession{}, &auth.AuthRefreshToken{}))
	shared := admin.AdminUser{
		TenantID: 99, Username: admin.NewInternalUsername(), Email: "shared@example.test",
		PasswordHash: "existing-hash", Role: adminperm.RoleAdmin, Status: admin.StatusActive, TokenVersion: 7,
	}
	require.NoError(t, svc.DB.Create(&shared).Error)
	require.NoError(t, svc.DB.Create(&admin.TenantMembership{
		TenantID: 99, UserID: shared.ID, Role: adminperm.RoleAdmin, Status: admin.MembershipActive,
	}).Error)
	session := auth.AuthSession{UserID: shared.ID, TenantID: 99, TokenVersion: 7, Status: auth.SessionStatusActive}
	require.NoError(t, svc.DB.Create(&session).Error)

	created, err := svc.Create(securityContext(actor), CreateBody{
		Email: shared.Email, Password: "test-password-123", Role: adminperm.RoleReviewer,
	}, &actor.ID)
	require.Error(t, err, "a tenant administrator cannot enrol an existing identity without its consent")
	require.Nil(t, created)

	var identity admin.AdminUser
	require.NoError(t, svc.DB.First(&identity, "id = ?", shared.ID).Error)
	require.EqualValues(t, 99, identity.TenantID)
	require.Equal(t, adminperm.RoleAdmin, identity.Role)
	require.Equal(t, shared.PasswordHash, identity.PasswordHash)
	require.Equal(t, shared.TokenVersion, identity.TokenVersion)
	var memberships int64
	require.NoError(t, svc.DB.Model(&admin.TenantMembership{}).Where("tenant_id = ? AND user_id = ?", actor.TenantID, shared.ID).Count(&memberships).Error)
	require.Zero(t, memberships)
	var unchangedSession auth.AuthSession
	require.NoError(t, svc.DB.First(&unchangedSession, "id = ?", session.ID).Error)
	require.Equal(t, auth.SessionStatusActive, unchangedSession.Status)
}

func TestUpdateTenantMemberRejectsDisabledCustomRole(t *testing.T) {
	svc, actor, target := tenantMembershipFixture(t)
	role := admin.RoleDefinition{
		TenantID: actor.TenantID, Code: "r_0123456789abcdef01234567", Name: "Disabled",
		PermissionsJSON: "[]", Enabled: false,
	}
	require.NoError(t, svc.DB.Create(&role).Error)

	_, err := svc.Update(securityContext(actor), target.ID, UpdateBody{Role: &role.Code}, &actor.ID)
	require.Error(t, err)
	var membership admin.TenantMembership
	require.NoError(t, svc.DB.First(&membership, "tenant_id = ? AND user_id = ?", actor.TenantID, target.ID).Error)
	require.Equal(t, adminperm.RoleOperator, membership.Role)
}

func TestSetTenantStorePermissionsRetainsOtherTenantGrants(t *testing.T) {
	svc, actor, target := tenantMembershipFixture(t)
	require.NoError(t, svc.DB.Model(&target).Updates(map[string]any{
		"tenant_id": 99, "role": adminperm.RoleAdmin,
	}).Error)
	currentShop := shop.Shop{TenantID: 17, Platform: "shopee", ShopName: "Current shop", Status: "active", AuthStatus: "ok"}
	foreignShop := shop.Shop{TenantID: 99, Platform: "shopee", ShopName: "Foreign shop", Status: "active", AuthStatus: "ok"}
	require.NoError(t, svc.DB.Create(&currentShop).Error)
	require.NoError(t, svc.DB.Create(&foreignShop).Error)
	currentGrant := admin.UserStorePermission{UserID: target.ID, StoreID: currentShop.ID, PermissionScope: admin.StorePermScopeView}
	foreignGrant := admin.UserStorePermission{UserID: target.ID, StoreID: foreignShop.ID, PermissionScope: admin.StorePermScopeManage}
	require.NoError(t, svc.DB.Create(&currentGrant).Error)
	require.NoError(t, svc.DB.Create(&foreignGrant).Error)

	updated, err := svc.SetStorePermissions(securityContext(actor), target.ID, SetStorePermissionsBody{}, &actor.ID)
	require.NoError(t, err)
	require.Equal(t, adminperm.RoleOperator, updated.Role, "membership role is authoritative for store grant policy")
	require.Empty(t, updated.StorePermissions)
	require.Error(t, svc.DB.First(&admin.UserStorePermission{}, "id = ?", currentGrant.ID).Error)
	require.NoError(t, svc.DB.First(&admin.UserStorePermission{}, "id = ?", foreignGrant.ID).Error)

	var foreignCount int64
	require.NoError(t, svc.DB.Model(&admin.UserStorePermission{}).
		Where("user_id = ? AND store_id = ?", target.ID, foreignShop.ID).Count(&foreignCount).Error)
	require.EqualValues(t, 1, foreignCount)
}

func TestDeleteTenantMemberRetainsSharedIdentity(t *testing.T) {
	svc, actor, target := tenantMembershipFixture(t)
	require.NoError(t, svc.DB.Model(&target).Updates(map[string]any{
		"tenant_id": 99, "role": adminperm.RoleAdmin,
	}).Error)
	originalVersion := target.TokenVersion

	err := svc.Delete(securityContext(actor), target.ID, &actor.ID)
	require.NoError(t, err)

	var membership admin.TenantMembership
	require.NoError(t, svc.DB.First(&membership, "tenant_id = ? AND user_id = ?", actor.TenantID, target.ID).Error)
	require.Equal(t, admin.MembershipRemoved, membership.Status)
	var identity admin.AdminUser
	require.NoError(t, svc.DB.First(&identity, "id = ?", target.ID).Error)
	require.EqualValues(t, 99, identity.TenantID)
	require.Equal(t, adminperm.RoleAdmin, identity.Role)
	require.Equal(t, originalVersion+1, identity.TokenVersion)
}

func TestDeleteTenantMemberRejectsAlreadyRemovedMembership(t *testing.T) {
	svc, actor, target := tenantMembershipFixture(t)
	require.NoError(t, svc.DB.Model(&admin.TenantMembership{}).
		Where("tenant_id = ? AND user_id = ?", actor.TenantID, target.ID).
		Update("status", admin.MembershipRemoved).Error)

	err := svc.Delete(securityContext(actor), target.ID, &actor.ID)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestResetPasswordRejectsSharedIdentityFromTenantAdmin(t *testing.T) {
	svc, actor, target := tenantMembershipFixture(t)
	require.NoError(t, svc.DB.Model(&target).Updates(map[string]any{
		"tenant_id": 99, "role": adminperm.RoleAdmin,
	}).Error)
	require.NoError(t, svc.DB.Create(&admin.TenantMembership{
		TenantID: 99, UserID: target.ID, Role: adminperm.RoleAdmin, Status: admin.MembershipActive,
	}).Error)

	passwordHash := target.PasswordHash
	_, err := svc.ResetPassword(securityContext(actor), target.ID, &actor.ID)
	require.Error(t, err)
	var identity admin.AdminUser
	require.NoError(t, svc.DB.First(&identity, "id = ?", target.ID).Error)
	require.Equal(t, passwordHash, identity.PasswordHash)
}

func TestResetPasswordRevokesIdentitySessionsAndRefreshTokens(t *testing.T) {
	svc, actor, target := tenantMembershipFixture(t)
	require.NoError(t, svc.DB.AutoMigrate(&auth.AuthSession{}, &auth.AuthRefreshToken{}))
	current := auth.AuthSession{TenantID: actor.TenantID, UserID: target.ID, Status: auth.SessionStatusActive}
	otherTenant := auth.AuthSession{TenantID: 99, UserID: target.ID, Status: auth.SessionStatusActive}
	actorSession := auth.AuthSession{TenantID: actor.TenantID, UserID: actor.ID, Status: auth.SessionStatusActive}
	require.NoError(t, svc.DB.Create(&current).Error)
	require.NoError(t, svc.DB.Create(&otherTenant).Error)
	require.NoError(t, svc.DB.Create(&actorSession).Error)
	activeRefresh := auth.AuthRefreshToken{
		TenantID: actor.TenantID, UserID: target.ID, SessionID: current.ID, TokenFamilyID: uuid.New(),
		TokenHash: "reset-active-refresh", Status: auth.RefreshStatusActive, ExpiresAt: time.Now().Add(time.Hour),
	}
	rotatedRefresh := auth.AuthRefreshToken{
		TenantID: 99, UserID: target.ID, SessionID: otherTenant.ID, TokenFamilyID: uuid.New(),
		TokenHash: "reset-rotated-refresh", Status: auth.RefreshStatusRotated, ExpiresAt: time.Now().Add(time.Hour),
	}
	actorRefresh := auth.AuthRefreshToken{
		TenantID: actor.TenantID, UserID: actor.ID, SessionID: actorSession.ID, TokenFamilyID: uuid.New(),
		TokenHash: "reset-actor-refresh", Status: auth.RefreshStatusActive, ExpiresAt: time.Now().Add(time.Hour),
	}
	require.NoError(t, svc.DB.Create(&activeRefresh).Error)
	require.NoError(t, svc.DB.Create(&rotatedRefresh).Error)
	require.NoError(t, svc.DB.Create(&actorRefresh).Error)

	_, err := svc.ResetPassword(securityContext(actor), target.ID, &actor.ID)
	require.NoError(t, err)

	for _, sessionID := range []uuid.UUID{current.ID, otherTenant.ID} {
		var session auth.AuthSession
		require.NoError(t, svc.DB.First(&session, "id = ?", sessionID).Error)
		require.Equal(t, auth.SessionStatusRevoked, session.Status)
		require.Equal(t, "password_reset", session.RevokeReason)
	}
	var unchangedSession auth.AuthSession
	require.NoError(t, svc.DB.First(&unchangedSession, "id = ?", actorSession.ID).Error)
	require.Equal(t, auth.SessionStatusActive, unchangedSession.Status)
	for _, tokenHash := range []string{activeRefresh.TokenHash, rotatedRefresh.TokenHash} {
		var refresh auth.AuthRefreshToken
		require.NoError(t, svc.DB.First(&refresh, "token_hash = ?", tokenHash).Error)
		require.Equal(t, auth.RefreshStatusRevoked, refresh.Status)
		require.Equal(t, "password_reset", refresh.RevokeReason)
	}
	var unchangedRefresh auth.AuthRefreshToken
	require.NoError(t, svc.DB.First(&unchangedRefresh, "token_hash = ?", actorRefresh.TokenHash).Error)
	require.Equal(t, auth.RefreshStatusActive, unchangedRefresh.Status)
}

func TestRoleUpdateCountsMembershipsAndInvalidatesSharedIdentity(t *testing.T) {
	svc, actor, target := tenantMembershipFixture(t)
	require.NoError(t, svc.DB.AutoMigrate(&auth.AuthSession{}, &auth.AuthRefreshToken{}))
	require.NoError(t, svc.DB.Create(&admin.TenantMembership{
		TenantID: 99, UserID: target.ID, Role: adminperm.RoleAdmin, Status: admin.MembershipActive,
	}).Error)
	role := admin.RoleDefinition{
		TenantID: actor.TenantID, Code: "r_0123456789abcdef01234567", Name: "Tenant role",
		PermissionsJSON: `["product.view"]`, Enabled: true,
	}
	require.NoError(t, svc.DB.Create(&role).Error)
	newRole := role.Code
	_, err := svc.Update(securityContext(actor), target.ID, UpdateBody{Role: &newRole}, &actor.ID)
	require.NoError(t, err)
	var before admin.AdminUser
	require.NoError(t, svc.DB.First(&before, "id = ?", target.ID).Error)
	require.NoError(t, svc.DB.Model(&admin.TenantMembership{}).
		Where("tenant_id = ? AND user_id = ?", actor.TenantID, target.ID).
		Update("role", "  "+strings.ToUpper(role.Code)+" ").Error)
	memberCount, err := tenantMembershipRoleCount(securityContext(actor).Request.Context(), svc.DB, actor.TenantID, role.Code)
	require.NoError(t, err)
	require.EqualValues(t, 1, memberCount)
	targetSession := auth.AuthSession{TenantID: actor.TenantID, UserID: target.ID, Status: auth.SessionStatusActive}
	otherTenantSession := auth.AuthSession{TenantID: 99, UserID: target.ID, Status: auth.SessionStatusActive}
	actorSession := auth.AuthSession{TenantID: actor.TenantID, UserID: actor.ID, Status: auth.SessionStatusActive}
	require.NoError(t, svc.DB.Create(&targetSession).Error)
	require.NoError(t, svc.DB.Create(&otherTenantSession).Error)
	require.NoError(t, svc.DB.Create(&actorSession).Error)
	targetRefresh := auth.AuthRefreshToken{
		TenantID: actor.TenantID, UserID: target.ID, SessionID: targetSession.ID, TokenFamilyID: uuid.New(),
		TokenHash: "role-update-target-refresh", Status: auth.RefreshStatusActive, ExpiresAt: time.Now().Add(time.Hour),
	}
	otherTenantRefresh := auth.AuthRefreshToken{
		TenantID: 99, UserID: target.ID, SessionID: otherTenantSession.ID, TokenFamilyID: uuid.New(),
		TokenHash: "role-update-target-other-tenant-refresh", Status: auth.RefreshStatusRotated, ExpiresAt: time.Now().Add(time.Hour),
	}
	actorRefresh := auth.AuthRefreshToken{
		TenantID: actor.TenantID, UserID: actor.ID, SessionID: actorSession.ID, TokenFamilyID: uuid.New(),
		TokenHash: "role-update-actor-refresh", Status: auth.RefreshStatusActive, ExpiresAt: time.Now().Add(time.Hour),
	}
	require.NoError(t, svc.DB.Create(&targetRefresh).Error)
	require.NoError(t, svc.DB.Create(&otherTenantRefresh).Error)
	require.NoError(t, svc.DB.Create(&actorRefresh).Error)

	w := roleRequest(t, svc, actor, http.MethodPatch, "/api/v1/admin/roles/"+role.Code, map[string]any{"enabled": false})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var result struct {
		Data RoleRow `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	require.EqualValues(t, 1, result.Data.MemberCount)
	var after admin.AdminUser
	require.NoError(t, svc.DB.First(&after, "id = ?", target.ID).Error)
	require.Equal(t, before.TokenVersion+1, after.TokenVersion)
	for _, sessionID := range []uuid.UUID{targetSession.ID, otherTenantSession.ID} {
		var revokedSession auth.AuthSession
		require.NoError(t, svc.DB.First(&revokedSession, "id = ?", sessionID).Error)
		require.Equal(t, auth.SessionStatusRevoked, revokedSession.Status)
		require.Equal(t, "role_changed", revokedSession.RevokeReason)
	}
	var activeActorSession auth.AuthSession
	require.NoError(t, svc.DB.First(&activeActorSession, "id = ?", actorSession.ID).Error)
	require.Equal(t, auth.SessionStatusActive, activeActorSession.Status)
	for _, tokenHash := range []string{targetRefresh.TokenHash, otherTenantRefresh.TokenHash} {
		var revokedRefresh auth.AuthRefreshToken
		require.NoError(t, svc.DB.First(&revokedRefresh, "token_hash = ?", tokenHash).Error)
		require.Equal(t, auth.RefreshStatusRevoked, revokedRefresh.Status)
		require.Equal(t, "role_changed", revokedRefresh.RevokeReason)
	}
	var activeActorRefresh auth.AuthRefreshToken
	require.NoError(t, svc.DB.First(&activeActorRefresh, "token_hash = ?", actorRefresh.TokenHash).Error)
	require.Equal(t, auth.RefreshStatusActive, activeActorRefresh.Status)
}
