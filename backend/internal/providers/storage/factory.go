package storage

import (
	"fmt"
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/providers/storage/local"
)

// NewFromPlain builds a storage Provider from decrypted settings.storage map (snake_case keys).
func NewFromPlain(m map[string]string) (Provider, string, error) {
	kind := strings.ToLower(strings.TrimSpace(m["kind"]))
	if kind == "" {
		kind = "local"
	}
	switch kind {
	case "local":
		root := strings.TrimSpace(m["local_root"])
		if root == "" {
			root = "data/uploads"
		}
		pub := strings.TrimSpace(m["public_base"])
		p, err := local.New(root, pub)
		if err != nil {
			return nil, kind, err
		}
		return p, kind, nil
	default:
		return nil, kind, fmt.Errorf("storage kind %q is not supported for file operations yet", kind)
	}
}
