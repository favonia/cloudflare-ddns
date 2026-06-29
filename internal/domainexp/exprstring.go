package domainexp

import (
	"fmt"
	"strings"
)

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
	case isExpr:
		parts := make([]string, len(e.domains))
		for i, d := range e.domains {
			parts[i] = d.String()
		}
		return "is(" + strings.Join(parts, ", ") + ")"
	case subExpr:
		parts := make([]string, len(e.suffixes))
		for i, s := range e.suffixes {
			parts[i] = s.String()
		}
		return "sub(" + strings.Join(parts, ", ") + ")"
	case unaryExpr:
		return wrap("!"+exprStringPrec(e.operand, precNot), precNot, ctx)
	case binaryExpr:
		prec, ok := map[formID]int{formAnd: precAnd, formOr: precOr}[e.operator]
		if !ok {
			// Unreachable: the parser only builds formAnd/formOr operators. This
			// renderer must always produce a valid canonical expression (its output
			// is shown in user-facing warnings), so returning "" would silently emit
			// corrupt text. Panic to surface the bug instead of hiding it.
			panic("domainexp: exprStringPrec got an unknown binary operator; please report it")
		}
		s := fmt.Sprintf("%s %s %s", exprStringPrec(e.left, prec), e.operator, exprStringPrec(e.right, prec))
		return wrap(s, prec, ctx)
	default:
		// Unreachable: Expr is sealed (unexported expr()) and every concrete type is
		// handled above. As with the operator guard, "" would be a silently corrupt
		// rendering, so panic to surface the bug instead of hiding it.
		panic("domainexp: exprStringPrec got an unknown Expr type; please report it")
	}
}

func wrap(s string, prec, ctx int) string {
	if prec < ctx {
		return "(" + s + ")"
	}
	return s
}
