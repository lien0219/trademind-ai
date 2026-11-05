package adminuser

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (s *Service) tenantMembershipMode() bool {
	if s == nil || s.DB == nil {
		return false
	}
	registry, memberships := adminperm.TenantSchemaState(s.DB)
	return registry || memberships
}

func (s *Service) requireTenantMembershipSchema() error {
	registry, memberships := adminperm.TenantSchemaState(s.DB)
	if !registry || !memberships {
		return errors.New("adminuser: tenant membership schema incomplete")
	}
	return nil
}

func (s *Service) lockTenantManageTx(c *gin.Context, tx *gorm.DB, tenantID int64) error {
	if err := lockTenantAdminMutations(tx, tenantID); err != nil {
		return err
	}
	if !s.tenantMembershipMode() {
		return nil
	}
	if err := s.requireTenantMembershipSchema(); err != nil {
		return err
	}
	actorID := adminUUID(c)
	if actorID == nil {
		return adminperm.ErrTenantPermissionDenied
	}
	return adminperm.CheckTenantPermission(c.Request.Context(), tx, tenantID, *actorID, adminperm.PermUserManage)
}

type membershipRow struct {
	UserID uuid.UUID
	Role   string
	Status string
}

func membershipUser(ctx context.Context, tx *gorm.DB, userID uuid.UUID, tenantID int64, lock bool) (admin.AdminUser, admin.TenantMembership, error) {
	var member admin.TenantMembership
	query := tx.WithContext(ctx).Where("tenant_id = ? AND user_id = ?", tenantID, userID)
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	if err := query.First(&member).Error; err != nil {
		return admin.AdminUser{}, member, err
	}
	var user admin.AdminUser
	if err := tx.WithContext(ctx).First(&user, "id = ?", userID).Error; err != nil {
		return user, member, err
	}
	return user, member, nil
}

func membershipUserRow(ctx context.Context, svc *Service, user admin.AdminUser, member admin.TenantMembership, tenantID int64, withStores bool) (*UserRow, error) {
	user.TenantID = tenantID
	user.Role = member.Role
	user.Status = member.Status
	return svc.toUserRow(ctx, &user, withStores)
}

func (s *Service) countUserMemberships(ctx context.Context, tx *gorm.DB, userID uuid.UUID) (int64, error) {
	var count int64
	err := tx.WithContext(ctx).Model(&admin.TenantMembership{}).Where("user_id = ?", userID).Count(&count).Error
	return count, err
}

func revokeIdentitySessionsTx(tx *gorm.DB, userID uuid.UUID, reason string) error {
	if tx == nil || !tx.Migrator().HasTable("auth_sessions") {
		return nil
	}
	now := time.Now().UTC()
	if err := tx.Table("auth_sessions").Where("user_id = ? AND status = ?", userID, "active").Updates(map[string]any{
		"status": "revoked", "revoked_at": &now, "revoke_reason": reason,
	}).Error; err != nil {
		return err
	}
	if tx.Migrator().HasTable("auth_refresh_tokens") {
		if err := tx.Table("auth_refresh_tokens").Where("user_id = ? AND status IN ?", userID, []string{"active", "rotated"}).Updates(map[string]any{
			"status": "revoked", "revoked_at": &now, "revoke_reason": reason,
		}).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) ensureTenantMembership(ctx context.Context, tx *gorm.DB, userID uuid.UUID, tenantID int64) (admin.TenantMembership, error) {
	var member admin.TenantMembership
	if err := tx.WithContext(ctx).Where("tenant_id = ? AND user_id = ?", tenantID, userID).First(&member).Error; err != nil {
		return member, err
	}
	return member, nil
}

func activeTenantAdminCount(ctx context.Context, tx *gorm.DB, tenantID int64, exclude uuid.UUID) (int64, error) {
	q := tx.WithContext(ctx).Model(&admin.TenantMembership{}).Where("tenant_id = ? AND status = ? AND LOWER(TRIM(role)) = ?", tenantID, admin.MembershipActive, adminperm.RoleAdmin)
	if exclude != uuid.Nil {
		q = q.Where("user_id <> ?", exclude)
	}
	var count int64
	return count, q.Count(&count).Error
}

func tenantMembershipRoleCount(ctx context.Context, tx *gorm.DB, tenantID int64, role string) (int64, error) {
	var count int64
	err := tx.WithContext(ctx).Model(&admin.TenantMembership{}).
		Where("tenant_id = ? AND LOWER(TRIM(role)) = ? AND status IN ?", tenantID, strings.ToLower(strings.TrimSpace(role)), []string{admin.MembershipActive, admin.MembershipDisabled}).
		Count(&count).Error
	return count, err
}

func tenantMembershipRoleUserIDs(ctx context.Context, tx *gorm.DB, tenantID int64, role string) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	err := tx.WithContext(ctx).Model(&admin.TenantMembership{}).
		Where("tenant_id = ? AND LOWER(TRIM(role)) = ? AND status IN ?", tenantID, strings.ToLower(strings.TrimSpace(role)), []string{admin.MembershipActive, admin.MembershipDisabled}).
		Pluck("user_id", &ids).Error
	return ids, err
}

func validateMembershipTransition(member admin.TenantMembership, body UpdateBody, actorID *uuid.UUID) (string, string, error) {
	role := normalizeRole(member.Role)
	status := normalizeStatus(member.Status)
	if body.Role != nil {
		role = normalizeRole(*body.Role)
		if role == "" {
			return "", "", inputError("无效的角色")
		}
		if actorID != nil && *actorID == member.UserID && normalizeRole(member.Role) == adminperm.RoleAdmin && role != adminperm.RoleAdmin {
			return "", "", ErrSelfRoleDowngrade
		}
	}
	if body.Status != nil {
		value := strings.ToLower(strings.TrimSpace(*body.Status))
		if value != admin.StatusActive && value != admin.StatusDisabled {
			return "", "", inputError("无效的用户状态")
		}
		status = value
		if actorID != nil && *actorID == member.UserID && value == admin.StatusDisabled {
			return "", "", ErrSelfDisable
		}
	}
	return role, status, nil
}

func (s *Service) sharedIdentityChangeAllowed(ctx context.Context, tx *gorm.DB, userID uuid.UUID, tenantID int64, actorID *uuid.UUID) error {
	if actorID != nil && *actorID == userID {
		return nil
	}
	var identity admin.AdminUser
	if err := tx.WithContext(ctx).Select("tenant_id").First(&identity, "id = ?", userID).Error; err != nil {
		return err
	}
	if identity.TenantID != tenantID || tenantID <= 0 {
		return inputError("此身份的全局字段只能由身份所有者修改")
	}
	if tx.Migrator().HasTable(&admin.PlatformOperator{}) {
		var operators int64
		if err := tx.WithContext(ctx).Model(&admin.PlatformOperator{}).Where("user_id = ?", userID).Count(&operators).Error; err != nil {
			return err
		}
		if operators > 0 {
			return inputError("平台运营者的身份只能由身份所有者修改")
		}
	}
	count, err := s.countUserMemberships(ctx, tx, userID)
	if err != nil {
		return err
	}
	if count > 1 && (actorID == nil || *actorID != userID) {
		return inputError("共享身份的全局字段只能由身份所有者修改")
	}
	return nil
}

func isRecordMissing(err error) bool { return errors.Is(err, gorm.ErrRecordNotFound) }

func membershipAuditDetail(beforeRole, beforeStatus, afterRole, afterStatus string) string {
	return fmt.Sprintf("before={role:%s,status:%s};after={role:%s,status:%s}", beforeRole, beforeStatus, afterRole, afterStatus)
}

func (s *Service) listTenantMembers(c *gin.Context, tenantID int64, q ListQuery, page, pageSize int) (*ListResult, error) {
	query := s.DB.WithContext(c.Request.Context()).Model(&admin.TenantMembership{}).Where("tenant_id = ? AND status IN ?", tenantID, []string{admin.MembershipActive, admin.MembershipDisabled})
	if role := strings.TrimSpace(q.Role); role != "" {
		query = query.Where("LOWER(role) = ?", strings.ToLower(role))
	}
	if status := strings.TrimSpace(q.Status); status != "" {
		query = query.Where("LOWER(status) = ?", strings.ToLower(status))
	}
	if keyword := strings.TrimSpace(q.Keyword); keyword != "" {
		pattern := "%" + keyword + "%"
		query = query.Where("user_id IN (SELECT id FROM admin_users WHERE email ILIKE ? OR phone ILIKE ? OR display_name ILIKE ?)", pattern, pattern, pattern)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	var memberships []admin.TenantMembership
	if err := query.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&memberships).Error; err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, 0, len(memberships))
	for _, member := range memberships {
		ids = append(ids, member.UserID)
	}
	users := make(map[uuid.UUID]admin.AdminUser, len(ids))
	if len(ids) > 0 {
		var rows []admin.AdminUser
		if err := s.DB.WithContext(c.Request.Context()).Where("id IN ?", ids).Find(&rows).Error; err != nil {
			return nil, err
		}
		for _, user := range rows {
			users[user.ID] = user
		}
	}
	items := make([]UserRow, 0, len(memberships))
	for _, member := range memberships {
		user, ok := users[member.UserID]
		if !ok {
			continue
		}
		row, err := membershipUserRow(c.Request.Context(), s, user, member, tenantID, true)
		if err != nil {
			return nil, err
		}
		items = append(items, *row)
	}
	return &ListResult{Items: items, Total: total, Page: page, PageSize: pageSize, TotalPages: pagesOf(total, pageSize)}, nil
}

func (s *Service) createTenantMember(c *gin.Context, user *admin.AdminUser, email, phone, roleRaw string, actorID *uuid.UUID) (*UserRow, error) {
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	if tenantID <= 0 {
		return nil, inputError("tenant context required")
	}
	var member admin.TenantMembership
	err = s.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if err := s.lockTenantManageTx(c, tx, tenantID); err != nil {
			return err
		}
		if email != "" {
			if err := lockAccountIdentity(tx, "email:"+email); err != nil {
				return err
			}
		}
		if phone != "" {
			if err := lockAccountIdentity(tx, "phone:"+phone); err != nil {
				return err
			}
		}
		role, err := requireAssignableRole(c, tx, tenantID, roleRaw)
		if err != nil {
			return err
		}
		var existing admin.AdminUser
		lookup := tx
		if email != "" {
			lookup = lookup.Where("LOWER(TRIM(email)) = ?", email)
		} else {
			lookup = lookup.Where("phone = ?", phone)
		}
		err = lookup.First(&existing).Error
		existingIdentity := err == nil
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if existingIdentity {
			// Only authenticated invitation acceptance can attach a shared identity.
			// A tenant administrator must not enrol or revoke a foreign identity.
			return inputError("账号已存在，请通过成员邀请加入企业")
		}
		user.Role = role
		if err := tx.Create(user).Error; err != nil {
			return err
		}
		member = admin.TenantMembership{TenantID: tenantID, UserID: user.ID, Role: role, Status: admin.MembershipActive}
		if err := tx.Create(&member).Error; err != nil {
			return err
		}
		return writeSensitiveAudit(tx, s.OpLog, c, actorID, "user.create", "tenant_membership", member.ID.String(), fmt.Sprintf("tenantId=%d role=%s existingIdentity=%t", tenantID, role, existingIdentity))
	})
	if err != nil {
		return nil, err
	}
	return membershipUserRow(c.Request.Context(), s, *user, member, tenantID, true)
}

func bumpIdentityAndRevokeTx(tx *gorm.DB, userID uuid.UUID, reason string) error {
	if err := tx.Model(&admin.AdminUser{}).Where("id = ?", userID).UpdateColumn("token_version", gorm.Expr("token_version + 1")).Error; err != nil {
		return err
	}
	return revokeIdentitySessionsTx(tx, userID, reason)
}

func (s *Service) updateTenantMember(c *gin.Context, userID uuid.UUID, body UpdateBody, actorID *uuid.UUID) (*UserRow, error) {
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil || tenantID <= 0 {
		return nil, inputError("租户上下文无效")
	}
	var user admin.AdminUser
	var member admin.TenantMembership
	err = s.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if err := s.lockTenantManageTx(c, tx, tenantID); err != nil {
			return err
		}
		var err error
		user, member, err = membershipUser(c.Request.Context(), tx, userID, tenantID, true)
		if err != nil {
			return err
		}
		if member.Status != admin.MembershipActive && member.Status != admin.MembershipDisabled {
			return gorm.ErrRecordNotFound
		}
		if body.Role != nil {
			role, err := requireAssignableRole(c, tx, tenantID, *body.Role)
			if err != nil {
				return err
			}
			body.Role = &role
		}
		beforeRole, beforeStatus := member.Role, member.Status
		role, status, err := validateMembershipTransition(member, body, actorID)
		if err != nil {
			return err
		}
		securityChanged := role != normalizeRole(member.Role) || status != strings.ToLower(strings.TrimSpace(member.Status))
		if originalAdmin(member.Role, member.Status) && !originalAdmin(role, status) {
			count, err := activeTenantAdminCount(c.Request.Context(), tx, tenantID, userID)
			if err != nil {
				return err
			}
			if count == 0 {
				return inputError("必须保留至少一名启用的管理员")
			}
		}
		if body.DisplayName != nil {
			if len([]rune(*body.DisplayName)) > 128 {
				return inputError("显示名超出长度限制")
			}
			if err := s.sharedIdentityChangeAllowed(c.Request.Context(), tx, userID, tenantID, actorID); err != nil {
				return err
			}
			if err := tx.Model(&user).Update("display_name", strings.TrimSpace(*body.DisplayName)).Error; err != nil {
				return err
			}
			user.DisplayName = strings.TrimSpace(*body.DisplayName)
		}
		member.Role = role
		member.Status = status
		if err := tx.Model(&member).Updates(map[string]any{"role": role, "status": status}).Error; err != nil {
			return err
		}
		if securityChanged {
			if err := bumpIdentityAndRevokeTx(tx, userID, "membership_changed"); err != nil {
				return err
			}
		}
		return writeSensitiveAudit(tx, s.OpLog, c, actorID, "user.update", "tenant_membership", member.ID.String(), membershipAuditDetail(beforeRole, beforeStatus, role, status))
	})
	if err != nil {
		return nil, err
	}
	adminperm.InvalidateUserPermissionCache(userID)
	return membershipUserRow(c.Request.Context(), s, user, member, tenantID, true)
}

func originalAdmin(role, status string) bool {
	return normalizeRole(role) == adminperm.RoleAdmin && strings.EqualFold(strings.TrimSpace(status), admin.MembershipActive)
}

func (s *Service) deleteTenantMember(c *gin.Context, userID uuid.UUID, actorID *uuid.UUID) error {
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil || tenantID <= 0 {
		return inputError("租户上下文无效")
	}
	if actorID != nil && *actorID == userID {
		return inputError("不能删除当前登录账号")
	}
	err = s.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if err := s.lockTenantManageTx(c, tx, tenantID); err != nil {
			return err
		}
		user, member, err := membershipUser(c.Request.Context(), tx, userID, tenantID, true)
		if err != nil {
			return err
		}
		if member.Status != admin.MembershipActive && member.Status != admin.MembershipDisabled {
			return gorm.ErrRecordNotFound
		}
		if originalAdmin(member.Role, member.Status) {
			count, err := activeTenantAdminCount(c.Request.Context(), tx, tenantID, userID)
			if err != nil {
				return err
			}
			if count == 0 {
				return inputError("必须保留至少一名启用的管理员")
			}
		}
		if err := tx.Model(&member).Update("status", admin.MembershipRemoved).Error; err != nil {
			return err
		}
		if tx.Migrator().HasTable(&admin.UserStorePermission{}) && tx.Migrator().HasTable("shops") {
			if err := tx.Where("user_id = ? AND store_id IN (SELECT id FROM shops WHERE tenant_id = ?)", user.ID, tenantID).Delete(&admin.UserStorePermission{}).Error; err != nil {
				return err
			}
		}
		if err := bumpIdentityAndRevokeTx(tx, userID, "membership_removed"); err != nil {
			return err
		}
		return writeSensitiveAudit(tx, s.OpLog, c, actorID, "user.delete", "tenant_membership", member.ID.String(), fmt.Sprintf("tenantId=%d membership removed; shared identity retained", tenantID))
	})
	if err == nil {
		adminperm.InvalidateUserPermissionCache(userID)
	}
	return err
}

func (s *Service) setTenantStorePermissions(c *gin.Context, userID uuid.UUID, tenantID int64, body SetStorePermissionsBody, actorID *uuid.UUID) (*UserRow, error) {
	if tenantID <= 0 {
		return nil, inputError("tenant context required")
	}
	var before []StorePerm
	err := s.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if err := s.lockTenantManageTx(c, tx, tenantID); err != nil {
			return err
		}
		user, member, err := membershipUser(c.Request.Context(), tx, userID, tenantID, true)
		if err != nil {
			return err
		}
		if member.Status != admin.MembershipActive && member.Status != admin.MembershipDisabled {
			return gorm.ErrRecordNotFound
		}
		if normalizeRole(member.Role) == adminperm.RoleAdmin {
			return inputError("绠＄悊鍛樻棤闇€鍒嗛厤搴楅摵鏉冮檺")
		}
		before, err = s.loadStorePermsTx(tx, c.Request.Context(), userID, tenantID)
		if err != nil {
			return err
		}
		if err := tx.Where("user_id = ? AND store_id IN (SELECT id FROM shops WHERE tenant_id = ?)", userID, tenantID).
			Delete(&admin.UserStorePermission{}).Error; err != nil {
			return err
		}
		seen := make(map[uuid.UUID]struct{}, len(body.Items))
		for _, item := range body.Items {
			storeID, err := uuid.Parse(strings.TrimSpace(item.StoreID))
			if err != nil || storeID == uuid.Nil {
				return inputError("鏃犳晥鐨勫簵閾?ID")
			}
			if _, exists := seen[storeID]; exists {
				return inputError("搴楅摵鏉冮檺涓嶈兘閲嶅")
			}
			seen[storeID] = struct{}{}
			scope := strings.ToLower(strings.TrimSpace(item.PermissionScope))
			if scope != admin.StorePermScopeView && scope != admin.StorePermScopeOperate && scope != admin.StorePermScopeManage {
				return inputError("搴楅摵鏉冮檺鑼冨洿鏃犳晥")
			}
			var store shop.Shop
			if err := tx.First(&store, "id = ? AND tenant_id = ?", storeID, tenantID).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return inputError("shop not found")
				}
				return err
			}
			if item.Platform != "" && !strings.EqualFold(strings.TrimSpace(item.Platform), store.Platform) {
				return inputError("shop platform mismatch")
			}
			grant := admin.UserStorePermission{
				UserID: user.ID, StoreID: storeID, Platform: strings.TrimSpace(store.Platform),
				PermissionScope: scope, CreatedBy: actorID,
			}
			if err := tx.Create(&grant).Error; err != nil {
				return err
			}
		}
		if err := bumpIdentityAndRevokeTx(tx, userID, "store_permissions_changed"); err != nil {
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

func orderedTenantMemberships(rows []admin.TenantMembership) []admin.TenantMembership {
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].CreatedAt.After(rows[j].CreatedAt) })
	return rows
}
