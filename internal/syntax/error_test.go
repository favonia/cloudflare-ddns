package syntax_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/syntax"
)

func TestPrattExpectedTokenErrors(t *testing.T) {
	t.Parallel()

	_, err := arithmeticGrammar(t).Parse("sum)")
	require.NotNil(t, err)
	require.Equal(t, "bytes [3,4): unexpected token", err.Error())
	var unexpected *syntax.ExpectedTokenError
	require.ErrorAs(t, err, &unexpected)
	require.Equal(t, ")", unexpected.Got)
	require.Equal(t, "(", unexpected.Expected)

	_, err = arithmeticGrammar(t).Parse("sum(1")
	require.NotNil(t, err)
	require.Equal(t, "bytes [5,5): missing token at end", err.Error())
	var missing *syntax.MissingTokenError
	require.ErrorAs(t, err, &missing)
	require.Equal(t, ")", missing.Expected)
}

func TestParseErrorCauseContracts(t *testing.T) {
	t.Parallel()

	_, err := arithmeticGrammar(t).Parse("sum)")
	require.NotNil(t, err)
	require.Equal(t, "bytes [3,4): unexpected token", err.Error())
	require.Equal(t, "unexpected token", err.Cause.Error())

	_, err = syntax.MustNewPratt(syntax.Form("&word", syntax.Symbol("&word"))).Parse("&")
	require.NotNil(t, err)
	require.Equal(t, `bytes [0,1): unrecognized symbol starting with '&'`, err.Error())
	var symbol *syntax.UnrecognizedSymbolError
	require.ErrorAs(t, err, &symbol)
	require.Equal(t, `unrecognized symbol starting with '&'`, symbol.Error())
}

func TestParseErrorZeroValueAndCauseContracts(t *testing.T) {
	t.Parallel()

	var nilErr *syntax.ParseError
	require.Equal(t, "<nil>", nilErr.Error())
	require.NoError(t, nilErr.Unwrap())

	err := &syntax.ParseError{
		Span:  syntax.Span{Start: 0, End: 0},
		Cause: nil,
	}
	require.Equal(t, "bytes [0,0): parse error", err.Error())
	require.NoError(t, err.Unwrap())

	cause := syntax.ErrUnexpectedToken
	err.Cause = cause
	require.Equal(t, "bytes [0,0): unexpected token", err.Error())
	require.ErrorIs(t, err, cause)

	require.Equal(t, "missing token at end", (&syntax.MissingTokenError{Expected: ""}).Error())
}
