package updater_test

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/config"
	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/setter"
	"github.com/favonia/cloudflare-ddns/internal/updater"
)

//nolint:gochecknoglobals
var allHints = map[string]bool{
	"detect-ip4-fail": true,
	"detect-ip6-fail": true,
	"update-timeout":  true,
}

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
		proxied             mockproxied
		ok                  bool
		msg                 string
		ShouldDisplayHints  map[string]bool
		prepareMockPP       func(m *mocks.MockPP)
		prepareMockProvider mockproviders
		prepareMockSetter   func(ppfmt pp.PP, m *mocks.MockSetter)
	}{
		"none": {
			proxiedBoth, true, ``, allHints, nil, mockproviders{}, nil,
		},
		"ip4only": {
			proxiedNone,
			true,
			"",
			allHints,
			pp4only,
			mockproviders{ipnet.IP4: provider4},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				m.EXPECT().Set(gomock.Any(), ppfmt, domain.FQDN("ip4.hello"), ipnet.IP4, ip4, api.TTLAuto, false).
					Return(setter.ResponseNoUpdatesNeeded)
			},
		},
		"ip4only/setfail": {
			proxiedBoth,
			false,
			"Failed to set ip4.hello A",
			allHints,
			pp4only,
			mockproviders{ipnet.IP4: provider4},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				m.EXPECT().Set(gomock.Any(), ppfmt, domain.FQDN("ip4.hello"), ipnet.IP4, ip4, api.TTLAuto, true).
					Return(setter.ResponseUpdatesFailed)
			},
		},
		"ip6only": {
			proxiedNone,
			true,
			"Set ip6.hello AAAA to ::1",
			allHints,
			pp6only,
			mockproviders{ipnet.IP6: provider6},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				m.EXPECT().Set(gomock.Any(), ppfmt, domain.FQDN("ip6.hello"), ipnet.IP6, ip6, api.TTLAuto, false).
					Return(setter.ResponseUpdatesApplied)
			},
		},
		"ip6only/setfail": {
			proxiedBoth,
			false,
			"Failed to set ip6.hello AAAA",
			allHints,
			pp6only,
			mockproviders{ipnet.IP6: provider6},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				m.EXPECT().Set(gomock.Any(), ppfmt, domain.FQDN("ip6.hello"), ipnet.IP6, ip6, api.TTLAuto, true).
					Return(setter.ResponseUpdatesFailed)
			},
		},
		"both": {
			proxiedNone,
			true,
			"",
			allHints,
			ppBoth,
			mockproviders{ipnet.IP4: provider4, ipnet.IP6: provider6},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				gomock.InOrder(
					m.EXPECT().Set(gomock.Any(), ppfmt, domain.FQDN("ip4.hello"), ipnet.IP4, ip4, api.TTLAuto, false).
						Return(setter.ResponseNoUpdatesNeeded),
					m.EXPECT().Set(gomock.Any(), ppfmt, domain.FQDN("ip6.hello"), ipnet.IP6, ip6, api.TTLAuto, false).
						Return(setter.ResponseNoUpdatesNeeded),
				)
			},
		},
		"both/setfail1": {
			proxiedBoth,
			false,
			"Failed to set ip4.hello A",
			allHints,
			ppBoth,
			mockproviders{ipnet.IP4: provider4, ipnet.IP6: provider6},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				gomock.InOrder(
					m.EXPECT().Set(gomock.Any(), ppfmt, domain.FQDN("ip4.hello"), ipnet.IP4, ip4, api.TTLAuto, true).
						Return(setter.ResponseUpdatesFailed),
					m.EXPECT().Set(gomock.Any(), ppfmt, domain.FQDN("ip6.hello"), ipnet.IP6, ip6, api.TTLAuto, true).
						Return(setter.ResponseNoUpdatesNeeded),
				)
			},
		},
		"both/setfail2": {
			proxiedNone,
			false,
			"Failed to set ip6.hello AAAA",
			allHints,
			ppBoth,
			mockproviders{ipnet.IP4: provider4, ipnet.IP6: provider6},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				gomock.InOrder(
					m.EXPECT().Set(gomock.Any(), ppfmt, domain.FQDN("ip4.hello"), ipnet.IP4, ip4, api.TTLAuto, false).
						Return(setter.ResponseNoUpdatesNeeded),
					m.EXPECT().Set(gomock.Any(), ppfmt, domain.FQDN("ip6.hello"), ipnet.IP6, ip6, api.TTLAuto, false).
						Return(setter.ResponseUpdatesFailed),
				)
			},
		},
		"ip4fails": {
			proxiedBoth,
			false,
			"Failed to detect the IPv4 address",
			allHints,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Errorf(pp.EmojiError, "%s", "Failed to detect the IPv4 address"),
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
				m.EXPECT().Set(gomock.Any(), ppfmt, domain.FQDN("ip6.hello"), ipnet.IP6, ip6, api.TTLAuto, true).
					Return(setter.ResponseNoUpdatesNeeded)
			},
		},
		"ip6fails": {
			proxiedNone,
			false,
			"Failed to detect the IPv6 address",
			allHints,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Infof(pp.EmojiInternet, "Detected the %s address: %v", "IPv4", ip4),
					m.EXPECT().Errorf(pp.EmojiError, "%s", "Failed to detect the IPv6 address"),
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
				m.EXPECT().Set(gomock.Any(), ppfmt, domain.FQDN("ip4.hello"), ipnet.IP4, ip4, api.TTLAuto, false).
					Return(setter.ResponseNoUpdatesNeeded)
			},
		},
		"ip6fails/again": {
			proxiedBoth,
			false,
			"Failed to detect the IPv6 address",
			map[string]bool{"detect-ip4-fail": true, "detect-ip6-fail": false, "update-timeout": true},
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Infof(pp.EmojiInternet, "Detected the %s address: %v", "IPv4", ip4),
					m.EXPECT().Errorf(pp.EmojiError, "%s", "Failed to detect the IPv6 address"),
				)
			},
			mockproviders{
				ipnet.IP4: provider4,
				ipnet.IP6: func(ppfmt pp.PP, m *mocks.MockProvider) {
					m.EXPECT().GetIP(gomock.Any(), ppfmt, ipnet.IP6, true).Return(netip.Addr{}, false)
				},
			},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				m.EXPECT().Set(gomock.Any(), ppfmt, domain.FQDN("ip4.hello"), ipnet.IP4, ip4, api.TTLAuto, true).
					Return(setter.ResponseNoUpdatesNeeded)
			},
		},
		"bothfail": {
			proxiedNone,
			false,
			"Failed to detect the IPv4 address\nFailed to detect the IPv6 address",
			allHints,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Errorf(pp.EmojiError, "%s", "Failed to detect the IPv4 address"),
					m.EXPECT().Infof(pp.EmojiConfig, "If your network does not support IPv4, you can disable it with IP4_PROVIDER=none"), //nolint:lll
					m.EXPECT().Errorf(pp.EmojiError, "%s", "Failed to detect the IPv6 address"),
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
			mockproxied{},
			true,
			"Set ip4.hello A to 127.0.0.1",
			allHints,
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
				m.EXPECT().Set(gomock.Any(), ppfmt, domain.FQDN("ip4.hello"), ipnet.IP4, ip4, api.TTLAuto, false).
					Return(setter.ResponseUpdatesApplied)
			},
		},
		"slow-setting": {
			proxiedNone,
			false,
			"Failed to set ip4.hello A",
			allHints,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Infof(pp.EmojiInternet, "Detected the %s address: %v", "IPv4", ip4),
					m.EXPECT().Infof(pp.EmojiConfig, "If your network is working but with high latency, consider increasing the value of UPDATE_TIMEOUT"), //nolint:lll
				)
			},
			mockproviders{
				ipnet.IP4: func(ppfmt pp.PP, m *mocks.MockProvider) {
					m.EXPECT().GetIP(gomock.Any(), ppfmt, ipnet.IP4, true).Return(ip4, true)
				},
			},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				m.EXPECT().Set(gomock.Any(), ppfmt, domain.FQDN("ip4.hello"), ipnet.IP4, ip4, api.TTLAuto, false).
					DoAndReturn(
						func(_ context.Context, _ pp.PP, _ domain.Domain, _ ipnet.Type, _ netip.Addr, _ api.TTL, _ bool) setter.ResponseCode { //nolint:lll
							time.Sleep(2 * time.Second)
							return setter.ResponseUpdatesFailed
						})
			},
		},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			ctx := context.Background()
			conf := config.Default()
			conf.Domains = domains
			conf.TTL = api.TTLAuto
			conf.Proxied = tc.proxied
			conf.Use1001 = true
			conf.UpdateTimeout = time.Second
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}
			for k := range updater.ShouldDisplayHints {
				updater.ShouldDisplayHints[k] = tc.ShouldDisplayHints[k]
			}
			for _, ipnet := range [...]ipnet.Type{ipnet.IP4, ipnet.IP6} {
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
func TestDeleteIPs(t *testing.T) {
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
		proxied             mockproxied
		ok                  bool
		msg                 string
		ShouldDisplayHints  map[string]bool
		prepareMockPP       func(m *mocks.MockPP)
		prepareMockProvider mockproviders
		prepareMockSetter   func(ppfmt pp.PP, m *mocks.MockSetter)
	}{
		"none": {
			proxiedNone,
			true,
			``,
			allHints,
			nil,
			mockproviders{},
			nil,
		},
		"ip4only": {
			proxiedNone,
			true,
			"Deleted ip4.hello A",
			allHints,
			nil,
			mockproviders{ipnet.IP4: true},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				m.EXPECT().Delete(gomock.Any(), ppfmt, domain4, ipnet.IP4).
					Return(setter.ResponseUpdatesApplied)
			},
		},
		"ip4only/setfail": {
			proxiedNone,
			false,
			"Failed to delete ip4.hello A",
			allHints,
			nil,
			mockproviders{ipnet.IP4: true},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				m.EXPECT().Delete(gomock.Any(), ppfmt, domain4, ipnet.IP4).Return(setter.ResponseUpdatesFailed)
			},
		},
		"ip6only": {
			proxiedNone,
			true,
			"Deleted ip6.hello AAAA",
			allHints,
			nil,
			mockproviders{ipnet.IP6: true},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				m.EXPECT().Delete(gomock.Any(), ppfmt, domain6, ipnet.IP6).Return(setter.ResponseUpdatesApplied)
			},
		},
		"ip6only/setfail": {
			proxiedNone,
			false,
			"Failed to delete ip6.hello AAAA",
			allHints,
			nil,
			mockproviders{ipnet.IP6: true},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				m.EXPECT().Delete(gomock.Any(), ppfmt, domain6, ipnet.IP6).Return(setter.ResponseUpdatesFailed)
			},
		},
		"both": {
			proxiedNone,
			true,
			"Deleted ip4.hello A\nDeleted ip6.hello AAAA",
			allHints,
			nil,
			mockproviders{ipnet.IP4: true, ipnet.IP6: true},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				gomock.InOrder(
					m.EXPECT().Delete(gomock.Any(), ppfmt, domain4, ipnet.IP4).Return(setter.ResponseUpdatesApplied),
					m.EXPECT().Delete(gomock.Any(), ppfmt, domain6, ipnet.IP6).Return(setter.ResponseUpdatesApplied),
				)
			},
		},
		"both/setfail1": {
			proxiedNone,
			false,
			"Failed to delete ip4.hello A",
			allHints,
			nil,
			mockproviders{ipnet.IP4: true, ipnet.IP6: true},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				gomock.InOrder(
					m.EXPECT().Delete(gomock.Any(), ppfmt, domain4, ipnet.IP4).Return(setter.ResponseUpdatesFailed),
					m.EXPECT().Delete(gomock.Any(), ppfmt, domain6, ipnet.IP6).Return(setter.ResponseNoUpdatesNeeded),
				)
			},
		},
		"both/setfail2": {
			proxiedNone,
			false,
			"Failed to delete ip6.hello AAAA",
			allHints,
			nil,
			mockproviders{ipnet.IP4: true, ipnet.IP6: true},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				gomock.InOrder(
					m.EXPECT().Delete(gomock.Any(), ppfmt, domain4, ipnet.IP4).Return(setter.ResponseNoUpdatesNeeded),
					m.EXPECT().Delete(gomock.Any(), ppfmt, domain6, ipnet.IP6).Return(setter.ResponseUpdatesFailed),
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
			conf.TTL = api.TTLAuto
			conf.Proxied = tc.proxied
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}
			for k := range updater.ShouldDisplayHints {
				updater.ShouldDisplayHints[k] = tc.ShouldDisplayHints[k]
			}
			for _, ipnet := range [...]ipnet.Type{ipnet.IP4, ipnet.IP6} {
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
			ok, msg := updater.DeleteIPs(ctx, mockPP, conf, mockSetter)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.msg, msg)
		})
	}
}
