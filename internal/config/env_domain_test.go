package config_test

// vim: nowrap

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/config"
	"github.com/favonia/cloudflare-ddns/internal/domain"
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
		"wildcard1": {true, "*.a", ds{}, ds{w("a")}, true, nil},
		"wildcard2": {true, "*.a.b", ds{}, ds{w("a.b")}, true, nil},
		"test1":     {true, "書.org ,  Bücher.org  ", ds{f("random.org")}, ds{f("xn--rov.org"), f("xn--bcher-kva.org")}, true, nil},
		"test2":     {true, "  \txn--rov.org    ,   xn--Bcher-kva.org  ", ds{f("random.org")}, ds{f("xn--rov.org"), f("xn--bcher-kva.org")}, true, nil},
		"illformed1": {
			true, "xn--:D.org,a.org",
			ds{f("random.org")},
			ds{f("random.org")},
			false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError, "%s (%q) contains an ill-formed domain %q: %v", key, "xn--:D.org,a.org", "xn--:d.org", gomock.Any())
			},
		},
		"illformed2": {
			true, "*.xn--:D.org,a.org",
			ds{f("random.org")},
			ds{f("random.org")},
			false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError, "%s (%q) contains an ill-formed domain %q: %v", key, "*.xn--:D.org,a.org", "*.xn--:d.org", gomock.Any())
			},
		},
		"illformed3": {
			true, "hi.org,(",
			ds{},
			ds{},
			false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError, "%s (%q) has unexpected token %q", key, "hi.org,(", "(")
			},
		},
		"illformed4": {
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
func TestReadDomainMap(t *testing.T) {
	for name, tc := range map[string]struct {
		domains       string
		ip4Domains    string
		ip6Domains    string
		expected      map[ipnet.Type][]domain.Domain
		ok            bool
		prepareMockPP func(*mocks.MockPP)
	}{
		"full": {
			"  a1.com, a2.com", "b1.com,  b2.com,b2.com", "c1.com,c2.com",
			map[ipnet.Type][]domain.Domain{
				ipnet.IP4: {domain.FQDN("a1.com"), domain.FQDN("a2.com"), domain.FQDN("b1.com"), domain.FQDN("b2.com")},
				ipnet.IP6: {domain.FQDN("a1.com"), domain.FQDN("a2.com"), domain.FQDN("c1.com"), domain.FQDN("c2.com")},
			},
			true,
			nil,
		},
		"duplicate": {
			"  a1.com, a1.com", "a1.com,  a1.com,a1.com", "*.a1.com,a1.com,*.a1.com,*.a1.com",
			map[ipnet.Type][]domain.Domain{
				ipnet.IP4: {domain.FQDN("a1.com")},
				ipnet.IP6: {domain.FQDN("a1.com"), domain.Wildcard("a1.com")},
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
			true,
			nil,
		},
		"ill-formed": {
			" ", "   ", "*.*", nil, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError, "%s (%q) contains an ill-formed domain %q: %v", "IP6_DOMAINS", "*.*", "*.*", gomock.Any())
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)

			store(t, "DOMAINS", tc.domains)
			store(t, "IP4_DOMAINS", tc.ip4Domains)
			store(t, "IP6_DOMAINS", tc.ip6Domains)

			var field map[ipnet.Type][]domain.Domain
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}
			ok := config.ReadDomainMap(mockPP, &field)
			require.Equal(t, tc.ok, ok)
			require.ElementsMatch(t, tc.expected[ipnet.IP4], field[ipnet.IP4])
			require.ElementsMatch(t, tc.expected[ipnet.IP6], field[ipnet.IP6])
		})
	}
}
