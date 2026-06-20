package ipfilter

import (
	"errors"
	"fmt"
	"net/netip"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/syntax"
)

type bareAddrError struct {
	addr      netip.Addr
	suggested netip.Prefix
}

func (e bareAddrError) Error() string { return "bare IP address is not accepted" }

type prefixParseError struct {
	text string
	err  error
}

func (e prefixParseError) Error() string {
	return fmt.Sprintf("failed to parse %q as a CIDR prefix: %v", e.text, e.err)
}

type wrongFamilyError struct {
	family ipnet.Family
	prefix netip.Prefix
}

func (e wrongFamilyError) Error() string { return "wrong IP family" }

func reportParseError(ppfmt pp.PP, key string, input string, err *syntax.ParseError) {
	var bare bareAddrError
	var wrong wrongFamilyError
	var expected *syntax.ExpectedTokenError
	var missing *syntax.MissingTokenError
	switch {
	case errors.As(err.Cause, &bare):
		ppfmt.Noticef(pp.EmojiUserError,
			`%s (%q) uses bare IP address %q; use %q`,
			key, input, bare.addr.String(), bare.suggested.String())
	case errors.As(err.Cause, &wrong):
		ppfmt.Noticef(pp.EmojiUserError,
			`%s (%q) contains %s prefix %q in an %s filter`,
			key, input, oppositeFamily(wrong.family).Describe(), wrong.prefix.String(), wrong.family.Describe())
	case errors.As(err.Cause, &expected):
		ppfmt.Noticef(pp.EmojiUserError,
			`%s (%q) has unexpected token %q when %q is expected`,
			key, input, expected.Got, expected.Expected)
	case errors.As(err.Cause, &missing):
		ppfmt.Noticef(pp.EmojiUserError,
			`%s (%q) is missing %q at the end`,
			key, input, missing.Expected)
	case errors.Is(err.Cause, errNotFilterExpression), errors.Is(err.Cause, errUnexpectedToken):
		ppfmt.Noticef(pp.EmojiUserError, `%s (%q) is not a detection filter expression`, key, input)
	case errors.Is(err.Cause, syntax.ErrUnexpectedToken):
		ppfmt.Noticef(pp.EmojiUserError, `%s (%q) is not a detection filter expression`, key, input)
	default:
		ppfmt.Noticef(pp.EmojiUserError, `%s (%q) is malformed: %v`, key, input, err.Cause)
	}
}

func oppositeFamily(family ipnet.Family) ipnet.Family {
	if family == ipnet.IP4 {
		return ipnet.IP6
	}
	return ipnet.IP4
}
