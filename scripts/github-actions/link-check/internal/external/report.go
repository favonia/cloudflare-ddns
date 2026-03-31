package external

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// writeFindings writes warnings and failures in operator-facing diagnostic
// form and reports whether failures were present.
func writeFindings(stderr io.Writer, failures, warnings []probeResult) bool {
	for _, warning := range warnings {
		_, _ = fmt.Fprintln(stderr, "warning: "+formatResult(warning))
	}
	for _, failure := range failures {
		_, _ = fmt.Fprintln(stderr, "failure: "+formatResult(failure))
	}
	return len(failures) > 0
}

// writeFindingsForGithub writes a GitHub Actions step summary and workflow
// command annotations so findings surface in the GitHub UI.
func writeFindingsForGithub(output io.Writer, failures, warnings []probeResult) {
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

	if len(failures) > 0 {
		_, _ = fmt.Fprintf(githubSummaryFile, "# Failures (%d)\n", len(failures))
		for _, failure := range failures {
			_, _ = fmt.Fprintln(githubSummaryFile, "- "+formatResultInMarkdown(failure))
		}
		_, _ = fmt.Fprintln(githubSummaryFile)
	}
	if len(warnings) > 0 {
		_, _ = fmt.Fprintf(githubSummaryFile, "# Warnings (%d)\n", len(warnings))
		for _, warning := range warnings {
			_, _ = fmt.Fprintln(githubSummaryFile, "- "+formatResultInMarkdown(warning))
		}
		_, _ = fmt.Fprintln(githubSummaryFile)
	}

	for _, failure := range failures {
		writeGitHubAnnotations(output, "error", failure)
	}
	for _, warning := range warnings {
		writeGitHubAnnotations(output, "warning", warning)
	}
}

// writeGitHubAnnotations emits GitHub Actions workflow commands for one
// finding so it appears as an inline annotation in the PR diff view.
// See https://docs.github.com/actions/reference/workflow-commands-for-github-actions#setting-an-error-message
func writeGitHubAnnotations(output io.Writer, level string, result probeResult) {
	message := formatProbeChain(result)
	for _, source := range result.Sources {
		_, _ = fmt.Fprintf(output, "::%s file=%s:%d::%s\n", level, source.Path, source.Line, message)
	}
}
