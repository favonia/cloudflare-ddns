package updater_test

import (
	"context"
	"net/netip"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/config"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/updater"
)

//nolint:funlen,paralleltest,maintidx // updater.IPv6MessageDisplayed is a global variable
func TestUpdateIPs(t *testing.T) {
	domain4 := api.FQDN("ip4.hello")
	domain6 := api.FQDN("ip6.hello")
	domains := map[ipnet.Type][]api.Domain{
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

	type mockproxied = map[api.Domain]bool
	proxiedNone := mockproxied{domain4: false, domain6: false}
	proxiedBoth := mockproxied{domain4: true, domain6: true}

	for name, tc := range map[string]struct {
		proxiedByDomain      mockproxied
		ok                   bool
		MessageShouldDisplay map[ipnet.Type]bool
		prepareMockPP        func(m *mocks.MockPP)
		prepareMockProvider  mockproviders
		prepareMockSetter    func(ppfmt pp.PP, m *mocks.MockSetter)
	}{
		"none": {proxiedBoth, true, map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: true}, nil, mockproviders{}, nil},
		"ip4only": {
			proxiedNone,
			true,
			map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: true},
			pp4only,
			mockproviders{ipnet.IP4: provider4},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				m.EXPECT().Set(gomock.Any(), ppfmt, api.FQDN("ip4.hello"), ipnet.IP4, ip4, false).Return(true)
			},
		},
		"ip4only/setfail": {
			proxiedBoth,
			false,
			map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: true},
			pp4only,
			mockproviders{ipnet.IP4: provider4},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				m.EXPECT().Set(gomock.Any(), ppfmt, api.FQDN("ip4.hello"), ipnet.IP4, ip4, true).Return(false)
			},
		},
		"ip6only": {
			proxiedNone,
			true,
			map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: true},
			pp6only,
			mockproviders{ipnet.IP6: provider6},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				m.EXPECT().Set(gomock.Any(), ppfmt, api.FQDN("ip6.hello"), ipnet.IP6, ip6, false).Return(true)
			},
		},
		"ip6only/setfail": {
			proxiedBoth,
			false,
			map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: true},
			pp6only,
			mockproviders{ipnet.IP6: provider6},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				m.EXPECT().Set(gomock.Any(), ppfmt, api.FQDN("ip6.hello"), ipnet.IP6, ip6, true).Return(false)
			},
		},
		"both": {
			proxiedNone,
			true,
			map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: true},
			ppBoth,
			mockproviders{ipnet.IP4: provider4, ipnet.IP6: provider6},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				gomock.InOrder(
					m.EXPECT().Set(gomock.Any(), ppfmt, api.FQDN("ip4.hello"), ipnet.IP4, ip4, false).Return(true),
					m.EXPECT().Set(gomock.Any(), ppfmt, api.FQDN("ip6.hello"), ipnet.IP6, ip6, false).Return(true),
				)
			},
		},
		"both/setfail1": {
			proxiedBoth,
			false,
			map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: true},
			ppBoth,
			mockproviders{ipnet.IP4: provider4, ipnet.IP6: provider6},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				gomock.InOrder(
					m.EXPECT().Set(gomock.Any(), ppfmt, api.FQDN("ip4.hello"), ipnet.IP4, ip4, true).Return(false),
					m.EXPECT().Set(gomock.Any(), ppfmt, api.FQDN("ip6.hello"), ipnet.IP6, ip6, true).Return(true),
				)
			},
		},
		"both/setfail2": {
			proxiedNone,
			false,
			map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: true},
			ppBoth,
			mockproviders{ipnet.IP4: provider4, ipnet.IP6: provider6},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				gomock.InOrder(
					m.EXPECT().Set(gomock.Any(), ppfmt, api.FQDN("ip4.hello"), ipnet.IP4, ip4, false).Return(true),
					m.EXPECT().Set(gomock.Any(), ppfmt, api.FQDN("ip6.hello"), ipnet.IP6, ip6, false).Return(false),
				)
			},
		},
		"ip4fails": {
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
				m.EXPECT().Set(gomock.Any(), ppfmt, api.FQDN("ip6.hello"), ipnet.IP6, ip6, true).Return(true)
			},
		},
		"ip6fails": {
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
				m.EXPECT().Set(gomock.Any(), ppfmt, api.FQDN("ip4.hello"), ipnet.IP4, ip4, false).Return(true)
			},
		},
		"ip6fails/again": {
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
				m.EXPECT().Set(gomock.Any(), ppfmt, api.FQDN("ip4.hello"), ipnet.IP4, ip4, true).Return(true)
			},
		},
		"bothfail": {
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
			mockproxied{},
			true,
			map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: true},
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Infof(pp.EmojiInternet, "Detected the %s address: %v", "IPv4", ip4),
					m.EXPECT().Warningf(pp.EmojiImpossible,
						"Internal failure: ProxiedByDomain[%s] was not set, and is reset to %t",
						domain4.Describe(), false,
					),
					m.EXPECT().Warningf(pp.EmojiImpossible,
						"Please report the bug at https://github.com/favonia/cloudflare-ddns/issues/new",
					),
				)
			},
			mockproviders{ipnet.IP4: provider4},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				m.EXPECT().Set(gomock.Any(), ppfmt, api.FQDN("ip4.hello"), ipnet.IP4, ip4, false).Return(true)
			},
		},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			ctx := context.Background()
			conf := config.Default()
			conf.Domains = domains
			conf.ProxiedByDomain = tc.proxiedByDomain
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
	domain4 := api.FQDN("ip4.hello")
	domain6 := api.FQDN("ip6.hello")
	domains := map[ipnet.Type][]api.Domain{
		ipnet.IP4: {domain4},
		ipnet.IP6: {domain6},
	}

	type mockproviders = map[ipnet.Type]bool

	type mockproxied = map[api.Domain]bool
	proxiedNone := mockproxied{domain4: false, domain6: false}
	proxiedBoth := mockproxied{domain4: true, domain6: true}

	for name, tc := range map[string]struct {
		ok                   bool
		MessageShouldDisplay map[ipnet.Type]bool
		prepareMockPP        func(m *mocks.MockPP)
		prepareMockProvider  mockproviders
		prepareMockSetter    func(ppfmt pp.PP, m *mocks.MockSetter, proxied bool)
	}{
		"none": {
			true,
			map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: true},
			nil,
			mockproviders{},
			nil,
		},
		"ip4only": {
			true,
			map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: true},
			nil,
			mockproviders{ipnet.IP4: true},
			func(ppfmt pp.PP, m *mocks.MockSetter, proxied bool) {
				m.EXPECT().Set(gomock.Any(), ppfmt, api.FQDN("ip4.hello"), ipnet.IP4, netip.Addr{}, proxied).Return(true)
			},
		},
		"ip4only/setfail": {
			false,
			map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: true},
			nil,
			mockproviders{ipnet.IP4: true},
			func(ppfmt pp.PP, m *mocks.MockSetter, proxied bool) {
				m.EXPECT().Set(gomock.Any(), ppfmt, api.FQDN("ip4.hello"), ipnet.IP4, netip.Addr{}, proxied).Return(false)
			},
		},
		"ip6only": {
			true,
			map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: true},
			nil,
			mockproviders{ipnet.IP6: true},
			func(ppfmt pp.PP, m *mocks.MockSetter, proxied bool) {
				m.EXPECT().Set(gomock.Any(), ppfmt, api.FQDN("ip6.hello"), ipnet.IP6, netip.Addr{}, proxied).Return(true)
			},
		},
		"ip6only/setfail": {
			false,
			map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: true},
			nil,
			mockproviders{ipnet.IP6: true},
			func(ppfmt pp.PP, m *mocks.MockSetter, proxied bool) {
				m.EXPECT().Set(gomock.Any(), ppfmt, api.FQDN("ip6.hello"), ipnet.IP6, netip.Addr{}, proxied).Return(false)
			},
		},
		"both": {
			true,
			map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: true},
			nil,
			mockproviders{ipnet.IP4: true, ipnet.IP6: true},
			func(ppfmt pp.PP, m *mocks.MockSetter, proxied bool) {
				gomock.InOrder(
					m.EXPECT().Set(gomock.Any(), ppfmt, api.FQDN("ip4.hello"), ipnet.IP4, netip.Addr{}, proxied).Return(true),
					m.EXPECT().Set(gomock.Any(), ppfmt, api.FQDN("ip6.hello"), ipnet.IP6, netip.Addr{}, proxied).Return(true),
				)
			},
		},
		"both/setfail1": {
			false,
			map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: true},
			nil,
			mockproviders{ipnet.IP4: true, ipnet.IP6: true},
			func(ppfmt pp.PP, m *mocks.MockSetter, proxied bool) {
				gomock.InOrder(
					m.EXPECT().Set(gomock.Any(), ppfmt, api.FQDN("ip4.hello"), ipnet.IP4, netip.Addr{}, proxied).Return(false),
					m.EXPECT().Set(gomock.Any(), ppfmt, api.FQDN("ip6.hello"), ipnet.IP6, netip.Addr{}, proxied).Return(true),
				)
			},
		},
		"both/setfail2": {
			false,
			map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: true},
			nil,
			mockproviders{ipnet.IP4: true, ipnet.IP6: true},
			func(ppfmt pp.PP, m *mocks.MockSetter, proxied bool) {
				gomock.InOrder(
					m.EXPECT().Set(gomock.Any(), ppfmt, api.FQDN("ip4.hello"), ipnet.IP4, netip.Addr{}, proxied).Return(true),
					m.EXPECT().Set(gomock.Any(), ppfmt, api.FQDN("ip6.hello"), ipnet.IP6, netip.Addr{}, proxied).Return(false),
				)
			},
		},
	} {
		tc := tc
		for proxied, proxiedByDomain := range map[bool]mockproxied{false: proxiedNone, true: proxiedBoth} {
			t.Run(name, func(t *testing.T) {
				mockCtrl := gomock.NewController(t)
				ctx := context.Background()
				conf := config.Default()
				conf.Domains = domains
				conf.ProxiedByDomain = proxiedByDomain
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
					tc.prepareMockSetter(mockPP, mockSetter, proxied)
				}
				ok := updater.ClearIPs(ctx, mockPP, conf, mockSetter)
				require.Equal(t, tc.ok, ok)
			})
		}
	}
}
