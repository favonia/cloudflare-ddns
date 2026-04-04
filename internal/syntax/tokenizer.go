package syntax

import (
	"cmp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

type tokenizer struct {
	symbols []string
	// A rune that starts any symbol is reserved: it cannot start or continue an atom.
	leadingRunes map[rune]bool
}

// newTokenizer normalizes symbols for longest-match tokenization and reserves
// every rune that can begin a symbol.
func newTokenizer(symbols []string) tokenizer {
	normalized := slices.Clone(symbols)
	// Try longer symbols first so overlapping declarations use longest-match tokenization.
	slices.SortFunc(normalized, func(left, right string) int {
		return cmp.Or(len(right)-len(left), strings.Compare(left, right))
	})
	normalized = slices.Compact(normalized)
	leadingRunes := make(map[rune]bool, len(normalized))
	for _, symbol := range normalized {
		r, _ := utf8.DecodeRuneInString(symbol)
		leadingRunes[r] = true
	}
	return tokenizer{symbols: normalized, leadingRunes: leadingRunes}
}

// matchSymbol returns the longest declared symbol beginning at start.
func (table tokenizer) matchSymbol(input string, start int) (Token, bool) {
	rest := input[start:]
	for _, symbol := range table.symbols {
		if strings.HasPrefix(rest, symbol) {
			return newToken(tokenSymbol, symbol, Span{Start: start, End: start + len(symbol)}), true
		}
	}
	return Token{kind: tokenEOF, Text: "", Span: Span{Start: 0, End: 0}}, false
}

// tokenize splits input into atoms and declared symbols and appends an EOF token.
func (table tokenizer) tokenize(input string) ([]Token, *ParseError) {
	tokens := make([]Token, 0, len(input)+1)
	for index := 0; index < len(input); {
		r, size := utf8.DecodeRuneInString(input[index:])
		if r == utf8.RuneError && size == 1 {
			return nil, newParseError(Span{Start: index, End: index + 1}, ErrInvalidUTF8)
		}
		if unicode.IsSpace(r) {
			index += size
			continue
		}
		if symbol, ok := table.matchSymbol(input, index); ok {
			tokens = append(tokens, symbol)
			index = symbol.Span.End
			continue
		}
		if table.leadingRunes[r] {
			return nil, newParseError(
				Span{Start: index, End: index + size},
				&UnrecognizedSymbolError{LeadingRune: r},
			)
		}
		start := index
		index += size
		for index < len(input) {
			r, size = utf8.DecodeRuneInString(input[index:])
			if r == utf8.RuneError && size == 1 {
				return nil, newParseError(Span{Start: index, End: index + 1}, ErrInvalidUTF8)
			}
			if unicode.IsSpace(r) {
				break
			}
			if _, ok := table.matchSymbol(input, index); ok || table.leadingRunes[r] {
				break
			}
			index += size
		}
		tokens = append(tokens, newToken(tokenAtom, input[start:index], Span{Start: start, End: index}))
	}
	tokens = append(tokens, newToken(tokenEOF, "", Span{Start: len(input), End: len(input)}))
	return tokens, nil
}
