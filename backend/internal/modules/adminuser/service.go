package adminuser

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrSelfDisable       = errors.New("不能禁用当前登录账号")
	ErrSelfRoleDowngrade = errors.New("不能将当前账号降级为非管理员")
	ErrDuplicateAccount  = errors.New("邮箱或手机号已被使用")
	ErrAuditFailure      = errors.New("security audit persistence failed")
)

// inputError contains only a deliberate, safe client-facing validation message.
type inputError string

func (e inputError) Error() string { return string(e) }

type storePermissionAuditDetails struct {
	Before []StorePerm `json:"before"`
	After  []StorePerm `json:"after"`
}

// Service manages admin users and store permissions.
type Service struct {
	DB    *gorm.DB
	OpLog *operationlog.Service
}

// ListQuery filters admin users.
type ListQuery struct {
	Page     int
	PageSize int
	Role     string
	Status   string
	Keyword  string
}

// ListResult paginated users.
type ListResult struct {
	Items      []UserRow
	Total      int64
	Page       int
	PageSize   int
	TotalPages int
}

// CreateBody creates a new admin user.
type CreateBody struct {
	Email       string `json:"email"`
	Phone       string `json:"phone"`
	Password    string `json:"password"`
	DisplayName string `json:"displayName"`
	Role        string `json:"role"`
}

// UpdateBody patches user fields.
type UpdateBody struct {
	DisplayName *string `json:"displayName"`
	Role        *string `json:"role"`
	Status      *string `json:"status"`
}

// SetStorePermissionsBody replaces store grants.
type SetStorePermissionsBody struct {
	Items []StorePermInput `json:"items"`
}

// StorePermInput is one store grant input.
type StorePermInput struct {
	StoreID         string `json:"storeId"`
	Platform        string `json:"platform"`
	PermissionScope string `json:"permissionScope"`
}

func pagesOf(total int64, ps int) int {
	if ps < 1 {
		ps = 20
	}
	p := int(total) / ps
	if int(total)%ps != 0 {
		p++
	}
	if p == 0 && total > 0 {
		p = 1
	}
	return p
}

func hashPassword(raw string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(raw), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func normalizeRole(role string) string {
	r := strings.TrimSpace(strings.ToLower(role))
	switch r {
	case adminperm.RoleAdmin, adminperm.RoleOperator, adminperm.RoleReadonly, adminperm.RoleReviewer:
		return r
	default:
		if adminperm.IsCustomRole(r) {
			return r
		}
		return ""
	}
}

func normalizeStatus(status string) string {
	s := strings.TrimSpace(strings.ToLower(status))
	if s == admin.StatusActive {
		return admin.StatusActive
	}
	if s == admin.StatusDisabled || s == "inactive" {
		return "disabled"
	}
	return admin.StatusDisabled
}

func lockTenantAdminMutations(tx *gorm.DB, tenantID int64) error {
	return lockAdminMutationScope(tx, fmt.Sprintf("trademind:admin-users:%d", tenantID))
}

func lockAccountIdentity(tx *gorm.DB, identity string) error {
	if strings.TrimSpace(identity) == "" {
		return nil
	}
	// Account identities are unique across tenants, so registration uses one
	// global lock row without persisting the email or phone in the lock table.
	return lockAdminMutationScope(tx, "trademind:admin-account-identities")
}

func lockAdminMutationScope(tx *gorm.DB, scope string) error {
	if tx == nil || tx.Dialector == nil {
		return fmt.Errorf("adminuser: mutation lock unavailable")
	}
	sum := sha256.Sum256([]byte(scope))
	lock := admin.AdminMutationLock{KeyHash: hex.EncodeToString(sum[:])}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&lock).Error; err != nil {
		return err
	}
	return tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("key_hash = ?", lock.KeyHash).
		Take(&lock).Error
}

func writeSensitiveAudit(tx *gorm.DB, logs *operationlog.Service, c *gin.Context, actorID *uuid.UUID, action, resource, resourceID, message string) error {
	if logs == nil {
		return ErrAuditFailure
	}
	if err := logs.WriteTx(c, tx, operationlog.WriteOpts{
		AdminUserID: actorID, Action: action, Resource: resource, ResourceID: resourceID,
		Status: "success", Message: message,
	}); err != nil {
		return fmt.Errorf("%w: %w", ErrAuditFailure, err)
	}
	return nil
}

func (s *Service) loadStorePerms(ctx context.Context, userID uuid.UUID, tenantID int64) ([]StorePerm, error) {
	var rows []admin.UserStorePermission
	if err := s.DB.WithContext(ctx).
		Where("user_id = ? AND store_id IN (SELECT id FROM shops WHERE tenant_id = ?)", userID, tenantID).
		Order("created_at ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]StorePerm, 0, len(rows))
	for _, r := range rows {
		sp := StorePerm{
			ID:              r.ID.String(),
			StoreID:         r.StoreID.String(),
			Platform:        strings.TrimSpace(r.Platform),
			PermissionScope: admin.NormalizeStorePermScope(r.PermissionScope),
		}
		var sh shop.Shop
		if err := s.DB.WithContext(ctx).Select("shop_name").First(&sh, "id = ? AND tenant_id = ?", r.StoreID, tenantID).Error; err == nil {
			sp.StoreName = strings.TrimSpace(sh.ShopName)
		}
		out = append(out, sp)
	}
	return out, nil
}

func (s *Service) lastOperationAt(ctx context.Context, userID uuid.UUID) *string {
	var row operationlog.OperationLog
	if err := s.DB.WithContext(ctx).
		Where("admin_user_id = ?", userID).
		Order("created_at DESC").
		Limit(1).
		First(&row).Error; err != nil {
		return nil
	}
	t := row.CreatedAt.UTC().Format(time.RFC3339)
	return &t
}

func (s *Service) toUserRow(ctx context.Context, u *admin.AdminUser, withStores bool) (*UserRow, error) {
	if u == nil {
		return nil, fmt.Errorf("nil user")
	}
	row := &UserRow{
		ID:          u.ID.String(),
		Username:    u.LoginLabel(),
		Email:       strings.TrimSpace(u.Email),
		Phone:       strings.TrimSpace(u.Phone),
		DisplayName: strings.TrimSpace(u.DisplayName),
		Role:        normalizeRole(u.Role),
		Status:      normalizeStatus(u.Status),
		CreatedAt:   u.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:   u.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if dn := row.DisplayName; dn == "" {
		row.DisplayName = row.Username
	}
	if withStores && normalizeRole(u.Role) != adminperm.RoleAdmin {
		perms, err := s.loadStorePerms(ctx, u.ID, u.TenantID)
		if err != nil {
			return nil, err
		}
		row.StorePermissions = perms
	}
	if t := s.lastOperationAt(ctx, u.ID); t != nil {
		row.LastOperationAt = t
	}
	return row, nil
}

// List returns paginated admin users.
func (s *Service) List(c *gin.Context, q ListQuery) (*ListResult, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("adminuser: no db")
	}
	page := q.Page
	if page < 1 {
		page = 1
	}
	ps := q.PageSize
	if ps < 1 {
		ps = 20
	}
	if ps > 100 {
		ps = 100
	}
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	tx := s.DB.WithContext(c.Request.Context()).Model(&admin.AdminUser{}).Where("tenant_id = ?", tenantID)
	if v := strings.TrimSpace(q.Role); v != "" {
		tx = tx.Where("LOWER(role) = ?", strings.ToLower(v))
	}
	if v := strings.TrimSpace(q.Status); v != "" {
		tx = tx.Where("LOWER(status) = ?", strings.ToLower(v))
	}
	if kw := strings.TrimSpace(q.Keyword); kw != "" {
		pat := "%" + kw + "%"
		tx = tx.Where("(email ILIKE ? OR phone ILIKE ? OR display_name ILIKE ?)", pat, pat, pat)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, err
	}
	offset := (page - 1) * ps
	var rows []admin.AdminUser
	if err := tx.Order("created_at DESC").Offset(offset).Limit(ps).Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]UserRow, 0, len(rows))
	for i := range rows {
		u, err := s.toUserRow(c.Request.Context(), &rows[i], true)
		if err != nil {
			return nil, err
		}
		items = append(items, *u)
	}
	return &ListResult{
		Items:      items,
		Total:      total,
		Page:       page,
		PageSize:   ps,
		TotalPages: pagesOf(total, ps),
	}, nil
}

// Get returns one admin user.
func (s *Service) Get(c *gin.Context, userID uuid.UUID) (*UserRow, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("adminuser: no db")
	}
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	var u admin.AdminUser
	if err := s.DB.WithContext(c.Request.Context()).First(&u, "id = ? AND tenant_id = ?", userID, tenantID).Error; err != nil {
		return nil, err
	}
	return s.toUserRow(c.Request.Context(), &u, true)
}

// Create registers a new admin user.
func (s *Service) Create(c *gin.Context, body CreateBody, actorID *uuid.UUID) (*UserRow, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("adminuser: no db")
	}
	em, ph, ok := admin.ParseLoginAccount(strings.TrimSpace(body.Email))
	if !ok && strings.TrimSpace(body.Phone) != "" {
		em, ph, ok = admin.ParseLoginAccount(strings.TrimSpace(body.Phone))
	}
	if !ok {
		return nil, inputError("请输入有效的邮箱或手机号")
	}
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	pw := body.Password
	if len([]rune(pw)) < 12 || len([]byte(pw)) > 72 {
		return nil, inputError("密码至少 12 位且不超过 72 字节")
	}
	if len([]rune(body.DisplayName)) > 128 {
		return nil, inputError("显示名超出长度限制")
	}
	hash, err := hashPassword(pw)
	if err != nil {
		return nil, err
	}
	u := &admin.AdminUser{
		TenantID:           tenantID,
		Username:           admin.NewInternalUsername(),
		Email:              em,
		Phone:              ph,
		PasswordHash:       hash,
		MustChangePassword: true,
		DisplayName:        strings.TrimSpace(body.DisplayName),
		Status:             "active",
	}
	if err := s.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if err := lockTenantAdminMutations(tx, tenantID); err != nil {
			return err
		}
		if em != "" {
			if err := lockAccountIdentity(tx, "email:"+em); err != nil {
				return err
			}
		}
		if ph != "" {
			if err := lockAccountIdentity(tx, "phone:"+ph); err != nil {
				return err
			}
		}
		if em != "" {
			var count int64
			if err := tx.Model(&admin.AdminUser{}).Where("LOWER(TRIM(email)) = ?", em).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				return ErrDuplicateAccount
			}
		}
		if ph != "" {
			var count int64
			if err := tx.Model(&admin.AdminUser{}).Where("phone = ?", ph).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				return ErrDuplicateAccount
			}
		}
		role, err := requireAssignableRole(c, tx, tenantID, body.Role)
		if err != nil {
			return err
		}
		u.Role = role
		if err := tx.Create(u).Error; err != nil {
			return err
		}
		return writeSensitiveAudit(tx, s.OpLog, c, actorID, "user.create", "admin_user", u.ID.String(), fmt.Sprintf("role=%s mustChangePassword=true", role))
	}); err != nil {
		return nil, err
	}
	return s.toUserRow(c.Request.Context(), u, true)
}

// Update patches role/status/displayName and revokes credentials atomically.
func (s *Service) Update(c *gin.Context, userID uuid.UUID, body UpdateBody, actorID *uuid.UUID) (*UserRow, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("adminuser: no db")
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	var u admin.AdminUser
	err = s.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		// Serialize role/status transitions of tenant administrators. Without a
		// shared lock two concurrent demotions can both observe one other admin.
		if body.Role != nil || body.Status != nil {
			if err := lockTenantAdminMutations(tx, tid); err != nil {
				return err
			}
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&u, "id = ? AND tenant_id = ?", userID, tid).Error; err != nil {
			return err
		}
		originalRole := normalizeRole(u.Role)
		before := map[string]string{"role": u.Role, "status": u.Status}
		if body.Role != nil {
			newRole, err := requireAssignableRole(c, tx, tid, *body.Role)
			if err != nil {
				return err
			}
			if actorID != nil && *actorID == userID && originalRole == adminperm.RoleAdmin && newRole != adminperm.RoleAdmin {
				return ErrSelfRoleDowngrade
			}
			u.Role = newRole
		}
		if body.Status != nil {
			status := strings.ToLower(strings.TrimSpace(*body.Status))
			if status != "active" && status != "disabled" {
				return inputError("无效的用户状态")
			}
			if actorID != nil && *actorID == userID && status == "disabled" {
				return ErrSelfDisable
			}
			u.Status = status
		}
		if body.DisplayName != nil {
			if len([]rune(*body.DisplayName)) > 128 {
				return inputError("显示名超出长度限制")
			}
			u.DisplayName = strings.TrimSpace(*body.DisplayName)
		}
		if originalRole == adminperm.RoleAdmin && (u.Role != adminperm.RoleAdmin || u.Status != admin.StatusActive) {
			var count int64
			if err := tx.Model(&admin.AdminUser{}).
				Where("tenant_id = ? AND role = ? AND status = ? AND id <> ?", tid, adminperm.RoleAdmin, admin.StatusActive, userID).
				Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				return inputError("必须保留至少一名启用的管理员")
			}
		}
		changedSecurity := u.Role != originalRole || body.Status != nil
		updates := map[string]any{
			"role": u.Role, "status": u.Status, "display_name": u.DisplayName,
		}
		if changedSecurity {
			updates["token_version"] = gorm.Expr("token_version + 1")
		}
		if err := tx.Model(&admin.AdminUser{}).
			Where("id = ? AND tenant_id = ?", userID, tid).Updates(updates).Error; err != nil {
			return err
		}
		detail, err := json.Marshal(map[string]any{"before": before, "after": map[string]string{"role": u.Role, "status": u.Status}})
		if err != nil {
			return err
		}
		return writeSensitiveAudit(tx, s.OpLog, c, actorID, "user.update", "admin_user", userID.String(), string(detail))
	})
	if err != nil {
		return nil, err
	}
	adminperm.InvalidateUserPermissionCache(userID)
	return s.Get(c, userID)
}

func (s *Service) SetStorePermissions(c *gin.Context, userID uuid.UUID, body SetStorePermissionsBody, actorID *uuid.UUID) (*UserRow, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("adminuser: no db")
	}
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	var u admin.AdminUser
	if err := s.DB.WithContext(c.Request.Context()).First(&u, "id = ? AND tenant_id = ?", userID, tenantID).Error; err != nil {
		return nil, err
	}
	if normalizeRole(u.Role) == adminperm.RoleAdmin {
		return nil, inputError("管理员无需分配店铺权限")
	}
	var before []StorePerm
	err = s.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&u, "id = ? AND tenant_id = ?", userID, tenantID).Error; err != nil {
			return err
		}
		if normalizeRole(u.Role) == adminperm.RoleAdmin {
			return inputError("管理员无需分配店铺权限")
		}
		before, err = s.loadStorePermsTx(tx, c.Request.Context(), userID, tenantID)
		if err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&admin.UserStorePermission{}).Error; err != nil {
			return err
		}
		seen := make(map[uuid.UUID]struct{}, len(body.Items))
		for _, it := range body.Items {
			sid, err := uuid.Parse(strings.TrimSpace(it.StoreID))
			if err != nil || sid == uuid.Nil {
				return inputError("无效的店铺 ID")
			}
			if _, exists := seen[sid]; exists {
				return inputError("店铺权限不能重复")
			}
			seen[sid] = struct{}{}
			scope := strings.ToLower(strings.TrimSpace(it.PermissionScope))
			if scope != admin.StorePermScopeView && scope != admin.StorePermScopeOperate && scope != admin.StorePermScopeManage {
				return inputError("店铺权限范围无效")
			}
			var sh shop.Shop
			if err := tx.First(&sh, "id = ? AND tenant_id = ?", sid, tenantID).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return inputError("店铺不存在")
				}
				return err
			}
			row := admin.UserStorePermission{
				UserID:          userID,
				StoreID:         sid,
				Platform:        strings.TrimSpace(sh.Platform),
				PermissionScope: scope,
				CreatedBy:       actorID,
			}
			if it.Platform != "" && !strings.EqualFold(strings.TrimSpace(it.Platform), sh.Platform) {
				return inputError("店铺平台不匹配")
			}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
		}
		if err := tx.Model(&admin.AdminUser{}).
			Where("id = ? AND tenant_id = ?", userID, tenantID).
			UpdateColumn("token_version", gorm.Expr("token_version + 1")).Error; err != nil {
			return err
		}
		after, err := s.loadStorePermsTx(tx, c.Request.Context(), userID, tenantID)
		if err != nil {
			return err
		}
		details, err := json.Marshal(storePermissionAuditDetails{Before: before, After: after})
		if err != nil {
			return err
		}
		return writeSensitiveAudit(tx, s.OpLog, c, actorID, "user.store_permissions.set", "admin_user", userID.String(), string(details))
	})
	if err != nil {
		return nil, err
	}
	adminperm.InvalidateUserPermissionCache(userID)
	return s.Get(c, userID)
}

func (s *Service) loadStorePermsTx(tx *gorm.DB, ctx context.Context, userID uuid.UUID, tenantID int64) ([]StorePerm, error) {
	var rows []admin.UserStorePermission
	if err := tx.WithContext(ctx).Where("user_id = ? AND store_id IN (SELECT id FROM shops WHERE tenant_id = ?)", userID, tenantID).Order("created_at ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]StorePerm, 0, len(rows))
	for _, row := range rows {
		var store shop.Shop
		if err := tx.WithContext(ctx).Select("shop_name").First(&store, "id = ? AND tenant_id = ?", row.StoreID, tenantID).Error; err != nil {
			return nil, err
		}
		result = append(result, StorePerm{ID: row.ID.String(), StoreID: row.StoreID.String(), Platform: strings.TrimSpace(row.Platform), PermissionScope: admin.NormalizeStorePermScope(row.PermissionScope), StoreName: strings.TrimSpace(store.ShopName)})
	}
	return result, nil
}

// requireAssignableRole prohibits revoked or cross-tenant role assignments.
func (s *Service) requireAssignableRole(c *gin.Context, tenantID int64, raw string) (string, error) {
	return requireAssignableRole(c, s.DB, tenantID, raw)
}

func requireAssignableRole(c *gin.Context, db *gorm.DB, tenantID int64, raw string) (string, error) {
	role := normalizeRole(raw)
	if role == "" {
		return "", inputError("无效的角色")
	}
	if !adminperm.IsCustomRole(role) {
		return role, nil
	}
	var found admin.RoleDefinition
	if err := db.WithContext(c.Request.Context()).Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND code = ? AND enabled = ?", tenantID, role, true).First(&found).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", inputError("自定义角色不存在或已停用")
		}
		return "", err
	}
	return role, nil
}
