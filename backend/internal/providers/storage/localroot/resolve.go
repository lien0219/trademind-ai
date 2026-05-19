package localroot

import (
	"path/filepath"
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/pkg/reporoot"
)

// DefaultRelative is the settings default for storage.local_root (resolved under repo root when possible).
const DefaultRelative = "data/uploads"

// Resolve turns storage.local_root into an absolute directory.
// Relative paths are anchored to the monorepo root, not the process working directory.
// When the repo root cannot be detected (e.g. Docker WORKDIR /app), falls back to cwd-relative Abs.
func Resolve(configured string) (string, error) {
	configured = strings.TrimSpace(configured)
	if configured == "" {
		configured = DefaultRelative
	}
	if filepath.IsAbs(configured) {
		return filepath.Abs(configured)
	}
	if root, ok := reporoot.Find(); ok {
		return filepath.Abs(filepath.Join(root, configured))
	}
	return filepath.Abs(configured)
}
