package storage

import (
	"fmt"
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/providers/storage/local"
	"github.com/trademind-ai/trademind/backend/internal/providers/storage/s3store"
)

// NewFromPlain builds a storage Provider from decrypted settings.storage map (snake_case keys).
// The operational kind follows settings.kind (used for uploads and current-policy operations).
func NewFromPlain(m map[string]string) (Provider, string, error) {
	return NewFromPlainForStoredKind(m, "")
}

// NewFromPlainForStoredKind uses storedKind when set (recommended for deletes / historical objects),
// otherwise resolves to settings.kind.
func NewFromPlainForStoredKind(m map[string]string, storedKind string) (Provider, string, error) {
	kind := strings.TrimSpace(strings.ToLower(storedKind))
	if kind == "" {
		kind = normalizedKindFromSettings(m)
	}
	return providerForOperationalKind(m, kind)
}

func normalizedKindFromSettings(m map[string]string) string {
	k := strings.TrimSpace(strings.ToLower(m["kind"]))
	if k == "" {
		return "local"
	}
	return k
}

func providerForOperationalKind(m map[string]string, kind string) (Provider, string, error) {
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
	case "s3", "r2", "minio":
		p, err := s3store.NewFromSettingsMap(kind, m)
		if err != nil {
			return nil, kind, err
		}
		return p, kind, nil
	case "cos", "oss":
		return nil, kind, fmt.Errorf("storage provider %q is not implemented yet (use AWS S3, R2, or MinIO for now)", kind)
	default:
		return nil, kind, fmt.Errorf("storage kind %q is not supported", kind)
	}
}
