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
func lintExpr(t *testing.T, ppfmt *mocks.MockPP, key, input string) {
	t.Helper()
	expr, ok := domainexp.ParseExpression(ppfmt, key, input)
	if !ok {
		t.Fatalf("ParseExpression(%q) failed unexpectedly", input)
	}
	domainexp.LintExpression(ppfmt, key, input, expr)
}

func TestLintExpressionClean(t *testing.T) {
	t.Parallel()
	for _, input := range []string{
		"is(a.org)",
		"is(a.org) || sub(a.org)",
		"!is(a.org)",
		"!is(a.org) && !sub(a.org)",
		"sub(a.org) && !sub(b.a.org)",
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
