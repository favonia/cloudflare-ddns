// vim: nowrap

package domainexp

import "testing"

func TestExprString(t *testing.T) {
	t.Parallel()
	is := func(d ...string) Expr { return callExpr{function: "is", domains: d} }
	sub := func(d ...string) Expr { return callExpr{function: "sub", domains: d} }
	not := func(e Expr) Expr { return unaryExpr{operator: formNot, operand: e} }
	and := func(l, r Expr) Expr { return binaryExpr{operator: formAnd, left: l, right: r} }
	or := func(l, r Expr) Expr { return binaryExpr{operator: formOr, left: l, right: r} }

	for name, tc := range map[string]struct {
		expr Expr
		want string
	}{
		"true":       {literalExpr{value: true}, "true"},
		"false":      {literalExpr{value: false}, "false"},
		"is":         {is("a.org"), "is(a.org)"},
		"is-multi":   {is("a.org", "b.org"), "is(a.org, b.org)"},
		"not-atom":   {not(is("a.org")), "!is(a.org)"},
		"not-binary": {not(and(is("a.org"), sub("b.org"))), "!(is(a.org) && sub(b.org))"},
		"and-tight":  {or(is("a.org"), and(sub("b.org"), is("c.org"))), "is(a.org) || sub(b.org) && is(c.org)"},
		"or-in-and":  {and(or(is("a.org"), is("b.org")), is("c.org")), "(is(a.org) || is(b.org)) && is(c.org)"},
		"and-chain":  {and(and(is("a.org"), is("b.org")), is("c.org")), "is(a.org) && is(b.org) && is(c.org)"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := exprString(tc.expr); got != tc.want {
				t.Fatalf("exprString = %q, want %q", got, tc.want)
			}
		})
	}
}
