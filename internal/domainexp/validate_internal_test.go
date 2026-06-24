package domainexp

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/mocks"
)

func parseQuiet(t *testing.T, input string) (Expr, bool) {
	t.Helper()
	ctrl := gomock.NewController(t)
	ppfmt := mocks.NewMockPP(ctrl)
	// Accept any Noticef regardless of vararg count (error messages vary in arity).
	// Every expression-path message carries at least one trailing format argument,
	// so the matchers start at three arguments (emoji + format + one arg).
	ppfmt.EXPECT().Noticef(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	ppfmt.EXPECT().Noticef(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	ppfmt.EXPECT().Noticef(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	ppfmt.EXPECT().Noticef(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	ppfmt.EXPECT().Noticef(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
		gomock.Any(), gomock.Any()).AnyTimes()
	ppfmt.EXPECT().Noticef(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
		gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	return ParseExpression(ppfmt, "PROXIED", input)
}

func TestValidateArguments(t *testing.T) {
	t.Parallel()
	for _, bad := range []string{"is(b.*.a.org)", "is(a.*.b)"} {
		_, ok := parseQuiet(t, bad)
		require.Falsef(t, ok, "expected %q to be rejected", bad)
	}
	for _, good := range []string{"is(*.a.org)", "sub(a.org)", "is(a.org, b.org)", "sub(.)", "sub(org)", "is(.)"} {
		_, ok := parseQuiet(t, good)
		require.Truef(t, ok, "expected %q to be accepted", good)
	}

	// The AST renders arguments via domain.String() (Unicode for an IDN).
	expr, ok := parseQuiet(t, "is(café.example)")
	require.True(t, ok)
	require.Equal(t, "is(café.example)", exprString(expr))
}

func TestRejectionMessages(t *testing.T) {
	t.Parallel()

	// malformed: quotes the domain and passes through the underlying cause.
	ctrl := gomock.NewController(t)
	ppfmt := mocks.NewMockPP(ctrl)
	ppfmt.EXPECT().Noticef(
		gomock.Any(),
		gomock.Any(),
		"PROXIED", "is(b.*.a.org)", "b.*.a.org", gomock.Any(),
	)
	_, ok := ParseExpression(ppfmt, "PROXIED", "is(b.*.a.org)")
	require.False(t, ok)
}

func TestShortIsTargetAdvisory(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	ppfmt := mocks.NewMockPP(ctrl)
	// One advisory naming the joined too-short targets; kept (parse succeeds).
	ppfmt.EXPECT().Noticef(gomock.Any(), gomock.Any(), "PROXIED", "is(org, com)", "org and com")
	_, ok := ParseExpression(ppfmt, "PROXIED", "is(org, com)")
	require.True(t, ok)
}

func TestSubWildcardAdvisory(t *testing.T) {
	t.Parallel()
	// Skipped + advisory; the term keeps parsing and renders as an empty sub().
	expr, ok := parseQuiet(t, "sub(*.a.org)")
	require.True(t, ok)
	require.Equal(t, "sub()", exprString(expr)) // wildcard skipped -> empty subExpr
}
