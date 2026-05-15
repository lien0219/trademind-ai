package admin

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// Store is a thin DB access helpers for AdminUser (no separate repository package).
type Store struct {
	DB *gorm.DB
}

// ByUsername returns the admin or gorm.ErrRecordNotFound.
func (s *Store) ByUsername(ctx context.Context, username string) (*AdminUser, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("admin store: no db")
	}
	var u AdminUser
	if err := s.DB.WithContext(ctx).Where("username = ?", username).First(&u).Error; err != nil {
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
