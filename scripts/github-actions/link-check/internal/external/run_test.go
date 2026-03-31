package external

import (
	"bytes"
	"slices"
	"strings"
	"testing"

	"github.com/favonia/cloudflare-ddns/scripts/github-actions/link-check/internal/extract"
	"github.com/favonia/cloudflare-ddns/scripts/github-actions/link-check/internal/testutil"
)

func TestCollectURLsKeepsSourceLocations(t *testing.T) {
	root := t.TempDir()
	testutil.WriteFile(t, root, "docs/example.markdown", strings.Join([]string{
		"See [issue](https://github.com/favonia/cloudflare-ddns/issues/102).",
		"",
		"Another mention: https://github.com/favonia/cloudflare-ddns/issues/102",
		"[ref]: https://example.com/ref",
		"Use [reference][ref].",
	}, "\n"))

	urls := collectURLs(root, []string{"docs/example.markdown"}, nil, nil)

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
		if !slices.Contains(cfg.Links.TargetURLs.IgnoreExact, url) {
			t.Fatalf("expected default config to ignore %q", url)
		}
	}
}

func TestDefaultConfigHasNoOperationalWarningURLPatterns(t *testing.T) {
	cfg := defaultConfig()

	if len(cfg.Probe.WarningURLPatterns) != 0 {
		t.Fatalf(
			"expected operational endpoints to be ignored instead of downgraded, got %#v",
			cfg.Probe.WarningURLPatterns,
		)
	}
}

func TestDefaultConfigExcludesItsOwnFileFromExternalScan(t *testing.T) {
	cfg := defaultConfig()

	if !slices.Contains(cfg.Links.SourceFiles.ExcludeGlobs, "scripts/github-actions/link-check/internal/external/config.go") {
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

func TestShouldSuppressWarningUsesPolicyHooks(t *testing.T) {
	result := probeResult{
		URL: "https://example.com/docs",
		Hops: []probeHop{
			{URL: "https://example.com/docs", Status: 301},
			{URL: "https://example.com/docs/", Status: 200},
		},
	}

	if !shouldSuppressWarning(result) {
		t.Fatal("expected policy hook to suppress trailing-slash redirect warning")
	}
}

func TestFormatResultUsesClearCategories(t *testing.T) {
	t.Run("network error", func(t *testing.T) {
		got := formatResult("warning", probeResult{
			URL:  "https://api6.ipify.org",
			Err:  "dial tcp [2607:f2d8:1:3c::4]:443: connect: network is unreachable",
			Hops: []probeHop{{URL: "https://api6.ipify.org"}},
			Sources: []extract.LinkSource{{
				Path: "docs/example.markdown",
				Line: 12,
			}},
		})
		want := "warning: network error: https://api6.ipify.org " +
			"(network error: dial tcp [2607:f2d8:1:3c::4]:443: connect: network is unreachable) " +
			"[docs/example.markdown:12]"
		if got != want {
			t.Fatalf("unexpected formatted result:\nwant %q\ngot  %q", want, got)
		}
	})

	t.Run("redirect", func(t *testing.T) {
		got := formatResult("warning", probeResult{
			URL:    "https://containrrr.dev/shoutrrr",
			Status: 200,
			Hops: []probeHop{
				{URL: "https://containrrr.dev/shoutrrr", Status: 301},
				{URL: "https://containrrr.dev/shoutrrr/", Status: 200},
			},
			Sources: []extract.LinkSource{{
				Path: "README.markdown",
				Line: 7,
			}},
		})
		want := "warning: https://containrrr.dev/shoutrrr (301 Moved Permanently) -> https://containrrr.dev/shoutrrr/ (200 OK) [README.markdown:7]"
		if got != want {
			t.Fatalf("unexpected formatted result:\nwant %q\ngot  %q", want, got)
		}
	})

	t.Run("status", func(t *testing.T) {
		got := formatResult("failure", probeResult{
			URL:    "https://example.com/missing",
			Status: 404,
			Hops: []probeHop{
				{URL: "https://example.com/missing", Status: 404},
			},
			Sources: []extract.LinkSource{{
				Path: "CHANGELOG.markdown",
				Line: 375,
			}},
		})
		want := "failure: https://example.com/missing (404 Not Found) [CHANGELOG.markdown:375]"
		if got != want {
			t.Fatalf("unexpected formatted result:\nwant %q\ngot  %q", want, got)
		}
	})
}

func TestWriteFindingsWritesDiagnosticsToStderr(t *testing.T) {
	var stderr bytes.Buffer
	failed := writeFindings(&stderr, []probeResult{{
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
		Sources: []extract.LinkSource{{
			Path: "docs/example.markdown",
			Line: 8,
		}},
	}})

	if !failed {
		t.Fatal("expected failures to produce a failing result")
	}

	output := strings.TrimSpace(stderr.String())
	if !strings.Contains(
		output,
		"warning: https://example.com/old (302 Found) -> https://example.com/new (200 OK) [docs/example.markdown:8]",
	) {
		t.Fatalf("expected warning in stderr, got %q", output)
	}
	if !strings.Contains(output, "failure: https://example.com/fail (500 Internal Server Error) [source unknown]") {
		t.Fatalf("expected failure in stderr, got %q", output)
	}
}
