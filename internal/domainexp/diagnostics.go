package domainexp

// This file holds the operator-facing diagnostics for the parse-and-report
// parsers (ParseList in list.go and ParseExpression in expression.go), which
// emit messages directly through a pp.PP as they parse.

import (
	"errors"
	"slices"

	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/syntax"
)

type parserState struct {
	// Empty-call functions are kept in first-occurrence order and deduplicated.
	emptyCallFunctions []string
	// Extra-comma diagnostics are intentionally deduplicated across one parse.
	extraComma bool
	// Missing-comma diagnostics are intentionally deduplicated across one parse.
	missingComma bool
	// Short is(...) targets (domain.ErrTooFewLabels) kept and reported once, in
	// first-occurrence order, deduplicated (#1).
	shortIsTargets []string
	// sub(...) wildcard arguments skipped and reported, deduplicated (#2/L1).
	subWildcards []domain.Domain
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

// reportListDiagnostics emits the compatibility warnings accumulated while flattening a domain list.
func reportListDiagnostics(ppfmt pp.PP, key string, input string, state *parserState) {
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
			`%s (%q) uses %s() with an empty domain list, which always evaluates to false`,
			key, input, state.emptyCallFunctions[0])
	default:
		functions := pp.EnglishJoinMapOrEmptyLabel(
			func(function string) string { return function + "()" },
			state.emptyCallFunctions,
			"",
		)
		ppfmt.Noticef(pp.EmojiUserWarning,
			`%s (%q) uses %s with empty domain lists, which always evaluate to false`,
			key, input, functions)
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
	default:
		ppfmt.Noticef(pp.EmojiUserError, "%s (%q) is malformed: %v", key, input, err.Cause)
	}
}
