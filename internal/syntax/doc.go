// Package syntax provides a small, table-driven Pratt parser for configuration syntaxes.
//
// A grammar declares exact atom keywords, structural symbols, and expression
// holes. Structural symbols are also the complete tokenizer configuration:
// [NewPratt] derives and compiles tokenization from the grammar.
//
// Binding powers determine precedence and associativity. For example, using
// Hole(20) on the left and Hole(21) on the right makes an infix form
// left-associative. Candidate forms are tried in declaration order, so order is
// part of the grammar when forms share a leading token. An [ImplicitForm] can
// infer an infix operation between adjacent expressions.
package syntax
