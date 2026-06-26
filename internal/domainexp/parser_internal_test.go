// vim: nowrap

package domainexp

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/syntax"
)

// The grammars in this package only ever produce the three value-type
// implementations of [syntax.Tree], so the default cases tested here are
// unreachable through [ParseList] and [ParseExpression]. These tests pin the
// defensive contract: junk trees degrade to a parse error instead of panicking.
//
// The pointer form is junk that the type system cannot rule out: value-receiver
// methods are in the method set of *Atom, so it satisfies the sealed interface,
// yet it matches no value-type case in a type switch.

func newParserState() *parserState {
	return &parserState{
		emptyCallFunctions: nil, extraComma: false, missingComma: false,
		shortIsTargets: nil, subWildcards: nil,
	}
}

func TestFlattenDomainListImpossibleTrees(t *testing.T) {
	t.Parallel()
	for name, tree := range map[string]syntax.Tree[formID]{
		"nil":     nil,
		"pointer": &syntax.Atom[formID]{Token: syntax.Token{Text: "a.a", Span: syntax.Span{Start: 0, End: 3}}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			state := newParserState()
			list, err := flattenDomainList(tree, state)
			require.Nil(t, list)
			require.ErrorIs(t, err, syntax.ErrUnexpectedToken)
			require.Equal(t, syntax.Span{Start: 0, End: 0}, err.Span)
			require.Equal(t, newParserState(), state)
		})
	}
}

// TestReportParseError pins the error-classification contract of
// reportParseError, including the unexpected-token message inherited from the
// pre-Pratt parser. That branch is currently unreachable through [ParseList]
// because the domain-list grammar's only symbol is "," (every token can start
// or continue a list, so its parser fails only on invalid UTF-8), but the
// message is kept for future grammars with more symbols.
func TestReportParseError(t *testing.T) {
	t.Parallel()
	key := "key"
	input := "a,(b),c"
	for name, tc := range map[string]struct {
		err           *syntax.ParseError
		prepareMockPP func(m *mocks.MockPP)
	}{
		"unexpected-token": {
			&syntax.ParseError{
				Span: syntax.Span{Start: 2, End: 5}, Cause: syntax.ErrUnexpectedToken,
			},
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError,
					`%s (%q) has unexpected token %q when "," is expected`, key, input, "(b)")
			},
		},
		"malformed": {
			&syntax.ParseError{
				Span: syntax.Span{Start: 0, End: 1}, Cause: syntax.ErrInvalidUTF8,
			},
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError,
					"%s (%q) is malformed: %v", key, input, syntax.ErrInvalidUTF8)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			mockCtrl := gomock.NewController(t)
			mockPP := mocks.NewMockPP(mockCtrl)
			tc.prepareMockPP(mockPP)
			reportParseError(mockPP, key, input, tc.err)
		})
	}
}

func TestBuildExprImpossibleTrees(t *testing.T) {
	t.Parallel()
	for name, tree := range map[string]syntax.Tree[formID]{
		"nil":     nil,
		"pointer": &syntax.Atom[formID]{Token: syntax.Token{Text: "true", Span: syntax.Span{Start: 0, End: 4}}},
		// expressionGrammar has no syntax.Empty rule, so EmptyTree never reaches buildExpr.
		"empty": syntax.EmptyTree[formID]{},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			state := newParserState()
			expr, err := buildExpr(tree, state)
			require.Nil(t, expr)
			require.ErrorIs(t, err, errNotBooleanExpression)
			require.Equal(t, syntax.Span{Start: 0, End: 0}, err.Span)
			require.Equal(t, newParserState(), state)
		})
	}
}
