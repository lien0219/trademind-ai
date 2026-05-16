package image

import (
	"fmt"
	"strings"
)

// NewProvider returns a named image provider implementation.
func NewProvider(name string) (Provider, error) {
	switch strings.TrimSpace(strings.ToLower(name)) {
	case "noop":
		return NoopProvider{}, nil
	default:
		return nil, fmt.Errorf("image: unknown provider %q", name)
	}
}
