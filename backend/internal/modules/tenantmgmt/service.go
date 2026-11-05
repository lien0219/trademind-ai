// Package tenantmgmt owns tenant lifecycle writes. HTTP authentication must
// authenticate platform operators separately from tenant-local RBAC.
package tenantmgmt

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/auth"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/authutil"
	"github.com/trademind-ai/trademind/backend/internal/pkg/passwordpolicy"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var codePattern = regexp.MustCompile(`^[a-z][a-z0-9-]{2,47}$`)

var ErrTenantVersionConflict = errors.New("tenant version conflict")

type Service struct {
	DB    *gorm.DB
	Audit *operationlog.Service
}

type CreateTenantInput struct {
	Code             string
	Name             string
	Description      string
	LegalName        string
	ContactEmail     string
	ContactPhone     string
	SettingsJSON     string
	OwnerEmail       string
	OwnerPassword    string
	OwnerDisplayName string
	IdempotencyKey   string
}

func (s *Service) requireOperator(ctx context.Context, actor uuid.UUID) error {
	if s == nil || s.DB == nil || actor == uuid.Nil {
		return errors.New("platform operator authorization unavailable")
	}
	var row admin.PlatformOperator
	if err := s.DB.WithContext(ctx).Where("user_id = ? AND enabled = ?", actor, true).First(&row).Error; err != nil {
		return errors.New("platform operator required")
	}
	return nil
}

// Create provisions metadata atomically, with auditable actor attribution.
// Creating a tenant does not imply access to its business data or grant
// the caller membership in that tenant.
func (s *Service) Create(ctx context.Context, actor uuid.UUID, input CreateTenantInput) (*admin.Tenant, error) {
	if err := s.requireOperator(ctx, actor); err != nil {
		return nil, err
	}
	input.Code = strings.ToLower(strings.TrimSpace(input.Code))
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	input.LegalName = strings.TrimSpace(input.LegalName)
	input.ContactEmail = strings.TrimSpace(strings.ToLower(input.ContactEmail))
	input.ContactPhone = strings.TrimSpace(input.ContactPhone)
	input.OwnerDisplayName = strings.TrimSpace(input.OwnerDisplayName)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if !codePattern.MatchString(input.Code) || input.Name == "" || len([]rune(input.Name)) > 128 || len([]rune(input.Description)) > 512 || len([]rune(input.LegalName)) > 256 || len([]rune(input.OwnerDisplayName)) > 128 {
		return nil, errors.New("invalid tenant metadata")
	}
	email, _, ownerOK := admin.ParseLoginAccount(input.OwnerEmail)
	if !ownerOK || email == "" {
		return nil, errors.New("tenant owner email is required")
	}
	if input.IdempotencyKey != "" && (len(input.IdempotencyKey) < 16 || len(input.IdempotencyKey) > 64 || !regexp.MustCompile(`^[A-Za-z0-9_-]+$`).MatchString(input.IdempotencyKey)) {
		return nil, errors.New("invalid idempotency key")
	}
	if s.Audit == nil {
		return nil, errors.New("tenant audit unavailable")
	}
	settingsJSON, err := normalizeSettingsJSON(input.SettingsJSON)
	if err != nil {
		return nil, err
	}
	input.SettingsJSON = settingsJSON
	var provisionKey *string
	if input.IdempotencyKey != "" {
		provisionKey = &input.IdempotencyKey
	}
	requestHashBytes, err := json.Marshal(struct{ Code, Name, Description, LegalName, ContactEmail, ContactPhone, OwnerEmail, OwnerDisplayName, Settings string }{input.Code, input.Name, input.Description, input.LegalName, input.ContactEmail, input.ContactPhone, email, input.OwnerDisplayName, input.SettingsJSON})
	if err != nil {
		return nil, fmt.Errorf("hash tenant request: %w", err)
	}
	requestHashSum := sha256.Sum256(requestHashBytes)
	requestHash := hex.EncodeToString(requestHashSum[:])
	if provisionKey != nil {
		var existing admin.Tenant
		if err := s.DB.WithContext(ctx).Where("provision_key = ?", *provisionKey).First(&existing).Error; err == nil {
			if existing.ProvisionHash != requestHash {
				return nil, errors.New("idempotency key reused with different request")
			}
			return &existing, nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}
	owner := &admin.AdminUser{}
	ownerErr := s.DB.WithContext(ctx).Where("LOWER(TRIM(email)) = ?", email).First(owner).Error
	if ownerErr != nil && !errors.Is(ownerErr, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("resolve tenant owner: %w", ownerErr)
	}
	if errors.Is(ownerErr, gorm.ErrRecordNotFound) {
		if err := passwordpolicy.Validate(input.OwnerPassword); err != nil {
			return nil, err
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(input.OwnerPassword), bcrypt.DefaultCost)
		if err != nil {
			return nil, fmt.Errorf("hash tenant owner password: %w", err)
		}
		owner = &admin.AdminUser{Username: admin.NewInternalUsername(), Email: email, PasswordHash: string(hash), DisplayName: input.OwnerDisplayName, Role: adminperm.RoleAdmin, Status: admin.StatusActive, TokenVersion: 1, MustChangePassword: true}
	} else if !strings.EqualFold(strings.TrimSpace(owner.Status), admin.StatusActive) {
		return nil, errors.New("tenant owner account is inactive")
	}
	ownerInitiallyExisted := owner.ID != uuid.Nil
	row := &admin.Tenant{
		Code: input.Code, Name: input.Name, Description: input.Description, LegalName: input.LegalName,
		ContactEmail: input.ContactEmail, ContactPhone: input.ContactPhone, SettingsJSON: input.SettingsJSON,
		Status: admin.TenantPending, CreatedBy: &actor, Version: 1, ProvisionKey: provisionKey, ProvisionHash: requestHash,
	}
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.requireOperatorTx(ctx, tx, actor); err != nil {
			return err
		}
		if err := lockGlobalAccountIdentity(tx); err != nil {
			return err
		}
		var persistedOwner admin.AdminUser
		lookupErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("LOWER(TRIM(email)) = ?", email).First(&persistedOwner).Error
		switch {
		case lookupErr == nil:
			owner = &persistedOwner
			ownerInitiallyExisted = true
		case errors.Is(lookupErr, gorm.ErrRecordNotFound) && ownerInitiallyExisted:
			return errors.New("tenant owner identity changed during provisioning")
		case errors.Is(lookupErr, gorm.ErrRecordNotFound):
			ownerInitiallyExisted = false
		default:
			return fmt.Errorf("resolve tenant owner in transaction: %w", lookupErr)
		}
		if owner.ID != uuid.Nil && !strings.EqualFold(strings.TrimSpace(owner.Status), admin.StatusActive) {
			return errors.New("tenant owner account is inactive")
		}
		if err := tx.Create(row).Error; err != nil {
			return fmt.Errorf("create tenant: %w", err)
		}
		if owner.ID == uuid.Nil {
			owner.TenantID = row.ID
			if err := tx.Create(owner).Error; err != nil {
				return fmt.Errorf("create tenant owner: %w", err)
			}
		}
		if err := tx.Model(owner).Update("last_tenant_id", row.ID).Error; err != nil {
			return fmt.Errorf("set tenant owner preference: %w", err)
		}
		if ownerInitiallyExisted {
			if err := revokeUserSecurityTx(tx, owner.ID, "tenant_membership_added"); err != nil {
				return fmt.Errorf("revoke tenant owner credentials: %w", err)
			}
		}
		membership := &admin.TenantMembership{TenantID: row.ID, UserID: owner.ID, Role: adminperm.RoleAdmin, Status: admin.MembershipActive}
		if err := tx.Create(membership).Error; err != nil {
			return fmt.Errorf("create tenant owner membership: %w", err)
		}
		now := time.Now().UTC()
		row.OwnerUserID = &owner.ID
		row.Status = admin.TenantActive
		row.ActivatedAt = &now
		if err := tx.Model(row).Updates(map[string]any{"owner_user_id": owner.ID, "status": row.Status, "activated_at": now}).Error; err != nil {
			return fmt.Errorf("activate tenant: %w", err)
		}
		return s.Audit.WriteTxContext(ctx, tx, operationlog.WriteOpts{
			TenantID: row.ID, AdminUserID: &actor,
			Action: "tenant.create", Resource: "tenant",
			ResourceID: fmt.Sprintf("%d", row.ID), Status: "success",
			Message: "tenant registered and initial tenant administrator membership provisioned",
		})
	})
	if err != nil {
		if provisionKey != nil {
			var existing admin.Tenant
			if lookupErr := s.DB.WithContext(ctx).Where("provision_key = ?", *provisionKey).First(&existing).Error; lookupErr == nil {
				if existing.ProvisionHash == requestHash {
					return &existing, nil
				}
				return nil, errors.New("idempotency key reused with different request")
			}
		}
		return nil, err
	}
	return row, nil
}

func (s *Service) requireOperatorTx(ctx context.Context, tx *gorm.DB, actor uuid.UUID) error {
	var row admin.PlatformOperator
	if err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "SHARE"}).
		Where("user_id = ? AND enabled = ?", actor, true).First(&row).Error; err != nil {
		return errors.New("platform operator required")
	}
	return nil
}

// SetStatus is fail-closed: closed tenants cannot be reopened through this
// operation, and state changes must commit together with an audit record.
func (s *Service) SetStatus(ctx context.Context, actor uuid.UUID, id int64, status string, expectedVersion int64) (*admin.Tenant, error) {
	if err := s.requireOperator(ctx, actor); err != nil {
		return nil, err
	}
	if id <= 0 || expectedVersion <= 0 || !validTenantStatus(status) {
		return nil, errors.New("invalid tenant status")
	}
	if s.Audit == nil {
		return nil, errors.New("tenant audit unavailable")
	}
	var row admin.Tenant
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.requireOperatorTx(ctx, tx, actor); err != nil {
			return err
		}
		if err := adminperm.LockTenantMutation(tx, id); err != nil {
			return fmt.Errorf("lock tenant mutation %d: %w", id, err)
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, "id = ?", id).Error; err != nil {
			return err
		}
		if row.Code == admin.RootWorkspaceCode {
			return errors.New("Root workspace cannot be suspended")
		}
		if row.Version != expectedVersion {
			return ErrTenantVersionConflict
		}
		if !validTenantTransition(row.Status, status) {
			return errors.New("invalid tenant status transition")
		}
		before := row.Status
		updates := map[string]any{"status": status, "version": row.Version + 1}
		now := time.Now().UTC()
		if status == admin.TenantSuspended {
			updates["suspended_at"] = now
		}
		if status == admin.TenantArchived {
			updates["archived_at"] = now
		}
		if status == admin.TenantActive && row.Status == admin.TenantSuspended {
			updates["suspended_at"] = nil
		}
		result := tx.Model(&row).Where("version = ?", expectedVersion).Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrTenantVersionConflict
		}
		row.Status = status
		row.Version = expectedVersion + 1
		if status == admin.TenantSuspended {
			row.SuspendedAt = &now
		}
		if status == admin.TenantArchived {
			row.ArchivedAt = &now
		}
		if status == admin.TenantActive && before == admin.TenantSuspended {
			row.SuspendedAt = nil
		}
		if err := s.Audit.WriteTxContext(ctx, tx, operationlog.WriteOpts{
			TenantID: id, AdminUserID: &actor, Action: "tenant.status.update",
			Resource: "tenant", ResourceID: fmt.Sprintf("%d", id),
			Status: "success", Message: fmt.Sprintf("before=%s after=%s", before, status),
		}); err != nil {
			return err
		}
		if (status == admin.TenantSuspended || status == admin.TenantArchived || status == admin.TenantClosed) && before == admin.TenantActive {
			var memberIDs []uuid.UUID
			if err := tx.Model(&admin.TenantMembership{}).Where("tenant_id = ? AND status = ?", id, admin.MembershipActive).Pluck("user_id", &memberIDs).Error; err != nil {
				return err
			}
			if len(memberIDs) > 0 {
				now := time.Now().UTC()
				if err := tx.Model(&auth.AuthSession{}).Where("user_id IN ? AND tenant_id = ? AND status = ?", memberIDs, id, auth.SessionStatusActive).Updates(map[string]any{"status": auth.SessionStatusRevoked, "revoked_at": &now, "revoke_reason": "tenant_suspended"}).Error; err != nil {
					return err
				}
				if err := tx.Model(&auth.AuthRefreshToken{}).Where("user_id IN ? AND tenant_id = ? AND status IN ?", memberIDs, id, []string{auth.RefreshStatusActive, auth.RefreshStatusRotated}).Updates(map[string]any{"status": auth.RefreshStatusRevoked, "revoked_at": &now, "revoke_reason": "tenant_suspended"}).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func validTenantStatus(status string) bool {
	switch strings.TrimSpace(strings.ToLower(status)) {
	case admin.TenantPending, admin.TenantActive, admin.TenantSuspended, admin.TenantArchived, admin.TenantClosed:
		return true
	default:
		return false
	}
}

func validTenantTransition(before, after string) bool {
	if before == after {
		return true
	}
	if before == admin.TenantClosed || before == admin.TenantArchived {
		return false
	}
	switch before {
	case admin.TenantPending:
		return after == admin.TenantActive || after == admin.TenantArchived
	case admin.TenantActive:
		return after == admin.TenantSuspended || after == admin.TenantArchived || after == admin.TenantClosed
	case admin.TenantSuspended:
		return after == admin.TenantActive || after == admin.TenantArchived || after == admin.TenantClosed
	default:
		return false
	}
}

// revokeUserSecurityTx invalidates every active credential after membership
// or store scope changes. TokenVersion remains the fast read-side guard.
func revokeUserSecurityTx(tx *gorm.DB, userID uuid.UUID, reason string) error {
	if userID == uuid.Nil {
		return errors.New("user id required")
	}
	if err := tx.Model(&admin.AdminUser{}).Where("id = ?", userID).UpdateColumn("token_version", gorm.Expr("token_version + 1")).Error; err != nil {
		return err
	}
	now := time.Now().UTC()
	if err := tx.Model(&auth.AuthSession{}).Where("user_id = ? AND status = ?", userID, auth.SessionStatusActive).Updates(map[string]any{"status": auth.SessionStatusRevoked, "revoked_at": &now, "revoke_reason": reason}).Error; err != nil {
		return err
	}
	return tx.Model(&auth.AuthRefreshToken{}).Where("user_id = ? AND status IN ?", userID, []string{auth.RefreshStatusActive, auth.RefreshStatusRotated}).Updates(map[string]any{"status": auth.RefreshStatusRevoked, "revoked_at": &now, "revoke_reason": reason}).Error
}

func invitationHash(token string) string {
	return authutil.HashToken(token, "trademind-tenant-invitation")
}
