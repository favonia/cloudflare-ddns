package local

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/favonia/cloudflare-ddns/scripts/github-actions/link-check/internal/extract"
)

// writeIssuesForGithub writes a GitHub Actions step summary and workflow
// command annotations so local-link findings surface in the GitHub UI.
func writeIssuesForGithub(output io.Writer, issues []extract.Issue) {
	githubSummaryFilepath := strings.TrimSpace(os.Getenv("GITHUB_STEP_SUMMARY"))
	if githubSummaryFilepath == "" {
		return
	}

	//nolint:gosec // path from trusted GITHUB_STEP_SUMMARY env var
	githubSummaryFile, err := os.OpenFile(
		githubSummaryFilepath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o666)
	if err != nil {
		_, _ = fmt.Fprintln(output, err)
		return
	}
	defer githubSummaryFile.Close()

	_, _ = fmt.Fprintf(githubSummaryFile, "# Local Link Issues (%d)\n", len(issues))
	for _, issue := range issues {
		_, _ = fmt.Fprintf(githubSummaryFile, "- **%s**: `%s` — %s\n", issue.Kind, issue.Path, issue.Detail)
	}

	for _, issue := range issues {
		_, _ = fmt.Fprintf(output, "::error file=%s::%s: %s\n", issue.Path, issue.Kind, issue.Detail)
	}
}
