package syntax_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/syntax"
)

func TestPrattSpans(t *testing.T) {
	t.Parallel()

	tree, err := syntax.MustNewPratt[string]().Parse("  atom  ")
	require.Nil(t, err)
	require.Equal(t, syntax.Span{Start: 2, End: 6}, tree.Span())

	tree, err = arithmeticGrammar(t).Parse("  sum(1 + 2)  ")
	require.Nil(t, err)
	require.Equal(t, syntax.Span{Start: 2, End: 12}, tree.Span())
	op := tree.(syntax.Op[string]) //nolint:forcetypeassert // Test checks the exact tree shape.
	require.Equal(t, syntax.Span{Start: 6, End: 11}, op.Args[0].Span())

	tree, err = syntax.MustNewPratt(syntax.Empty[string]()).Parse("  ")
	require.Nil(t, err)
	require.Equal(t, syntax.Span{Start: 0, End: 0}, tree.Span())
}

func TestPrattPublicTreeContract(t *testing.T) {
	t.Parallel()

	tree, err := arithmeticGrammar(t).Parse("sum(value)")
	require.Nil(t, err)
	op := tree.(syntax.Op[string]) //nolint:forcetypeassert // Test checks the public tree contract.
	require.Equal(t, []string{"sum", "(", ")"}, []string{
		op.Tokens[0].Text,
		op.Tokens[1].Text,
		op.Tokens[2].Text,
	})
	require.Equal(t, "value", op.Args[0].(syntax.Atom[string]).Token.Text) //nolint:forcetypeassert
}
