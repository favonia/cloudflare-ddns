package testutil

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// WriteFile creates one test file relative to root.
func WriteFile(t *testing.T, root, relativePath, contents string) {
	t.Helper()

	fullPath := filepath.Join(root, filepath.FromSlash(relativePath))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o750); err != nil {
		t.Fatalf("create parent directory: %v", err)
	}
	if err := os.WriteFile(fullPath, []byte(contents), 0o600); err != nil {
		t.Fatalf("write test file: %v", err)
	}
}

// InitTrackedRepo creates an empty temporary Git repository for tests.
func InitTrackedRepo(t *testing.T) string {
	t.Helper()

	repoRoot := t.TempDir()
	command := exec.CommandContext(context.Background(), "git", "init")
	command.Dir = repoRoot
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, output)
	}
	return repoRoot
}

// WriteTrackedFile writes one file and stages it in the test repository.
func WriteTrackedFile(t *testing.T, root, relativePath, contents string) {
	t.Helper()

	WriteFile(t, root, relativePath, contents)

	command := exec.CommandContext(context.Background(), "git", "add", "--all")
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git add --all after writing %s: %v\n%s", relativePath, err, output)
	}
}
