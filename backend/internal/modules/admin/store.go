package admin

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// Store is a thin DB access helpers for AdminUser (no separate repository package).
type Store struct {
	DB *gorm.DB
}

// ByLoginAccount finds a user by normalized email or phone (never by internal Username).
func (s *Store) ByLoginAccount(ctx context.Context, account string) (*AdminUser, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("admin store: no db")
	}
	em, ph, ok := ParseLoginAccount(account)
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	var u AdminUser
	q := s.DB.WithContext(ctx)
	if em != "" {
		if err := q.Where("LOWER(TRIM(email)) = ?", em).First(&u).Error; err != nil {
			return nil, err
		}
		return &u, nil
	}
	if err := q.Where("phone = ?", ph).First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

// ByEmail returns a user by canonical email (lowered) or ErrRecordNotFound.
func (s *Store) ByEmail(ctx context.Context, email string) (*AdminUser, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("admin store: no db")
	}
	e := strings.ToLower(strings.TrimSpace(email))
	var u AdminUser
	if err := s.DB.WithContext(ctx).Where("LOWER(TRIM(email)) = ?", e).First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

// ByID returns the admin or gorm.ErrRecordNotFound.
func (s *Store) ByID(ctx context.Context, id uuid.UUID) (*AdminUser, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("admin store: no db")
	}
	var u AdminUser
	if err := s.DB.WithContext(ctx).First(&u, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

// CheckPassword compares bcrypt hash with plain password.
func CheckPassword(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}
