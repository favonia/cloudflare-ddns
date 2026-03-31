package githubactions

import (
	"fmt"
	"html"
	"strings"
)

// EscapeWorkflowCommandMessage escapes data written after the second :: in a
// GitHub Actions workflow command.
func EscapeWorkflowCommandMessage(message string) string {
	replacer := strings.NewReplacer(
		"%", "%25",
		"\r", "%0D",
		"\n", "%0A",
	)
	return replacer.Replace(message)
}

// EscapeWorkflowCommandProperty escapes a workflow command property value such
// as a file path.
func EscapeWorkflowCommandProperty(value string) string {
	replacer := strings.NewReplacer(
		"%", "%25",
		"\r", "%0D",
		"\n", "%0A",
		":", "%3A",
		",", "%2C",
	)
	return replacer.Replace(value)
}

// EscapeSummaryText escapes user-controlled text for safe single-line HTML
// rendering in a GitHub step summary.
func EscapeSummaryText(text string) string {
	return html.EscapeString(singleLine(text))
}

// SummaryCode renders a single-line string as HTML code in a GitHub step
// summary.
func SummaryCode(text string) string {
	return fmt.Sprintf("<code>%s</code>", EscapeSummaryText(text))
}

// SummaryLink renders a single-line hyperlink in a GitHub step summary.
func SummaryLink(target, label string) string {
	return fmt.Sprintf(
		`<a href="%s">%s</a>`,
		EscapeSummaryText(target),
		EscapeSummaryText(label),
	)
}

func singleLine(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = strings.ReplaceAll(text, "\n", " ")
	return text
}
