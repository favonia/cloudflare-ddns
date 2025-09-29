// vim: nowrap
package config_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/config"
	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/domainexp"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

//nolint:paralleltest // environment vars are global
func TestReadDomains(t *testing.T) {
	key := keyPrefix + "DOMAINS"
	type ds = []domain.Domain
	type f = domain.FQDN
	type w = domain.Wildcard
	for name, tc := range map[string]struct {
		set           bool
		val           string
		oldField      ds
		newField      ds
		ok            bool
		prepareMockPP func(*mocks.MockPP)
	}{
		"nil":   {false, "", ds{f("test.org")}, ds{}, true, nil},
		"empty": {true, "", ds{f("test.org")}, ds{}, true, nil},
		"star": {
			true, "*",
			ds{},
			ds{},
			false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError, `%s (%q) contains a domain %q that is probably not fully qualified; a fully qualified domain name (FQDN) would look like "*.example.org" or "sub.example.org"`, key, "*", "*")
			},
		},
		"wildcard/1": {true, "*.a", ds{}, ds{w("a")}, true, nil},
		"wildcard/2": {true, "*.a.b", ds{}, ds{w("a.b")}, true, nil},
		"idn/1":      {true, "書.org ,  Bücher.org  ", ds{f("random.org")}, ds{f("xn--rov.org"), f("xn--bcher-kva.org")}, true, nil},
		"idn/2":      {true, "  \txn--rov.org    ,   xn--Bcher-kva.org  ", ds{f("random.org")}, ds{f("xn--rov.org"), f("xn--bcher-kva.org")}, true, nil},
		"ill-formed/1": {
			true, "xn--:D.org,a.org",
			ds{f("random.org")},
			ds{f("random.org")},
			false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError, "%s (%q) contains an ill-formed domain %q: %v", key, "xn--:D.org,a.org", "xn--:d.org", gomock.Any())
			},
		},
		"ill-formed/2": {
			true, "*.xn--:D.org,a.org",
			ds{f("random.org")},
			ds{f("random.org")},
			false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError, "%s (%q) contains an ill-formed domain %q: %v", key, "*.xn--:D.org,a.org", "*.xn--:d.org", gomock.Any())
			},
		},
		"ill-formed/3": {
			true, "hi.org,(",
			ds{},
			ds{},
			false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError, "%s (%q) has unexpected token %q", key, "hi.org,(", "(")
			},
		},
		"ill-formed/4": {
			true, ")",
			ds{},
			ds{},
			false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError, "%s (%q) has unexpected token %q", key, ")", ")")
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			set(t, key, tc.set, tc.val)
			field := tc.oldField
			mockCtrl := gomock.NewController(t)
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}

			ok := config.ReadDomains(mockPP, key, &field)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.newField, field)
		})
	}
}

//nolint:paralleltest // environment vars are global
func TestReadDomainHostIDs(t *testing.T) {
	key := keyPrefix + "IP6_DOMAINS"
	type h = ipnet.HostID
	dh := func(d domain.Domain, h h) domainexp.DomainHostID { return domainexp.DomainHostID{Domain: d, HostID: h} }
	type dhs = []domainexp.DomainHostID
	type f = domain.FQDN
	type w = domain.Wildcard
	for name, tc := range map[string]struct {
		set           bool
		val           string
		oldField      dhs
		newField      dhs
		ok            bool
		prepareMockPP func(*mocks.MockPP)
	}{
		"nil":   {false, "", dhs{dh(f("test.org"), nil)}, dhs{}, true, nil},
		"empty": {true, "", dhs{dh(f("test.org"), nil)}, dhs{}, true, nil},
		"star": {
			true, "*[::1]",
			dhs{},
			dhs{},
			false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError, `%s (%q) contains a domain %q that is probably not fully qualified; a fully qualified domain name (FQDN) would look like "*.example.org" or "sub.example.org"`, key, "*[::1]", "*")
			},
		},
		"wildcard/1": {true, "*.a", dhs{}, dhs{dh(w("a"), nil)}, true, nil},
		"wildcard/1/host": {
			true, "*.a[::2]",
			dhs{},
			dhs{dh(w("a"), ipnet.IP6Suffix{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x02})},
			true, nil,
		},
		"wildcard/2": {true, "*.a.b", dhs{}, dhs{dh(w("a.b"), nil)}, true, nil},
		"idn/1":      {true, "書.org ,  Bücher.org  ", dhs{dh(f("random.org"), nil)}, dhs{dh(f("xn--rov.org"), nil), dh(f("xn--bcher-kva.org"), nil)}, true, nil},
		"idn/1/host": {
			true, "書.org [ 01:02:03:04:05:06 ],  Bücher.org [ 0a:0b:0c:0d:0e:0f ] ",
			dhs{dh(f("random.org"), ipnet.EUI48{0x00, 0x00, 0x00, 0x00, 0x00, 0x00})},
			dhs{
				dh(f("xn--rov.org"), ipnet.EUI48{0x01, 0x02, 0x03, 0x04, 0x05, 0x06}),
				dh(f("xn--bcher-kva.org"), ipnet.EUI48{0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f}),
			},
			true, nil,
		},
		"idn/2": {true, "  \txn--rov.org    ,   xn--Bcher-kva.org  ", dhs{dh(f("random.org"), nil)}, dhs{dh(f("xn--rov.org"), nil), dh(f("xn--bcher-kva.org"), nil)}, true, nil},
		"ill-formed/1": {
			true, "xn--:D.org,a.org",
			dhs{dh(f("random.org"), nil)},
			dhs{dh(f("random.org"), nil)},
			false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError, "%s (%q) contains an ill-formed domain %q: %v", key, "xn--:D.org,a.org", "xn--:d.org", gomock.Any())
			},
		},
		"ill-formed/2": {
			true, "*.xn--:D.org,a.org",
			dhs{},
			dhs{},
			false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError, "%s (%q) contains an ill-formed domain %q: %v", key, "*.xn--:D.org,a.org", "*.xn--:d.org", gomock.Any())
			},
		},
		"ill-formed/3": {
			true, "hi.org,(",
			dhs{},
			dhs{},
			false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError, "%s (%q) has unexpected token %q", key, "hi.org,(", "(")
			},
		},
		"ill-formed/4": {
			true, ")",
			dhs{},
			dhs{},
			false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError, "%s (%q) has unexpected token %q", key, ")", ")")
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			set(t, key, tc.set, tc.val)
			field := tc.oldField
			mockCtrl := gomock.NewController(t)
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}

			ok := config.ReadDomainHostIDs(mockPP, key, &field)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.newField, field)
		})
	}
}

//nolint:paralleltest // environment vars are global
func TestReadDomainMap(t *testing.T) {
	type h = ipnet.HostID
	type f = domain.FQDN
	type w = domain.Wildcard
	for name, tc := range map[string]struct {
		domains        string
		ip4Domains     string
		ip6Domains     string
		expectedDomain map[ipnet.Type][]domain.Domain
		expectedHostID map[domain.Domain]h
		ok             bool
		prepareMockPP  func(*mocks.MockPP)
	}{
		"full": {
			"  a1.com, a2.com", "b1.com,  b2.com,b2.com", "c1.com,c2.com[::1]",
			map[ipnet.Type][]domain.Domain{
				ipnet.IP4: {f("a1.com"), f("a2.com"), f("b1.com"), f("b2.com")},
				ipnet.IP6: {f("a1.com"), f("a2.com"), f("c1.com"), f("c2.com")},
			},
			map[domain.Domain]h{
				f("c2.com"): ipnet.IP6Suffix{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x01},
			},
			true,
			nil,
		},
		"duplicate": {
			"  a1.com, a1.com", "a1.com,  a1.com,a1.com", "*.a1.com,a1.com,*.a1.com[::2],*.a1.com[::0:2]",
			map[ipnet.Type][]domain.Domain{
				ipnet.IP4: {f("a1.com")},
				ipnet.IP6: {f("a1.com"), w("a1.com")},
			},
			map[domain.Domain]h{
				w("a1.com"): ipnet.IP6Suffix{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x02},
			},
			true,
			nil,
		},
		"empty": {
			" ", "   ", "",
			map[ipnet.Type][]domain.Domain{
				ipnet.IP4: {},
				ipnet.IP6: {},
			},
			map[domain.Domain]h{},
			true,
			nil,
		},
		"ill-formed": {
			" ", "   ", "*.*", nil, nil, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError, "%s (%q) contains an ill-formed domain %q: %v", "IP6_DOMAINS", "*.*", "*.*", gomock.Any())
			},
		},
		"ill-formed/different-host-ids": {
			" ", "   ", "a.com[::1],a.com[::2]", nil, nil, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError, "Domain %q is associated with different host IDs %s and %s", "a.com", "::1", "::2")
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)

			store(t, "DOMAINS", tc.domains)
			store(t, "IP4_DOMAINS", tc.ip4Domains)
			store(t, "IP6_DOMAINS", tc.ip6Domains)

			var fieldDomain map[ipnet.Type][]domain.Domain
			var fieldHostID map[domain.Domain]h
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}
			ok := config.ReadDomainMap(mockPP, &fieldDomain, &fieldHostID)
			require.Equal(t, tc.ok, ok)
			require.ElementsMatch(t, tc.expectedDomain[ipnet.IP4], fieldDomain[ipnet.IP4])
			require.ElementsMatch(t, tc.expectedDomain[ipnet.IP6], fieldDomain[ipnet.IP6])
			require.Equal(t, tc.expectedHostID, fieldHostID)
		})
	}
}
