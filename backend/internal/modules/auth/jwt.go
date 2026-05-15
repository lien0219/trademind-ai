package auth

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/config"
)

// Claims is the JWT payload for admin sessions.
type Claims struct {
	Username  string `json:"username"`
	TokenType string `json:"typ"`
	jwt.RegisteredClaims
}

// MintToken issues a signed JWT for the given admin.
func MintToken(cfg *config.Config, adminID uuid.UUID, username string) (string, time.Time, error) {
	if cfg == nil {
		return "", time.Time{}, fmt.Errorf("jwt: nil config")
	}
	secret := []byte(cfg.JWTSecret)
	if len(secret) == 0 {
		return "", time.Time{}, errors.New("jwt: empty JWT_SECRET")
	}
	exp := time.Now().UTC().Add(time.Duration(cfg.JWTExpHrs) * time.Hour)
	claims := Claims{
		Username:  username,
		TokenType: "access",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   adminID.String(),
			ExpiresAt: jwt.NewNumericDate(exp),
			IssuedAt:  jwt.NewNumericDate(time.Now().UTC()),
		},
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := t.SignedString(secret)
	if err != nil {
		return "", time.Time{}, err
	}
	return signed, exp, nil
}

// ParseToken validates the token and returns claims.
func ParseToken(cfg *config.Config, tokenStr string) (*Claims, error) {
	if cfg == nil {
		return nil, fmt.Errorf("jwt: nil config")
	}
	secret := []byte(cfg.JWTSecret)
	if len(secret) == 0 {
		return nil, errors.New("jwt: empty JWT_SECRET")
	}
	t, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("jwt: unexpected signing method %v", t.Header["alg"])
		}
		return secret, nil
	})
	if err != nil {
		return nil, err
	}
	c, ok := t.Claims.(*Claims)
	if !ok || !t.Valid {
		return nil, errors.New("jwt: invalid token")
	}
	if c.TokenType != "" && c.TokenType != "access" {
		return nil, errors.New("jwt: wrong token type")
	}
	if strings.TrimSpace(c.Subject) == "" {
		return nil, errors.New("jwt: empty subject")
	}
	return c, nil
}
