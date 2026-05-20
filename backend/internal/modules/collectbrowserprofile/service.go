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

	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/collectdomain"
)

var (
	ErrProfileNotFound       = errors.New("PROFILE_NOT_FOUND: collect browser profile not found or disabled")
	ErrProfileDomainMismatch = errors.New("PROFILE_DOMAIN_MISMATCH: profile domain does not match url host")
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

func (s *Service) activeByID(ctx context.Context, id uuid.UUID) (*CollectBrowserProfile, error) {
	var row CollectBrowserProfile
	err := s.DB.WithContext(ctx).First(&row, "id = ? AND status = ?", id, StatusActive).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProfileNotFound
		}
		return nil, err
	}
	return &row, nil
}

func (s *Service) List(ctx context.Context, q ListQuery) ([]RowDTO, int64, error) {
	if s == nil || s.DB == nil {
		return nil, 0, fmt.Errorf("collectbrowserprofile: no db")
	}
	page, ps := s.clampPage(q.Page, q.PageSize)
	tx := s.DB.WithContext(ctx).Model(&CollectBrowserProfile{})
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
	name := strings.TrimSpace(body.Name)
	domain := collectdomain.NormalizeRuleDomain(body.Domain)
	if name == "" || domain == "" {
		return nil, fmt.Errorf("name and domain are required")
	}
	provider := strings.TrimSpace(body.Provider)
	if provider == "" {
		provider = "custom"
	}
	row := CollectBrowserProfile{
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

	if err := s.DB.WithContext(c.Request.Context()).Create(&row).Error; err != nil {
		return nil, err
	}
	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      "collect.browser_profile.create",
			Resource:    "collect_browser_profile",
			ResourceID:  row.ID.String(),
			Status:      "success",
			Message:     fmt.Sprintf("domain=%s profileKey=%s", domain, row.ProfileKey),
		})
	}
	dto := rowToDTO(&row)
	return &CreateResultDTO{ProfileID: row.ID, ProfileKey: row.ProfileKey, Row: dto}, nil
}

func (s *Service) Disable(c *gin.Context, id uuid.UUID, adminID *uuid.UUID) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("collectbrowserprofile: no db")
	}
	res := s.DB.WithContext(c.Request.Context()).Model(&CollectBrowserProfile{}).
		Where("id = ?", id).
		Update("status", StatusDisabled)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrProfileNotFound
	}
	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      "collect.browser_profile.disable",
			Resource:    "collect_browser_profile",
			ResourceID:  id.String(),
			Status:      "success",
		})
	}
	return nil
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
	row, err := s.activeByID(c.Request.Context(), id)
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
	ctx, cancel := s.collectorCtx(c.Request.Context())
	defer cancel()
	msg, err := s.Collector.OpenProfileLogin(ctx, row.ProfileKey, rawURL)
	if err != nil {
		return nil, err
	}
	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      "collect.browser_profile.open_login",
			Resource:    "collect_browser_profile",
			ResourceID:  id.String(),
			Status:      "success",
			Message:     "opened headed browser for manual login",
		})
	}
	return &OpenLoginResultDTO{Message: msg, ProfileKey: row.ProfileKey}, nil
}

func (s *Service) Check(c *gin.Context, id uuid.UUID, body URLBody, adminID *uuid.UUID) (*CheckResultDTO, error) {
	if s.Collector == nil {
		return nil, fmt.Errorf("collector unavailable")
	}
	row, err := s.activeByID(c.Request.Context(), id)
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
	ctx, cancel := s.collectorCtx(c.Request.Context())
	defer cancel()
	out, err := s.Collector.CheckProfileAccess(ctx, row.ProfileKey, rawURL)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	updates := map[string]any{
		"last_check_status": accessToLastCheck(out.AccessStatus),
		"last_check_url":    rawURL,
		"last_check_at":     &now,
		"last_error_code":   strings.TrimSpace(out.ErrorCode),
	}
	_ = s.DB.WithContext(c.Request.Context()).Model(&CollectBrowserProfile{}).Where("id = ?", id).Updates(updates).Error
	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      "collect.browser_profile.check",
			Resource:    "collect_browser_profile",
			ResourceID:  id.String(),
			Status:      "success",
			Message:     fmt.Sprintf("access=%s", out.AccessStatus),
		})
	}
	return out, nil
}

// MergeIntoRequestOptions adds profile snapshot fields to collect_tasks.request_options JSON.
func (s *Service) MergeIntoRequestOptions(
	ctx context.Context,
	requestOptionsJSON []byte,
	profileID *uuid.UUID,
	useBrowserProfile bool,
	rawURL string,
) ([]byte, error) {
	if !useBrowserProfile || profileID == nil || *profileID == uuid.Nil {
		return requestOptionsJSON, nil
	}
	row, err := s.activeByID(ctx, *profileID)
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
	opts map[string]any,
	profileID *uuid.UUID,
	useBrowserProfile bool,
	rawURL string,
) error {
	if !useBrowserProfile || profileID == nil || *profileID == uuid.Nil {
		return nil
	}
	row, err := s.activeByID(ctx, *profileID)
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
