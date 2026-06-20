package ipfilter

import (
	"errors"
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

var (
	errNotFilterExpression = errors.New("not a detection filter expression")
	errUnexpectedToken     = errors.New("unexpected token in detection filter expression")
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
		if errors.Is(err, syntax.ErrUnexpectedEOF) {
			err = &syntax.ParseError{Span: err.Span, Cause: errNotFilterExpression}
		}
		reportParseError(ppfmt, key, input, err)
		return Filter{}, false //nolint:exhaustruct
	}
	expr, err := buildExpr(tree, family, input)
	if err != nil {
		reportParseError(ppfmt, key, input, err)
		return Filter{}, false //nolint:exhaustruct
	}
	return Filter{expr: expr, text: expr.string()}, true
}

func buildExpr(tree syntax.Tree[formID], family ipnet.Family, input string) (expr, *syntax.ParseError) {
	switch tree := tree.(type) {
	case syntax.Atom[formID]:
		return nil, &syntax.ParseError{Span: tree.Token.Span, Cause: errUnexpectedToken}
	case syntax.Op[formID]:
		switch tree.ID {
		case formKeepAll:
			return literalExpr(true), nil
		case formAddrIn:
			return buildAddrIn(tree.Args[0], family, input)
		case formNot:
			inner, err := buildExpr(tree.Args[0], family, input)
			if err != nil {
				return nil, err
			}
			return notExpr{inner: inner}, nil
		case formGroup:
			return buildExpr(tree.Args[0], family, input)
		case formAnd, formOr:
			left, err := buildExpr(tree.Args[0], family, input)
			if err != nil {
				return nil, err
			}
			right, err := buildExpr(tree.Args[1], family, input)
			if err != nil {
				return nil, err
			}
			return binaryExpr{op: tree.ID, left: left, right: right}, nil
		default:
			return nil, &syntax.ParseError{Span: tree.Span(), Cause: errUnexpectedToken}
		}
	default:
		return nil, &syntax.ParseError{Span: syntax.Span{Start: 0, End: len(input)}, Cause: errNotFilterExpression}
	}
}

func buildAddrIn(tree syntax.Tree[formID], family ipnet.Family, input string) (expr, *syntax.ParseError) {
	atom, ok := tree.(syntax.Atom[formID])
	if !ok {
		return nil, &syntax.ParseError{Span: tree.Span(), Cause: errUnexpectedToken}
	}
	text := atom.Token.Text
	prefix, err := netip.ParsePrefix(text)
	if err != nil {
		if addr, addrErr := netip.ParseAddr(text); addrErr == nil {
			bits := 32
			if addr.Is6() {
				bits = 128
			}
			return nil, &syntax.ParseError{
				Span:  atom.Token.Span,
				Cause: bareAddrError{addr: addr, suggested: netip.PrefixFrom(addr, bits)},
			}
		}
		return nil, &syntax.ParseError{Span: atom.Token.Span, Cause: prefixParseError{text: text, err: err}}
	}
	if prefix.Addr().Is4() != (family == ipnet.IP4) {
		return nil, &syntax.ParseError{Span: atom.Token.Span, Cause: wrongFamilyError{family: family, prefix: prefix}}
	}
	return addrInExpr{prefix: prefix}, nil
}
