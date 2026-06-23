// vim: nowrap

package domainexp

import "testing"

func TestLiteralRelations(t *testing.T) {
	t.Parallel()
	is := func(d string) atomSet { return atomSet{kind: litIs, domain: d} }
	sub := func(d string) atomSet { return atomSet{kind: litSub, domain: d} }

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
