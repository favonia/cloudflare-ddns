package main

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
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

func TestCollectExternalURLsKeepsSourceLocations(t *testing.T) {
	oldRoot := root
	root = t.TempDir()
	t.Cleanup(func() { root = oldRoot })

	writeTestFile(t, "docs/example.markdown", strings.Join([]string{
		"See [issue](https://github.com/favonia/cloudflare-ddns/issues/102).",
		"",
		"Another mention: https://github.com/favonia/cloudflare-ddns/issues/102",
		"[ref]: https://example.com/ref",
		"Use [reference][ref].",
	}, "\n"))

	urls := collectExternalURLs([]string{"docs/example.markdown"}, nil, nil)

	if len(urls) != 2 {
		t.Fatalf("expected 2 URLs, got %d", len(urls))
	}

	if urls[0].URL != "https://example.com/ref" {
		t.Fatalf("expected first URL to be the reference target, got %q", urls[0].URL)
	}
	if len(urls[0].Sources) != 1 || urls[0].Sources[0].Render() != "docs/example.markdown:4" {
		t.Fatalf("expected reference URL source at line 4, got %#v", urls[0].Sources)
	}

	if urls[1].URL != "https://github.com/favonia/cloudflare-ddns/issues/102" {
		t.Fatalf("expected second URL to be the issue link, got %q", urls[1].URL)
	}
	gotSources := []string{urls[1].Sources[0].Render(), urls[1].Sources[1].Render()}
	wantSources := []string{"docs/example.markdown:1", "docs/example.markdown:3"}
	if !slices.Equal(gotSources, wantSources) {
		t.Fatalf("unexpected sources:\nwant %#v\ngot  %#v", wantSources, gotSources)
	}
}

func TestValidateRepoPathsIgnoresURLTextInsideGoStrings(t *testing.T) {
	oldRoot := root
	root = t.TempDir()
	t.Cleanup(func() { root = oldRoot })

	writeTestFile(t, "scripts/example.go", strings.Join([]string{
		"package main",
		`var _ = "https://github.com/favonia/cloudflare-ddns/issues/102 docs/example.markdown"`,
	}, "\n"))

	issues := validateRepoPaths([]string{"scripts/example.go"}, []string{"scripts/example.go"}, nil)

	if len(issues) != 0 {
		t.Fatalf("expected no repo-path issues from Go string literals, got %#v", issues)
	}
}

func TestValidateRepoPathsFlagsDirectoryLikeCommentPaths(t *testing.T) {
	oldRoot := root
	root = t.TempDir()
	t.Cleanup(func() { root = oldRoot })

	writeTestFile(t, "internal/example.go", strings.Join([]string{
		"package internal",
		"// See internal/ppppppp/oooo before changing this logic.",
	}, "\n"))

	issues := validateRepoPaths([]string{"internal/example.go"}, []string{"internal/example.go"}, nil)

	if len(issues) != 1 {
		t.Fatalf("expected 1 repo-path issue, got %#v", issues)
	}
	if issues[0].Kind != "broken-repo-path" || issues[0].Detail != "internal/ppppppp/oooo" {
		t.Fatalf("unexpected repo-path issue: %#v", issues[0])
	}
}

func TestValidateRepoPathsIgnoresURLPathSegmentsInsideComments(t *testing.T) {
	oldRoot := root
	root = t.TempDir()
	t.Cleanup(func() { root = oldRoot })

	writeTestFile(t, "internal/example.go", strings.Join([]string{
		"package internal",
		"// See https://example.com/internal/ppppppp/oooo for context.",
	}, "\n"))

	issues := validateRepoPaths([]string{"internal/example.go"}, []string{"internal/example.go"}, nil)

	if len(issues) != 0 {
		t.Fatalf("expected no repo-path issues from URL path segments, got %#v", issues)
	}
}

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

func TestDefaultConfigIgnoresItsOwnRepoPath(t *testing.T) {
	cfg := defaultConfig()

	if !slices.Contains(cfg.RepoPaths.TargetPaths.IgnoreExact, "scripts/github-actions/link-check/config.go") {
		t.Fatal("expected default config to ignore its own repo path")
	}
}

func TestDefaultConfigExcludesCloudflareDocWatchCasesFromRepoPathScan(t *testing.T) {
	cfg := defaultConfig()

	if !slices.Contains(cfg.RepoPaths.SourceFiles.ExcludeGlobs, "scripts/github-actions/cloudflare-doc-watch/cases.go") {
		t.Fatal("expected default config to exclude cloudflare-doc-watch cases from repo-path scanning")
	}
}

func TestDefaultConfigIgnoresSameRepoIssueURLs(t *testing.T) {
	cfg := defaultConfig()

	if !slices.Contains(cfg.External.TargetURLs.IgnorePatterns, "^https://github\\.com/favonia/cloudflare-ddns/issues/[0-9]+$") {
		t.Fatal("expected default config to ignore same-repo issue URLs")
	}
}

func TestDefaultConfigIgnoresOperationalEndpoints(t *testing.T) {
	cfg := defaultConfig()

	want := []string{
		"https://one.one.one.one/cdn-cgi/trace",
		"https://api.cloudflare.com/cdn-cgi/trace",
		"https://api.cloudflare.com/client/v4/user/tokens/verify",
		"https://token.actions.githubusercontent.com",
		"https://cloudflare-dns.com/dns-query",
		"https://api4.ipify.org",
		"https://api6.ipify.org",
	}
	for _, url := range want {
		if !slices.Contains(cfg.External.TargetURLs.IgnoreExact, url) {
			t.Fatalf("expected default config to ignore %q", url)
		}
	}
}

func TestDefaultConfigHasNoOperationalWarningURLPatterns(t *testing.T) {
	cfg := defaultConfig()

	if len(cfg.ExternalProbe.WarningURLPatterns) != 0 {
		t.Fatalf("expected operational endpoints to be ignored instead of downgraded, got %#v", cfg.ExternalProbe.WarningURLPatterns)
	}
}

func TestDefaultConfigExcludesItsOwnFileFromExternalScan(t *testing.T) {
	cfg := defaultConfig()

	if !slices.Contains(cfg.External.SourceFiles.ExcludeGlobs, "scripts/github-actions/link-check/config.go") {
		t.Fatal("expected default config to exclude its own file from external scanning")
	}
}

func TestSuppressTrailingSlashRedirectWarning(t *testing.T) {
	t.Run("suppressed", func(t *testing.T) {
		result := probeResult{
			URL: "https://example.com/docs",
			Hops: []probeHop{
				{URL: "https://example.com/docs", Status: 301},
				{URL: "https://example.com/docs/", Status: 200},
			},
		}

		if !suppressTrailingSlashRedirectWarning(result) {
			t.Fatal("expected trailing-slash redirect warning to be suppressed")
		}
	})

	t.Run("query change not suppressed", func(t *testing.T) {
		result := probeResult{
			URL: "https://example.com/docs",
			Hops: []probeHop{
				{URL: "https://example.com/docs", Status: 301},
				{URL: "https://example.com/docs/?ref=1", Status: 200},
			},
		}

		if suppressTrailingSlashRedirectWarning(result) {
			t.Fatal("expected redirect with query change to remain visible")
		}
	})

	t.Run("trailing slash removed", func(t *testing.T) {
		result := probeResult{
			URL: "https://pkg.go.dev/github.com/favonia/cloudflare-ddns/",
			Hops: []probeHop{
				{URL: "https://pkg.go.dev/github.com/favonia/cloudflare-ddns/", Status: 301},
				{URL: "https://pkg.go.dev/github.com/favonia/cloudflare-ddns", Status: 200},
			},
		}

		if !suppressTrailingSlashRedirectWarning(result) {
			t.Fatal("expected trailing-slash removal redirect warning to be suppressed")
		}
	})
}

func TestShouldSuppressExternalWarningUsesPolicyHooks(t *testing.T) {
	result := probeResult{
		URL: "https://example.com/docs",
		Hops: []probeHop{
			{URL: "https://example.com/docs", Status: 301},
			{URL: "https://example.com/docs/", Status: 200},
		},
	}

	if !shouldSuppressExternalWarning(result) {
		t.Fatal("expected policy hook to suppress trailing-slash redirect warning")
	}
}

func TestFormatExternalResultUsesClearCategories(t *testing.T) {
	t.Run("network error", func(t *testing.T) {
		got := formatExternalResult("warning", probeResult{
			URL:  "https://api6.ipify.org",
			Err:  "dial tcp [2607:f2d8:1:3c::4]:443: connect: network is unreachable",
			Hops: []probeHop{{URL: "https://api6.ipify.org"}},
			Sources: []linkSource{{
				Path: "docs/example.markdown",
				Line: 12,
			}},
		})
		want := "warning: network error: https://api6.ipify.org (network error: dial tcp [2607:f2d8:1:3c::4]:443: connect: network is unreachable) [docs/example.markdown:12]"
		if got != want {
			t.Fatalf("unexpected formatted result:\nwant %q\ngot  %q", want, got)
		}
	})

	t.Run("redirect", func(t *testing.T) {
		got := formatExternalResult("warning", probeResult{
			URL:    "https://containrrr.dev/shoutrrr",
			Status: 200,
			Hops: []probeHop{
				{URL: "https://containrrr.dev/shoutrrr", Status: 301},
				{URL: "https://containrrr.dev/shoutrrr/", Status: 200},
			},
			Sources: []linkSource{{
				Path: "README.markdown",
				Line: 7,
			}},
		})
		want := "warning: https://containrrr.dev/shoutrrr (301) -> https://containrrr.dev/shoutrrr/ (200) [README.markdown:7]"
		if got != want {
			t.Fatalf("unexpected formatted result:\nwant %q\ngot  %q", want, got)
		}
	})

	t.Run("status", func(t *testing.T) {
		got := formatExternalResult("failure", probeResult{
			URL:    "https://example.com/missing",
			Status: 404,
			Hops: []probeHop{
				{URL: "https://example.com/missing", Status: 404},
			},
			Sources: []linkSource{{
				Path: "CHANGELOG.markdown",
				Line: 375,
			}},
		})
		want := "failure: https://example.com/missing (404) [CHANGELOG.markdown:375]"
		if got != want {
			t.Fatalf("unexpected formatted result:\nwant %q\ngot  %q", want, got)
		}
	})
}

func TestWriteExternalFindingsWritesDiagnosticsToStderr(t *testing.T) {
	var stderr bytes.Buffer
	failed := writeExternalFindings(&stderr, []probeResult{{
		URL:    "https://example.com/fail",
		Status: 500,
		Hops: []probeHop{
			{URL: "https://example.com/fail", Status: 500},
		},
	}}, []probeResult{{
		URL:    "https://example.com/old",
		Status: 200,
		Hops: []probeHop{
			{URL: "https://example.com/old", Status: 302},
			{URL: "https://example.com/new", Status: 200},
		},
		Sources: []linkSource{{
			Path: "docs/example.markdown",
			Line: 8,
		}},
	}})

	if !failed {
		t.Fatal("expected failures to produce a failing result")
	}

	output := strings.TrimSpace(stderr.String())
	if !strings.Contains(output, "warning: https://example.com/old (302) -> https://example.com/new (200) [docs/example.markdown:8]") {
		t.Fatalf("expected warning in stderr, got %q", output)
	}
	if !strings.Contains(output, "failure: https://example.com/fail (500) [source unknown]") {
		t.Fatalf("expected failure in stderr, got %q", output)
	}
}

func writeTestFile(t *testing.T, relativePath string, contents string) {
	t.Helper()

	fullPath := filepath.Join(root, filepath.FromSlash(relativePath))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o750); err != nil {
		t.Fatalf("create parent directory: %v", err)
	}
	if err := os.WriteFile(fullPath, []byte(contents), 0o600); err != nil {
		t.Fatalf("write test file: %v", err)
	}
}
