package updater_test

import (
	"context"
	"net/netip"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/config"
	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/updater"
)

//nolint:funlen,paralleltest,maintidx // updater.IPv6MessageDisplayed is a global variable
func TestUpdateIPs(t *testing.T) {
	domain4 := domain.FQDN("ip4.hello")
	domain6 := domain.FQDN("ip6.hello")
	domains := map[ipnet.Type][]domain.Domain{
		ipnet.IP4: {domain4},
		ipnet.IP6: {domain6},
	}

	ip4 := netip.MustParseAddr("127.0.0.1")
	ip6 := netip.MustParseAddr("::1")

	pp4only := func(m *mocks.MockPP) { m.EXPECT().Infof(pp.EmojiInternet, "Detected the %s address: %v", "IPv4", ip4) }
	pp6only := func(m *mocks.MockPP) { m.EXPECT().Infof(pp.EmojiInternet, "Detected the %s address: %v", "IPv6", ip6) }
	ppBoth := func(m *mocks.MockPP) {
		gomock.InOrder(
			m.EXPECT().Infof(pp.EmojiInternet, "Detected the %s address: %v", "IPv4", ip4),
			m.EXPECT().Infof(pp.EmojiInternet, "Detected the %s address: %v", "IPv6", ip6),
		)
	}

	type mockproviders = map[ipnet.Type]func(ppfmt pp.PP, m *mocks.MockProvider)
	provider4 := func(ppfmt pp.PP, m *mocks.MockProvider) { m.EXPECT().GetIP(gomock.Any(), ppfmt, ipnet.IP4).Return(ip4) }
	provider6 := func(ppfmt pp.PP, m *mocks.MockProvider) { m.EXPECT().GetIP(gomock.Any(), ppfmt, ipnet.IP6).Return(ip6) }

	type mockttl = map[domain.Domain]api.TTL
	ttlAuto := mockttl{domain4: api.TTLAuto, domain6: api.TTLAuto}

	type mockproxied = map[domain.Domain]bool
	proxiedNone := mockproxied{domain4: false, domain6: false}
	proxiedBoth := mockproxied{domain4: true, domain6: true}

	for name, tc := range map[string]struct {
		ttl                  mockttl
		proxied              mockproxied
		ok                   bool
		MessageShouldDisplay map[ipnet.Type]bool
		prepareMockPP        func(m *mocks.MockPP)
		prepareMockProvider  mockproviders
		prepareMockSetter    func(ppfmt pp.PP, m *mocks.MockSetter)
	}{
		"none": {
			ttlAuto, proxiedBoth, true, map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: true}, nil, mockproviders{}, nil,
		},
		"ip4only": {
			ttlAuto,
			proxiedNone,
			true,
			map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: true},
			pp4only,
			mockproviders{ipnet.IP4: provider4},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				m.EXPECT().Set(gomock.Any(), ppfmt, domain.FQDN("ip4.hello"), ipnet.IP4, ip4, api.TTL(1), false).Return(true)
			},
		},
		"ip4only/setfail": {
			ttlAuto,
			proxiedBoth,
			false,
			map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: true},
			pp4only,
			mockproviders{ipnet.IP4: provider4},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				m.EXPECT().Set(gomock.Any(), ppfmt, domain.FQDN("ip4.hello"), ipnet.IP4, ip4, api.TTL(1), true).Return(false)
			},
		},
		"ip6only": {
			ttlAuto,
			proxiedNone,
			true,
			map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: true},
			pp6only,
			mockproviders{ipnet.IP6: provider6},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				m.EXPECT().Set(gomock.Any(), ppfmt, domain.FQDN("ip6.hello"), ipnet.IP6, ip6, api.TTL(1), false).Return(true)
			},
		},
		"ip6only/setfail": {
			ttlAuto,
			proxiedBoth,
			false,
			map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: true},
			pp6only,
			mockproviders{ipnet.IP6: provider6},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				m.EXPECT().Set(gomock.Any(), ppfmt, domain.FQDN("ip6.hello"), ipnet.IP6, ip6, api.TTL(1), true).Return(false)
			},
		},
		"both": {
			ttlAuto,
			proxiedNone,
			true,
			map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: true},
			ppBoth,
			mockproviders{ipnet.IP4: provider4, ipnet.IP6: provider6},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				gomock.InOrder(
					m.EXPECT().Set(gomock.Any(), ppfmt, domain.FQDN("ip4.hello"), ipnet.IP4, ip4, api.TTL(1), false).Return(true),
					m.EXPECT().Set(gomock.Any(), ppfmt, domain.FQDN("ip6.hello"), ipnet.IP6, ip6, api.TTL(1), false).Return(true),
				)
			},
		},
		"both/setfail1": {
			ttlAuto,
			proxiedBoth,
			false,
			map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: true},
			ppBoth,
			mockproviders{ipnet.IP4: provider4, ipnet.IP6: provider6},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				gomock.InOrder(
					m.EXPECT().Set(gomock.Any(), ppfmt, domain.FQDN("ip4.hello"), ipnet.IP4, ip4, api.TTL(1), true).Return(false),
					m.EXPECT().Set(gomock.Any(), ppfmt, domain.FQDN("ip6.hello"), ipnet.IP6, ip6, api.TTL(1), true).Return(true),
				)
			},
		},
		"both/setfail2": {
			ttlAuto,
			proxiedNone,
			false,
			map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: true},
			ppBoth,
			mockproviders{ipnet.IP4: provider4, ipnet.IP6: provider6},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				gomock.InOrder(
					m.EXPECT().Set(gomock.Any(), ppfmt, domain.FQDN("ip4.hello"), ipnet.IP4, ip4, api.TTL(1), false).Return(true),
					m.EXPECT().Set(gomock.Any(), ppfmt, domain.FQDN("ip6.hello"), ipnet.IP6, ip6, api.TTL(1), false).Return(false),
				)
			},
		},
		"ip4fails": {
			ttlAuto,
			proxiedBoth,
			false,
			map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: true},
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Errorf(pp.EmojiError, "Failed to detect the %s address", "IPv4"),
					m.EXPECT().Infof(pp.EmojiConfig, "If your network does not support IPv4, you can disable it with IP4_PROVIDER=none"), //nolint:lll
					m.EXPECT().Infof(pp.EmojiInternet, "Detected the %s address: %v", "IPv6", ip6),
				)
			},
			mockproviders{
				ipnet.IP4: func(ppfmt pp.PP, m *mocks.MockProvider) {
					m.EXPECT().GetIP(gomock.Any(), ppfmt, ipnet.IP4).Return(netip.Addr{})
				},
				ipnet.IP6: provider6,
			},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				m.EXPECT().Set(gomock.Any(), ppfmt, domain.FQDN("ip6.hello"), ipnet.IP6, ip6, api.TTL(1), true).Return(true)
			},
		},
		"ip6fails": {
			ttlAuto,
			proxiedNone,
			false,
			map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: true},
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Infof(pp.EmojiInternet, "Detected the %s address: %v", "IPv4", ip4),
					m.EXPECT().Errorf(pp.EmojiError, "Failed to detect the %s address", "IPv6"),
					m.EXPECT().Infof(pp.EmojiConfig, "If you are using Docker or Kubernetes, IPv6 often requires additional setups"),     //nolint:lll
					m.EXPECT().Infof(pp.EmojiConfig, "Read more about IPv6 networks at https://github.com/favonia/cloudflare-ddns"),      //nolint:lll
					m.EXPECT().Infof(pp.EmojiConfig, "If your network does not support IPv6, you can disable it with IP6_PROVIDER=none"), //nolint:lll
				)
			},
			mockproviders{
				ipnet.IP4: provider4,
				ipnet.IP6: func(ppfmt pp.PP, m *mocks.MockProvider) {
					m.EXPECT().GetIP(gomock.Any(), ppfmt, ipnet.IP6).Return(netip.Addr{})
				},
			},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				m.EXPECT().Set(gomock.Any(), ppfmt, domain.FQDN("ip4.hello"), ipnet.IP4, ip4, api.TTL(1), false).Return(true)
			},
		},
		"ip6fails/again": {
			ttlAuto,
			proxiedBoth,
			false,
			map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: false},
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Infof(pp.EmojiInternet, "Detected the %s address: %v", "IPv4", ip4),
					m.EXPECT().Errorf(pp.EmojiError, "Failed to detect the %s address", "IPv6"),
				)
			},
			mockproviders{
				ipnet.IP4: provider4,
				ipnet.IP6: func(ppfmt pp.PP, m *mocks.MockProvider) {
					m.EXPECT().GetIP(gomock.Any(), ppfmt, ipnet.IP6).Return(netip.Addr{})
				},
			},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				m.EXPECT().Set(gomock.Any(), ppfmt, domain.FQDN("ip4.hello"), ipnet.IP4, ip4, api.TTL(1), true).Return(true)
			},
		},
		"bothfail": {
			ttlAuto,
			proxiedNone,
			false,
			map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: true},
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Errorf(pp.EmojiError, "Failed to detect the %s address", "IPv4"),
					m.EXPECT().Infof(pp.EmojiConfig, "If your network does not support IPv4, you can disable it with IP4_PROVIDER=none"), //nolint:lll
					m.EXPECT().Errorf(pp.EmojiError, "Failed to detect the %s address", "IPv6"),
					m.EXPECT().Infof(pp.EmojiConfig, "If you are using Docker or Kubernetes, IPv6 often requires additional setups"),     //nolint:lll
					m.EXPECT().Infof(pp.EmojiConfig, "Read more about IPv6 networks at https://github.com/favonia/cloudflare-ddns"),      //nolint:lll
					m.EXPECT().Infof(pp.EmojiConfig, "If your network does not support IPv6, you can disable it with IP6_PROVIDER=none"), //nolint:lll
				)
			},
			mockproviders{
				ipnet.IP4: func(ppfmt pp.PP, m *mocks.MockProvider) {
					m.EXPECT().GetIP(gomock.Any(), ppfmt, ipnet.IP4).Return(netip.Addr{})
				},
				ipnet.IP6: func(ppfmt pp.PP, m *mocks.MockProvider) {
					m.EXPECT().GetIP(gomock.Any(), ppfmt, ipnet.IP6).Return(netip.Addr{})
				},
			},
			nil,
		},
		"ip4only-proxied-nil": {
			mockttl{},
			mockproxied{},
			true,
			map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: true},
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Infof(pp.EmojiInternet, "Detected the %s address: %v", "IPv4", ip4),
					m.EXPECT().Warningf(pp.EmojiImpossible,
						"TTL[%s] not initialized; please report the bug at https://github.com/favonia/cloudflare-ddns/issues/new",
						"ip4.hello",
					),
					m.EXPECT().Warningf(pp.EmojiImpossible,
						"Proxied[%s] not initialized; please report the bug at https://github.com/favonia/cloudflare-ddns/issues/new",
						"ip4.hello",
					),
				)
			},
			mockproviders{ipnet.IP4: provider4},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				m.EXPECT().Set(gomock.Any(), ppfmt, domain.FQDN("ip4.hello"), ipnet.IP4, ip4, api.TTL(1), false).Return(true)
			},
		},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			ctx := context.Background()
			conf := config.Default()
			conf.Domains = domains
			conf.TTL = tc.ttl
			conf.Proxied = tc.proxied
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}
			for _, ipnet := range [...]ipnet.Type{ipnet.IP4, ipnet.IP6} {
				updater.MessageShouldDisplay[ipnet] = tc.MessageShouldDisplay[ipnet]
				if tc.prepareMockProvider[ipnet] == nil {
					conf.Provider[ipnet] = nil
					continue
				}
				mockProvider := mocks.NewMockProvider(mockCtrl)
				tc.prepareMockProvider[ipnet](mockPP, mockProvider)
				conf.Provider[ipnet] = mockProvider
			}
			mockSetter := mocks.NewMockSetter(mockCtrl)
			if tc.prepareMockSetter != nil {
				tc.prepareMockSetter(mockPP, mockSetter)
			}
			ok := updater.UpdateIPs(ctx, mockPP, conf, mockSetter)
			require.Equal(t, tc.ok, ok)
		})
	}
}

//nolint:funlen,paralleltest // updater.IPv6MessageDisplayed is a global variable
func TestClearIPs(t *testing.T) {
	domain4 := domain.FQDN("ip4.hello")
	domain6 := domain.FQDN("ip6.hello")
	domains := map[ipnet.Type][]domain.Domain{
		ipnet.IP4: {domain4},
		ipnet.IP6: {domain6},
	}

	type mockproviders = map[ipnet.Type]bool

	type mockttl = map[domain.Domain]api.TTL
	ttlAuto := mockttl{domain4: api.TTLAuto, domain6: api.TTLAuto}

	type mockproxied = map[domain.Domain]bool
	proxiedNone := mockproxied{domain4: false, domain6: false}

	for name, tc := range map[string]struct {
		ttl                  mockttl
		proxied              mockproxied
		ok                   bool
		MessageShouldDisplay map[ipnet.Type]bool
		prepareMockPP        func(m *mocks.MockPP)
		prepareMockProvider  mockproviders
		prepareMockSetter    func(ppfmt pp.PP, m *mocks.MockSetter)
	}{
		"none": {
			ttlAuto,
			proxiedNone,
			true,
			map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: true},
			nil,
			mockproviders{},
			nil,
		},
		"ip4only": {
			ttlAuto,
			proxiedNone,
			true,
			map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: true},
			nil,
			mockproviders{ipnet.IP4: true},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				m.EXPECT().Set(gomock.Any(), ppfmt, domain4, ipnet.IP4, netip.Addr{}, api.TTL(1), false).Return(true)
			},
		},
		"ip4only/setfail": {
			ttlAuto,
			proxiedNone,
			false,
			map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: true},
			nil,
			mockproviders{ipnet.IP4: true},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				m.EXPECT().Set(gomock.Any(), ppfmt, domain4, ipnet.IP4, netip.Addr{}, api.TTL(1), false).Return(false)
			},
		},
		"ip6only": {
			ttlAuto,
			proxiedNone,
			true,
			map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: true},
			nil,
			mockproviders{ipnet.IP6: true},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				m.EXPECT().Set(gomock.Any(), ppfmt, domain6, ipnet.IP6, netip.Addr{}, api.TTL(1), false).Return(true)
			},
		},
		"ip6only/setfail": {
			ttlAuto,
			proxiedNone,
			false,
			map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: true},
			nil,
			mockproviders{ipnet.IP6: true},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				m.EXPECT().Set(gomock.Any(), ppfmt, domain6, ipnet.IP6, netip.Addr{}, api.TTL(1), false).Return(false)
			},
		},
		"both": {
			ttlAuto,
			proxiedNone,
			true,
			map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: true},
			nil,
			mockproviders{ipnet.IP4: true, ipnet.IP6: true},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				gomock.InOrder(
					m.EXPECT().Set(gomock.Any(), ppfmt, domain4, ipnet.IP4, netip.Addr{}, api.TTL(1), false).Return(true),
					m.EXPECT().Set(gomock.Any(), ppfmt, domain6, ipnet.IP6, netip.Addr{}, api.TTL(1), false).Return(true),
				)
			},
		},
		"both/setfail1": {
			ttlAuto,
			proxiedNone,
			false,
			map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: true},
			nil,
			mockproviders{ipnet.IP4: true, ipnet.IP6: true},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				gomock.InOrder(
					m.EXPECT().Set(gomock.Any(), ppfmt, domain4, ipnet.IP4, netip.Addr{}, api.TTL(1), false).Return(false),
					m.EXPECT().Set(gomock.Any(), ppfmt, domain6, ipnet.IP6, netip.Addr{}, api.TTL(1), false).Return(true),
				)
			},
		},
		"both/setfail2": {
			ttlAuto,
			proxiedNone,
			false,
			map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: true},
			nil,
			mockproviders{ipnet.IP4: true, ipnet.IP6: true},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				gomock.InOrder(
					m.EXPECT().Set(gomock.Any(), ppfmt, domain4, ipnet.IP4, netip.Addr{}, api.TTL(1), false).Return(true),
					m.EXPECT().Set(gomock.Any(), ppfmt, domain6, ipnet.IP6, netip.Addr{}, api.TTL(1), false).Return(false),
				)
			},
		},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			ctx := context.Background()
			conf := config.Default()
			conf.Domains = domains
			conf.TTL = tc.ttl
			conf.Proxied = tc.proxied
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}
			for _, ipnet := range [...]ipnet.Type{ipnet.IP4, ipnet.IP6} {
				updater.MessageShouldDisplay[ipnet] = tc.MessageShouldDisplay[ipnet]
				if !tc.prepareMockProvider[ipnet] {
					conf.Provider[ipnet] = nil
					continue
				}

				conf.Provider[ipnet] = mocks.NewMockProvider(mockCtrl)
			}
			mockSetter := mocks.NewMockSetter(mockCtrl)
			if tc.prepareMockSetter != nil {
				tc.prepareMockSetter(mockPP, mockSetter)
			}
			ok := updater.ClearIPs(ctx, mockPP, conf, mockSetter)
			require.Equal(t, tc.ok, ok)
		})
	}
}
