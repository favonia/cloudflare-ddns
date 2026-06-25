package domainexp

// This file holds the semantic lint pass (R3, R4). It reasons about is/sub set
// relations over single-domain atoms. Multi-domain atoms and unrecognized
// shapes are treated as opaque and produce no findings.

import "github.com/favonia/cloudflare-ddns/internal/domain"

type litKind int

const (
	litIs litKind = iota
	litSub
)

// atomSet is the set denoted by a single-domain is/sub atom. is(d) is the
// singleton {d}; sub(s) is the set of strict subdomains of suffix s.
type atomSet struct {
	kind   litKind
	domain domain.Domain // valid when kind == litIs
	suffix domain.Suffix // valid when kind == litSub
}

// literal is an atomSet with a polarity.
type literal struct {
	negated bool
	set     atomSet
}

// subsumes reports whether set(p) is a superset of (or equal to) set(q).
func subsumes(p, q atomSet) bool {
	switch p.kind {
	case litIs:
		return q.kind == litIs && q.domain.DNSNameASCII() == p.domain.DNSNameASCII()
	case litSub:
		switch q.kind {
		case litIs:
			return q.domain.HasStrictSuffix(p.suffix)
		case litSub:
			return q.suffix == p.suffix || q.suffix.HasStrictSuffix(p.suffix)
		}
	}
	return false
}

// disjoint reports whether set(p) and set(q) share no element.
func disjoint(p, q atomSet) bool {
	switch {
	case p.kind == litIs && q.kind == litIs:
		return p.domain.DNSNameASCII() != q.domain.DNSNameASCII()
	case p.kind == litIs && q.kind == litSub:
		return !p.domain.HasStrictSuffix(q.suffix)
	case p.kind == litSub && q.kind == litIs:
		return !q.domain.HasStrictSuffix(p.suffix)
	default: // both sub
		return p.suffix != q.suffix &&
			!p.suffix.HasStrictSuffix(q.suffix) &&
			!q.suffix.HasStrictSuffix(p.suffix)
	}
}

// constantFinding is R3: a subexpression that is statically always true/false.
type constantFinding struct {
	value bool
}

func (f constantFinding) message(key, input string) string {
	if f.value {
		return key + ` ("` + input + `") always matches every domain`
	}
	return key + ` ("` + input + `") can never match any domain`
}

// term is one operand of a flattened &&/|| chain. Pointer fields avoid nested
// exhaustruct construction: nil means "not that kind of term".
type term struct {
	lit *literal // non-nil if the operand is a single-domain is/sub literal
	con *bool    // non-nil if the operand is a Boolean constant
}

func classifyTerm(e Expr) term {
	switch e := e.(type) {
	case literalExpr:
		v := e.value
		return term{lit: nil, con: &v}
	case isExpr:
		if l, ok := asLiteral(e, false); ok {
			return term{lit: &l, con: nil}
		}
	case subExpr:
		if l, ok := asLiteral(e, false); ok {
			return term{lit: &l, con: nil}
		}
	case unaryExpr:
		if l, ok := asLiteral(e.operand, true); ok {
			return term{lit: &l, con: nil}
		}
	}
	return term{lit: nil, con: nil} // opaque
}

// asLiteral converts a single-domain is/sub call into a literal. Multi-domain
// or empty calls are not single literals and return ok=false.
func asLiteral(e Expr, negated bool) (literal, bool) {
	switch e := e.(type) {
	case isExpr:
		if len(e.domains) != 1 {
			return literal{negated: false, set: atomSet{}}, false
		}
		return literal{negated: negated, set: atomSet{kind: litIs, domain: e.domains[0]}}, true
	case subExpr:
		if len(e.suffixes) != 1 {
			return literal{negated: false, set: atomSet{}}, false
		}
		return literal{negated: negated, set: atomSet{kind: litSub, suffix: e.suffixes[0]}}, true
	default:
		return literal{negated: false, set: atomSet{}}, false
	}
}

func semanticFindings(expr Expr) []finding {
	var findings []finding
	walk(expr, func(e Expr) {
		b, ok := e.(binaryExpr)
		if !ok {
			return
		}
		switch b.operator {
		case formAnd:
			findings = append(findings, analyzeConjunction(flatten(e, formAnd))...)
		case formOr:
			findings = append(findings, analyzeDisjunction(flatten(e, formOr))...)
		default:
			// other binary forms are not && or || — ignore
		}
	})
	// Report R3 constancy whenever the expression both contains an always-true
	// sub atom such as sub(.) and is statically determined as a whole. This
	// complements the &&/|| chain analysis above, which only fires when a
	// literalExpr constant or an is/sub relation forces the result. The guard on
	// containsAlwaysTrueAtom keeps R3 from flagging plain literal expressions like
	// "true" or "!true" (those are not the kind of non-obvious constancy R3 warns
	// about). The constValue evaluation only reports when the whole expression is
	// statically determined, so e.g. "sub(.) && is(a.org)" (not constant) is left
	// to the redundancy pass instead of being mislabeled constant-true.
	if containsAlwaysTrueAtom(expr) {
		if value, ok := constValue(expr); ok {
			findings = append(findings, constantFinding{value: value})
		}
	}
	return findings
}

// isAlwaysTrueAtom reports whether e is a sub() atom that matches every domain,
// i.e. a single root suffix. sub(.) is the only such atom.
func isAlwaysTrueAtom(e Expr) bool {
	s, ok := e.(subExpr)
	return ok && len(s.suffixes) == 1 && s.suffixes[0] == domain.Suffix("")
}

// containsAlwaysTrueAtom reports whether e contains an always-true sub atom.
func containsAlwaysTrueAtom(e Expr) bool {
	found := false
	walk(e, func(n Expr) {
		if isAlwaysTrueAtom(n) {
			found = true
		}
	})
	return found
}

// constValue computes the static truth value of e, treating an always-true sub
// atom (sub(.)) as true. known is false when the value depends on the matched
// domain. Negation flips the value; && and || short-circuit on a known operand.
// is()/sub() atoms other than the always-true one are treated as unknown.
func constValue(e Expr) (value bool, known bool) {
	switch e := e.(type) {
	case literalExpr:
		return e.value, true
	case subExpr:
		if isAlwaysTrueAtom(e) {
			return true, true
		}
		return false, false
	case unaryExpr:
		if v, ok := constValue(e.operand); ok {
			return !v, true
		}
		return false, false
	case binaryExpr:
		lv, lok := constValue(e.left)
		rv, rok := constValue(e.right)
		switch e.operator {
		case formAnd:
			if (lok && !lv) || (rok && !rv) {
				return false, true
			}
			if lok && rok {
				return lv && rv, true
			}
		case formOr:
			if (lok && lv) || (rok && rv) {
				return true, true
			}
			if lok && rok {
				return lv || rv, true
			}
		default:
			// other binary forms are not && or || — treat as unknown
		}
		return false, false
	default:
		return false, false
	}
}

func analyzeConjunction(operands []Expr) []finding {
	var findings []finding
	var lits []literal
	for _, e := range operands {
		switch tm := classifyTerm(e); {
		case tm.con != nil && !*tm.con:
			findings = append(findings, constantFinding{value: false})
		case tm.lit != nil:
			lits = append(lits, *tm.lit)
		}
	}
	if contradictory(lits) {
		findings = append(findings, constantFinding{value: false})
	}
	for _, e := range operands {
		if tm := classifyTerm(e); tm.con != nil && *tm.con && len(operands) > 1 {
			findings = append(findings, redundantTermFinding{term: "true"})
		}
	}
	for i := range lits {
		for j := range lits {
			if i == j || lits[i].negated || lits[j].negated {
				continue
			}
			// In A && B, if set(A) superset-or-equal set(B) then A is redundant.
			// Use a strict tie-break for equal sets: drop the later index only.
			if subsumes(lits[i].set, lits[j].set) && (lits[i].set != lits[j].set || i >= j) {
				findings = append(findings, redundantTermFinding{term: litString(lits[i])})
			}
		}
	}
	return findings
}

func analyzeDisjunction(operands []Expr) []finding {
	var findings []finding
	var lits []literal
	for _, e := range operands {
		switch tm := classifyTerm(e); {
		case tm.con != nil && *tm.con:
			findings = append(findings, constantFinding{value: true})
		case tm.lit != nil:
			lits = append(lits, *tm.lit)
		}
	}
	if tautological(lits) {
		findings = append(findings, constantFinding{value: true})
	}
	for _, e := range operands {
		if tm := classifyTerm(e); tm.con != nil && !*tm.con && len(operands) > 1 {
			findings = append(findings, redundantTermFinding{term: "false"})
		}
	}
	for i := range lits {
		for j := range lits {
			if i == j || lits[i].negated || lits[j].negated {
				continue
			}
			// In A || B, if set(A) subset-or-equal set(B) then A is redundant.
			if subsumes(lits[j].set, lits[i].set) && (lits[i].set != lits[j].set || i >= j) {
				findings = append(findings, redundantTermFinding{term: litString(lits[i])})
			}
		}
	}
	return findings
}

// contradictory reports whether some pair of conjoined literals is always false:
// two positives with disjoint sets, or a positive p and a negative !q with
// set(p) subset of set(q).
//
// The loop must visit every ordered pair (full i != j), not a triangular
// j := i+1, because the positive/negated branch is asymmetric: it only fires
// when the positive literal is a and the negated literal is b. A triangular
// loop would miss contradictions where the negated literal comes first, e.g.
// "!sub(a.org) && sub(x.a.org)".
func contradictory(lits []literal) bool {
	for i := range lits {
		for j := range lits {
			if i == j {
				continue
			}
			a, b := lits[i], lits[j]
			if !a.negated && !b.negated && disjoint(a.set, b.set) {
				return true
			}
			if !a.negated && b.negated && subsumes(b.set, a.set) {
				return true
			}
		}
	}
	return false
}

// tautological reports whether some pair of disjoined literals always holds:
// a positive p and a negative !q with set(q) subset of set(p), so p || !q is all.
func tautological(lits []literal) bool {
	for i := range lits {
		for j := range lits {
			if i == j {
				continue
			}
			a, b := lits[i], lits[j]
			if !a.negated && b.negated && subsumes(a.set, b.set) {
				return true
			}
		}
	}
	return false
}

// redundantTermFinding is R4: a literal with no effect given another.
type redundantTermFinding struct {
	term string // canonical text of the redundant atom
}

func (f redundantTermFinding) message(key, input string) string {
	return key + ` ("` + input + `") contains a redundant term "` + f.term +
		`"; removing it means the same thing`
}

// litString renders a literal back to canonical text for messages.
func litString(l literal) string {
	var atom string
	if l.set.kind == litSub {
		atom = exprString(subExpr{suffixes: []domain.Suffix{l.set.suffix}})
	} else {
		atom = exprString(isExpr{domains: []domain.Domain{l.set.domain}})
	}
	if l.negated {
		return "!" + atom
	}
	return atom
}
