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

//nolint:funlen,paralleltest // updater.IPv6MessageDisplayed is a global variable
func TestUpdateIPs(t *testing.T) {
	domains := map[ipnet.Type][]api.Domain{
		ipnet.IP4: {api.FQDN("ip4.hello")},
		ipnet.IP6: {api.FQDN("ip6.hello")},
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

	type mockmap = map[ipnet.Type]func(ppfmt pp.PP, m *mocks.MockProvider)
	provider4 := func(ppfmt pp.PP, m *mocks.MockProvider) { m.EXPECT().GetIP(gomock.Any(), ppfmt, ipnet.IP4).Return(ip4) }
	provider6 := func(ppfmt pp.PP, m *mocks.MockProvider) { m.EXPECT().GetIP(gomock.Any(), ppfmt, ipnet.IP6).Return(ip6) }

	for name, tc := range map[string]struct {
		ok                   bool
		MessageShouldDisplay map[ipnet.Type]bool
		prepareMockPP        func(m *mocks.MockPP)
		prepareMockProvider  mockmap
		prepareMockSetter    func(ppfmt pp.PP, m *mocks.MockSetter)
	}{
		"none": {true, map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: true}, nil, mockmap{}, nil},
		"ip4only": {
			true,
			map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: true},
			pp4only,
			mockmap{ipnet.IP4: provider4},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				m.EXPECT().Set(gomock.Any(), ppfmt, api.FQDN("ip4.hello"), ipnet.IP4, ip4).Return(true)
			},
		},
		"ip4only/setfail": {
			false,
			map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: true},
			pp4only,
			mockmap{ipnet.IP4: provider4},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				m.EXPECT().Set(gomock.Any(), ppfmt, api.FQDN("ip4.hello"), ipnet.IP4, ip4).Return(false)
			},
		},
		"ip6only": {
			true,
			map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: true},
			pp6only,
			mockmap{ipnet.IP6: provider6},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				m.EXPECT().Set(gomock.Any(), ppfmt, api.FQDN("ip6.hello"), ipnet.IP6, ip6).Return(true)
			},
		},
		"ip6only/setfail": {
			false,
			map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: true},
			pp6only,
			mockmap{ipnet.IP6: provider6},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				m.EXPECT().Set(gomock.Any(), ppfmt, api.FQDN("ip6.hello"), ipnet.IP6, ip6).Return(false)
			},
		},
		"both": {
			true,
			map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: true},
			ppBoth,
			mockmap{ipnet.IP4: provider4, ipnet.IP6: provider6},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				gomock.InOrder(
					m.EXPECT().Set(gomock.Any(), ppfmt, api.FQDN("ip4.hello"), ipnet.IP4, ip4).Return(true),
					m.EXPECT().Set(gomock.Any(), ppfmt, api.FQDN("ip6.hello"), ipnet.IP6, ip6).Return(true),
				)
			},
		},
		"both/setfail1": {
			false,
			map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: true},
			ppBoth,
			mockmap{ipnet.IP4: provider4, ipnet.IP6: provider6},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				gomock.InOrder(
					m.EXPECT().Set(gomock.Any(), ppfmt, api.FQDN("ip4.hello"), ipnet.IP4, ip4).Return(false),
					m.EXPECT().Set(gomock.Any(), ppfmt, api.FQDN("ip6.hello"), ipnet.IP6, ip6).Return(true),
				)
			},
		},
		"both/setfail2": {
			false,
			map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: true},
			ppBoth,
			mockmap{ipnet.IP4: provider4, ipnet.IP6: provider6},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				gomock.InOrder(
					m.EXPECT().Set(gomock.Any(), ppfmt, api.FQDN("ip4.hello"), ipnet.IP4, ip4).Return(true),
					m.EXPECT().Set(gomock.Any(), ppfmt, api.FQDN("ip6.hello"), ipnet.IP6, ip6).Return(false),
				)
			},
		},
		"ip4fails": {
			false,
			map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: true},
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Errorf(pp.EmojiError, "Failed to detect the %s address", "IPv4"),
					m.EXPECT().Infof(pp.EmojiConfig, "If your network does not support IPv4, you can disable IPv4 with IP4_PROVIDER=none"), //nolint:lll
					m.EXPECT().Infof(pp.EmojiInternet, "Detected the %s address: %v", "IPv6", ip6),
				)
			},
			mockmap{
				ipnet.IP4: func(ppfmt pp.PP, m *mocks.MockProvider) {
					m.EXPECT().GetIP(gomock.Any(), ppfmt, ipnet.IP4).Return(netip.Addr{})
				},
				ipnet.IP6: provider6,
			},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				m.EXPECT().Set(gomock.Any(), ppfmt, api.FQDN("ip6.hello"), ipnet.IP6, ip6).Return(true)
			},
		},
		"ip6fails": {
			false,
			map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: true},
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Infof(pp.EmojiInternet, "Detected the %s address: %v", "IPv4", ip4),
					m.EXPECT().Errorf(pp.EmojiError, "Failed to detect the %s address", "IPv6"),
					m.EXPECT().Infof(pp.EmojiConfig, "If you are using Docker, Kubernetes, or other frameworks, IPv6 networks often require additional setups."), //nolint:lll
					m.EXPECT().Infof(pp.EmojiConfig, "Read more about IPv6 networks in the README at https://github.com/favonia/cloudflare-ddns"),                //nolint:lll
					m.EXPECT().Infof(pp.EmojiConfig, "If your network does not support IPv6, you can disable IPv6 with IP6_PROVIDER=none"),                       //nolint:lll
				)
			},
			mockmap{
				ipnet.IP4: provider4,
				ipnet.IP6: func(ppfmt pp.PP, m *mocks.MockProvider) {
					m.EXPECT().GetIP(gomock.Any(), ppfmt, ipnet.IP6).Return(netip.Addr{})
				},
			},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				m.EXPECT().Set(gomock.Any(), ppfmt, api.FQDN("ip4.hello"), ipnet.IP4, ip4).Return(true)
			},
		},
		"ip6fails/again": {
			false,
			map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: false},
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Infof(pp.EmojiInternet, "Detected the %s address: %v", "IPv4", ip4),
					m.EXPECT().Errorf(pp.EmojiError, "Failed to detect the %s address", "IPv6"),
				)
			},
			mockmap{
				ipnet.IP4: provider4,
				ipnet.IP6: func(ppfmt pp.PP, m *mocks.MockProvider) {
					m.EXPECT().GetIP(gomock.Any(), ppfmt, ipnet.IP6).Return(netip.Addr{})
				},
			},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				m.EXPECT().Set(gomock.Any(), ppfmt, api.FQDN("ip4.hello"), ipnet.IP4, ip4).Return(true)
			},
		},
		"bothfail": {
			false,
			map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: true},
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Errorf(pp.EmojiError, "Failed to detect the %s address", "IPv4"),
					m.EXPECT().Infof(pp.EmojiConfig, "If your network does not support IPv4, you can disable IPv4 with IP4_PROVIDER=none"), //nolint:lll
					m.EXPECT().Errorf(pp.EmojiError, "Failed to detect the %s address", "IPv6"),
					m.EXPECT().Infof(pp.EmojiConfig, "If you are using Docker, Kubernetes, or other frameworks, IPv6 networks often require additional setups."), //nolint:lll
					m.EXPECT().Infof(pp.EmojiConfig, "Read more about IPv6 networks in the README at https://github.com/favonia/cloudflare-ddns"),                //nolint:lll
					m.EXPECT().Infof(pp.EmojiConfig, "If your network does not support IPv6, you can disable IPv6 with IP6_PROVIDER=none"),                       //nolint:lll
				)
			},
			mockmap{
				ipnet.IP4: func(ppfmt pp.PP, m *mocks.MockProvider) {
					m.EXPECT().GetIP(gomock.Any(), ppfmt, ipnet.IP4).Return(netip.Addr{})
				},
				ipnet.IP6: func(ppfmt pp.PP, m *mocks.MockProvider) {
					m.EXPECT().GetIP(gomock.Any(), ppfmt, ipnet.IP6).Return(netip.Addr{})
				},
			},
			nil,
		},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			ctx := context.Background()
			conf := config.Default()
			conf.Domains = domains
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

//nolint:funlen
func TestClearIPs(t *testing.T) {
	t.Parallel()

	domains := map[ipnet.Type][]api.Domain{
		ipnet.IP4: {api.FQDN("ip4.hello")},
		ipnet.IP6: {api.FQDN("ip6.hello")},
	}

	type mockmap = map[ipnet.Type]bool

	//nolint: paralleltest // updater.IPv6MessageDisplayed is a global variable
	for name, tc := range map[string]struct {
		ok                   bool
		MessageShouldDisplay map[ipnet.Type]bool
		prepareMockPP        func(m *mocks.MockPP)
		prepareMockProvider  mockmap
		prepareMockSetter    func(ppfmt pp.PP, m *mocks.MockSetter)
	}{
		"none": {
			true,
			map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: true},
			nil,
			mockmap{},
			nil,
		},
		"ip4only": {
			true,
			map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: true},
			nil,
			mockmap{ipnet.IP4: true},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				m.EXPECT().Set(gomock.Any(), ppfmt, api.FQDN("ip4.hello"), ipnet.IP4, netip.Addr{}).Return(true)
			},
		},
		"ip4only/setfail": {
			false,
			map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: true},
			nil,
			mockmap{ipnet.IP4: true},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				m.EXPECT().Set(gomock.Any(), ppfmt, api.FQDN("ip4.hello"), ipnet.IP4, netip.Addr{}).Return(false)
			},
		},
		"ip6only": {
			true,
			map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: true},
			nil,
			mockmap{ipnet.IP6: true},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				m.EXPECT().Set(gomock.Any(), ppfmt, api.FQDN("ip6.hello"), ipnet.IP6, netip.Addr{}).Return(true)
			},
		},
		"ip6only/setfail": {
			false,
			map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: true},
			nil,
			mockmap{ipnet.IP6: true},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				m.EXPECT().Set(gomock.Any(), ppfmt, api.FQDN("ip6.hello"), ipnet.IP6, netip.Addr{}).Return(false)
			},
		},
		"both": {
			true,
			map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: true},
			nil,
			mockmap{ipnet.IP4: true, ipnet.IP6: true},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				gomock.InOrder(
					m.EXPECT().Set(gomock.Any(), ppfmt, api.FQDN("ip4.hello"), ipnet.IP4, netip.Addr{}).Return(true),
					m.EXPECT().Set(gomock.Any(), ppfmt, api.FQDN("ip6.hello"), ipnet.IP6, netip.Addr{}).Return(true),
				)
			},
		},
		"both/setfail1": {
			false,
			map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: true},
			nil,
			mockmap{ipnet.IP4: true, ipnet.IP6: true},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				gomock.InOrder(
					m.EXPECT().Set(gomock.Any(), ppfmt, api.FQDN("ip4.hello"), ipnet.IP4, netip.Addr{}).Return(false),
					m.EXPECT().Set(gomock.Any(), ppfmt, api.FQDN("ip6.hello"), ipnet.IP6, netip.Addr{}).Return(true),
				)
			},
		},
		"both/setfail2": {
			false,
			map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: true},
			nil,
			mockmap{ipnet.IP4: true, ipnet.IP6: true},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				gomock.InOrder(
					m.EXPECT().Set(gomock.Any(), ppfmt, api.FQDN("ip4.hello"), ipnet.IP4, netip.Addr{}).Return(true),
					m.EXPECT().Set(gomock.Any(), ppfmt, api.FQDN("ip6.hello"), ipnet.IP6, netip.Addr{}).Return(false),
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
