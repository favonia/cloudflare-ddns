package syntax

type tokenKind int

const (
	tokenAtom tokenKind = iota
	tokenSymbol
	tokenEOF
)

// Token is one lexical token with its source location.
type Token struct {
	kind tokenKind
	// Text is the matched source text for atoms and symbols, or empty for EOF.
	Text string
	// Span is the byte range of this token in the source input.
	Span Span
}

func (t Token) isAtom() bool {
	return t.kind == tokenAtom
}

func (t Token) isEOF() bool {
	return t.kind == tokenEOF
}

// newToken constructs a token with its source span.
func newToken(kind tokenKind, text string, span Span) Token {
	return Token{
		kind: kind,
		Text: text,
		Span: span,
	}
}
