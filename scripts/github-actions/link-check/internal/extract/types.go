package extract

import (
	"fmt"
	"strings"
)

// Issue describes one local validation finding.
type Issue struct {
	// Kind identifies the check that emitted the finding.
	Kind string
	// Path identifies the tracked file where the finding was reported.
	Path string
	// Detail provides the check-specific explanation for the finding.
	Detail string
}

// Render formats the finding for operator-facing diagnostics.
func (issue Issue) Render() string {
	return fmt.Sprintf("%s: %s: %s", issue.Kind, issue.Path, issue.Detail)
}

// LinkSource identifies one occurrence of an extracted link in a tracked file.
type LinkSource struct {
	// Path is the tracked repository path containing the link occurrence.
	Path string
	// Line is the 1-based line number of the link occurrence.
	Line int
}

// Render formats the source as path:line.
func (source LinkSource) Render() string {
	return fmt.Sprintf("%s:%d", source.Path, source.Line)
}

// RenderInMarkdown formats the source as a Markdown link with line number.
func (source LinkSource) RenderInMarkdown() string {
	return fmt.Sprintf("[%s](%s):%d", source.Path, source.Path, source.Line)
}

// Compare orders sources by path and then by line number.
func (source LinkSource) Compare(other LinkSource) int {
	if diff := strings.Compare(source.Path, other.Path); diff != 0 {
		return diff
	}
	return source.Line - other.Line
}

// ExternalLink is one unique external URL plus all tracked-file occurrences
// that referenced it.
type ExternalLink struct {
	// URL is the extracted external URL string.
	URL string
	// Sources are the tracked-file locations where the URL appeared.
	Sources []LinkSource
}

// LocalReference is one extracted Markdown target that should be interpreted
// as a repository-local reference by the local checker.
type LocalReference struct {
	// Target is the extracted Markdown target string.
	Target string
	// Line is the 1-based line number where the target begins.
	Line int
}

// HTTPLinkTarget is one extracted HTTP or HTTPS target plus its source line.
type HTTPLinkTarget struct {
	// URL is the extracted HTTP or HTTPS target string.
	URL string
	// Line is the 1-based line number where the target begins.
	Line int
}

// FileLinks groups the classified data extracted from one tracked file.
type FileLinks struct {
	// LocalReferences are extracted targets that should be interpreted as local
	// references by the local checker.
	LocalReferences []LocalReference
	// HTTPLinks are extracted HTTP or HTTPS targets.
	HTTPLinks []HTTPLinkTarget
	// CommentBlocks are comment-text blocks that participate in repository-path
	// checks for the file type.
	CommentBlocks []string
}

// LocalTargetInfo describes the local-link destinations exposed by one file.
type LocalTargetInfo struct {
	// SupportsFragments reports whether fragment validation is defined for this
	// file type.
	SupportsFragments bool
	// Fragments contains the valid fragment destinations exposed by the file.
	Fragments map[string]bool
}

type rawTarget struct {
	Target string
	Line   int
}
