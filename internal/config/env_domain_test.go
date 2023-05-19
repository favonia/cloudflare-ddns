package config_test

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/config"
	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

//nolint:paralleltest,funlen // environment vars are global
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
			"  a1, a2", "b1,  b2,b2", "c1,c2",
			map[ipnet.Type][]domain.Domain{
				ipnet.IP4: {domain.FQDN("a1"), domain.FQDN("a2"), domain.FQDN("b1"), domain.FQDN("b2")},
				ipnet.IP6: {domain.FQDN("a1"), domain.FQDN("a2"), domain.FQDN("c1"), domain.FQDN("c2")},
			},
			true,
			nil,
		},
		"duplicate": {
			"  a1, a1", "a1,  a1,a1", "*.a1,a1,*.a1,*.a1",
			map[ipnet.Type][]domain.Domain{
				ipnet.IP4: {domain.FQDN("a1")},
				ipnet.IP6: {domain.FQDN("a1"), domain.Wildcard("a1")},
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
				m.EXPECT().Errorf(pp.EmojiUserError,
					"%s (%q) contains an ill-formed domain %q: %v",
					"IP6_DOMAINS", "*.*", "*.*", gomock.Any())
			},
		},
	} {
		tc := tc
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
