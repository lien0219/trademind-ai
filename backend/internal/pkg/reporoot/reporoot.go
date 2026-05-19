package reporoot

import (
	"os"
	"path/filepath"
	"strings"
)

// Find returns the monorepo root (directory containing pnpm-workspace.yaml and backend/go.mod).
// Set TRADEMIND_REPO_ROOT to override. Returns ("", false) when not found (e.g. production image WORKDIR /app).
func Find() (string, bool) {
	if v := strings.TrimSpace(os.Getenv("TRADEMIND_REPO_ROOT")); v != "" {
		abs, err := filepath.Abs(v)
		if err != nil {
			return "", false
		}
		if st, err := os.Stat(abs); err == nil && st.IsDir() {
			return abs, true
		}
		return "", false
	}
	dir, err := os.Getwd()
	if err != nil {
		return "", false
	}
	for {
		if isRepoRoot(dir) {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

func isRepoRoot(dir string) bool {
	if st, err := os.Stat(filepath.Join(dir, "pnpm-workspace.yaml")); err != nil || st.IsDir() {
		return false
	}
	if st, err := os.Stat(filepath.Join(dir, "backend", "go.mod")); err != nil || st.IsDir() {
		return false
	}
	return true
}
