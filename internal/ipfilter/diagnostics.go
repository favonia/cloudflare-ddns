package ipfilter

import (
	"errors"
	"net/netip"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/syntax"
)

// fault is a detection-filter problem found while parsing one filter expression.
// Each variant knows how to report itself; the set is closed to this package, so
// reporting is a method call rather than an [errors.As] ladder over [error] values.
type fault interface {
	report(ppfmt pp.PP, key string, input string)
}

// bareAddrFault is a predicate argument written as a bare IP address.
type bareAddrFault struct {
	addr      netip.Addr
	suggested netip.Prefix
}

func (f bareAddrFault) report(ppfmt pp.PP, key string, input string) {
	ppfmt.Noticef(pp.EmojiUserError,
		`%s (%q) uses bare IP address %q; use %q`,
		key, input, f.addr.String(), f.suggested.String())
}

// wrongFamilyFault is a predicate argument whose IP family does not match the filter.
type wrongFamilyFault struct {
	family ipnet.Family
	prefix netip.Prefix
}

func (f wrongFamilyFault) report(ppfmt pp.PP, key string, input string) {
	ppfmt.Noticef(pp.EmojiUserError,
		`%s (%q) contains %s prefix %q in an %s filter`,
		key, input, oppositeFamily(f.family).Describe(), f.prefix.String(), f.family.Describe())
}

// prefixParseFault is a predicate argument that is not a valid CIDR prefix.
type prefixParseFault struct {
	text string
	err  error
}

func (f prefixParseFault) report(ppfmt pp.PP, key string, input string) {
	ppfmt.Noticef(pp.EmojiUserError,
		`%s (%q) is malformed: failed to parse %q as a CIDR prefix: %v`,
		key, input, f.text, f.err)
}

// notFilterFault is a structurally valid token sequence that is not a filter expression.
type notFilterFault struct{}

func (notFilterFault) report(ppfmt pp.PP, key string, input string) {
	ppfmt.Noticef(pp.EmojiUserError, `%s (%q) is not a detection filter expression`, key, input)
}

// syntaxFault wraps a foreign error from the shared [syntax] parser. Inspecting it
// with [errors.As]/[errors.Is] is appropriate here because the error crosses a
// package boundary; the other faults are minted and consumed inside this package.
type syntaxFault struct {
	err *syntax.ParseError
}

func (f syntaxFault) report(ppfmt pp.PP, key string, input string) {
	var expected *syntax.ExpectedTokenError
	var missing *syntax.MissingTokenError
	switch {
	case errors.As(f.err.Cause, &expected):
		ppfmt.Noticef(pp.EmojiUserError,
			`%s (%q) has unexpected token %q when %q is expected`,
			key, input, expected.Got, expected.Expected)
	case errors.As(f.err.Cause, &missing):
		ppfmt.Noticef(pp.EmojiUserError,
			`%s (%q) is missing %q at the end`,
			key, input, missing.Expected)
	case errors.Is(f.err.Cause, syntax.ErrUnexpectedToken), errors.Is(f.err.Cause, syntax.ErrUnexpectedEOF):
		ppfmt.Noticef(pp.EmojiUserError, `%s (%q) is not a detection filter expression`, key, input)
	default:
		ppfmt.Noticef(pp.EmojiUserError, `%s (%q) is malformed: %v`, key, input, f.err.Cause)
	}
}

func oppositeFamily(family ipnet.Family) ipnet.Family {
	if family == ipnet.IP4 {
		return ipnet.IP6
	}
	return ipnet.IP4
}
