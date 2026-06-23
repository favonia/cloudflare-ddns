package domainexp

// This file holds the semantic lint pass (R3, R4). It reasons about is/sub set
// relations over single-domain atoms. Multi-domain atoms and unrecognized
// shapes are treated as opaque and produce no findings.

type litKind int

const (
	litIs litKind = iota
	litSub
)

// atomSet is the set denoted by a single-domain is/sub atom. is(d) is the
// singleton {d}; sub(d) is the set of strict subdomains of d.
type atomSet struct {
	kind   litKind
	domain string // ASCII
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
		// {p} can only contain q if q is exactly {p}.
		return q.kind == litIs && q.domain == p.domain
	case litSub:
		switch q.kind {
		case litIs:
			// q == {q.domain} subset of sub(p) iff q.domain is a strict subdomain of p.
			return hasStrictSuffix(q.domain, p.domain)
		case litSub:
			// sub(q) subset of sub(p) iff q == p or q is under p.
			return q.domain == p.domain || hasStrictSuffix(q.domain, p.domain)
		}
	}
	return false
}

// disjoint reports whether set(p) and set(q) share no element.
func disjoint(p, q atomSet) bool {
	switch {
	case p.kind == litIs && q.kind == litIs:
		return p.domain != q.domain
	case p.kind == litIs && q.kind == litSub:
		return !hasStrictSuffix(p.domain, q.domain)
	case p.kind == litSub && q.kind == litIs:
		return !hasStrictSuffix(q.domain, p.domain)
	default: // both sub: share a strict subdomain iff their domains are nested or equal.
		return p.domain != q.domain &&
			!hasStrictSuffix(p.domain, q.domain) &&
			!hasStrictSuffix(q.domain, p.domain)
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
	case callExpr:
		if l, ok := asLiteral(e, false); ok {
			return term{lit: &l, con: nil}
		}
	case unaryExpr:
		if call, ok := e.operand.(callExpr); ok {
			if l, ok := asLiteral(call, true); ok {
				return term{lit: &l, con: nil}
			}
		}
	}
	return term{lit: nil, con: nil} // opaque
}

// asLiteral converts a single-domain is/sub call into a literal. Multi-domain
// or empty calls are not representable and return ok=false.
func asLiteral(c callExpr, negated bool) (literal, bool) {
	if len(c.domains) != 1 {
		return literal{negated: false, set: atomSet{kind: litIs, domain: ""}}, false
	}
	kind := litIs
	if c.function == "sub" {
		kind = litSub
	}
	return literal{negated: negated, set: atomSet{kind: kind, domain: c.domains[0]}}, true
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
	return findings
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
	return findings
}

// contradictory reports whether some pair of conjoined literals is always false:
// two positives with disjoint sets, or a positive p and a negative !q with
// set(p) subset of set(q).
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
