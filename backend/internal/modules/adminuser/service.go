package adminuser

import (
	"context"
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
)

var (
	ErrSelfDisable       = errors.New("不能禁用当前登录账号")
	ErrSelfRoleDowngrade = errors.New("不能将当前账号降级为非管理员")
	ErrDuplicateAccount  = errors.New("邮箱或手机号已被使用")
)

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
	case adminperm.RoleAdmin, adminperm.RoleOperator, adminperm.RoleReadonly:
		return r
	default:
		return adminperm.RoleOperator
	}
}

func normalizeStatus(status string) string {
	s := strings.TrimSpace(strings.ToLower(status))
	if s == "disabled" || s == "inactive" {
		return "disabled"
	}
	return "active"
}

func (s *Service) loadStorePerms(ctx context.Context, userID uuid.UUID) ([]StorePerm, error) {
	var rows []admin.UserStorePermission
	if err := s.DB.WithContext(ctx).Where("user_id = ?", userID).Order("created_at ASC").Find(&rows).Error; err != nil {
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
		if err := s.DB.WithContext(ctx).Select("shop_name").First(&sh, "id = ?", r.StoreID).Error; err == nil {
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
		perms, err := s.loadStorePerms(ctx, u.ID)
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
	tx := s.DB.WithContext(c.Request.Context()).Model(&admin.AdminUser{})
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
	var u admin.AdminUser
	if err := s.DB.WithContext(c.Request.Context()).First(&u, "id = ?", userID).Error; err != nil {
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
		if strings.TrimSpace(body.Email) != "" {
			em = strings.ToLower(strings.TrimSpace(body.Email))
			ok = em != ""
		}
	}
	if !ok {
		return nil, fmt.Errorf("邮箱或手机号必填")
	}
	pw := strings.TrimSpace(body.Password)
	if len(pw) < 6 {
		return nil, fmt.Errorf("密码至少 6 位")
	}
	if em != "" {
		var cnt int64
		if err := s.DB.WithContext(c.Request.Context()).Model(&admin.AdminUser{}).
			Where("LOWER(TRIM(email)) = ?", em).Count(&cnt).Error; err != nil {
			return nil, err
		}
		if cnt > 0 {
			return nil, ErrDuplicateAccount
		}
	}
	if ph != "" {
		var cnt int64
		if err := s.DB.WithContext(c.Request.Context()).Model(&admin.AdminUser{}).
			Where("phone = ?", ph).Count(&cnt).Error; err != nil {
			return nil, err
		}
		if cnt > 0 {
			return nil, ErrDuplicateAccount
		}
	}
	hash, err := hashPassword(pw)
	if err != nil {
		return nil, err
	}
	u := &admin.AdminUser{
		Username:     admin.NewInternalUsername(),
		Email:        em,
		Phone:        ph,
		PasswordHash: hash,
		DisplayName:  strings.TrimSpace(body.DisplayName),
		Role:         normalizeRole(body.Role),
		Status:       "active",
	}
	if err := s.DB.WithContext(c.Request.Context()).Create(u).Error; err != nil {
		return nil, err
	}
	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: actorID,
			Action:      "user.create",
			Resource:    "admin_user",
			ResourceID:  u.ID.String(),
			Status:      "success",
			Message:     fmt.Sprintf("userId=%s role=%s email=%s", u.ID.String(), u.Role, em),
		})
	}
	return s.toUserRow(c.Request.Context(), u, true)
}

// Update patches role/status/displayName.
func (s *Service) Update(c *gin.Context, userID uuid.UUID, body UpdateBody, actorID *uuid.UUID) (*UserRow, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("adminuser: no db")
	}
	var u admin.AdminUser
	if err := s.DB.WithContext(c.Request.Context()).First(&u, "id = ?", userID).Error; err != nil {
		return nil, err
	}
	oldRole := normalizeRole(u.Role)
	if body.Role != nil {
		newRole := normalizeRole(*body.Role)
		if actorID != nil && *actorID == userID && newRole != adminperm.RoleAdmin && oldRole == adminperm.RoleAdmin {
			return nil, ErrSelfRoleDowngrade
		}
		u.Role = newRole
	}
	if body.Status != nil {
		st := normalizeStatus(*body.Status)
		if actorID != nil && *actorID == userID && st == "disabled" {
			return nil, ErrSelfDisable
		}
		u.Status = st
	}
	if body.DisplayName != nil {
		u.DisplayName = strings.TrimSpace(*body.DisplayName)
	}
	if err := s.DB.WithContext(c.Request.Context()).Save(&u).Error; err != nil {
		return nil, err
	}
	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: actorID,
			Action:      "user.update",
			Resource:    "admin_user",
			ResourceID:  userID.String(),
			Status:      "success",
			Message:     fmt.Sprintf("userId=%s role=%s status=%s", userID.String(), u.Role, u.Status),
		})
	}
	return s.toUserRow(c.Request.Context(), &u, true)
}

// SetStorePermissions replaces store grants for a user.
func (s *Service) SetStorePermissions(c *gin.Context, userID uuid.UUID, body SetStorePermissionsBody, actorID *uuid.UUID) (*UserRow, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("adminuser: no db")
	}
	var u admin.AdminUser
	if err := s.DB.WithContext(c.Request.Context()).First(&u, "id = ?", userID).Error; err != nil {
		return nil, err
	}
	if normalizeRole(u.Role) == adminperm.RoleAdmin {
		return nil, fmt.Errorf("管理员无需分配店铺权限")
	}
	err := s.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", userID).Delete(&admin.UserStorePermission{}).Error; err != nil {
			return err
		}
		for _, it := range body.Items {
			sid, err := uuid.Parse(strings.TrimSpace(it.StoreID))
			if err != nil || sid == uuid.Nil {
				return fmt.Errorf("无效的店铺 ID")
			}
			var sh shop.Shop
			if err := tx.First(&sh, "id = ?", sid).Error; err != nil {
				return fmt.Errorf("店铺不存在")
			}
			row := admin.UserStorePermission{
				UserID:          userID,
				StoreID:         sid,
				Platform:        strings.TrimSpace(sh.Platform),
				PermissionScope: admin.NormalizeStorePermScope(it.PermissionScope),
				CreatedBy:       actorID,
			}
			if it.Platform != "" {
				row.Platform = strings.TrimSpace(it.Platform)
			}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: actorID,
			Action:      "user.store_permissions.set",
			Resource:    "admin_user",
			ResourceID:  userID.String(),
			Status:      "success",
			Message:     fmt.Sprintf("userId=%s grantCount=%d", userID.String(), len(body.Items)),
		})
	}
	return s.Get(c, userID)
}
