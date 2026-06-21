// Package ipfilter parses and evaluates detection-side IP filters.
package ipfilter

import (
	"net/netip"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
)

// Filter is a validated detection-side IP filter expression.
type Filter struct {
	expr expr
	text string
}

// KeepAll returns the default filter that keeps every detected raw entry.
func KeepAll() Filter {
	return Filter{expr: keepAllExpr{}, text: "keep-all"}
}

// String returns the canonical filter expression.
func (f Filter) String() string {
	if f.text == "" {
		return KeepAll().String()
	}
	return f.text
}

// IsDefault reports whether f is semantically the default keep-all filter.
func (f Filter) IsDefault() bool {
	return f.String() == "keep-all"
}

// Evaluate evaluates f against one detected raw entry.
func (f Filter) Evaluate(entry ipnet.RawEntry) bool {
	if f.expr == nil {
		return true
	}
	return f.expr.evaluate(entry)
}

// Partition splits entries into those kept and those dropped by f, preserving
// the input ordering within each group.
func (f Filter) Partition(entries []ipnet.RawEntry) (kept, dropped []ipnet.RawEntry) {
	for _, entry := range entries {
		if f.Evaluate(entry) {
			kept = append(kept, entry)
		} else {
			dropped = append(dropped, entry)
		}
	}
	return kept, dropped
}

type expr interface {
	evaluate(entry ipnet.RawEntry) bool
	string() string
}

// keepAllExpr is the expression form of the keep-all sentinel: it keeps every
// detected entry. It is the only literal the grammar can produce, because the
// parser rejects keep-all anywhere a sub-expression is expected.
type keepAllExpr struct{}

func (keepAllExpr) evaluate(ipnet.RawEntry) bool { return true }
func (keepAllExpr) string() string               { return "keep-all" }

type addrInExpr struct {
	prefix netip.Prefix
}

func (e addrInExpr) evaluate(entry ipnet.RawEntry) bool {
	return e.prefix.Contains(entry.Addr())
}

func (e addrInExpr) string() string {
	return "addr-in(" + e.prefix.String() + ")"
}

type notExpr struct {
	inner expr
}

func (e notExpr) evaluate(entry ipnet.RawEntry) bool { return !e.inner.evaluate(entry) }
func (e notExpr) string() string                     { return "!(" + e.inner.string() + ")" }

type binaryExpr struct {
	op          formID
	left, right expr
}

func (e binaryExpr) evaluate(entry ipnet.RawEntry) bool {
	switch e.op {
	case formAnd:
		return e.left.evaluate(entry) && e.right.evaluate(entry)
	case formOr:
		return e.left.evaluate(entry) || e.right.evaluate(entry)
	default:
		return false
	}
}

func (e binaryExpr) string() string {
	return "(" + e.left.string() + " " + string(e.op) + " " + e.right.string() + ")"
}
