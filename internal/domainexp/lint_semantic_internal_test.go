// vim: nowrap

package domainexp

import "testing"

func mustParse(t *testing.T, input string) Expr {
	t.Helper()
	tree, err := expressionGrammar.Parse(input)
	if err != nil {
		t.Fatalf("parse %q: %v", input, err)
	}
	// parserState fields are listed explicitly: this repo enables the exhaustruct linter.
	expr, perr := buildExpr(tree, &parserState{emptyCallFunctions: nil, extraComma: false, missingComma: false})
	if perr != nil {
		t.Fatalf("build %q: %v", input, perr)
	}
	return expr
}

func messages(t *testing.T, input string) []string {
	t.Helper()
	findings := semanticFindings(mustParse(t, input))
	out := make([]string, 0, len(findings))
	for _, f := range findings {
		out = append(out, f.message("PROXIED", input))
	}
	return out
}

func TestLintR3Constant(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		input string
		want  string
	}{
		"contradiction-same": {
			"is(a.org) && !is(a.org)",
			`PROXIED ("is(a.org) && !is(a.org)") can never match any domain`,
		},
		"contradiction-is-sub": {
			"is(a.org) && sub(a.org)",
			`PROXIED ("is(a.org) && sub(a.org)") can never match any domain`,
		},
		"contradiction-child-parent": {
			"sub(x.a.org) && !sub(a.org)",
			`PROXIED ("sub(x.a.org) && !sub(a.org)") can never match any domain`,
		},
		// Regression guard for contradictory's full i != j loop: the negated
		// literal appears before the positive one, so a triangular j := i+1 loop
		// would miss this contradiction. See the comment on contradictory.
		"contradiction-negated-first": {
			"!sub(a.org) && sub(x.a.org)",
			`PROXIED ("!sub(a.org) && sub(x.a.org)") can never match any domain`,
		},
		"tautology": {
			"is(a.org) || !is(a.org)",
			`PROXIED ("is(a.org) || !is(a.org)") always matches every domain`,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := messages(t, tc.input)
			if len(got) != 1 || got[0] != tc.want {
				t.Fatalf("messages = %#v, want exactly [%q]", got, tc.want)
			}
		})
	}
}

func TestLintR4Redundant(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		input string
		want  string
	}{
		"duplicate-or": {
			"is(a.org) || is(a.org)",
			`PROXIED ("is(a.org) || is(a.org)") contains a redundant term "is(a.org)"; removing it means the same thing`,
		},
		"subsumed-or": {
			"sub(a.org) || sub(x.a.org)",
			`PROXIED ("sub(a.org) || sub(x.a.org)") contains a redundant term "sub(x.a.org)"; removing it means the same thing`,
		},
		"subsumed-and": {
			"is(x.a.org) && sub(a.org)",
			`PROXIED ("is(x.a.org) && sub(a.org)") contains a redundant term "sub(a.org)"; removing it means the same thing`,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := messages(t, tc.input)
			if len(got) != 1 || got[0] != tc.want {
				t.Fatalf("messages = %#v, want exactly [%q]", got, tc.want)
			}
		})
	}
}

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
