package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"gorm.io/gorm"
)

// LoginService handles credential checks and token issuance.
type LoginService struct {
	Cfg    *config.Config
	Admins *admin.Store
}

// LoginResult is returned to HTTP layer.
type LoginResult struct {
	Token     string
	ExpiresAt int64 // unix seconds
	User      userView
}

type userView struct {
	ID          string `json:"id"`
	Username    string `json:"username"` // login identity (email or phone), not internal DB username field
	Email       string `json:"email,omitempty"`
	Phone       string `json:"phone,omitempty"`
	DisplayName string `json:"displayName"`
}

// Login verifies credentials and returns a JWT.
func (s *LoginService) Login(ctx context.Context, account, password string) (*LoginResult, error) {
	if s == nil || s.Admins == nil || s.Cfg == nil {
		return nil, fmt.Errorf("auth: misconfigured")
	}
	u, err := s.Admins.ByLoginAccount(ctx, account)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("invalid account or password")
		}
		return nil, err
	}
	if err := admin.CheckPassword(u.PasswordHash, password); err != nil {
		return nil, errors.New("invalid account or password")
	}
	label := u.LoginLabel()
	token, exp, err := MintToken(s.Cfg, u.ID, label)
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
