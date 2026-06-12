package syntax

import (
	"errors"
	"fmt"
)

var (
	// ErrInvalidGrammar reports an invalid Pratt grammar definition.
	ErrInvalidGrammar = errors.New("invalid Pratt grammar")
	// ErrInvalidUTF8 reports invalid UTF-8 input.
	ErrInvalidUTF8 = errors.New("invalid UTF-8 string")
	// ErrUnexpectedEOF reports an unexpected end of input.
	ErrUnexpectedEOF = errors.New("unexpected end of input")
	// ErrUnexpectedToken reports an unexpected token.
	ErrUnexpectedToken = errors.New("unexpected token")
)

// ParseError is a structured parse failure.
type ParseError struct {
	// Span is the source location of the error.
	Span Span
	// Cause describes and classifies the failure and may wrap a more specific underlying error.
	Cause error
	// progress is the number of tokens consumed when the error occurred.
	progress int
}

// newParseError attaches source location context to a parse failure.
func newParseError(span Span, cause error) *ParseError {
	return &ParseError{
		Span:     span,
		Cause:    cause,
		progress: 0,
	}
}

func (err *ParseError) Error() string {
	if err == nil {
		return "<nil>"
	}
	if err.Cause != nil {
		return fmt.Sprintf("bytes [%d,%d): %v", err.Span.Start, err.Span.End, err.Cause)
	}
	return fmt.Sprintf("bytes [%d,%d): parse error", err.Span.Start, err.Span.End)
}

func (err *ParseError) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.Cause
}

// UnrecognizedSymbolError reports a reserved symbol-leading rune that did not
// begin any complete symbol.
type UnrecognizedSymbolError struct {
	LeadingRune rune
}

func (err *UnrecognizedSymbolError) Error() string {
	return fmt.Sprintf("unrecognized symbol starting with %q", err.LeadingRune)
}

// ExpectedTokenError is the [ParseError.Cause] when a specific token was expected.
type ExpectedTokenError struct {
	Got      string
	Expected string
}

func (*ExpectedTokenError) Error() string {
	return "unexpected token"
}

// MissingTokenError is the [ParseError.Cause] when input ends before a specific token.
type MissingTokenError struct {
	Expected string
}

func (*MissingTokenError) Error() string {
	return "missing token at end"
}
