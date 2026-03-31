package local

import (
	"fmt"
	"io"

	"github.com/favonia/cloudflare-ddns/scripts/github-actions/link-check/internal/extract"
	"github.com/favonia/cloudflare-ddns/scripts/github-actions/link-check/internal/githubactions"
)

// writeIssuesForGithub writes a GitHub Actions step summary and workflow
// command annotations so local-link findings surface in the GitHub UI.
func writeIssuesForGithub(output io.Writer, issues []extract.Issue) {
	githubactions.WriteSummaryFromEnv(output, []githubactions.SummarySection{{
		Heading: fmt.Sprintf("Local Link Issues (%d)", len(issues)),
		Items:   issueSummaryItems(issues),
	}})
	githubactions.WriteAnnotations(output, issueAnnotations(issues))
}

func issueSummaryItems(issues []extract.Issue) []string {
	items := make([]string, 0, len(issues))
	for _, issue := range issues {
		items = append(items, formatIssueSummary(issue))
	}
	return items
}

func issueAnnotations(issues []extract.Issue) []githubactions.Annotation {
	annotations := make([]githubactions.Annotation, 0, len(issues))
	for _, issue := range issues {
		annotations = append(annotations, githubactions.Annotation{
			Level:   "error",
			File:    issue.Path,
			Message: fmt.Sprintf("%s: %s", issue.Kind, issue.Detail),
		})
	}
	return annotations
}
