package updater_test

import (
	"context"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/config"
	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/updater"
)

//nolint:funlen,paralleltest // updater.IPv6MessageDisplayed is a global variable
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
	provider4 := func(ppfmt pp.PP, m *mocks.MockProvider) {
		m.EXPECT().GetIP(gomock.Any(), ppfmt, ipnet.IP4, true).Return(ip4, true)
	}
	provider6 := func(ppfmt pp.PP, m *mocks.MockProvider) {
		m.EXPECT().GetIP(gomock.Any(), ppfmt, ipnet.IP6, true).Return(ip6, true)
	}

	type mockproxied = map[domain.Domain]bool
	proxiedNone := mockproxied{domain4: false, domain6: false}
	proxiedBoth := mockproxied{domain4: true, domain6: true}

	for name, tc := range map[string]struct {
		ttl                       api.TTL
		proxied                   mockproxied
		ok                        bool
		msg                       string
		ShouldDisplayHelpMessages map[ipnet.Type]bool
		prepareMockPP             func(m *mocks.MockPP)
		prepareMockProvider       mockproviders
		prepareMockSetter         func(ppfmt pp.PP, m *mocks.MockSetter)
	}{
		"none": {
			api.TTLAuto, proxiedBoth, true, ``, map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: true}, nil, mockproviders{}, nil,
		},
		"ip4only": {
			api.TTLAuto,
			proxiedNone,
			true,
			"",
			map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: true},
			pp4only,
			mockproviders{ipnet.IP4: provider4},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				m.EXPECT().Set(gomock.Any(), ppfmt, domain.FQDN("ip4.hello"), ipnet.IP4, ip4, api.TTL(1), false).Return(true, "")
			},
		},
		"ip4only/setfail": {
			api.TTLAuto,
			proxiedBoth,
			false,
			"error",
			map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: true},
			pp4only,
			mockproviders{ipnet.IP4: provider4},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				m.EXPECT().Set(gomock.Any(), ppfmt, domain.FQDN("ip4.hello"), ipnet.IP4, ip4, api.TTL(1), true).Return(false, "error") //nolint:lll
			},
		},
		"ip6only": {
			api.TTLAuto,
			proxiedNone,
			true,
			"ok",
			map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: true},
			pp6only,
			mockproviders{ipnet.IP6: provider6},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				m.EXPECT().Set(gomock.Any(), ppfmt, domain.FQDN("ip6.hello"), ipnet.IP6, ip6, api.TTL(1), false).Return(true, "ok")
			},
		},
		"ip6only/setfail": {
			api.TTLAuto,
			proxiedBoth,
			false,
			"bad",
			map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: true},
			pp6only,
			mockproviders{ipnet.IP6: provider6},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				m.EXPECT().Set(gomock.Any(), ppfmt, domain.FQDN("ip6.hello"), ipnet.IP6, ip6, api.TTL(1), true).Return(false, "bad")
			},
		},
		"both": {
			api.TTLAuto,
			proxiedNone,
			true,
			"",
			map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: true},
			ppBoth,
			mockproviders{ipnet.IP4: provider4, ipnet.IP6: provider6},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				gomock.InOrder(
					m.EXPECT().Set(gomock.Any(), ppfmt, domain.FQDN("ip4.hello"), ipnet.IP4, ip4, api.TTL(1), false).Return(true, ""),
					m.EXPECT().Set(gomock.Any(), ppfmt, domain.FQDN("ip6.hello"), ipnet.IP6, ip6, api.TTL(1), false).Return(true, ""),
				)
			},
		},
		"both/setfail1": {
			api.TTLAuto,
			proxiedBoth,
			false,
			"hey!",
			map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: true},
			ppBoth,
			mockproviders{ipnet.IP4: provider4, ipnet.IP6: provider6},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				gomock.InOrder(
					m.EXPECT().Set(gomock.Any(), ppfmt, domain.FQDN("ip4.hello"), ipnet.IP4, ip4, api.TTL(1), true).Return(false, "hey!"), //nolint:lll
					m.EXPECT().Set(gomock.Any(), ppfmt, domain.FQDN("ip6.hello"), ipnet.IP6, ip6, api.TTL(1), true).Return(true, ""),
				)
			},
		},
		"both/setfail2": {
			api.TTLAuto,
			proxiedNone,
			false,
			"wrong",
			map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: true},
			ppBoth,
			mockproviders{ipnet.IP4: provider4, ipnet.IP6: provider6},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				gomock.InOrder(
					m.EXPECT().Set(gomock.Any(), ppfmt, domain.FQDN("ip4.hello"), ipnet.IP4, ip4, api.TTL(1), false).Return(true, ""),
					m.EXPECT().Set(gomock.Any(), ppfmt, domain.FQDN("ip6.hello"), ipnet.IP6, ip6, api.TTL(1), false).Return(false, "wrong"), //nolint:lll
				)
			},
		},
		"ip4fails": {
			api.TTLAuto,
			proxiedBoth,
			false,
			"looking good",
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
					m.EXPECT().GetIP(gomock.Any(), ppfmt, ipnet.IP4, true).Return(netip.Addr{}, false)
				},
				ipnet.IP6: provider6,
			},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				m.EXPECT().Set(gomock.Any(), ppfmt, domain.FQDN("ip6.hello"), ipnet.IP6, ip6, api.TTL(1), true).Return(true, "looking good") //nolint:lll
			},
		},
		"ip6fails": {
			api.TTLAuto,
			proxiedNone,
			false,
			"good",
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
					m.EXPECT().GetIP(gomock.Any(), ppfmt, ipnet.IP6, true).Return(netip.Addr{}, false)
				},
			},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				m.EXPECT().Set(gomock.Any(), ppfmt, domain.FQDN("ip4.hello"), ipnet.IP4, ip4, api.TTL(1), false).Return(true, "good") //nolint:lll
			},
		},
		"ip6fails/again": {
			api.TTLAuto,
			proxiedBoth,
			false,
			"",
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
					m.EXPECT().GetIP(gomock.Any(), ppfmt, ipnet.IP6, true).Return(netip.Addr{}, false)
				},
			},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				m.EXPECT().Set(gomock.Any(), ppfmt, domain.FQDN("ip4.hello"), ipnet.IP4, ip4, api.TTL(1), true).Return(true, "")
			},
		},
		"bothfail": {
			api.TTLAuto,
			proxiedNone,
			false,
			"",
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
					m.EXPECT().GetIP(gomock.Any(), ppfmt, ipnet.IP4, true).Return(netip.Addr{}, false)
				},
				ipnet.IP6: func(ppfmt pp.PP, m *mocks.MockProvider) {
					m.EXPECT().GetIP(gomock.Any(), ppfmt, ipnet.IP6, true).Return(netip.Addr{}, false)
				},
			},
			nil,
		},
		"ip4only-proxied-nil": {
			api.TTLAuto,
			mockproxied{},
			true,
			"response",
			map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: true},
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Infof(pp.EmojiInternet, "Detected the %s address: %v", "IPv4", ip4),
					m.EXPECT().Warningf(pp.EmojiImpossible,
						"Proxied[%s] not initialized; this should not happen; please report the bug at https://github.com/favonia/cloudflare-ddns/issues/new", //nolint:lll
						"ip4.hello",
					),
				)
			},
			mockproviders{ipnet.IP4: provider4},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				m.EXPECT().Set(gomock.Any(), ppfmt, domain.FQDN("ip4.hello"), ipnet.IP4, ip4, api.TTL(1), false).Return(true, "response") //nolint:lll
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
			conf.Use1001 = true
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}
			for _, ipnet := range [...]ipnet.Type{ipnet.IP4, ipnet.IP6} {
				updater.ShouldDisplayHelpMessages[ipnet] = tc.ShouldDisplayHelpMessages[ipnet]
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
			ok, msg := updater.UpdateIPs(ctx, mockPP, conf, mockSetter)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.msg, msg)
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

	type mockproxied = map[domain.Domain]bool
	proxiedNone := mockproxied{domain4: false, domain6: false}

	for name, tc := range map[string]struct {
		ttl                       api.TTL
		proxied                   mockproxied
		ok                        bool
		msg                       string
		ShouldDisplayHelpMessages map[ipnet.Type]bool
		prepareMockPP             func(m *mocks.MockPP)
		prepareMockProvider       mockproviders
		prepareMockSetter         func(ppfmt pp.PP, m *mocks.MockSetter)
	}{
		"none": {
			api.TTLAuto,
			proxiedNone,
			true,
			``,
			map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: true},
			nil,
			mockproviders{},
			nil,
		},
		"ip4only": {
			api.TTLAuto,
			proxiedNone,
			true,
			"hello",
			map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: true},
			nil,
			mockproviders{ipnet.IP4: true},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				m.EXPECT().Clear(gomock.Any(), ppfmt, domain4, ipnet.IP4).Return(true, "hello")
			},
		},
		"ip4only/setfail": {
			api.TTLAuto,
			proxiedNone,
			false,
			"err",
			map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: true},
			nil,
			mockproviders{ipnet.IP4: true},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				m.EXPECT().Clear(gomock.Any(), ppfmt, domain4, ipnet.IP4).Return(false, "err")
			},
		},
		"ip6only": {
			api.TTLAuto,
			proxiedNone,
			true,
			"",
			map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: true},
			nil,
			mockproviders{ipnet.IP6: true},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				m.EXPECT().Clear(gomock.Any(), ppfmt, domain6, ipnet.IP6).Return(true, "")
			},
		},
		"ip6only/setfail": {
			api.TTLAuto,
			proxiedNone,
			false,
			"test",
			map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: true},
			nil,
			mockproviders{ipnet.IP6: true},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				m.EXPECT().Clear(gomock.Any(), ppfmt, domain6, ipnet.IP6).Return(false, "test")
			},
		},
		"both": {
			api.TTLAuto,
			proxiedNone,
			true,
			"both\nneither",
			map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: true},
			nil,
			mockproviders{ipnet.IP4: true, ipnet.IP6: true},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				gomock.InOrder(
					m.EXPECT().Clear(gomock.Any(), ppfmt, domain4, ipnet.IP4).Return(true, "both"),
					m.EXPECT().Clear(gomock.Any(), ppfmt, domain6, ipnet.IP6).Return(true, "neither"),
				)
			},
		},
		"both/setfail1": {
			api.TTLAuto,
			proxiedNone,
			false,
			"999",
			map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: true},
			nil,
			mockproviders{ipnet.IP4: true, ipnet.IP6: true},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				gomock.InOrder(
					m.EXPECT().Clear(gomock.Any(), ppfmt, domain4, ipnet.IP4).Return(false, ""),
					m.EXPECT().Clear(gomock.Any(), ppfmt, domain6, ipnet.IP6).Return(true, "999"),
				)
			},
		},
		"both/setfail2": {
			api.TTLAuto,
			proxiedNone,
			false,
			"1\n2",
			map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: true},
			nil,
			mockproviders{ipnet.IP4: true, ipnet.IP6: true},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				gomock.InOrder(
					m.EXPECT().Clear(gomock.Any(), ppfmt, domain4, ipnet.IP4).Return(true, "1"),
					m.EXPECT().Clear(gomock.Any(), ppfmt, domain6, ipnet.IP6).Return(false, "2"),
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
				updater.ShouldDisplayHelpMessages[ipnet] = tc.ShouldDisplayHelpMessages[ipnet]
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
			ok, msg := updater.ClearIPs(ctx, mockPP, conf, mockSetter)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.msg, msg)
		})
	}
}
