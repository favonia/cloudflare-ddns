// vim: nowrap

package domainexp_test

import (
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/domainexp"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
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
