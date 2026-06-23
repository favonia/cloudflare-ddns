package domainexp

import "strings"

// Precedence for canonical printing: higher binds tighter.
const (
	precOr  = 1
	precAnd = 2
	precNot = 3
)

// exprString renders e as a valid, canonical PROXIED expression with the
// minimum parentheses needed to preserve structure. It is the basis for
// warning text and for R1/R4 suggested rewrites.
func exprString(e Expr) string {
	return exprStringPrec(e, 0)
}

func exprStringPrec(e Expr, ctx int) string {
	switch e := e.(type) {
	case literalExpr:
		if e.value {
			return "true"
		}
		return "false"
	case callExpr:
		return e.function + "(" + strings.Join(e.domains, ", ") + ")"
	case unaryExpr:
		return wrap("!"+exprStringPrec(e.operand, precNot), precNot, ctx)
	case binaryExpr:
		op, prec := " && ", precAnd
		if e.operator == formOr {
			op, prec = " || ", precOr
		}
		s := exprStringPrec(e.left, prec) + op + exprStringPrec(e.right, prec)
		return wrap(s, prec, ctx)
	default:
		return ""
	}
}

func wrap(s string, prec, ctx int) string {
	if prec < ctx {
		return "(" + s + ")"
	}
	return s
}
