package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/metrics"
	"github.com/trademind-ai/trademind/backend/internal/pkg/runtimediag"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// LoginService handles credential checks and token issuance.
type LoginService struct {
	Cfg      *config.Config
	Admins   *admin.Store
	Sessions *SessionService
	Metrics  *metrics.Catalog
}

// LoginResult is returned to HTTP layer.
type LoginResult struct {
	Token        string
	RefreshToken string
	ExpiresAt    int64 // unix seconds
	User         userView
}

type userView struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	Email       string `json:"email,omitempty"`
	Phone       string `json:"phone,omitempty"`
	DisplayName string `json:"displayName"`
}

// Login verifies credentials and returns tokens (session or legacy).
func (s *LoginService) Login(ctx context.Context, account, password, ip, userAgent string) (*LoginResult, error) {
	if s == nil || s.Admins == nil || s.Cfg == nil {
		return nil, fmt.Errorf("auth: misconfigured")
	}
	s.ObserveAuth("login_attempt", "success", "attempt", "password")
	if s.Sessions != nil && (s.Cfg.UsesSecureSession() || s.Cfg.Auth.SessionMode == config.AuthSessionModeSecure) {
		res, err := s.Sessions.CreateSession(ctx, account, password, ip, userAgent)
		if err != nil {
			s.ObserveAuth("login_failure", "failure", classifyAuthReason(err), "password")
			return nil, err
		}
		s.ObserveAuth("login_success", "success", "success", "password")
		return &LoginResult{
			Token:        res.AccessToken,
			RefreshToken: res.RefreshToken,
			ExpiresAt:    res.AccessExp.Unix(),
			User:         res.User,
		}, nil
	}
	res, err := s.legacyLogin(ctx, account, password)
	if err != nil {
		s.ObserveAuth("login_failure", "failure", classifyAuthReason(err), "password")
		return nil, err
	}
	s.ObserveAuth("login_success", "success", "success", "password")
	return res, nil
}

func (s *LoginService) legacyLogin(ctx context.Context, account, password string) (*LoginResult, error) {
	stageStart := time.Now()
	var u *admin.AdminUser
	var db *gorm.DB
	if s.Admins != nil {
		db = s.Admins.DB
	}
	timing, err := runtimediag.TimedGorm(db, func() error {
		var lookupErr error
		u, lookupErr = s.Admins.ByLoginAccount(ctx, account)
		return lookupErr
	})
	outcome := authOutcome(err)
	runtimediag.ObserveStage(runtimediag.RouteAuthInvalidLogin, "account_lookup", outcome, stageStart)
	runtimediag.ObserveDBOperation(runtimediag.RouteAuthInvalidLogin, "account_lookup", outcome, stageStart)
	runtimediag.ObserveSQL(runtimediag.RouteAuthInvalidLogin, "auth", "auth.account_lookup", "select", "admin_users", outcome, false, timing)
	runtimediag.Count(runtimediag.RouteAuthInvalidLogin, "accountLookupCount", 1)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			runtimediag.Path(runtimediag.RouteAuthInvalidLogin, "account_missing")
			runtimediag.Path(runtimediag.RouteAuthInvalidLogin, runtimediag.PathUnknownAccount)
			runtimediag.Count(runtimediag.RouteAuthInvalidLogin, "unknownAccountQueryCount", 1)
			runtimediag.ObservePasswordVerify(runtimediag.PathUnknownAccount, runtimediag.PasswordAlgoBcrypt, bcrypt.DefaultCost, 0, time.Now())
			return nil, errors.New(ErrInvalidCredentials)
		}
		return nil, err
	}
	stageStart = time.Now()
	cost := bcrypt.DefaultCost
	if c, cerr := bcrypt.Cost([]byte(u.PasswordHash)); cerr == nil {
		cost = c
	}
	if err := admin.CheckPassword(u.PasswordHash, password); err != nil {
		runtimediag.ObserveStage(runtimediag.RouteAuthInvalidLogin, "password_verify", runtimediag.OutcomeExpectedRejection, stageStart)
		runtimediag.ObservePasswordVerify(runtimediag.PathKnownWrongPassword, runtimediag.PasswordAlgoBcrypt, cost, 1, stageStart)
		runtimediag.Count(runtimediag.RouteAuthInvalidLogin, "passwordVerifyCount", 1)
		runtimediag.Count(runtimediag.RouteAuthInvalidLogin, "wrongPasswordQueryCount", 1)
		runtimediag.Path(runtimediag.RouteAuthInvalidLogin, "wrong_password")
		runtimediag.Path(runtimediag.RouteAuthInvalidLogin, runtimediag.PathKnownWrongPassword)
		return nil, errors.New(ErrInvalidCredentials)
	}
	runtimediag.ObserveStage(runtimediag.RouteAuthInvalidLogin, "password_verify", runtimediag.OutcomeSuccess, stageStart)
	runtimediag.ObservePasswordVerify(runtimediag.PathSuccessVerify, runtimediag.PasswordAlgoBcrypt, cost, 1, stageStart)
	runtimediag.Count(runtimediag.RouteAuthInvalidLogin, "passwordVerifyCount", 1)
	if st := strings.TrimSpace(strings.ToLower(u.Status)); st == "disabled" || st == "inactive" {
		return nil, errors.New(ErrUserDisabled)
	}
	if u.MustChangePassword {
		return nil, errors.New(ErrPasswordChangeRequired)
	}
	label := u.LoginLabel()
	ks, err := BuildKeySet(s.Cfg)
	if err != nil {
		return nil, err
	}
	token, exp, err := MintAccessToken(s.Cfg, ks, MintAccessInput{
		UserID:       u.ID,
		Username:     label,
		TenantID:     u.TenantID,
		TokenVersion: u.TokenVersion,
	})
	if err != nil {
		return nil, err
	}
	dn := u.DisplayName
	if dn == "" {
		dn = label
	}
	return &LoginResult{
		Token:     token,
		ExpiresAt: exp.Unix(),
		User: userView{
			ID:          u.ID.String(),
			Username:    label,
			Email:       u.Email,
			Phone:       u.Phone,
			DisplayName: dn,
		},
	}, nil
}

func authOutcome(err error) string {
	if err == nil {
		return runtimediag.OutcomeSuccess
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return runtimediag.OutcomeExpectedRejection
	}
	return runtimediag.OutcomeError
}
