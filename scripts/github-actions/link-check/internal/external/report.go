package external

import (
	"fmt"
	"io"

	"github.com/favonia/cloudflare-ddns/scripts/github-actions/link-check/internal/githubactions"
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
	sections := make([]githubactions.SummarySection, 0, 2)
	if len(failures) > 0 {
		sections = append(sections, summarySectionForResults("Failures", failures))
	}
	if len(warnings) > 0 {
		sections = append(sections, summarySectionForResults("Warnings", warnings))
	}
	githubactions.WriteSummaryFromEnv(output, sections)

	annotationCapacity := 0
	for _, failure := range failures {
		annotationCapacity += max(1, len(failure.Sources))
	}
	for _, warning := range warnings {
		annotationCapacity += max(1, len(warning.Sources))
	}
	annotations := make([]githubactions.Annotation, 0, annotationCapacity)
	for _, failure := range failures {
		annotations = append(annotations, annotationsForResult("error", failure)...)
	}
	for _, warning := range warnings {
		annotations = append(annotations, annotationsForResult("warning", warning)...)
	}
	githubactions.WriteAnnotations(output, annotations)
}

func summarySectionForResults(heading string, results []probeResult) githubactions.SummarySection {
	items := make([]string, 0, len(results))
	for _, result := range results {
		items = append(items, formatResultInMarkdown(result))
	}
	return githubactions.SummarySection{
		Heading: fmt.Sprintf("%s (%d)", heading, len(results)),
		Items:   items,
	}
}

func annotationsForResult(level string, result probeResult) []githubactions.Annotation {
	message := formatProbeChain(result)
	if len(result.Sources) == 0 {
		return []githubactions.Annotation{{
			Level:   level,
			Message: message,
		}}
	}
	annotations := make([]githubactions.Annotation, 0, len(result.Sources))
	for _, source := range result.Sources {
		annotations = append(annotations, githubactions.Annotation{
			Level:   level,
			File:    source.Path,
			Line:    source.Line,
			Message: message,
		})
	}
	return annotations
}
