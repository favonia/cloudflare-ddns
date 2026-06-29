// vim: nowrap

package domainexp_test

import (
	"testing"

	"github.com/stretchr/testify/require"
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

// TestLintR3SubRoot pins that sub(.) denotes the strict subdomains of the
// root suffix, i.e. every domain, so it is statically constant-true and trips
// R3. Negating it is constant-false, and in a disjunction the constant-true
// folds to the whole expression.
func TestLintR3SubRoot(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		input string
		want  []string
	}{
		// A bare root-suffix sub() always matches every domain.
		"bare": {
			"sub(.)",
			[]string{`PROXIED ("sub(.)") always matches every domain`},
		},
		// In a disjunction the constant-true folds to the whole expression, and
		// the other disjunct is redundant because sub(.) already covers it.
		"disjunct": {
			"sub(.) || is(a.org)",
			[]string{
				`PROXIED ("sub(.) || is(a.org)") always matches every domain`,
				`PROXIED ("sub(.) || is(a.org)") contains a redundant term "is(a.org)"; removing it means the same thing`,
			},
		},
		// Negating an always-true atom is constant-false.
		"negated": {
			"!sub(.)",
			[]string{`PROXIED ("!sub(.)") can never match any domain`},
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

// TestLintR3SubRootMultiArgPartial pins what the multi-arg redundancy pass resolves and what it does
// not: a multi-arg sub() that contains the root suffix, e.g. sub(org, .), is now
// expanded into its single-atom literals, so the redundancy pass sees that org
// is covered by the all-matching "." and flags sub(org). The whole-call R3
// constant-true detection (its union covers every domain) remains out of scope,
// because constValue still treats a multi-arg call as opaque.
func TestLintR3SubRootMultiArgPartial(t *testing.T) {
	t.Parallel()
	mockCtrl := gomock.NewController(t)
	ppfmt := mocks.NewMockPP(mockCtrl)
	expectWarnings(ppfmt,
		`PROXIED ("sub(org, .)") contains a redundant term "sub(org)"; removing it means the same thing`)
	lintExpr(t, ppfmt, "PROXIED", "sub(org, .)")
}

// TestLintR3SubRootGuard pins the cases that justify the constancy detector's
// guard and folding logic: a non-constant expression containing sub(.) must not
// be mislabeled constant, a plain constant without sub(.) must not gain an R3
// "always/never matches" line, and the negation/short-circuit folding must hold.
func TestLintR3SubRootGuard(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		input string
		want  []string
	}{
		// sub(.) && is(a.org) == is(a.org): NOT constant. Only the redundancy of
		// the all-covering sub(.) is reported, never "always matches every domain".
		"not-constant-with-subroot": {
			"sub(.) && is(a.org)",
			[]string{`PROXIED ("sub(.) && is(a.org)") contains a redundant term "sub(.)"; removing it means the same thing`},
		},
		// A plain true constant (no sub(.)) is not the non-obvious constancy R3
		// warns about: the guard keeps R3 silent here.
		"plain-true-no-r3": {
			"true",
			nil,
		},
		// !true is an R1 finding only; the guard keeps R3 from also firing.
		"negated-true-no-r3": {
			"!true",
			[]string{`PROXIED ("!true") negates a constant; "false" means the same thing`},
		},
		// sub(.) && false short-circuits to constant-false.
		"subroot-and-false": {
			"sub(.) && false",
			[]string{`PROXIED ("sub(.) && false") can never match any domain`},
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

// TestLintR4RedundantArguments pins that a multi-arg is/sub call is the
// disjunction of its single-atom literals, so redundancy is detected both within
// one call (any context) and across || terms drawn from different calls.
func TestLintR4RedundantArguments(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		input string
		want  []string
	}{
		// Two identical atoms in one call: the duplicate is redundant, reported once.
		"intra-call-duplicate": {
			"is(a.org, a.org)",
			[]string{`PROXIED ("is(a.org, a.org)") contains a redundant term "is(a.org)"; removing it means the same thing`},
		},
		// sub.org is under org, so its atom is the redundant smaller set in the call.
		"intra-call-subsumption": {
			"sub(org, sub.org)",
			[]string{`PROXIED ("sub(org, sub.org)") contains a redundant term "sub(sub.org)"; removing it means the same thing`},
		},
		// b.com (in the first call) is under com (the second call), so it is
		// redundant across the || boundary.
		"cross-or": {
			"sub(a.org, b.com) || sub(com)",
			[]string{`PROXIED ("sub(a.org, b.com) || sub(com)") contains a redundant term "sub(b.com)"; removing it means the same thing`},
		},
		// Two disjoint atoms in one call: nothing is redundant.
		"no-false-positive": {
			"sub(a.org, b.com)",
			nil,
		},
		// A negated operand in a || must not be used as a redundancy cover, nor be
		// flagged itself. The only finding here is R2 (the !sub(com) branch has no
		// included domain); crucially there is no R4 "redundant term".
		"negated-operand-not-cover": {
			"sub(b.com) || !sub(com)",
			[]string{`PROXIED ("sub(b.com) || !sub(com)") has an || branch "!sub(com)" with no included domain, only exclusions; it usually matches far more than intended`},
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

// TestParseL1SubWildcard pins the sub()-of-a-wildcard advisory, which now fires
// at parse time (the wildcard is skipped and recorded by buildSubCall), not in
// LintExpression. Behavior is pinned by positional Noticef args, not prose.
func TestParseL1SubWildcard(t *testing.T) {
	t.Parallel()

	// Warns at parse time, naming the wildcard argument (once, even with a
	// second non-wildcard arg). The parse still succeeds; the wildcard is
	// skipped because it matches nothing.
	for _, input := range []string{"sub(*.a.org)", "sub(*.a.org, b.org)"} {
		t.Run(input, func(t *testing.T) {
			t.Parallel()
			mockCtrl := gomock.NewController(t)
			ppfmt := mocks.NewMockPP(mockCtrl)
			ppfmt.EXPECT().Noticef(pp.EmojiUserWarning, gomock.Any(),
				"PROXIED", input, "*.a.org", "*.a.org", "a.org", "a.org")
			_, ok := domainexp.ParseExpression(ppfmt, "PROXIED", input)
			require.True(t, ok)
		})
	}

	// No warning: is() of a wildcard and sub() of a plain domain are fine.
	for _, input := range []string{"is(*.a.org)", "sub(a.org)"} {
		t.Run(input, func(t *testing.T) {
			t.Parallel()
			mockCtrl := gomock.NewController(t)
			ppfmt := mocks.NewMockPP(mockCtrl)
			_, ok := domainexp.ParseExpression(ppfmt, "PROXIED", input) // no Noticef expected
			require.True(t, ok)
		})
	}
}
