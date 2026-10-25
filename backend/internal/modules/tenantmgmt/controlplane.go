package tenantmgmt

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/authutil"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type TenantUpdateInput struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	LegalName    string `json:"legalName"`
	ContactEmail string `json:"contactEmail"`
	ContactPhone string `json:"contactPhone"`
	SettingsJSON string `json:"settings"`
	Version      int64  `json:"version"`
}

type TenantSummary struct {
	Tenant      admin.Tenant `json:"tenant"`
	MemberCount int64        `json:"memberCount"`
	ShopCount   int64        `json:"shopCount"`
}

type MemberView struct {
	ID               uuid.UUID                   `json:"id"`
	UserID           uuid.UUID                   `json:"userId"`
	Email            string                      `json:"email"`
	DisplayName      string                      `json:"displayName"`
	Role             string                      `json:"role"`
	Status           string                      `json:"status"`
	StorePermissions []admin.UserStorePermission `json:"storePermissions"`
}

type InvitationInput struct {
	Email            string
	Role             string
	StorePermissions []admin.UserStorePermission `json:"storePermissions"`
	ExpiresInHours   int                         `json:"expiresInHours"`
}

const maxTenantSettingsBytes = 64 * 1024

// normalizeSettingsJSON accepts only a bounded JSON object. Settings are
// stored as text for dialect compatibility, but malformed or unbounded input
// must never enter the control plane.
func normalizeSettingsJSON(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "{}", nil
	}
	if len([]byte(raw)) > maxTenantSettingsBytes {
		return "", errors.New("tenant settings too large")
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &object); err != nil || object == nil {
		return "", errors.New("tenant settings must be a JSON object")
	}
	normalized, err := json.Marshal(object)
	if err != nil {
		return "", errors.New("invalid tenant settings")
	}
	return string(normalized), nil
}

func tenantFromContext(ctx context.Context, db *gorm.DB) (int64, error) {
	if ctx == nil || db == nil {
		return 0, errors.New("tenant context unavailable")
	}
	var tenantID int64
	if raw, ok := ctx.Value(tenantContextValue{}).(int64); ok {
		tenantID = raw
	}
	if tenantID <= 0 {
		return 0, errors.New("tenant context unavailable")
	}
	return tenantID, nil
}

// TenantIDFromContext is intentionally a small adapter for handlers that
// already carry a trusted security context. HTTP callers should use the
// adminperm helper, which reads the authenticated Gin context.
type tenantContextValue struct{}

func (s *Service) requireTenantAdmin(ctx context.Context, tenantID int64, userID uuid.UUID) (*admin.TenantMembership, error) {
	return s.requireTenantPermission(ctx, tenantID, userID, adminperm.RoleAdmin)
}

func (s *Service) requireTenantPermission(ctx context.Context, tenantID int64, userID uuid.UUID, permission string) (*admin.TenantMembership, error) {
	if s == nil || s.DB == nil || tenantID <= 0 || userID == uuid.Nil {
		return nil, errors.New("tenant permission required")
	}
	var m admin.TenantMembership
	err := s.DB.WithContext(ctx).Where("tenant_id = ? AND user_id = ? AND status = ?", tenantID, userID, admin.MembershipActive).First(&m).Error
	if err != nil {
		return nil, errors.New("tenant permission required")
	}
	if permission == adminperm.RoleAdmin && isTenantAdminRole(m.Role) {
		return &m, nil
	}
	if !tenantRoleHasPermission(ctx, s.DB, tenantID, m.Role, permission) {
		return nil, errors.New("tenant permission required")
	}
	return &m, nil
}

func tenantRoleHasPermission(ctx context.Context, db *gorm.DB, tenantID int64, role, permission string) bool {
	role = strings.TrimSpace(strings.ToLower(role))
	if adminperm.IsCustomRole(role) {
		var definition admin.RoleDefinition
		if err := db.WithContext(ctx).Where("tenant_id = ? AND code = ? AND enabled = ?", tenantID, role, true).First(&definition).Error; err != nil {
			return false
		}
		var grants []string
		if json.Unmarshal([]byte(definition.PermissionsJSON), &grants) != nil {
			return false
		}
		validated, err := adminperm.ValidateCustomPermissions(grants)
		if err != nil {
			return false
		}
		for _, grant := range validated {
			if grant == permission {
				return true
			}
		}
		return false
	}
	return adminperm.HasPermission(role, permission)
}

func isTenantAdminRole(role string) bool {
	return strings.EqualFold(strings.TrimSpace(role), adminperm.RoleAdmin)
}

func normalizeTenantRole(role string) (string, error) {
	role = strings.TrimSpace(strings.ToLower(role))
	switch role {
	case adminperm.RoleAdmin, adminperm.RoleOperator, adminperm.RoleReviewer, adminperm.RoleReadonly:
		return role, nil
	}
	if adminperm.IsCustomRole(role) {
		return role, nil
	}
	return "", errors.New("invalid tenant role")
}

func lockTenantMutation(tx *gorm.DB, tenantID int64) error {
	if tx == nil || tenantID <= 0 {
		return errors.New("tenant mutation lock unavailable")
	}
	// Keep the same lock namespace used by adminuser so concurrent role and
	// membership changes serialize across old and new endpoints.
	sum := sha256.Sum256([]byte(fmt.Sprintf("trademind:admin-users:%d", tenantID)))
	keyHash := hex.EncodeToString(sum[:])
	lock := &admin.AdminMutationLock{KeyHash: keyHash}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(lock).Error; err != nil {
		return err
	}
	return tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("key_hash = ?", keyHash).Take(lock).Error
}

func lockGlobalAccountIdentity(tx *gorm.DB) error {
	if tx == nil {
		return errors.New("account identity lock unavailable")
	}
	sum := sha256.Sum256([]byte("trademind:admin-account-identities"))
	keyHash := hex.EncodeToString(sum[:])
	lock := &admin.AdminMutationLock{KeyHash: keyHash}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(lock).Error; err != nil {
		return err
	}
	return tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("key_hash = ?", keyHash).Take(lock).Error
}

func (s *Service) UpdateTenant(ctx context.Context, actor uuid.UUID, id int64, in TenantUpdateInput) (*admin.Tenant, error) {
	if err := s.requireOperator(ctx, actor); err != nil {
		return nil, err
	}
	if id <= 0 || strings.TrimSpace(in.Name) == "" || len([]rune(in.Name)) > 128 || in.Version <= 0 {
		return nil, errors.New("invalid tenant update")
	}
	var row admin.Tenant
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.requireOperatorTx(ctx, tx, actor); err != nil {
			return err
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, "id = ?", id).Error; err != nil {
			return err
		}
		if row.Version != in.Version {
			return errors.New("tenant version conflict")
		}
		updates := map[string]any{"name": strings.TrimSpace(in.Name), "description": strings.TrimSpace(in.Description), "legal_name": strings.TrimSpace(in.LegalName), "contact_email": strings.TrimSpace(strings.ToLower(in.ContactEmail)), "contact_phone": strings.TrimSpace(in.ContactPhone), "version": row.Version + 1}
		if strings.TrimSpace(in.SettingsJSON) != "" {
			settingsJSON, err := normalizeSettingsJSON(in.SettingsJSON)
			if err != nil {
				return err
			}
			updates["settings_json"] = settingsJSON
		}
		if err := tx.Model(&row).Updates(updates).Error; err != nil {
			return err
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, "id = ?", id).Error; err != nil {
			return err
		}
		return s.Audit.WriteTxContext(ctx, tx, operationlog.WriteOpts{TenantID: id, AdminUserID: &actor, Action: "tenant.update", Resource: "tenant", ResourceID: fmt.Sprint(id), Status: "success"})
	})
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (s *Service) UpdateTenantAsMember(ctx context.Context, actor uuid.UUID, tenantID int64, in TenantUpdateInput) (*admin.Tenant, error) {
	if _, err := s.requireTenantPermission(ctx, tenantID, actor, adminperm.PermSettingsManage); err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.Name) == "" || len([]rune(in.Name)) > 128 || in.Version <= 0 {
		return nil, errors.New("invalid tenant update")
	}
	var row admin.Tenant
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockTenantMutation(tx, tenantID); err != nil {
			return err
		}
		if err := requireTenantPermissionTx(ctx, tx, tenantID, actor, adminperm.PermSettingsManage); err != nil {
			return err
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, "id = ? AND status = ?", tenantID, admin.TenantActive).Error; err != nil {
			return err
		}
		if row.Version != in.Version {
			return errors.New("tenant version conflict")
		}
		updates := map[string]any{"name": strings.TrimSpace(in.Name), "description": strings.TrimSpace(in.Description), "legal_name": strings.TrimSpace(in.LegalName), "contact_email": strings.TrimSpace(strings.ToLower(in.ContactEmail)), "contact_phone": strings.TrimSpace(in.ContactPhone), "version": row.Version + 1}
		if strings.TrimSpace(in.SettingsJSON) != "" {
			settingsJSON, err := normalizeSettingsJSON(in.SettingsJSON)
			if err != nil {
				return err
			}
			updates["settings_json"] = settingsJSON
		}
		if err := tx.Model(&row).Updates(updates).Error; err != nil {
			return err
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, "id = ? AND status = ?", tenantID, admin.TenantActive).Error; err != nil {
			return err
		}
		return s.Audit.WriteTxContext(ctx, tx, operationlog.WriteOpts{TenantID: tenantID, AdminUserID: &actor, Action: "tenant.profile.update", Resource: "tenant", ResourceID: fmt.Sprint(tenantID), Status: "success"})
	})
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (s *Service) ListTenants(ctx context.Context, keyword, status string, page, pageSize int) ([]admin.Tenant, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 50
	}
	q := s.DB.WithContext(ctx).Model(&admin.Tenant{})
	if strings.TrimSpace(keyword) != "" {
		like := "%" + strings.TrimSpace(keyword) + "%"
		q = q.Where("code LIKE ? OR name LIKE ?", like, like)
	}
	if strings.TrimSpace(status) != "" {
		q = q.Where("status = ?", strings.TrimSpace(strings.ToLower(status)))
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []admin.Tenant
	if err := q.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (s *Service) TenantDetail(ctx context.Context, id int64) (*TenantSummary, error) {
	var row admin.Tenant
	if err := s.DB.WithContext(ctx).First(&row, "id = ?", id).Error; err != nil {
		return nil, err
	}
	var members, shops int64
	if err := s.DB.WithContext(ctx).Model(&admin.TenantMembership{}).Where("tenant_id = ? AND status = ?", id, admin.MembershipActive).Count(&members).Error; err != nil {
		return nil, err
	}
	if s.DB.Migrator().HasTable("shops") {
		if err := s.DB.WithContext(ctx).Table("shops").Where("tenant_id = ?", id).Count(&shops).Error; err != nil {
			return nil, err
		}
	}
	return &TenantSummary{Tenant: row, MemberCount: members, ShopCount: shops}, nil
}

func (s *Service) CurrentTenant(ctx context.Context, tenantID int64) (*TenantSummary, error) {
	return s.TenantDetail(ctx, tenantID)
}

func (s *Service) Members(ctx context.Context, tenantID int64) ([]MemberView, error) {
	var memberships []admin.TenantMembership
	if err := s.DB.WithContext(ctx).Where("tenant_id = ? AND status <> ?", tenantID, admin.MembershipRemoved).Order("created_at ASC").Find(&memberships).Error; err != nil {
		return nil, err
	}
	items := make([]MemberView, 0, len(memberships))
	for _, m := range memberships {
		var u admin.AdminUser
		if err := s.DB.WithContext(ctx).Select("id,email,display_name").First(&u, "id = ?", m.UserID).Error; err != nil {
			return nil, err
		}
		var grants []admin.UserStorePermission
		if s.DB.Migrator().HasTable(&admin.UserStorePermission{}) && s.DB.Migrator().HasTable("shops") {
			if err := s.DB.WithContext(ctx).Where("user_id = ? AND store_id IN (SELECT id FROM shops WHERE tenant_id = ?)", m.UserID, tenantID).Find(&grants).Error; err != nil {
				return nil, err
			}
		}
		items = append(items, MemberView{ID: m.ID, UserID: m.UserID, Email: u.Email, DisplayName: u.DisplayName, Role: m.Role, Status: m.Status, StorePermissions: grants})
	}
	return items, nil
}

func (s *Service) UpdateMember(ctx context.Context, actor uuid.UUID, tenantID int64, membershipID uuid.UUID, role, status string, grants []admin.UserStorePermission) error {
	if _, err := s.requireTenantPermission(ctx, tenantID, actor, adminperm.PermUserManage); err != nil {
		return err
	}
	role, err := normalizeTenantRole(role)
	if err != nil {
		return err
	}
	status = strings.TrimSpace(strings.ToLower(status))
	if status != admin.MembershipActive && status != admin.MembershipDisabled && status != admin.MembershipRemoved {
		return errors.New("invalid membership status")
	}
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockTenantMutation(tx, tenantID); err != nil {
			return err
		}
		if err := requireTenantPermissionTx(ctx, tx, tenantID, actor, adminperm.PermUserManage); err != nil {
			return err
		}
		var m admin.TenantMembership
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND tenant_id = ?", membershipID, tenantID).First(&m).Error; err != nil {
			return err
		}
		if m.Status != admin.MembershipActive && m.Status != admin.MembershipDisabled {
			return gorm.ErrRecordNotFound
		}
		if (status != admin.MembershipActive || !isTenantAdminRole(role)) && isTenantAdminRole(m.Role) {
			var count int64
			if err := tx.Model(&admin.TenantMembership{}).Where("tenant_id = ? AND status = ? AND role = ? AND id <> ?", tenantID, admin.MembershipActive, adminperm.RoleAdmin, m.ID).Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				return errors.New("cannot remove last tenant administrator")
			}
		}
		if err := validateTenantStoreGrants(tx, tenantID, grants); err != nil {
			return err
		}
		if adminperm.IsCustomRole(role) {
			var definition admin.RoleDefinition
			if err := tx.Where("tenant_id = ? AND code = ? AND enabled = ?", tenantID, role, true).First(&definition).Error; err != nil {
				return errors.New("tenant role unavailable")
			}
		}
		if err := tx.Model(&m).Updates(map[string]any{"role": role, "status": status}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ? AND store_id IN (SELECT id FROM shops WHERE tenant_id = ?)", m.UserID, tenantID).Delete(&admin.UserStorePermission{}).Error; err != nil {
			return err
		}
		for i := range grants {
			grants[i].ID = uuid.Nil
			grants[i].UserID = m.UserID
			grants[i].CreatedBy = &actor
			if grants[i].StoreID == uuid.Nil {
				continue
			}
			if err := tx.Create(&grants[i]).Error; err != nil {
				return err
			}
		}
		if err := revokeUserSecurityTx(tx, m.UserID, "tenant_membership_changed"); err != nil {
			return err
		}
		return s.Audit.WriteTxContext(ctx, tx, operationlog.WriteOpts{TenantID: tenantID, AdminUserID: &actor, Action: "tenant.membership.update", Resource: "tenant_membership", ResourceID: m.ID.String(), Status: "success"})
	})
}

func (s *Service) Leave(ctx context.Context, userID uuid.UUID, tenantID int64) error {
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockTenantMutation(tx, tenantID); err != nil {
			return err
		}
		var m admin.TenantMembership
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND user_id = ? AND status = ?", tenantID, userID, admin.MembershipActive).First(&m).Error; err != nil {
			return err
		}
		if isTenantAdminRole(m.Role) {
			var count int64
			if err := tx.Model(&admin.TenantMembership{}).Where("tenant_id = ? AND status = ? AND role = ? AND id <> ?", tenantID, admin.MembershipActive, adminperm.RoleAdmin, m.ID).Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				return errors.New("cannot leave as last tenant administrator")
			}
		}
		if err := tx.Model(&m).Update("status", admin.MembershipRemoved).Error; err != nil {
			return err
		}
		if err := revokeUserSecurityTx(tx, userID, "tenant_membership_left"); err != nil {
			return err
		}
		return s.Audit.WriteTxContext(ctx, tx, operationlog.WriteOpts{
			TenantID: tenantID, AdminUserID: &userID, Action: "tenant.membership.leave",
			Resource: "tenant_membership", ResourceID: m.ID.String(), Status: "success",
		})
	})
}

func (s *Service) CreateInvitation(ctx context.Context, actor uuid.UUID, tenantID int64, in InvitationInput) (*admin.TenantInvitation, string, error) {
	if _, err := s.requireTenantPermission(ctx, tenantID, actor, adminperm.PermUserManage); err != nil {
		return nil, "", err
	}
	email, _, ok := admin.ParseLoginAccount(in.Email)
	if !ok || email == "" {
		return nil, "", errors.New("invalid invitation email")
	}
	role, err := normalizeTenantRole(in.Role)
	if err != nil {
		return nil, "", err
	}
	if in.ExpiresInHours <= 0 || in.ExpiresInHours > 720 {
		in.ExpiresInHours = 168
	}
	if err := validateTenantStoreGrants(s.DB.WithContext(ctx), tenantID, in.StorePermissions); err != nil {
		return nil, "", err
	}
	if adminperm.IsCustomRole(role) {
		var definition admin.RoleDefinition
		if err := s.DB.WithContext(ctx).Where("tenant_id = ? AND code = ? AND enabled = ?", tenantID, role, true).First(&definition).Error; err != nil {
			return nil, "", errors.New("tenant role unavailable")
		}
	}
	token, err := authutil.NewOpaqueToken(32)
	if err != nil {
		return nil, "", err
	}
	grantsJSON, err := json.Marshal(in.StorePermissions)
	if err != nil {
		return nil, "", err
	}
	row := &admin.TenantInvitation{TenantID: tenantID, Email: email, Role: role, StoreGrantsJSON: string(grantsJSON), TokenHash: invitationHash(token), Status: admin.InvitationPending, ExpiresAt: time.Now().UTC().Add(time.Duration(in.ExpiresInHours) * time.Hour), CreatedBy: actor}
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockTenantMutation(tx, tenantID); err != nil {
			return err
		}
		if err := requireTenantPermissionTx(ctx, tx, tenantID, actor, adminperm.PermUserManage); err != nil {
			return err
		}
		var existing admin.AdminUser
		if err := tx.Where("LOWER(TRIM(email)) = ?", email).First(&existing).Error; err == nil {
			var member admin.TenantMembership
			memberErr := tx.Where("tenant_id = ? AND user_id = ? AND status <> ?", tenantID, existing.ID, admin.MembershipRemoved).First(&member).Error
			if memberErr == nil {
				return errors.New("user already belongs to tenant")
			}
			if !errors.Is(memberErr, gorm.ErrRecordNotFound) {
				return memberErr
			}
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND LOWER(email) = ? AND status = ?", tenantID, email, admin.InvitationPending).First(&admin.TenantInvitation{}).Error; err == nil {
			return errors.New("invitation already pending")
		}
		if err := tx.Create(row).Error; err != nil {
			return err
		}
		return s.Audit.WriteTxContext(ctx, tx, operationlog.WriteOpts{TenantID: tenantID, AdminUserID: &actor, Action: "tenant.invitation.create", Resource: "tenant_invitation", ResourceID: row.ID.String(), Status: "success"})
	})
	if err != nil {
		return nil, "", err
	}
	return row, token, nil
}

func (s *Service) Invitations(ctx context.Context, tenantID int64) ([]admin.TenantInvitation, error) {
	if tenantID <= 0 || s == nil || s.DB == nil {
		return nil, errors.New("tenant context required")
	}
	var rows []admin.TenantInvitation
	err := s.DB.WithContext(ctx).Where("tenant_id = ?", tenantID).Order("created_at DESC").Find(&rows).Error
	return rows, err
}

func (s *Service) CancelInvitation(ctx context.Context, actor uuid.UUID, tenantID int64, invitationID uuid.UUID) error {
	if _, err := s.requireTenantPermission(ctx, tenantID, actor, adminperm.PermUserManage); err != nil {
		return err
	}
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockTenantMutation(tx, tenantID); err != nil {
			return err
		}
		if err := requireTenantPermissionTx(ctx, tx, tenantID, actor, adminperm.PermUserManage); err != nil {
			return err
		}
		var row admin.TenantInvitation
		if err := tx.Where("id = ? AND tenant_id = ? AND status = ?", invitationID, tenantID, admin.InvitationPending).First(&row).Error; err != nil {
			return err
		}
		if err := tx.Model(&row).Update("status", admin.InvitationCancelled).Error; err != nil {
			return err
		}
		return s.Audit.WriteTxContext(ctx, tx, operationlog.WriteOpts{TenantID: tenantID, AdminUserID: &actor, Action: "tenant.invitation.cancel", Resource: "tenant_invitation", ResourceID: row.ID.String(), Status: "success"})
	})
}

func (s *Service) AcceptInvitation(ctx context.Context, userID uuid.UUID, invitationToken string, userEmail string) (int64, error) {
	if userID == uuid.Nil {
		return 0, errors.New("authentication required")
	}
	if strings.TrimSpace(invitationToken) == "" {
		return 0, errors.New("invalid invitation")
	}
	hash := invitationHash(invitationToken)
	var tenantID int64
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Discover the immutable tenant binding before locking. Every invitation
		// mutation takes the tenant lock before the invitation row lock.
		var binding admin.TenantInvitation
		if err := tx.Select("id", "tenant_id").Where("token_hash = ?", hash).First(&binding).Error; err != nil {
			return errors.New("invalid invitation")
		}
		if err := lockTenantMutation(tx, binding.TenantID); err != nil {
			return err
		}
		var inv admin.TenantInvitation
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND token_hash = ?", binding.ID, hash).First(&inv).Error; err != nil {
			return errors.New("invalid invitation")
		}
		if inv.Status != admin.InvitationPending || time.Now().UTC().After(inv.ExpiresAt) {
			return errors.New("invitation expired or used")
		}
		if !strings.EqualFold(strings.TrimSpace(inv.Email), strings.TrimSpace(userEmail)) {
			return errors.New("invitation account mismatch")
		}
		var tenant admin.Tenant
		if err := tx.First(&tenant, "id = ?", inv.TenantID).Error; err != nil || tenant.Status != admin.TenantActive {
			return errors.New("tenant unavailable")
		}
		if adminperm.IsCustomRole(inv.Role) {
			var definition admin.RoleDefinition
			if err := tx.Where("tenant_id = ? AND code = ? AND enabled = ?", inv.TenantID, inv.Role, true).First(&definition).Error; err != nil {
				return errors.New("tenant role unavailable")
			}
		}
		var m admin.TenantMembership
		if err := tx.Where("tenant_id = ? AND user_id = ?", inv.TenantID, userID).First(&m).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			m = admin.TenantMembership{TenantID: inv.TenantID, UserID: userID, Role: inv.Role, Status: admin.MembershipActive}
			if err := tx.Create(&m).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		} else if m.Status != admin.MembershipActive {
			if err := tx.Model(&m).Updates(map[string]any{"role": inv.Role, "status": admin.MembershipActive}).Error; err != nil {
				return err
			}
		} else {
			return errors.New("already a tenant member")
		}
		var storeGrants []admin.UserStorePermission
		if err := json.Unmarshal([]byte(inv.StoreGrantsJSON), &storeGrants); err != nil {
			return errors.New("invalid invitation store scope")
		}
		if err := validateTenantStoreGrants(tx, inv.TenantID, storeGrants); err != nil {
			return errors.New("invitation store scope unavailable")
		}
		if tx.Migrator().HasTable(&admin.UserStorePermission{}) && tx.Migrator().HasTable("shops") {
			if err := tx.Where("user_id = ? AND store_id IN (SELECT id FROM shops WHERE tenant_id = ?)", userID, inv.TenantID).
				Delete(&admin.UserStorePermission{}).Error; err != nil {
				return err
			}
		}
		for i := range storeGrants {
			storeGrants[i].ID = uuid.Nil
			storeGrants[i].UserID = userID
			if err := tx.Create(&storeGrants[i]).Error; err != nil {
				return err
			}
		}
		now := time.Now().UTC()
		if err := tx.Model(&inv).Updates(map[string]any{"status": admin.InvitationAccepted, "accepted_by": userID, "accepted_at": now}).Error; err != nil {
			return err
		}
		tenantID = inv.TenantID
		if err := revokeUserSecurityTx(tx, userID, "tenant_invitation_accepted"); err != nil {
			return err
		}
		return s.Audit.WriteTxContext(ctx, tx, operationlog.WriteOpts{TenantID: tenantID, AdminUserID: &userID, Action: "tenant.invitation.accept", Resource: "tenant_invitation", ResourceID: inv.ID.String(), Status: "success"})
	})
	return tenantID, err
}

func requireTenantAdminTx(ctx context.Context, tx *gorm.DB, tenantID int64, userID uuid.UUID) error {
	return requireTenantPermissionTx(ctx, tx, tenantID, userID, adminperm.RoleAdmin)
}

func requireTenantPermissionTx(ctx context.Context, tx *gorm.DB, tenantID int64, userID uuid.UUID, permission string) error {
	return adminperm.CheckTenantPermission(ctx, tx, tenantID, userID, permission)
}

func validateTenantStoreGrants(tx *gorm.DB, tenantID int64, grants []admin.UserStorePermission) error {
	seen := make(map[uuid.UUID]struct{}, len(grants))
	for _, grant := range grants {
		if grant.StoreID == uuid.Nil {
			return errors.New("store id required")
		}
		scope := strings.TrimSpace(strings.ToLower(grant.PermissionScope))
		if scope != admin.StorePermScopeView && scope != admin.StorePermScopeOperate && scope != admin.StorePermScopeManage {
			return errors.New("invalid store permission scope")
		}
		if _, exists := seen[grant.StoreID]; exists {
			return errors.New("duplicate store permission")
		}
		seen[grant.StoreID] = struct{}{}
		var count int64
		if err := tx.Table("shops").Where("id = ? AND tenant_id = ?", grant.StoreID, tenantID).Count(&count).Error; err != nil {
			return err
		}
		if count != 1 {
			return errors.New("store does not belong to tenant")
		}
	}
	return nil
}
