package storage

import (
	"fmt"
	"strings"

	cosstorage "github.com/trademind-ai/trademind/backend/internal/providers/storage/cos"
	"github.com/trademind-ai/trademind/backend/internal/providers/storage/local"
	"github.com/trademind-ai/trademind/backend/internal/providers/storage/localroot"
	ossstorage "github.com/trademind-ai/trademind/backend/internal/providers/storage/oss"
	"github.com/trademind-ai/trademind/backend/internal/providers/storage/s3store"
)

// defaultLocalPublicBase matches settings seed and dev proxy (/static → backend).
const defaultLocalPublicBase = "/static"

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
		root, err := localroot.Resolve(m["local_root"])
		if err != nil {
			return nil, kind, err
		}
		pub := strings.TrimSpace(m["public_base"])
		if pub == "" {
			pub = defaultLocalPublicBase
		}
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
	case "cos":
		p, err := cosstorage.NewFromSettingsMap(m)
		if err != nil {
			return nil, kind, err
		}
		return p, kind, nil
	case "oss":
		p, err := ossstorage.NewFromSettingsMap(m)
		if err != nil {
			return nil, kind, err
		}
		return p, kind, nil
	default:
		return nil, kind, fmt.Errorf("storage kind %q is not supported", kind)
	}
}
