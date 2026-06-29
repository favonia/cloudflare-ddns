// vim: nowrap

package domainexp

import (
	"testing"

	"github.com/favonia/cloudflare-ddns/internal/domain"
)

// TestLiteralRelations is a white-box unit test for the set-relation primitives
// subsumes and disjoint. These are private helpers with no exported surface;
// per docs/designs/guides/testing-boundaries.markdown they belong in a
// *_internal_test.go file. The rule behavior they feed (R3/R4) is verified
// black-box through LintExpression in lint_test.go.
func TestLiteralRelations(t *testing.T) {
	t.Parallel()
	is := func(d string) atomSet { return atomSet{kind: litIs, domain: domain.FQDN(d), suffix: ""} }
	sub := func(d string) atomSet { return atomSet{kind: litSub, domain: nil, suffix: domain.Suffix(d)} }

	for name, tc := range map[string]struct {
		p, q     atomSet
		subsumes bool // setP superset-or-equal of setQ
		disjoint bool
	}{
		"is-eq":            {is("a.org"), is("a.org"), true, false},
		"is-neq":           {is("a.org"), is("b.org"), false, true},
		"is-in-sub":        {sub("a.org"), is("x.a.org"), true, false},
		"is-not-in-sub":    {sub("a.org"), is("b.org"), false, true},
		"sub-self":         {sub("a.org"), sub("a.org"), true, false},
		"sub-child":        {sub("a.org"), sub("x.a.org"), true, false},
		"sub-parent":       {sub("x.a.org"), sub("a.org"), false, false},
		"sub-disjoint":     {sub("a.org"), sub("b.org"), false, true},
		"is-sub-never-sup": {is("a.org"), sub("a.org"), false, true},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := subsumes(tc.p, tc.q); got != tc.subsumes {
				t.Errorf("subsumes(%+v,%+v) = %v, want %v", tc.p, tc.q, got, tc.subsumes)
			}
			if got := disjoint(tc.p, tc.q); got != tc.disjoint {
				t.Errorf("disjoint(%+v,%+v) = %v, want %v", tc.p, tc.q, got, tc.disjoint)
			}
		})
	}
}

// TestLitString is a white-box unit test for the litString renderer. R4's
// redundancy passes only ever feed it positive literals (both loops skip
// negated operands, because subsumption-based redundancy is sound only for
// positive sets), so the negated branch is unreachable through LintExpression.
// litString is nonetheless a total renderer of the literal type, like
// exprString; this test pins its full contract, including negation.
func TestLitString(t *testing.T) {
	t.Parallel()
	is := func(negated bool, d string) literal {
		return literal{negated: negated, set: atomSet{kind: litIs, domain: domain.FQDN(d), suffix: ""}}
	}
	sub := func(negated bool, d string) literal {
		return literal{negated: negated, set: atomSet{kind: litSub, domain: nil, suffix: domain.Suffix(d)}}
	}
	for name, tc := range map[string]struct {
		lit  literal
		want string
	}{
		"is":      {is(false, "a.org"), "is(a.org)"},
		"not-is":  {is(true, "a.org"), "!is(a.org)"},
		"sub":     {sub(false, "a.org"), "sub(a.org)"},
		"not-sub": {sub(true, "a.org"), "!sub(a.org)"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := litString(tc.lit); got != tc.want {
				t.Errorf("litString(%+v) = %q, want %q", tc.lit, got, tc.want)
			}
		})
	}
}
