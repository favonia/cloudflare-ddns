package external

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/favonia/cloudflare-ddns/scripts/github-actions/link-check/internal/extract"
)

func TestWriteFindingsForGithubEscapesSummaryAndAnnotations(t *testing.T) {
	summaryPath := filepath.Join(t.TempDir(), "summary.md")
	t.Setenv("GITHUB_STEP_SUMMARY", summaryPath)

	var output bytes.Buffer
	writeFindingsForGithub(&output, []probeResult{{
		URL: "https://example.com/a?x=1&y=2",
		Err: "dial tcp: bad,\nnews",
		Hops: []probeHop{{
			URL: "https://example.com/a?x=1&y=2",
		}},
		Sources: []extract.LinkSource{{
			Path: "docs/a,b:c%file.markdown",
			Line: 7,
		}},
	}}, nil)

	summaryBytes, err := os.ReadFile(summaryPath)
	if err != nil {
		t.Fatalf("read summary: %v", err)
	}
	summary := string(summaryBytes)
	wantSummary := "# Failures (1)\n" +
		"- **network error:** <a href=\"https://example.com/a?x=1&amp;y=2\">https://example.com/a?x=1&amp;y=2</a> - network error: dial tcp: bad, news\n" +
		"  - Sources: <a href=\"docs/a,b:c%file.markdown\">docs/a,b:c%file.markdown</a>:7\n\n"
	if summary != wantSummary {
		t.Fatalf("unexpected summary:\nwant %q\ngot  %q", wantSummary, summary)
	}

	wantAnnotation := "::error file=docs/a%2Cb%3Ac%25file.markdown,line=7::https://example.com/a?x=1&y=2 (network error: dial tcp: bad,%0Anews)\n"
	if output.String() != wantAnnotation {
		t.Fatalf("unexpected annotation:\nwant %q\ngot  %q", wantAnnotation, output.String())
	}
}

func TestWriteFindingsForGithubEmitsAnnotationsWithoutSummaryFile(t *testing.T) {
	t.Setenv("GITHUB_STEP_SUMMARY", filepath.Clean(t.TempDir()))

	var output bytes.Buffer
	writeFindingsForGithub(&output, []probeResult{{
		URL:    "https://example.com/missing",
		Status: 404,
		Hops: []probeHop{{
			URL:    "https://example.com/missing",
			Status: 404,
		}},
		Sources: []extract.LinkSource{{
			Path: "docs/example.markdown",
			Line: 9,
		}},
	}}, nil)

	log := output.String()
	if !strings.Contains(log, "is a directory") {
		t.Fatalf("expected summary open failure in log output, got %q", log)
	}
	if !strings.Contains(log, "::error file=docs/example.markdown,line=9::https://example.com/missing (404 Not Found)\n") {
		t.Fatalf("expected annotation despite summary failure, got %q", log)
	}
}

func TestWriteFindingsForGithubEmitsLocationlessAnnotation(t *testing.T) {
	var output bytes.Buffer
	writeFindingsForGithub(&output, []probeResult{{
		URL:    "https://example.com/missing",
		Status: 404,
		Hops: []probeHop{{
			URL:    "https://example.com/missing",
			Status: 404,
		}},
	}}, nil)

	want := "::error::https://example.com/missing (404 Not Found)\n"
	if output.String() != want {
		t.Fatalf("unexpected locationless annotation:\nwant %q\ngot  %q", want, output.String())
	}
}
