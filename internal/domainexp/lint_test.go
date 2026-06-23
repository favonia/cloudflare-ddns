// vim: nowrap

package domainexp_test

import (
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/domainexp"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// lintExpr parses input (expecting success, no parse diagnostics) and lints it.
// key is kept explicit even though every case passes "PROXIED": both
// ParseExpression and LintExpression embed it in the operator-facing text the
// cases assert verbatim, so it is part of the contract, not hidden coupling.
//
//nolint:unparam // key is part of the keyed-message contract; see comment above.
func lintExpr(t *testing.T, ppfmt *mocks.MockPP, key, input string) {
	t.Helper()
	expr, ok := domainexp.ParseExpression(ppfmt, key, input)
	if !ok {
		t.Fatalf("ParseExpression(%q) failed unexpectedly", input)
	}
	domainexp.LintExpression(ppfmt, key, input, expr)
}

// expectWarnings sets one Noticef expectation per message, in any order.
func expectWarnings(ppfmt *mocks.MockPP, msgs ...string) {
	for _, msg := range msgs {
		ppfmt.EXPECT().Noticef(pp.EmojiUserWarning, "%s", msg)
	}
}

func TestLintExpressionClean(t *testing.T) {
	t.Parallel()
	for _, input := range []string{
		"is(a.org)",
		"is(a.org) || sub(a.org)",
		"!is(a.org)",
		"!is(a.org) && !sub(a.org)",
		"sub(a.org) && !sub(b.a.org)",
		// A multi-domain atom is opaque to the semantic pass: it cannot be a
		// single-domain literal, so no relation is inferred and nothing warns.
		"is(a.org, b.org) && sub(c.org)",
	} {
		t.Run(input, func(t *testing.T) {
			t.Parallel()
			mockCtrl := gomock.NewController(t)
			ppfmt := mocks.NewMockPP(mockCtrl)
			// No Noticef expectations: a clean expression must warn about nothing.
			lintExpr(t, ppfmt, "PROXIED", input)
		})
	}
}

func TestLintR1RedundantNegation(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		input string
		want  string
	}{
		"double-negation": {
			"!!is(a.org)",
			`PROXIED ("!!is(a.org)") negates a negation, which has no effect; "is(a.org)" means the same thing`,
		},
		"negated-true": {
			"!true",
			`PROXIED ("!true") negates a constant; "false" means the same thing`,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			mockCtrl := gomock.NewController(t)
			ppfmt := mocks.NewMockPP(mockCtrl)
			ppfmt.EXPECT().Noticef(pp.EmojiUserWarning, "%s", tc.want)
			lintExpr(t, ppfmt, "PROXIED", tc.input)
		})
	}
}

func TestLintR2ExclusionOnlyDisjunct(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		input string
		want  string
	}{
		"mixed": {
			"is(a.org) || !sub(b.org)",
			`PROXIED ("is(a.org) || !sub(b.org)") has an || branch "!sub(b.org)" with no included domain, only exclusions; it usually matches far more than intended`,
		},
		// A compound branch made only of exclusions also trips R2; the positive-
		// atom check recurses through the inner && to find no included domain.
		"compound-branch": {
			"is(a.org) || (!sub(b.org) && !sub(c.org))",
			`PROXIED ("is(a.org) || (!sub(b.org) && !sub(c.org))") has an || branch "!sub(b.org) && !sub(c.org)" with no included domain, only exclusions; it usually matches far more than intended`,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			mockCtrl := gomock.NewController(t)
			ppfmt := mocks.NewMockPP(mockCtrl)
			ppfmt.EXPECT().Noticef(pp.EmojiUserWarning, "%s", tc.want)
			lintExpr(t, ppfmt, "PROXIED", tc.input)
		})
	}
}

func TestLintR3Constant(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		input string
		want  []string
	}{
		"contradiction-same": {
			"is(a.org) && !is(a.org)",
			[]string{`PROXIED ("is(a.org) && !is(a.org)") can never match any domain`},
		},
		"contradiction-is-sub": {
			"is(a.org) && sub(a.org)",
			[]string{`PROXIED ("is(a.org) && sub(a.org)") can never match any domain`},
		},
		"contradiction-child-parent": {
			"sub(x.a.org) && !sub(a.org)",
			[]string{`PROXIED ("sub(x.a.org) && !sub(a.org)") can never match any domain`},
		},
		// The negated literal precedes the positive one, so contradictory's full
		// i != j loop is required: a triangular loop would miss this.
		"contradiction-negated-first": {
			"!sub(a.org) && sub(x.a.org)",
			[]string{`PROXIED ("!sub(a.org) && sub(x.a.org)") can never match any domain`},
		},
		// A false constant in a conjunction makes the whole expression false.
		"constant-false-conjunct": {
			"is(a.org) && false",
			[]string{`PROXIED ("is(a.org) && false") can never match any domain`},
		},
		// A true constant in a disjunction makes the whole expression true. The
		// true branch carries no atom, so R2 does not also fire.
		"constant-true-disjunct": {
			"is(a.org) || true",
			[]string{`PROXIED ("is(a.org) || true") always matches every domain`},
		},
		// Through the public path a tautology also trips R2: the !is(a.org) branch
		// has no included domain. Both warnings are correct and both are emitted.
		"tautology": {
			"is(a.org) || !is(a.org)",
			[]string{
				`PROXIED ("is(a.org) || !is(a.org)") has an || branch "!is(a.org)" with no included domain, only exclusions; it usually matches far more than intended`,
				`PROXIED ("is(a.org) || !is(a.org)") always matches every domain`,
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			mockCtrl := gomock.NewController(t)
			ppfmt := mocks.NewMockPP(mockCtrl)
			expectWarnings(ppfmt, tc.want...)
			lintExpr(t, ppfmt, "PROXIED", tc.input)
		})
	}
}

func TestLintR4Redundant(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		input string
		want  string
	}{
		"duplicate-or": {
			"is(a.org) || is(a.org)",
			`PROXIED ("is(a.org) || is(a.org)") contains a redundant term "is(a.org)"; removing it means the same thing`,
		},
		"subsumed-or": {
			"sub(a.org) || sub(x.a.org)",
			`PROXIED ("sub(a.org) || sub(x.a.org)") contains a redundant term "sub(x.a.org)"; removing it means the same thing`,
		},
		"subsumed-and": {
			"is(x.a.org) && sub(a.org)",
			`PROXIED ("is(x.a.org) && sub(a.org)") contains a redundant term "sub(a.org)"; removing it means the same thing`,
		},
		// A true constant adds nothing to a conjunction.
		"redundant-true-conjunct": {
			"is(a.org) && true",
			`PROXIED ("is(a.org) && true") contains a redundant term "true"; removing it means the same thing`,
		},
		// A false constant adds nothing to a disjunction.
		"redundant-false-disjunct": {
			"is(a.org) || false",
			`PROXIED ("is(a.org) || false") contains a redundant term "false"; removing it means the same thing`,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			mockCtrl := gomock.NewController(t)
			ppfmt := mocks.NewMockPP(mockCtrl)
			ppfmt.EXPECT().Noticef(pp.EmojiUserWarning, "%s", tc.want)
			lintExpr(t, ppfmt, "PROXIED", tc.input)
		})
	}
}

// TestLintExpressionMerges covers the LintExpression seams that the per-rule
// tests do not: shape and semantic findings combining on one expression, and
// deduplication of identical messages emitted more than once.
func TestLintExpressionMerges(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		input string
		want  []string
	}{
		// A shape finding (R1, double negation) and a semantic finding (R4,
		// redundant duplicate) on one expression both reach the operator.
		"shape-and-semantic": {
			"!!is(a.org) && is(b.org) && is(b.org)",
			[]string{
				`PROXIED ("!!is(a.org) && is(b.org) && is(b.org)") negates a negation, which has no effect; "is(a.org)" means the same thing`,
				`PROXIED ("!!is(a.org) && is(b.org) && is(b.org)") contains a redundant term "is(b.org)"; removing it means the same thing`,
			},
		},
		// Three identical disjuncts make R4 fire several times with the same
		// message; LintExpression collapses them to a single warning.
		"deduplicated": {
			"is(a.org) || is(a.org) || is(a.org)",
			[]string{`PROXIED ("is(a.org) || is(a.org) || is(a.org)") contains a redundant term "is(a.org)"; removing it means the same thing`},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			mockCtrl := gomock.NewController(t)
			ppfmt := mocks.NewMockPP(mockCtrl)
			expectWarnings(ppfmt, tc.want...)
			lintExpr(t, ppfmt, "PROXIED", tc.input)
		})
	}
}
