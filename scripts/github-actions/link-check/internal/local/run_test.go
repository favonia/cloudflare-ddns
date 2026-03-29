package local

import (
	"slices"
	"strings"
	"testing"

	"github.com/favonia/cloudflare-ddns/scripts/github-actions/link-check/internal/testutil"
)

func TestInferRepoPathPrefixesUsesTrackedTopLevelDirectories(t *testing.T) {
	got := inferRepoPathPrefixes([]string{
		"README.markdown",
		"internal/api/ttl.go",
		"scripts/github-actions/link-check/main.go",
		".github/workflows/test.yml",
		"docs/designs/README.markdown",
		"internal/config/env.go",
	})

	want := []string{".github", "docs", "internal", "scripts"}
	if !slices.Equal(got, want) {
		t.Fatalf("unexpected inferred prefixes:\nwant %#v\ngot  %#v", want, got)
	}
}

func TestValidateCommentRepoPathsIgnoresURLTextInsideGoStrings(t *testing.T) {
	root := t.TempDir()
	testutil.WriteFile(t, root, "scripts/example.go", strings.Join([]string{
		"package main",
		`var _ = "https://github.com/favonia/cloudflare-ddns/issues/102 docs/example.markdown"`,
	}, "\n"))

	issues := validateCommentRepoPaths(root, []string{"scripts/example.go"}, []string{"scripts/example.go"}, nil)

	if len(issues) != 0 {
		t.Fatalf("expected no repo-path issues from Go string literals, got %#v", issues)
	}
}

func TestValidateCommentRepoPathsFlagsDirectoryLikeCommentPaths(t *testing.T) {
	root := t.TempDir()
	testutil.WriteFile(t, root, "internal/example.go", strings.Join([]string{
		"package internal",
		"// See internal/ppppppp/oooo before changing this logic.",
	}, "\n"))

	issues := validateCommentRepoPaths(root, []string{"internal/example.go"}, []string{"internal/example.go"}, nil)

	if len(issues) != 1 {
		t.Fatalf("expected 1 repo-path issue, got %#v", issues)
	}
	if issues[0].Kind != "broken-repo-path" || issues[0].Detail != "internal/ppppppp/oooo" {
		t.Fatalf("unexpected repo-path issue: %#v", issues[0])
	}
}

func TestValidateCommentRepoPathsIgnoresURLPathSegmentsInsideComments(t *testing.T) {
	root := t.TempDir()
	testutil.WriteFile(t, root, "internal/example.go", strings.Join([]string{
		"package internal",
		"// See https://example.com/internal/ppppppp/oooo for context.",
	}, "\n"))

	issues := validateCommentRepoPaths(root, []string{"internal/example.go"}, []string{"internal/example.go"}, nil)

	if len(issues) != 0 {
		t.Fatalf("expected no repo-path issues from URL path segments, got %#v", issues)
	}
}

func TestDefaultConfigIgnoresItsOwnRepoPath(t *testing.T) {
	cfg := defaultConfig()

	if !slices.Contains(cfg.CommentRepoPaths.TargetPaths.IgnoreExact, "scripts/github-actions/link-check/internal/local/config.go") {
		t.Fatal("expected default config to ignore its own repo path")
	}
}

func TestDefaultConfigExcludesCloudflareDocWatchCasesFromRepoPathScan(t *testing.T) {
	cfg := defaultConfig()

	if !slices.Contains(cfg.CommentRepoPaths.SourceFiles.ExcludeGlobs, "scripts/github-actions/cloudflare-doc-watch/cases.go") {
		t.Fatal("expected default config to exclude cloudflare-doc-watch cases from repo-path scanning")
	}
}
