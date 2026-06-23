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
