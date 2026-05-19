package localroot

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolve_relativeUsesRepoRoot(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	// package dir: backend/internal/providers/storage/localroot
	repo := filepath.Clean(filepath.Join(wd, "..", "..", "..", "..", ".."))
	t.Setenv("TRADEMIND_REPO_ROOT", repo)

	got, err := Resolve("data/uploads")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(repo, "data", "uploads")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
