package syntax_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/syntax"
)

func TestPrattCopiesForms(t *testing.T) {
	t.Parallel()

	pattern := []syntax.Part{syntax.Keyword("before")}
	grammar := syntax.MustNewPratt(syntax.Form("keyword", pattern...))
	pattern[0] = syntax.Keyword("after")

	tree, err := grammar.Parse("before")
	require.Nil(t, err)
	require.Equal(t, "keyword", tree.(syntax.Op[string]).ID) //nolint:forcetypeassert
}

func TestNewPrattValidation(t *testing.T) {
	t.Parallel()

	for name, forms := range map[string][]syntax.Rule[string]{
		"zero-rule":     {{}},
		"empty-pattern": {syntax.Form("x")},
		"negative-hole": {syntax.Form("x", syntax.Hole(-1), syntax.Symbol("+"))},
		"negative-implicit-hole": {
			syntax.ImplicitForm("x", -1, 2),
		},
		"invalid-left":  {syntax.Form("x", syntax.Hole(1), syntax.Hole(2))},
		"empty-keyword": {syntax.Form("x", syntax.Keyword(""))},
		"whitespace":    {syntax.Form("x", syntax.Symbol(" "))},
		"invalid-utf8":  {syntax.Form("x", syntax.Keyword("\200"))},
		"keyword-is-symbol": {
			syntax.Form("symbol", syntax.Symbol("same")),
			syntax.Form("keyword", syntax.Keyword("same")),
		},
		"multiple-empties": {syntax.Empty[string](), syntax.Empty[string]()},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := syntax.NewPratt(forms...)
			require.Error(t, err)
			require.ErrorIs(t, err, syntax.ErrInvalidGrammar)
		})
	}
}

func TestMustNewPrattPanicsOnInvalidGrammar(t *testing.T) {
	t.Parallel()

	require.Panics(t, func() {
		syntax.MustNewPratt(syntax.Form(""))
	})
}
