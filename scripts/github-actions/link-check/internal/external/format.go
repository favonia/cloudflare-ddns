package external

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/favonia/cloudflare-ddns/scripts/github-actions/link-check/internal/extract"
	"github.com/favonia/cloudflare-ddns/scripts/github-actions/link-check/internal/githubactions"
)

// probeRenderer parameterizes how probe results are formatted so the same
// structural logic serves both plain-text diagnostics and Markdown summaries.
type probeRenderer struct {
	renderSource   func(extract.LinkSource) string
	noSource       string
	formatHop      func(url string, status int) string
	formatTerminal func(url string, status int) string
	formatError    func(url, err string) string
	chainSep       string
	combineResult  func(chain, sources string, isNetworkError bool) string
}

var plainRenderer = probeRenderer{
	renderSource: extract.LinkSource.Render,
	noSource:     "source unknown",
	formatHop: func(u string, s int) string {
		return fmt.Sprintf("%s (%d %s)", u, s, http.StatusText(s))
	},
	formatTerminal: func(u string, s int) string {
		return fmt.Sprintf("%s (HTTP %d %s)", u, s, http.StatusText(s))
	},
	formatError: func(u, e string) string {
		return fmt.Sprintf("%s (network error: %s)", u, e)
	},
	chainSep: " -> ",
	combineResult: func(chain, sources string, isErr bool) string {
		if isErr {
			return fmt.Sprintf("network error: %s [%s]", chain, sources)
		}
		return fmt.Sprintf("%s [%s]", chain, sources)
	},
}

var markdownRenderer = probeRenderer{
	renderSource: func(source extract.LinkSource) string {
		return fmt.Sprintf(
			"%s:%d",
			githubactions.SummaryLink(source.Path, source.Path),
			source.Line,
		)
	},
	noSource: "*source unknown*",
	formatHop: func(u string, s int) string {
		return fmt.Sprintf(
			"%s - %d %s",
			githubactions.SummaryLink(u, u),
			s,
			githubactions.EscapeSummaryText(http.StatusText(s)),
		)
	},
	formatTerminal: func(u string, s int) string {
		return fmt.Sprintf(
			"%s - %d %s",
			githubactions.SummaryLink(u, u),
			s,
			githubactions.EscapeSummaryText(http.StatusText(s)),
		)
	},
	formatError: func(u, e string) string {
		return fmt.Sprintf(
			"%s - network error: %s",
			githubactions.SummaryLink(u, u),
			githubactions.EscapeSummaryText(e),
		)
	},
	chainSep: " → ",
	combineResult: func(chain, sources string, isErr bool) string {
		if isErr {
			return fmt.Sprintf("**network error:** %s\n  - Sources: %s",
				chain, sources)
		}
		return fmt.Sprintf("%s\n  - Sources: %s", chain, sources)
	},
}

func (r probeRenderer) formatChain(result probeResult) string {
	if len(result.Hops) == 0 {
		if result.Err != "" {
			return r.formatError(result.URL, result.Err)
		}
		return r.formatTerminal(result.URL, result.Status)
	}
	parts := make([]string, 0, len(result.Hops))
	last := len(result.Hops) - 1
	for i, hop := range result.Hops {
		if result.Err != "" && i == last && hop.Status == 0 {
			parts = append(parts, r.formatError(hop.URL, result.Err))
			continue
		}
		parts = append(parts, r.formatHop(hop.URL, hop.Status))
	}
	return strings.Join(parts, r.chainSep)
}

func (r probeRenderer) formatSources(sources []extract.LinkSource) string {
	if len(sources) == 0 {
		return r.noSource
	}
	rendered := make([]string, 0, len(sources))
	for _, source := range sources {
		rendered = append(rendered, r.renderSource(source))
	}
	return strings.Join(rendered, ", ")
}

func (r probeRenderer) formatResultFull(result probeResult) string {
	return r.combineResult(
		r.formatChain(result),
		r.formatSources(result.Sources),
		result.Err != "",
	)
}

// formatResult renders one probe outcome for operator-facing diagnostics.
func formatResult(result probeResult) string {
	return plainRenderer.formatResultFull(result)
}

// formatProbeChain renders the observed redirect chain or terminal probe
// result in plain text.
func formatProbeChain(result probeResult) string {
	return plainRenderer.formatChain(result)
}

// formatResultInMarkdown renders one probe outcome in Markdown for the GitHub
// Actions step summary.
func formatResultInMarkdown(result probeResult) string {
	return markdownRenderer.formatResultFull(result)
}
