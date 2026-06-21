// vim: nowrap

package ipfilter_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/ipfilter"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func TestParse(t *testing.T) {
	t.Parallel()
	key := "TEST_FILTER"
	for name, tc := range map[string]struct {
		family        ipnet.Family
		input         string
		ok            bool
		prepareMockPP func(m *mocks.MockPP)
	}{
		"keep-all":    {ipnet.IP4, "keep-all", true, nil},
		"addr-in":     {ipnet.IP4, "addr-in(198.51.100.0/24)", true, nil},
		"negation":    {ipnet.IP4, "!addr-in(198.51.100.0/24)", true, nil},
		"disjunction": {ipnet.IP4, "addr-in(198.51.100.0/24)||addr-in(203.0.113.0/24)", true, nil},
		"grouped-negation": {
			ipnet.IP4, "!(addr-in(10.0.0.0/8) || addr-in(192.168.0.0/16))", true, nil,
		},
		"bare-host": {
			ipnet.IP4, "addr-in(8.8.8.8)", false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError,
					`%s (%q) uses bare IP address %q; use %q`,
					key, "addr-in(8.8.8.8)", "8.8.8.8", "8.8.8.8/32")
			},
		},
		"bare-host-ipv6": {
			ipnet.IP6, "addr-in(2001:db8::1)", false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError,
					`%s (%q) uses bare IP address %q; use %q`,
					key, "addr-in(2001:db8::1)", "2001:db8::1", "2001:db8::1/128")
			},
		},
		"ipv6-prefix-in-ipv4-filter": {
			ipnet.IP4, "addr-in(2001:db8::/32)", false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError,
					`%s (%q) contains %s prefix %q in an %s filter`,
					key, "addr-in(2001:db8::/32)", "IPv6", "2001:db8::/32", "IPv4")
			},
		},
		"ipv4-prefix-in-ipv6-filter": {
			ipnet.IP6, "addr-in(198.51.100.0/24)", false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError,
					`%s (%q) contains %s prefix %q in an %s filter`,
					key, "addr-in(198.51.100.0/24)", "IPv4", "198.51.100.0/24", "IPv6")
			},
		},
		"malformed-cidr": {
			ipnet.IP4, "addr-in(not-a-prefix)", false,
			func(m *mocks.MockPP) {
				// The wrapped netip error text is the syntax/netip package's contract, so match it loosely.
				m.EXPECT().Noticef(pp.EmojiUserError,
					`%s (%q) is malformed: failed to parse %q as a CIDR prefix: %v`,
					key, "addr-in(not-a-prefix)", "not-a-prefix", gomock.Any())
			},
		},
		"non-prefix-addr-in-argument": {
			// "keep-all" parses as the keep-all keyword, so addr-in receives a non-atom hole.
			ipnet.IP4, "addr-in(keep-all)", false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError,
					`%s (%q) is not a detection filter expression`,
					key, "addr-in(keep-all)")
			},
		},
		"bare-top-level-prefix": {
			ipnet.IP4, "198.51.100.0/24", false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError,
					`%s (%q) is not a detection filter expression`,
					key, "198.51.100.0/24")
			},
		},
		"trailing-expression": {
			ipnet.IP4, "addr-in(198.51.100.0/24) addr-in(203.0.113.0/24)", false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError,
					`%s (%q) is not a detection filter expression`,
					key, "addr-in(198.51.100.0/24) addr-in(203.0.113.0/24)")
			},
		},
		"dangling-operator": {
			ipnet.IP6, "addr-in(fc00::/7) &&", false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError,
					`%s (%q) is not a detection filter expression`,
					key, "addr-in(fc00::/7) &&")
			},
		},
		"unrecognized-symbol": {
			ipnet.IP4, "keep-all & keep-all", false,
			func(m *mocks.MockPP) {
				// The unrecognized-symbol detail belongs to the syntax package and is asserted there.
				m.EXPECT().Noticef(pp.EmojiUserError,
					`%s (%q) is malformed: %v`,
					key, "keep-all & keep-all", gomock.Any())
			},
		},
		"token-where-open-paren-expected": {
			ipnet.IP4, "addr-in 198.51.100.0/24", false,
			func(m *mocks.MockPP) {
				// The offending token text is determined by the lexer; only the expected token is ours to pin.
				m.EXPECT().Noticef(pp.EmojiUserError,
					`%s (%q) has unexpected token %q when %q is expected`,
					key, "addr-in 198.51.100.0/24", gomock.Any(), "(")
			},
		},
		"missing-closing-paren": {
			ipnet.IP4, "addr-in(198.51.100.0/24", false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError,
					`%s (%q) is missing %q at the end`,
					key, "addr-in(198.51.100.0/24", ")")
			},
		},
		"error-inside-negation": {
			ipnet.IP4, "!addr-in(2001:db8::/32)", false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError,
					`%s (%q) contains %s prefix %q in an %s filter`,
					key, "!addr-in(2001:db8::/32)", "IPv6", "2001:db8::/32", "IPv4")
			},
		},
		"error-in-left-operand": {
			ipnet.IP4, "addr-in(2001:db8::/32) && addr-in(198.51.100.0/24)", false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError,
					`%s (%q) contains %s prefix %q in an %s filter`,
					key, "addr-in(2001:db8::/32) && addr-in(198.51.100.0/24)", "IPv6", "2001:db8::/32", "IPv4")
			},
		},
		"error-in-right-operand": {
			ipnet.IP4, "addr-in(198.51.100.0/24) && addr-in(2001:db8::/32)", false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError,
					`%s (%q) contains %s prefix %q in an %s filter`,
					key, "addr-in(198.51.100.0/24) && addr-in(2001:db8::/32)", "IPv6", "2001:db8::/32", "IPv4")
			},
		},
		"keep-all-in-conjunction": {
			ipnet.IP4, "keep-all && addr-in(198.51.100.0/24)", false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError,
					`%s (%q) may use "keep-all" only as the whole expression, not as part of a larger expression`,
					key, "keep-all && addr-in(198.51.100.0/24)")
			},
		},
		"keep-all-in-negation": {
			ipnet.IP4, "!keep-all", false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError,
					`%s (%q) may use "keep-all" only as the whole expression, not as part of a larger expression`,
					key, "!keep-all")
			},
		},
		"parenthesized-keep-all": {
			ipnet.IP4, "(keep-all)", false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError,
					`%s (%q) may use "keep-all" only as the whole expression, not as part of a larger expression`,
					key, "(keep-all)")
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}

			filter, ok := ipfilter.Parse(mockPP, key, tc.family, tc.input)
			require.Equal(t, tc.ok, ok)
			if tc.ok {
				require.NotEmpty(t, filter.String())
			}
		})
	}
}
