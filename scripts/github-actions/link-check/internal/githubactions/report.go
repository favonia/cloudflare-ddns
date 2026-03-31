package githubactions

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// SummarySection is one titled block in a GitHub Actions step summary.
type SummarySection struct {
	Heading string
	Items   []string
}

// Annotation is one GitHub Actions workflow command annotation.
type Annotation struct {
	Level   string
	File    string
	Line    int
	Message string
}

// WriteSummaryFromEnv appends the provided summary sections to the GitHub step
// summary file when GITHUB_STEP_SUMMARY is available.
func WriteSummaryFromEnv(logOutput io.Writer, sections []SummarySection) {
	githubSummaryFilepath := strings.TrimSpace(os.Getenv("GITHUB_STEP_SUMMARY"))
	if githubSummaryFilepath == "" {
		return
	}

	//nolint:gosec // path from trusted GITHUB_STEP_SUMMARY env var
	githubSummaryFile, err := os.OpenFile(
		githubSummaryFilepath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o666)
	if err != nil {
		_, _ = fmt.Fprintln(logOutput, err)
		return
	}
	defer githubSummaryFile.Close()

	writeSummary(githubSummaryFile, sections)
}

func writeSummary(output io.Writer, sections []SummarySection) {
	for _, section := range sections {
		_, _ = fmt.Fprintf(output, "# %s\n", section.Heading)
		for _, item := range section.Items {
			_, _ = fmt.Fprintf(output, "- %s\n", item)
		}
		_, _ = fmt.Fprintln(output)
	}
}

// WriteAnnotations emits GitHub Actions workflow command annotations so
// findings surface in the PR diff view and workflow logs.
func WriteAnnotations(output io.Writer, annotations []Annotation) {
	for _, annotation := range annotations {
		writeAnnotation(output, annotation)
	}
}

func writeAnnotation(output io.Writer, annotation Annotation) {
	if annotation.File == "" {
		_, _ = fmt.Fprintf(
			output,
			"::%s::%s\n",
			annotation.Level,
			EscapeWorkflowCommandMessage(annotation.Message),
		)
		return
	}
	if annotation.Line > 0 {
		_, _ = fmt.Fprintf(
			output,
			"::%s file=%s,line=%d::%s\n",
			annotation.Level,
			EscapeWorkflowCommandProperty(annotation.File),
			annotation.Line,
			EscapeWorkflowCommandMessage(annotation.Message),
		)
		return
	}
	_, _ = fmt.Fprintf(
		output,
		"::%s file=%s::%s\n",
		annotation.Level,
		EscapeWorkflowCommandProperty(annotation.File),
		EscapeWorkflowCommandMessage(annotation.Message),
	)
}
