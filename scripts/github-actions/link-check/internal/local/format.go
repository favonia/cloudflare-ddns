package local

import (
	"fmt"

	"github.com/favonia/cloudflare-ddns/scripts/github-actions/link-check/internal/extract"
	"github.com/favonia/cloudflare-ddns/scripts/github-actions/link-check/internal/githubactions"
)

// formatIssue renders one local-link finding for operator-facing diagnostics.
func formatIssue(issue extract.Issue) string {
	return issue.Render()
}

// formatIssueSummary renders one local-link finding for the GitHub Actions step
// summary.
func formatIssueSummary(issue extract.Issue) string {
	return fmt.Sprintf(
		"<strong>%s</strong>: %s - %s",
		githubactions.EscapeSummaryText(issue.Kind),
		githubactions.SummaryCode(issue.Path),
		githubactions.EscapeSummaryText(issue.Detail),
	)
}
