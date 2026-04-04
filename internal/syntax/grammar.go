package syntax

import (
	"slices"
)

type partKind int

const (
	partKeyword partKind = iota
	partSymbol
	partHole
)

// Part is one element of a [Rule] pattern.
type Part struct {
	kind         partKind
	text         string
	bindingPower int
}

// Keyword creates a [Part] that matches an atom with the given text exactly.
func Keyword(text string) Part {
	return Part{kind: partKeyword, text: text, bindingPower: 0}
}

// Symbol creates a [Part] that declares and matches a structural symbol.
func Symbol(text string) Part {
	return Part{kind: partSymbol, text: text, bindingPower: 0}
}

// Hole creates a [Part] that matches a sub-expression with at least the given
// binding power. Matched sub-expressions are collected into [Op.Args].
func Hole(bindingPower int) Part {
	return Part{kind: partHole, text: "", bindingPower: bindingPower}
}

// Rule is an opaque grammar rule for Pratt parsing.
//
// If the first Part is a [Hole], this is a left-denotation
// (infix/postfix/implicit); otherwise it is a null-denotation
// (prefix/literal/grouping).
type Rule[ID any] struct {
	id       ID
	pattern  []Part
	empty    bool
	implicit bool
}

// Form creates a grammar rule. The ID is stored in [Op.ID] when the rule matches.
func Form[ID any](id ID, pattern ...Part) Rule[ID] {
	return Rule[ID]{id: id, pattern: slices.Clone(pattern), empty: false, implicit: false}
}

// ImplicitForm creates an infix rule inferred between two adjacent expressions.
// It is considered only when no explicit left rule matches the next token.
func ImplicitForm[ID any](id ID, leftBindingPower, rightBindingPower int) Rule[ID] {
	return Rule[ID]{
		id: id,
		pattern: []Part{
			Hole(leftBindingPower),
			Hole(rightBindingPower),
		},
		empty:    false,
		implicit: true,
	}
}

// Empty creates the unique rule that permits empty or whitespace-only input.
func Empty[ID any]() Rule[ID] {
	var id ID
	return Rule[ID]{id: id, pattern: nil, empty: true, implicit: false}
}
