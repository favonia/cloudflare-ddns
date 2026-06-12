package syntax_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/syntax"
)

func TestPrattDerivesTokenizer(t *testing.T) {
	t.Parallel()

	grammar := syntax.MustNewPratt(
		syntax.Form("&&", syntax.Hole(10), syntax.Symbol("&&"), syntax.Hole(11)),
		syntax.Form("&word", syntax.Symbol("&word"), syntax.Hole(20)),
	)

	tree, err := grammar.Parse("a&&b")
	require.Nil(t, err)
	require.Equal(t, "&&", tree.(syntax.Op[string]).ID) //nolint:forcetypeassert // Test checks the exact tree shape.

	tree, err = grammar.Parse("&word value")
	require.Nil(t, err)
	require.Equal(t, "&word", tree.(syntax.Op[string]).ID) //nolint:forcetypeassert // Test checks longest symbol matching.

	_, err = grammar.Parse("&")
	require.NotNil(t, err)
	var detail *syntax.UnrecognizedSymbolError
	require.ErrorAs(t, err, &detail)
	require.Equal(t, '&', detail.LeadingRune)
}

func TestPrattInvalidUTF8(t *testing.T) {
	t.Parallel()

	_, err := syntax.MustNewPratt[string]().Parse("valid\200tail")
	require.NotNil(t, err)
	require.ErrorIs(t, err, syntax.ErrInvalidUTF8)
	require.Equal(t, syntax.Span{Start: 5, End: 6}, err.Span)

	_, err = syntax.MustNewPratt[string]().Parse("\200tail")
	require.NotNil(t, err)
	require.ErrorIs(t, err, syntax.ErrInvalidUTF8)
	require.Equal(t, syntax.Span{Start: 0, End: 1}, err.Span)
}
