package domainexp

// This file holds the operator-facing diagnostics for the parse-and-report
// parsers (ParseList in list.go and ParseExpression in expression.go), which
// emit messages directly through a pp.PP as they parse.

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/syntax"
)

type domainContext uint8

const (
	domainList domainContext = iota
	domainIs
	domainSub
)

type dotTrimmingOccurrence struct {
	context   domainContext
	source    string
	effective string
}

func (context domainContext) describeSource(source string) string {
	switch context {
	case domainList:
		return fmt.Sprintf("%q", source)
	case domainIs:
		return fmt.Sprintf("is(%q)", source)
	case domainSub:
		return fmt.Sprintf("sub(%q)", source)
	}
	panic("domainexp: unknown domain context")
}

func (context domainContext) describeCorrection(effective string) string {
	switch context {
	case domainList:
		return effective
	case domainIs:
		return fmt.Sprintf("is(%s)", effective)
	case domainSub:
		return fmt.Sprintf("sub(%s)", effective)
	}
	panic("domainexp: unknown domain context")
}

func dotTrimmingMessage(key string, context domainContext, source, effective string) string {
	return fmt.Sprintf(`%s accepts %s for now; use %s instead because version 2.0.0 will reject the current spelling`,
		key, context.describeSource(source), context.describeCorrection(effective))
}

func emptyInteriorLabelMessage(key string, context domainContext, source string) string {
	return fmt.Sprintf(`%s has consecutive dots in %s; replace each run with a single dot`,
		key, context.describeSource(source))
}

type parserState struct {
	// Empty-call functions are kept in first-occurrence order and deduplicated.
	emptyCallFunctions []string
	// Extra-comma diagnostics are intentionally deduplicated across one parse.
	extraComma bool
	// Missing-comma diagnostics are intentionally deduplicated across one parse.
	missingComma bool
	// Short is(...) targets (domain.ErrTooFewLabels) kept and reported once, in
	// first-occurrence order, deduplicated.
	shortIsTargets []string
	// sub(...) wildcard arguments skipped and reported, deduplicated (the L1
	// advisory).
	subWildcards []domain.Domain
	// Dot trimming is reported per occurrence, in parse encounter order.
	dotTrimmingOccurrences []dotTrimmingOccurrence
}

// listSyntaxPreview formats potentially long list syntax for advisory messages.
func listSyntaxPreview(input string) string {
	return pp.QuotePreviewOrEmptyLabel(input, pp.AdvisoryPreviewLimit, "empty")
}

func (state *parserState) recordExtraComma() {
	state.extraComma = true
}

func (state *parserState) recordEmptyCall(function string) {
	if slices.Contains(state.emptyCallFunctions, function) {
		return
	}
	state.emptyCallFunctions = append(state.emptyCallFunctions, function)
}

func (state *parserState) recordMissingComma() {
	state.missingComma = true
}

func (state *parserState) recordShortIsTarget(target string) {
	if slices.Contains(state.shortIsTargets, target) {
		return
	}
	state.shortIsTargets = append(state.shortIsTargets, target)
}

func (state *parserState) recordSubWildcard(w domain.Domain) {
	for _, existing := range state.subWildcards {
		if existing.String() == w.String() {
			return
		}
	}
	state.subWildcards = append(state.subWildcards, w)
}

func (state *parserState) recordDotTrimming(
	context domainContext,
	source string,
	effective string,
	dotTrimming domain.DotTrimming,
) {
	if dotTrimming == (domain.DotTrimming{
		RemovedLeadingDots:       false,
		RemovedExtraTrailingDots: false,
	}) {
		return
	}
	state.dotTrimmingOccurrences = append(state.dotTrimmingOccurrences, dotTrimmingOccurrence{
		context: context, source: source, effective: effective,
	})
}

// reportListDiagnostics emits the compatibility warnings accumulated while flattening a domain list.
func reportListDiagnostics(ppfmt pp.PP, key string, input string, state *parserState) {
	for _, entry := range state.dotTrimmingOccurrences {
		ppfmt.Noticef(pp.EmojiUserWarning,
			"%s", dotTrimmingMessage(key, entry.context, entry.source, entry.effective))
	}
	if state.extraComma {
		ppfmt.Noticef(pp.EmojiUserWarning,
			"%s (%s) contains extra commas; this is accepted for now but will be rejected in version 2.0.0",
			key, listSyntaxPreview(input))
	}
	if state.missingComma {
		ppfmt.Noticef(pp.EmojiUserWarning,
			"%s (%s) is missing commas; this is accepted for now but will be rejected in version 2.0.0",
			key, listSyntaxPreview(input))
	}
}

// reportParseError translates a generic domain-list parse failure into an operator message.
func reportParseError(ppfmt pp.PP, key string, input string, err *syntax.ParseError) {
	if errors.Is(err, syntax.ErrUnexpectedToken) {
		ppfmt.Noticef(pp.EmojiUserError, `%s (%q) has unexpected token %q when "," is expected`,
			key, input, input[err.Span.Start:err.Span.End])
		return
	}
	ppfmt.Noticef(pp.EmojiUserError, "%s (%q) is malformed: %v", key, input, err.Cause)
}

// reportExpressionDiagnostics emits call and compatibility-list warnings in their intended message order.
func reportExpressionDiagnostics(ppfmt pp.PP, key string, input string, state *parserState) {
	switch len(state.emptyCallFunctions) {
	case 0:
	case 1:
		ppfmt.Noticef(pp.EmojiUserWarning,
			`%s (%s) uses %s() with an empty domain list, which always evaluates to false`,
			key, listSyntaxPreview(input), state.emptyCallFunctions[0])
	default:
		functions := pp.EnglishJoinMapOrEmptyLabel(
			func(function string) string { return function + "()" },
			state.emptyCallFunctions,
			"",
		)
		ppfmt.Noticef(pp.EmojiUserWarning,
			`%s (%s) uses %s with empty domain lists, which always evaluate to false`,
			key, listSyntaxPreview(input), functions)
	}
	if len(state.shortIsTargets) > 0 {
		targets := pp.EnglishJoinMapOrEmptyLabel(
			func(t string) string { return t }, state.shortIsTargets, "")
		ppfmt.Noticef(pp.EmojiUserWarning,
			`%s (%s) has the domain %q in is(...), which is too short to be a reasonable target `+
				`domain name; this is accepted but probably not what you want — a target domain `+
				`name should look like "*.example.org" or "sub.example.org"`,
			key, listSyntaxPreview(input), targets)
	}
	for _, w := range state.subWildcards {
		ws := w.String()                       // canonical "*.X"
		parent := strings.TrimPrefix(ws, "*.") // subdomain form sub(X)
		if parent == ws {
			// Bare star: there is no parent domain after the "*", so neither
			// is(...) nor sub(...) remediation applies. State the fact only.
			ppfmt.Noticef(pp.EmojiUserWarning,
				`%s (%s) has sub(%s), which matches no domain`,
				key, listSyntaxPreview(input), ws)
			continue
		}
		ppfmt.Noticef(pp.EmojiUserWarning,
			`%s (%s) has sub(%s), which matches no domain; use is(%s) to match the wildcard `+
				`record itself, or sub(%s) to match subdomains of %s`,
			key, listSyntaxPreview(input), ws, ws, parent, parent)
	}
	for _, entry := range state.dotTrimmingOccurrences {
		ppfmt.Noticef(pp.EmojiUserWarning,
			"%s", dotTrimmingMessage(key, entry.context, entry.source, entry.effective))
	}
	if state.extraComma {
		ppfmt.Noticef(
			pp.EmojiUserWarning,
			"%s (%s) contains extra commas inside is(...) or sub(...); "+
				"this is accepted for now but will be rejected in version 2.0.0",
			key, listSyntaxPreview(input),
		)
	}
	if state.missingComma {
		ppfmt.Noticef(
			pp.EmojiUserWarning,
			"%s (%s) is missing commas inside is(...) or sub(...); "+
				"this is accepted for now but will be rejected in version 2.0.0",
			key, listSyntaxPreview(input),
		)
	}
}

// reportExpressionError translates a classified expression failure into an operator message.
func reportExpressionError(ppfmt pp.PP, key string, input string, err *syntax.ParseError) {
	expectedToken, expectedTokenOK := errors.AsType[*syntax.ExpectedTokenError](err)
	missingToken, missingTokenOK := errors.AsType[*syntax.MissingTokenError](err)
	invalidDomain, invalidDomainOK := errors.AsType[*invalidDomainError](err)
	switch {
	case errors.Is(err, errNotBooleanExpression):
		ppfmt.Noticef(pp.EmojiUserError, "%s (%q) is not a boolean expression", key, input)
	case errors.Is(err, errUnexpectedBooleanToken):
		ppfmt.Noticef(
			pp.EmojiUserError,
			"%s (%q) is not a boolean expression: got unexpected token %q",
			key, input, input[err.Span.Start:err.Span.End],
		)
	case expectedTokenOK:
		ppfmt.Noticef(pp.EmojiUserError,
			`%s (%q) has unexpected token %q when %q is expected`,
			key, input, expectedToken.Got, expectedToken.Expected)
	case missingTokenOK:
		ppfmt.Noticef(pp.EmojiUserError, `%s (%q) is missing %q at the end`, key, input, missingToken.Expected)
	case errors.Is(err, syntax.ErrUnexpectedToken):
		ppfmt.Noticef(pp.EmojiUserError, `%s (%q) has unexpected token %q`, key, input, input[err.Span.Start:err.Span.End])
	case invalidDomainOK:
		if errors.Is(invalidDomain.cause, domain.ErrEmptyInteriorLabel) {
			source := input[err.Span.Start:err.Span.End]
			ppfmt.Noticef(pp.EmojiUserError,
				"%s", emptyInteriorLabelMessage(key, invalidDomain.context, source))
			return
		}
		ppfmt.Noticef(pp.EmojiUserError,
			`%s (%q) has the domain %q in is(...) or sub(...), but it is malformed: %v`,
			key, input, invalidDomain.domain, invalidDomain.cause)
	default:
		ppfmt.Noticef(pp.EmojiUserError, "%s (%q) is malformed: %v", key, input, err.Cause)
	}
}
