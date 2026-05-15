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
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
}

// Login verifies credentials and returns a JWT.
func (s *LoginService) Login(ctx context.Context, username, password string) (*LoginResult, error) {
	if s == nil || s.Admins == nil || s.Cfg == nil {
		return nil, fmt.Errorf("auth: misconfigured")
	}
	u, err := s.Admins.ByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("invalid username or password")
		}
		return nil, err
	}
	if err := admin.CheckPassword(u.PasswordHash, password); err != nil {
		return nil, errors.New("invalid username or password")
	}
	token, exp, err := MintToken(s.Cfg, u.ID, u.Username)
	if err != nil {
		return nil, err
	}
	dn := u.DisplayName
	if dn == "" {
		dn = u.Username
	}
	return &LoginResult{
		Token:     token,
		ExpiresAt: exp.Unix(),
		User: userView{
			ID:          u.ID.String(),
			Username:    u.Username,
			DisplayName: dn,
		},
	}, nil
}
