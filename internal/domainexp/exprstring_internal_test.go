// vim: nowrap

package domainexp

import (
	"testing"

	"github.com/favonia/cloudflare-ddns/internal/domain"
)

func TestExprString(t *testing.T) {
	t.Parallel()
	toDomains := func(ss ...string) []domain.Domain {
		ds := make([]domain.Domain, len(ss))
		for i, s := range ss {
			d, err := domain.New(s)
			if err != nil {
				t.Fatalf("domain.New(%q): %v", s, err)
			}
			ds[i] = d
		}
		return ds
	}
	toSuffixes := func(ss ...string) []domain.Suffix {
		suffixes := make([]domain.Suffix, len(ss))
		for i, s := range ss {
			suffix, err := domain.NewSuffix(s)
			if err != nil {
				t.Fatalf("domain.NewSuffix(%q): %v", s, err)
			}
			suffixes[i] = suffix
		}
		return suffixes
	}
	is := func(d ...string) Expr { return isExpr{domains: toDomains(d...)} }
	sub := func(d ...string) Expr { return subExpr{suffixes: toSuffixes(d...)} }
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
