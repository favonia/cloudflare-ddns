package local

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/favonia/cloudflare-ddns/scripts/github-actions/link-check/internal/extract"
)

func TestWriteIssuesForGithubEscapesSummaryAndAnnotations(t *testing.T) {
	summaryPath := filepath.Join(t.TempDir(), "summary.md")
	t.Setenv("GITHUB_STEP_SUMMARY", summaryPath)

	var output bytes.Buffer
	writeIssuesForGithub(&output, []extract.Issue{{
		Kind:   "broken**kind\r\nnext",
		Path:   "docs/a,b:c%file.markdown",
		Detail: "bad <detail>\nnext line",
	}})

	summaryBytes, err := os.ReadFile(summaryPath)
	if err != nil {
		t.Fatalf("read summary: %v", err)
	}
	summary := string(summaryBytes)
	wantSummary := "# Local Link Issues (1)\n" +
		"- <strong>broken**kind next</strong>: <code>docs/a,b:c%file.markdown</code> - bad &lt;detail&gt; next line\n\n"
	if summary != wantSummary {
		t.Fatalf("unexpected summary:\nwant %q\ngot  %q", wantSummary, summary)
	}

	wantAnnotation := "::error file=docs/a%2Cb%3Ac%25file.markdown::broken**kind%0D%0Anext: bad <detail>%0Anext line\n"
	if output.String() != wantAnnotation {
		t.Fatalf("unexpected annotation:\nwant %q\ngot  %q", wantAnnotation, output.String())
	}
}
