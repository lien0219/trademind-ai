package securitymod

import (
	"context"
	"fmt"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/crypto"
	"github.com/trademind-ai/trademind/backend/internal/pkg/metrics"
	"github.com/trademind-ai/trademind/backend/internal/pkg/security"
	"gorm.io/gorm"
)

// Service provides key rotation and audit integrity operations.
type Service struct {
	DB      *gorm.DB
	Cfg     *config.Config
	OpLogs  *operationlog.Service
	Metrics *metrics.Catalog
}

// RotationStatus summarizes master key rotation readiness.
type RotationStatus struct {
	ActiveKeyID        string `json:"activeKeyId"`
	PendingReencrypt   int64  `json:"pendingReencrypt"`
	PreviousKeyCount   int    `json:"previousKeyCount"`
	LastVerifiedAt     string `json:"lastVerifiedAt,omitempty"`
	IntegrityOK        bool   `json:"integrityOk"`
	IntegrityCheckedAt string `json:"integrityCheckedAt,omitempty"`
}

// PrepareRotation dry-run counts records that would be re-encrypted.
func (s *Service) PrepareRotation(ctx context.Context) (*RotationStatus, error) {
	if s == nil || s.Cfg == nil {
		return nil, fmt.Errorf("security: unavailable")
	}
	kr, err := s.keyRing()
	if err != nil {
		return nil, err
	}
	pending, err := s.countPendingReencrypt(ctx)
	if err != nil {
		return nil, err
	}
	return &RotationStatus{
		ActiveKeyID:      kr.ActiveID,
		PendingReencrypt: pending,
		PreviousKeyCount: len(kr.PreviousKeys),
	}, nil
}

// VerifyAuditIntegrity checks only the authenticated tenant's audit hash chain.
func (s *Service) VerifyAuditIntegrity(ctx context.Context, days int) (int, error) {
	if s == nil || s.OpLogs == nil {
		return 0, fmt.Errorf("security: unavailable")
	}
	tenant := security.FromContext(ctx)
	if tenant == nil || tenant.TenantID < 0 {
		return 0, security.ErrTenantContextMissing
	}
	if days <= 0 {
		days = 7
	}
	to := time.Now().UTC()
	from := to.Add(-time.Duration(days) * 24 * time.Hour)
	n, _, err := s.OpLogs.VerifyChain(ctx, tenant.TenantID, from, to)
	if err != nil {
		s.ObserveSecurity("operationlog", "audit_chain_mismatch", "failure", "critical")
	}
	return n, err
}

func (s *Service) keyRing() (*crypto.KeyRing, error) {
	activeID := s.Cfg.Auth.AppMasterActiveKeyID
	activeKey := s.Cfg.Auth.AppMasterActiveKey
	if activeKey == "" {
		activeKey = s.Cfg.MasterKey
	}
	return crypto.NewKeyRing(activeID, activeKey, s.Cfg.Auth.AppMasterPreviousKeys)
}

// SecurityOverview returns high-level security posture for the security center UI.
func (s *Service) SecurityOverview(ctx context.Context) (map[string]any, error) {
	if s == nil || s.Cfg == nil {
		return nil, fmt.Errorf("security: unavailable")
	}
	kr, err := s.keyRing()
	if err != nil {
		return nil, err
	}
	mode := "legacy_local_storage"
	if s.Cfg.UsesSecureSession() {
		mode = "secure_session"
	}
	debugSurface := config.IsProduction(s.Cfg.AppEnv) &&
		(s.Cfg.EnableDebugEndpoints || s.Cfg.EnableSwagger || s.Cfg.EnableDevRoutes)
	return map[string]any{
		"authSessionMode":        mode,
		"accessTokenTTLMinutes":  s.Cfg.Auth.AccessTokenTTLMinutes,
		"refreshTokenTTLDays":    s.Cfg.Auth.RefreshTokenTTLDays,
		"secureCookie":           s.Cfg.UsesSecureSession(),
		"loginMaxAttempts":       s.Cfg.Auth.LoginMaxAttempts,
		"passwordMinLength":      s.Cfg.Auth.PasswordMinLength,
		"jwtActiveKeyId":         s.Cfg.Auth.JWTActiveKeyID,
		"appMasterActiveKeyId":   kr.ActiveID,
		"activeSessionCount":     0,
		"productionDebugSurface": debugSurface,
	}, nil
}
