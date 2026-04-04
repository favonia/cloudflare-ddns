package syntax_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/syntax"
)

func TestPrattParser(t *testing.T) {
	t.Parallel()

	for input, expected := range map[string]string{
		"sum(1, 2) + !(3 * 4)": "(sum(1, 2) + (!((3 * 4))))",
		"1 + 2 + 3":            "((1 + 2) + 3)",
		"1 + 2 * 3":            "(1 + (2 * 3))",
	} {
		tree, err := arithmeticGrammar(t).Parse(input)
		require.Nil(t, err)
		require.Equal(t, expected, render(tree))
	}
}

func TestPrattEmpty(t *testing.T) {
	t.Parallel()

	grammar := syntax.MustNewPratt(syntax.Empty[string]())
	tree, err := grammar.Parse(" \t\n")
	require.Nil(t, err)
	require.IsType(t, syntax.EmptyTree[string]{}, tree)

	_, err = syntax.MustNewPratt[string]().Parse("")
	require.NotNil(t, err)
	require.ErrorIs(t, err, syntax.ErrUnexpectedEOF)
}

func TestPrattAtomPreservation(t *testing.T) {
	t.Parallel()

	tree, err := syntax.MustNewPratt[string]().Parse("203.0.113.0/24")
	require.Nil(t, err)
	require.Equal(t, "203.0.113.0/24", tree.(syntax.Atom[string]).Token.Text) //nolint:forcetypeassert

	_, err = syntax.MustNewPratt[string]().Parse("first second")
	require.NotNil(t, err)
	require.ErrorIs(t, err, syntax.ErrUnexpectedToken)
}

func TestPrattUsesDeclarationOrder(t *testing.T) {
	t.Parallel()

	grammar := syntax.MustNewPratt(
		syntax.Form("first", syntax.Keyword("same")),
		syntax.Form("second", syntax.Keyword("same")),
	)
	tree, err := grammar.Parse("same")
	require.Nil(t, err)
	require.Equal(t, "first", tree.(syntax.Op[string]).ID) //nolint:forcetypeassert
}

func TestPrattBacktracksAcrossAlternativeForms(t *testing.T) {
	t.Parallel()

	nullGrammar := syntax.MustNewPratt(
		syntax.Form("parenthesized",
			syntax.Keyword("call"), syntax.Symbol("("), syntax.Hole(0), syntax.Symbol(")"),
		),
		syntax.Form("bracketed",
			syntax.Keyword("call"), syntax.Symbol("["), syntax.Hole(0), syntax.Symbol("]"),
		),
	)
	tree, err := nullGrammar.Parse("call[value]")
	require.Nil(t, err)
	require.Equal(t, "bracketed", tree.(syntax.Op[string]).ID) //nolint:forcetypeassert

	_, err = nullGrammar.Parse("call(value]")
	require.NotNil(t, err)
	var expected *syntax.ExpectedTokenError
	require.ErrorAs(t, err, &expected)
	require.Equal(t, "]", expected.Got)
	require.Equal(t, ")", expected.Expected)

	leftGrammar := syntax.MustNewPratt(
		syntax.Form("then-group",
			syntax.Hole(10), syntax.Keyword("then"), syntax.Symbol("("), syntax.Hole(0), syntax.Symbol(")"),
		),
		syntax.Form("then-word",
			syntax.Hole(10), syntax.Keyword("then"), syntax.Keyword("word"),
		),
	)
	tree, err = leftGrammar.Parse("first then (second)")
	require.Nil(t, err)
	require.Equal(t, "then-group", tree.(syntax.Op[string]).ID) //nolint:forcetypeassert

	_, err = leftGrammar.Parse("first then (second]")
	require.NotNil(t, err)
	var missing *syntax.MissingTokenError
	require.ErrorAs(t, err, &missing)
	require.Equal(t, ")", missing.Expected)
}

func TestPrattImplicitForms(t *testing.T) {
	t.Parallel()

	grammar := syntax.MustNewPratt(
		syntax.Form("+", syntax.Hole(10), syntax.Symbol("+"), syntax.Hole(11)),
		syntax.ImplicitForm("juxtapose", 20, 21),
	)

	for input, expected := range map[string]string{
		"a b":       "(a b)",
		"a b c":     "((a b) c)",
		"a + b c":   "(a + (b c))",
		"a b + c":   "((a b) + c)",
		"a + b + c": "((a + b) + c)",
	} {
		tree, err := grammar.Parse(input)
		require.Nil(t, err)
		require.Equal(t, expected, render(tree))
	}
}

func TestPrattImplicitFormsNeedExpressionStart(t *testing.T) {
	t.Parallel()

	grammar := syntax.MustNewPratt(
		syntax.Form("group", syntax.Symbol("("), syntax.Hole(0), syntax.Symbol(")")),
		syntax.Form("+", syntax.Hole(10), syntax.Symbol("+"), syntax.Hole(11)),
		syntax.ImplicitForm("juxtapose", 20, 21),
	)

	_, err := grammar.Parse("a +")
	require.NotNil(t, err)
	require.ErrorIs(t, err, syntax.ErrUnexpectedEOF)

	_, err = grammar.Parse("a )")
	require.NotNil(t, err)
	require.ErrorIs(t, err, syntax.ErrUnexpectedToken)

	_, err = grammar.Parse(")")
	require.NotNil(t, err)
	require.ErrorIs(t, err, syntax.ErrUnexpectedToken)
}

func TestPrattReportsUnexpectedEOFInsideNullFormHole(t *testing.T) {
	t.Parallel()

	grammar := syntax.MustNewPratt(syntax.Form("prefix", syntax.Keyword("prefix"), syntax.Hole(0)))
	_, err := grammar.Parse("prefix")
	require.NotNil(t, err)
	require.ErrorIs(t, err, syntax.ErrUnexpectedEOF)
}

func TestPrattPrefersMissingTokenOverEOFInHole(t *testing.T) {
	t.Parallel()

	grammar := syntax.MustNewPratt(
		syntax.Form("call()",
			syntax.Keyword("call"), syntax.Symbol("("), syntax.Symbol(")"),
		),
		syntax.Form("call(...)",
			syntax.Keyword("call"), syntax.Symbol("("), syntax.Hole(0), syntax.Symbol(")"),
		),
	)

	// The empty-call alternative's missing ")" must win over the other
	// alternative's unexpected EOF inside the hole.
	_, err := grammar.Parse("call(")
	require.NotNil(t, err)
	var missing *syntax.MissingTokenError
	require.ErrorAs(t, err, &missing)
	require.Equal(t, ")", missing.Expected)
}

func TestPrattPreservesFarthestFailureAcrossNestedBacktracking(t *testing.T) {
	t.Parallel()

	grammar := syntax.MustNewPratt(
		syntax.Form("outer-expression",
			syntax.Keyword("start"), syntax.Hole(0), syntax.Symbol("!"),
		),
		syntax.Form("outer-literal",
			syntax.Keyword("start"), syntax.Keyword("call"), syntax.Symbol("("), syntax.Keyword("z"),
		),
		syntax.Form("call",
			syntax.Keyword("call"), syntax.Symbol("("), syntax.Hole(0), syntax.Symbol(")"),
		),
		syntax.Form("+",
			syntax.Hole(10), syntax.Symbol("+"), syntax.Hole(11),
		),
	)

	_, err := grammar.Parse("start call(a +")
	require.NotNil(t, err)
	require.ErrorIs(t, err, syntax.ErrUnexpectedEOF)
}

func TestPrattZeroValueID(t *testing.T) {
	t.Parallel()

	grammar := syntax.MustNewPratt(syntax.Form(0, syntax.Keyword("zero")))
	tree, err := grammar.Parse("zero")
	require.Nil(t, err)
	require.Equal(t, 0, tree.(syntax.Op[int]).ID) //nolint:forcetypeassert // Test checks the public tree contract.
}
