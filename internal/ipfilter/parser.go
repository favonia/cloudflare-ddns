package ipfilter

import (
	"net/netip"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/syntax"
)

type formID string

const (
	formKeepAll formID = "keep-all"
	formAddrIn  formID = "addr-in(...)"
	formNot     formID = "!"
	formGroup   formID = "(...)"
	formAnd     formID = "&&"
	formOr      formID = "||"
)

//nolint:gochecknoglobals // Immutable compiled grammar shared by all parse calls.
var grammar = syntax.MustNewPratt(
	syntax.Form(formKeepAll, syntax.Keyword("keep-all")),
	syntax.Form(formAddrIn, syntax.Keyword("addr-in"), syntax.Symbol("("), syntax.Hole(0), syntax.Symbol(")")),
	syntax.Form(formNot, syntax.Symbol("!"), syntax.Hole(30)),
	syntax.Form(formGroup, syntax.Symbol("("), syntax.Hole(0), syntax.Symbol(")")),
	syntax.Form(formAnd, syntax.Hole(20), syntax.Symbol("&&"), syntax.Hole(21)),
	syntax.Form(formOr, syntax.Hole(10), syntax.Symbol("||"), syntax.Hole(11)),
)

// Parse parses a detection filter for one IP family.
func Parse(ppfmt pp.PP, key string, family ipnet.Family, input string) (Filter, bool) {
	tree, err := grammar.Parse(input)
	if err != nil {
		syntaxFault{err: err}.report(ppfmt, key, input)
		return Filter{}, false //nolint:exhaustruct
	}
	expr, f := buildExpr(tree, family, true)
	if f != nil {
		f.report(ppfmt, key, input)
		return Filter{}, false //nolint:exhaustruct
	}
	return Filter{expr: expr, text: expr.string()}, true
}

// buildExpr converts a parse tree into an [expr]. topLevel is true only for the
// whole expression; it is false inside operators, negations, and parentheses.
// "keep-all" is a mode sentinel ("filtering disabled"), not a predicate, so it is
// valid only as the entire expression and rejected anywhere a sub-expression is
// expected.
func buildExpr(tree syntax.Tree[formID], family ipnet.Family, topLevel bool) (expr, fault) {
	switch tree := tree.(type) {
	case syntax.Atom[formID]:
		return nil, notFilterFault{}
	case syntax.Op[formID]:
		switch tree.ID {
		case formKeepAll:
			if !topLevel {
				return nil, keepAllNotTopLevelFault{}
			}
			return keepAllExpr{}, nil
		case formAddrIn:
			return buildAddrIn(tree.Args[0], family)
		case formNot:
			inner, f := buildExpr(tree.Args[0], family, false)
			if f != nil {
				return nil, f
			}
			return notExpr{inner: inner}, nil
		case formGroup:
			return buildExpr(tree.Args[0], family, false)
		case formAnd, formOr:
			left, f := buildExpr(tree.Args[0], family, false)
			if f != nil {
				return nil, f
			}
			right, f := buildExpr(tree.Args[1], family, false)
			if f != nil {
				return nil, f
			}
			return binaryExpr{op: tree.ID, left: left, right: right}, nil
		default:
			return nil, notFilterFault{}
		}
	default:
		return nil, notFilterFault{}
	}
}

func buildAddrIn(tree syntax.Tree[formID], family ipnet.Family) (expr, fault) {
	atom, ok := tree.(syntax.Atom[formID])
	if !ok {
		return nil, notFilterFault{}
	}
	text := atom.Token.Text
	prefix, err := netip.ParsePrefix(text)
	if err != nil {
		if addr, addrErr := netip.ParseAddr(text); addrErr == nil {
			bits := 32
			if addr.Is6() {
				bits = 128
			}
			return nil, bareAddrFault{addr: addr, suggested: netip.PrefixFrom(addr, bits)}
		}
		return nil, prefixParseFault{text: text, err: err}
	}
	if prefix.Addr().Is4() != (family == ipnet.IP4) {
		return nil, wrongFamilyFault{family: family, prefix: prefix}
	}
	return addrInExpr{prefix: prefix}, nil
}
