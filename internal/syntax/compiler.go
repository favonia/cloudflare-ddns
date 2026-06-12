package syntax

import (
	"fmt"
	"slices"
	"unicode"
	"unicode/utf8"
)

// Pratt is a compiled grammar and its derived tokenizer.
type Pratt[ID any] struct {
	nullRules     map[tokenKey][]Rule[ID]
	leftRules     map[tokenKey][]Rule[ID]
	implicitRules []Rule[ID]
	tokenizer     tokenizer
	empty         bool
}

type tokenKey struct {
	kind tokenKind
	text string
}

// NewPratt validates and compiles forms into a Pratt grammar.
func NewPratt[ID any](forms ...Rule[ID]) (Pratt[ID], error) {
	forms = slices.Clone(forms)
	for i := range forms {
		forms[i].pattern = slices.Clone(forms[i].pattern)
	}
	symbols := make([]string, 0)
	empty := false
	for formIndex, form := range forms {
		if form.empty {
			// Empty is the only constructor setting empty, always with a nil pattern.
			if empty {
				return Pratt[ID]{}, fmt.Errorf("%w: form %d: multiple empty forms", ErrInvalidGrammar, formIndex)
			}
			empty = true
			continue
		}
		if len(form.pattern) == 0 {
			return Pratt[ID]{}, fmt.Errorf("%w: form %v: empty pattern", ErrInvalidGrammar, form.id)
		}
		if form.implicit &&
			(len(form.pattern) != 2 || form.pattern[0].kind != partHole || form.pattern[1].kind != partHole) {
			return Pratt[ID]{}, fmt.Errorf(
				"%w: form %v: implicit form must contain exactly two holes", ErrInvalidGrammar, form.id,
			)
		}
		for partIndex, part := range form.pattern {
			if part.kind == partHole {
				if part.bindingPower < 0 {
					return Pratt[ID]{}, fmt.Errorf(
						"%w: form %v part %d: negative binding power", ErrInvalidGrammar, form.id, partIndex,
					)
				}
				continue
			}
			if problem, ok := validateLiteral(part.text); !ok {
				return Pratt[ID]{}, fmt.Errorf("%w: form %v part %d: %s", ErrInvalidGrammar, form.id, partIndex, problem)
			}
			if part.kind == partSymbol {
				symbols = append(symbols, part.text)
			}
		}
		if !form.implicit && form.pattern[0].kind == partHole &&
			(len(form.pattern) < 2 || form.pattern[1].kind == partHole) {
			return Pratt[ID]{}, fmt.Errorf(
				"%w: form %v: left form must be followed by a keyword or symbol", ErrInvalidGrammar, form.id,
			)
		}
	}

	tokenizer := newTokenizer(symbols)
	pratt := Pratt[ID]{
		nullRules:     map[tokenKey][]Rule[ID]{},
		leftRules:     map[tokenKey][]Rule[ID]{},
		implicitRules: nil,
		tokenizer:     tokenizer,
		empty:         empty,
	}
	for _, form := range forms {
		if form.empty {
			continue
		}
		for partIndex, part := range form.pattern {
			if part.kind != partKeyword {
				continue
			}
			tokens, err := tokenizer.tokenize(part.text)
			if err != nil || len(tokens) != 2 || !tokens[0].isAtom() || tokens[0].Text != part.text {
				return Pratt[ID]{}, fmt.Errorf(
					"%w: form %v part %d: keyword cannot be tokenized as one atom",
					ErrInvalidGrammar, form.id, partIndex,
				)
			}
		}
		first := form.pattern[0]
		if form.implicit {
			pratt.implicitRules = append(pratt.implicitRules, form)
			continue
		}
		if first.kind == partHole {
			key := keyForPart(form.pattern[1])
			pratt.leftRules[key] = append(pratt.leftRules[key], form)
			continue
		}
		key := keyForPart(first)
		pratt.nullRules[key] = append(pratt.nullRules[key], form)
	}
	return pratt, nil
}

// MustNewPratt is like [NewPratt] but panics if the grammar is invalid.
func MustNewPratt[ID any](forms ...Rule[ID]) Pratt[ID] {
	pratt, err := NewPratt(forms...)
	if err != nil {
		panic(fmt.Errorf("syntax.MustNewPratt: %w", err))
	}
	return pratt
}

// validateLiteral checks the lexical requirements shared by keywords and
// symbols. It returns a grammar-validation problem and false when invalid.
func validateLiteral(text string) (string, bool) {
	if text == "" {
		return "empty literal", false
	}
	if !utf8.ValidString(text) {
		return "invalid UTF-8 literal", false
	}
	for _, r := range text {
		if unicode.IsSpace(r) {
			return "literal contains whitespace", false
		}
	}
	return "", true
}

// keyForPart returns the token kind and text required to match a literal part.
func keyForPart(part Part) tokenKey {
	if part.kind == partSymbol {
		return tokenKey{kind: tokenSymbol, text: part.text}
	}
	return tokenKey{kind: tokenAtom, text: part.text}
}

// keyForToken returns the lookup key used to find forms matching token.
func keyForToken(token Token) tokenKey {
	return tokenKey{kind: token.kind, text: token.Text}
}
