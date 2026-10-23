package collectbrowserprofile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/collectdomain"
)

var (
	ErrProfileNotFound       = errors.New("PROFILE_NOT_FOUND: collect browser profile not found or disabled")
	ErrProfileDomainMismatch = errors.New("PROFILE_DOMAIN_MISMATCH: profile domain does not match url host")
	ErrAuditFailure          = errors.New("browser profile audit persistence failed")
)

// Service manages collect_browser_profiles metadata (login cookies stay on Collector disk).
type Service struct {
	DB        *gorm.DB
	Collector CollectorGateway
	OpLog     *operationlog.Service
	Timeout   time.Duration
}

func (s *Service) clampPage(page, ps int) (int, int) {
	if page < 1 {
		page = 1
	}
	if ps < 1 {
		ps = 20
	}
	if ps > 100 {
		ps = 100
	}
	return page, ps
}

func rowToDTO(row *CollectBrowserProfile) RowDTO {
	return RowDTO{
		ID:              row.ID,
		Name:            row.Name,
		Domain:          row.Domain,
		ProfileKey:      row.ProfileKey,
		Provider:        row.Provider,
		Status:          row.Status,
		LastCheckStatus: row.LastCheckStatus,
		LastCheckURL:    row.LastCheckURL,
		LastCheckAt:     row.LastCheckAt,
		LastErrorCode:   row.LastErrorCode,
		Remark:          row.Remark,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
	}
}

func (s *Service) hostnameFromURL(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", err
	}
	h := strings.ToLower(strings.TrimSpace(u.Hostname()))
	if h == "" {
		return "", fmt.Errorf("missing host")
	}
	return h, nil
}

func accessToLastCheck(access string) string {
	switch strings.ToLower(strings.TrimSpace(access)) {
	case "public":
		return LastCheckLoggedIn
	case "login_required":
		return LastCheckLoginRequired
	case "verify_required":
		return LastCheckVerifyRequired
	case "unknown":
		return LastCheckUnknown
	default:
		return LastCheckFailed
	}
}

func tenantIDFromGin(c *gin.Context) (int64, error) {
	return adminperm.TenantIDFromGin(c)
}

func (s *Service) writeAuditTx(tx *gorm.DB, c *gin.Context, adminID *uuid.UUID, action, resourceID, status, message string) error {
	if s == nil || s.OpLog == nil {
		return ErrAuditFailure
	}
	tenantID, err := tenantIDFromGin(c)
	if err != nil {
		return fmt.Errorf("%w: resolve tenant: %v", ErrAuditFailure, err)
	}
	if err := s.OpLog.WriteTx(c, tx, operationlog.WriteOpts{
		TenantID: tenantID, AdminUserID: adminID, Action: action,
		Resource: "collect_browser_profile", ResourceID: resourceID,
		Status: status, Message: message,
	}); err != nil {
		return fmt.Errorf("%w: %w", ErrAuditFailure, err)
	}
	return nil
}

func (s *Service) writeActionAudit(c *gin.Context, adminID *uuid.UUID, action, resourceID, status, message string) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("collectbrowserprofile: no db")
	}
	return s.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		return s.writeAuditTx(tx, c, adminID, action, resourceID, status, message)
	})
}

func safeCheckURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	u.User = nil
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}

func (s *Service) byID(ctx context.Context, tenantID int64, id uuid.UUID) (*CollectBrowserProfile, error) {
	var row CollectBrowserProfile
	err := s.DB.WithContext(ctx).First(&row, "id = ? AND tenant_id = ?", id, tenantID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProfileNotFound
		}
		return nil, err
	}
	return &row, nil
}

func (s *Service) activeByID(ctx context.Context, tenantID int64, id uuid.UUID) (*CollectBrowserProfile, error) {
	row, err := s.byID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if row.Status != StatusActive {
		return nil, ErrProfileNotFound
	}
	return row, nil
}

func (s *Service) List(ctx context.Context, tenantID int64, q ListQuery) ([]RowDTO, int64, error) {
	if s == nil || s.DB == nil {
		return nil, 0, fmt.Errorf("collectbrowserprofile: no db")
	}
	page, ps := s.clampPage(q.Page, q.PageSize)
	tx := s.DB.WithContext(ctx).Model(&CollectBrowserProfile{})
	tx = tx.Where("tenant_id = ?", tenantID)
	if v := strings.TrimSpace(q.Domain); v != "" {
		tx = tx.Where("LOWER(domain) LIKE ?", "%"+strings.ToLower(v)+"%")
	}
	if v := strings.TrimSpace(q.Provider); v != "" {
		tx = tx.Where("provider = ?", v)
	}
	if v := strings.TrimSpace(q.Status); v != "" {
		tx = tx.Where("status = ?", v)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []CollectBrowserProfile
	if err := tx.Order("updated_at DESC").Offset((page - 1) * ps).Limit(ps).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	out := make([]RowDTO, 0, len(rows))
	for i := range rows {
		out = append(out, rowToDTO(&rows[i]))
	}
	return out, total, nil
}

func (s *Service) Create(c *gin.Context, body CreateBody, adminID *uuid.UUID) (*CreateResultDTO, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("collectbrowserprofile: no db")
	}
	if _, err := tenantIDFromGin(c); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(body.Name)
	domain := collectdomain.NormalizeRuleDomain(body.Domain)
	if name == "" || domain == "" {
		return nil, fmt.Errorf("name and domain are required")
	}
	provider := strings.TrimSpace(body.Provider)
	if provider == "" {
		provider = "custom"
	}
	tenantID, err := tenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	row := CollectBrowserProfile{
		TenantID:   tenantID,
		Name:       name,
		Domain:     domain,
		Provider:   provider,
		Status:     StatusActive,
		Remark:     strings.TrimSpace(body.Remark),
		CreatedBy:  adminID,
		ProfileKey: "", // set below
	}
	row.ID = uuid.New()
	row.ProfileKey = "custom_" + strings.ReplaceAll(row.ID.String(), "-", "")

	if err := s.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		return s.writeAuditTx(tx, c, adminID, "collect.browser_profile.create", row.ID.String(), "success", "browser profile metadata created")
	}); err != nil {
		return nil, err
	}
	dto := rowToDTO(&row)
	return &CreateResultDTO{ProfileID: row.ID, ProfileKey: row.ProfileKey, Row: dto}, nil
}

func (s *Service) Disable(c *gin.Context, id uuid.UUID, adminID *uuid.UUID) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("collectbrowserprofile: no db")
	}
	tenantID, err := tenantIDFromGin(c)
	if err != nil {
		return err
	}
	return s.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var row CollectBrowserProfile
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, "id = ? AND tenant_id = ?", id, tenantID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrProfileNotFound
			}
			return err
		}
		if row.Status == StatusDisabled {
			return nil
		}
		if err := tx.Model(&row).Update("status", StatusDisabled).Error; err != nil {
			return err
		}
		return s.writeAuditTx(tx, c, adminID, "collect.browser_profile.disable", id.String(), "success", "browser profile disabled")
	})
}

func (s *Service) Enable(c *gin.Context, id uuid.UUID, adminID *uuid.UUID) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("collectbrowserprofile: no db")
	}
	tenantID, err := tenantIDFromGin(c)
	if err != nil {
		return err
	}
	return s.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var row CollectBrowserProfile
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, "id = ? AND tenant_id = ?", id, tenantID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrProfileNotFound
			}
			return err
		}
		if row.Status == StatusActive {
			return nil
		}
		if err := tx.Model(&row).Update("status", StatusActive).Error; err != nil {
			return err
		}
		return s.writeAuditTx(tx, c, adminID, "collect.browser_profile.enable", id.String(), "success", "browser profile enabled")
	})
}

func (s *Service) Delete(c *gin.Context, id uuid.UUID, adminID *uuid.UUID) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("collectbrowserprofile: no db")
	}
	tenantID, err := tenantIDFromGin(c)
	if err != nil {
		return err
	}
	return s.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var row CollectBrowserProfile
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, "id = ? AND tenant_id = ?", id, tenantID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrProfileNotFound
			}
			return err
		}
		if err := tx.Delete(&row).Error; err != nil {
			return err
		}
		return s.writeAuditTx(tx, c, adminID, "collect.browser_profile.delete", id.String(), "success", "browser profile deleted")
	})
}

func (s *Service) collectorCtx(parent context.Context) (context.Context, context.CancelFunc) {
	t := s.Timeout
	if t <= 0 {
		t = 90 * time.Second
	}
	return context.WithTimeout(parent, t)
}

func (s *Service) OpenLogin(c *gin.Context, id uuid.UUID, body URLBody, adminID *uuid.UUID) (*OpenLoginResultDTO, error) {
	if s.Collector == nil {
		return nil, fmt.Errorf("collector unavailable")
	}
	tenantID, err := tenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	row, err := s.activeByID(c.Request.Context(), tenantID, id)
	if err != nil {
		return nil, err
	}
	rawURL := strings.TrimSpace(body.URL)
	if rawURL == "" {
		return nil, fmt.Errorf("url is required")
	}
	host, err := s.hostnameFromURL(rawURL)
	if err != nil || !collectdomain.DomainMatches(host, row.Domain) {
		return nil, ErrProfileDomainMismatch
	}
	if err := s.writeActionAudit(c, adminID, "collect.browser_profile.open_login", id.String(), "started", "manual login window requested"); err != nil {
		return nil, err
	}
	ctx, cancel := s.collectorCtx(c.Request.Context())
	defer cancel()
	msg, err := s.Collector.OpenProfileLogin(ctx, row.ProfileKey, rawURL)
	if err != nil {
		auditErr := s.writeActionAudit(c, adminID, "collect.browser_profile.open_login", id.String(), "failed", "collector request failed")
		return nil, errors.Join(err, auditErr)
	}
	if err := s.writeActionAudit(c, adminID, "collect.browser_profile.open_login", id.String(), "success", "opened headed browser for manual login"); err != nil {
		return nil, err
	}
	return &OpenLoginResultDTO{Message: msg, ProfileKey: row.ProfileKey}, nil
}

func (s *Service) Check(c *gin.Context, id uuid.UUID, body URLBody, adminID *uuid.UUID) (*CheckResultDTO, error) {
	if s.Collector == nil {
		return nil, fmt.Errorf("collector unavailable")
	}
	tenantID, err := tenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	row, err := s.activeByID(c.Request.Context(), tenantID, id)
	if err != nil {
		return nil, err
	}
	rawURL := strings.TrimSpace(body.URL)
	if rawURL == "" {
		return nil, fmt.Errorf("url is required")
	}
	host, err := s.hostnameFromURL(rawURL)
	if err != nil || !collectdomain.DomainMatches(host, row.Domain) {
		return nil, ErrProfileDomainMismatch
	}
	if err := s.writeActionAudit(c, adminID, "collect.browser_profile.check", id.String(), "started", "browser profile access check requested"); err != nil {
		return nil, err
	}
	ctx, cancel := s.collectorCtx(c.Request.Context())
	defer cancel()
	out, err := s.Collector.CheckProfileAccess(ctx, row.ProfileKey, rawURL)
	if err != nil {
		auditErr := s.writeActionAudit(c, adminID, "collect.browser_profile.check", id.String(), "failed", "collector request failed")
		return nil, errors.Join(err, auditErr)
	}
	now := time.Now()
	updates := map[string]any{
		"last_check_status": accessToLastCheck(out.AccessStatus),
		"last_check_url":    safeCheckURL(rawURL),
		"last_check_at":     &now,
		"last_error_code":   strings.TrimSpace(out.ErrorCode),
	}
	if err := s.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&CollectBrowserProfile{}).
			Where("id = ? AND tenant_id = ?", id, tenantID).
			Updates(updates).Error; err != nil {
			return err
		}
		return s.writeAuditTx(tx, c, adminID, "collect.browser_profile.check", id.String(), "success", fmt.Sprintf("access=%s", out.AccessStatus))
	}); err != nil {
		return nil, err
	}
	return out, nil
}

// MergeIntoRequestOptions adds profile snapshot fields to collect_tasks.request_options JSON.
func (s *Service) MergeIntoRequestOptions(
	ctx context.Context,
	tenantID int64,
	requestOptionsJSON []byte,
	profileID *uuid.UUID,
	useBrowserProfile bool,
	rawURL string,
) ([]byte, error) {
	if !useBrowserProfile || profileID == nil || *profileID == uuid.Nil {
		return requestOptionsJSON, nil
	}
	row, err := s.activeByID(ctx, tenantID, *profileID)
	if err != nil {
		return nil, err
	}
	host, err := s.hostnameFromURL(rawURL)
	if err != nil || !collectdomain.DomainMatches(host, row.Domain) {
		return nil, ErrProfileDomainMismatch
	}
	var m map[string]any
	if len(requestOptionsJSON) > 0 {
		if err := json.Unmarshal(requestOptionsJSON, &m); err != nil {
			return nil, err
		}
	} else {
		m = map[string]any{}
	}
	m["profileId"] = row.ID.String()
	m["profileKey"] = row.ProfileKey
	m["useBrowserProfile"] = true
	return json.Marshal(m)
}

// EnrichCollectorOptions adds profileKey for Collector HTTP options map.
func (s *Service) EnrichCollectorOptions(
	ctx context.Context,
	tenantID int64,
	opts map[string]any,
	profileID *uuid.UUID,
	useBrowserProfile bool,
	rawURL string,
) error {
	if !useBrowserProfile || profileID == nil || *profileID == uuid.Nil {
		return nil
	}
	row, err := s.activeByID(ctx, tenantID, *profileID)
	if err != nil {
		return err
	}
	host, err := s.hostnameFromURL(rawURL)
	if err != nil || !collectdomain.DomainMatches(host, row.Domain) {
		return ErrProfileDomainMismatch
	}
	opts["profileId"] = row.ID.String()
	opts["profileKey"] = row.ProfileKey
	opts["useBrowserProfile"] = true
	return nil
}
