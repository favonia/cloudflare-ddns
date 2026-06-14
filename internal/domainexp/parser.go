// Package domainexp parses expressions containing domains.
package domainexp

import (
	"errors"

	"github.com/favonia/cloudflare-ddns/internal/syntax"
)

var (
	// ErrSingleAnd is triggered by a single & (which should have been &&) in an expression.
	// In a domain list, & is just an invalid domain character.
	ErrSingleAnd = errors.New(`use "&&" instead of "&"`)

	// ErrSingleOr is triggered by a single | (which should have been ||) in an expression.
	// In a domain list, | is just an invalid domain character.
	ErrSingleOr = errors.New(`use "||" instead of "|"`)

	// ErrUTF8 is triggered by invalid UTF-8 strings.
	ErrUTF8 = syntax.ErrInvalidUTF8

	errNotBooleanExpression   = errors.New("not a boolean expression")
	errUnexpectedBooleanToken = errors.New("unexpected token in boolean expression")
)

// formID distinguishes the accepted expression and compatibility-list shapes.
type formID string

// unexpectedTokenError reports token as the source of a generic unexpected-token failure.
func unexpectedTokenError(token syntax.Token) *syntax.ParseError {
	return &syntax.ParseError{
		Span: token.Span, Cause: syntax.ErrUnexpectedToken,
	}
}

// mustFirstToken returns the first token of a successfully parsed expression.
func mustFirstToken(tree syntax.Tree[formID]) syntax.Token {
	switch tree := tree.(type) {
	case syntax.Atom[formID]:
		return tree.Token
	case syntax.Op[formID]:
		if len(tree.Tokens) != 0 &&
			(len(tree.Args) == 0 || tree.Tokens[0].Span.Start < tree.Args[0].Span().Start) {
			return tree.Tokens[0]
		}
		return mustFirstToken(tree.Args[0])
	default:
		panic("domainexp: parsed expression has no token")
	}
}
