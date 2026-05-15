// Package id defines UUID identifiers for all persisted entities and foreign keys.
package id

import (
	"github.com/google/uuid"
)

// New returns a new random UUID v4.
func New() uuid.UUID {
	return uuid.New()
}

// IsNil reports whether u is unset.
func IsNil(u uuid.UUID) bool {
	return u == uuid.Nil
}

// Ensure sets *dst to a new UUID when it is currently nil/zero.
func Ensure(dst *uuid.UUID) {
	if dst == nil || *dst != uuid.Nil {
		return
	}
	*dst = New()
}

// Parse wraps uuid.Parse for path/query parameters.
func Parse(s string) (uuid.UUID, error) {
	return uuid.Parse(s)
}
