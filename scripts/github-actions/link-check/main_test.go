package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMarkdownIDsUsesExplicitHTMLIDs(t *testing.T) {
	oldRoot := root
	root = t.TempDir()
	t.Cleanup(func() { root = oldRoot })

	writeTestFile(t, "docs/example.markdown", `<a id="alpha"></a><p id="beta">text</p>`)

	ids := markdownIDs("docs/example.markdown")

	if !ids["alpha"] {
		t.Fatal("expected explicit anchor id to be collected")
	}
	if !ids["beta"] {
		t.Fatal("expected non-anchor HTML element id to be collected")
	}
}

func TestMarkdownIDsIgnoresMarkdownHeadingSlugs(t *testing.T) {
	oldRoot := root
	root = t.TempDir()
	t.Cleanup(func() { root = oldRoot })

	writeTestFile(t, "docs/example.markdown", "## Docker Compose Special Setups\n")

	ids := markdownIDs("docs/example.markdown")

	if ids["docker-compose-special-setups"] {
		t.Fatal("expected Markdown heading slug to be ignored")
	}
}

func writeTestFile(t *testing.T, relativePath string, contents string) {
	t.Helper()

	fullPath := filepath.Join(root, filepath.FromSlash(relativePath))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatalf("create parent directory: %v", err)
	}
	if err := os.WriteFile(fullPath, []byte(contents), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}
}
