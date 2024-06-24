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
	"github.com/favonia/cloudflare-ddns/internal/response"
	"github.com/favonia/cloudflare-ddns/internal/setter"
	"github.com/favonia/cloudflare-ddns/internal/updater"
)

//nolint:funlen,paralleltest // updater.IPv6MessageDisplayed is a global variable
func TestUpdateIPsMultiple(t *testing.T) {
	domain4_1 := domain.FQDN("ip4.hello1")
	domain4_2 := domain.FQDN("ip4.hello2")
	domain4_3 := domain.FQDN("ip4.hello3")
	domain4_4 := domain.FQDN("ip4.hello4")
	domains := map[ipnet.Type][]domain.Domain{
		ipnet.IP4: {domain4_1, domain4_2, domain4_3, domain4_4},
	}

	ip4 := netip.MustParseAddr("127.0.0.1")

	type mockproviders = map[ipnet.Type]func(ppfmt pp.PP, m *mocks.MockProvider)
	provider4 := func(ppfmt pp.PP, m *mocks.MockProvider) {
		m.EXPECT().GetIP(gomock.Any(), ppfmt, ipnet.IP4, true).Return(ip4, true)
	}

	for name, tc := range map[string]struct {
		ok                  bool
		monitorMessages     []string
		notifierMessages    []string
		prepareMockPP       func(m *mocks.MockPP)
		prepareMockProvider mockproviders
		prepareMockSetter   func(ppfmt pp.PP, m *mocks.MockSetter)
	}{
		"2yes1no": { //nolint:dupl
			false,
			[]string{"Failed to set A (127.0.0.1): ip4.hello2"},
			[]string{"Failed to finish updating A records of ip4.hello2 with 127.0.0.1; those of ip4.hello1 and ip4.hello4 were updated."}, //nolint:lll
			func(m *mocks.MockPP) {
				m.EXPECT().Infof(pp.EmojiInternet, "Detected the %s address: %v", "IPv4", ip4)
			},
			mockproviders{ipnet.IP4: provider4},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				gomock.InOrder(
					m.EXPECT().Set(gomock.Any(), ppfmt, domain.FQDN("ip4.hello1"), ipnet.IP4, ip4, api.TTLAuto, false).
						Return(setter.ResponseUpdated),
					m.EXPECT().Set(gomock.Any(), ppfmt, domain.FQDN("ip4.hello2"), ipnet.IP4, ip4, api.TTLAuto, false).
						Return(setter.ResponseFailed),
					m.EXPECT().Set(gomock.Any(), ppfmt, domain.FQDN("ip4.hello3"), ipnet.IP4, ip4, api.TTLAuto, false).
						Return(setter.ResponseNoop),
					m.EXPECT().Set(gomock.Any(), ppfmt, domain.FQDN("ip4.hello4"), ipnet.IP4, ip4, api.TTLAuto, false).
						Return(setter.ResponseUpdated),
				)
			},
		},
		"3yes": { //nolint:dupl
			true,
			[]string{"Set A (127.0.0.1): ip4.hello1, ip4.hello3, ip4.hello4"},
			[]string{"Updated A records of ip4.hello1, ip4.hello3, and ip4.hello4 with 127.0.0.1."},
			func(m *mocks.MockPP) {
				m.EXPECT().Infof(pp.EmojiInternet, "Detected the %s address: %v", "IPv4", ip4)
			},
			mockproviders{ipnet.IP4: provider4},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				gomock.InOrder(
					m.EXPECT().Set(gomock.Any(), ppfmt, domain.FQDN("ip4.hello1"), ipnet.IP4, ip4, api.TTLAuto, false).
						Return(setter.ResponseUpdated),
					m.EXPECT().Set(gomock.Any(), ppfmt, domain.FQDN("ip4.hello2"), ipnet.IP4, ip4, api.TTLAuto, false).
						Return(setter.ResponseNoop),
					m.EXPECT().Set(gomock.Any(), ppfmt, domain.FQDN("ip4.hello3"), ipnet.IP4, ip4, api.TTLAuto, false).
						Return(setter.ResponseUpdated),
					m.EXPECT().Set(gomock.Any(), ppfmt, domain.FQDN("ip4.hello4"), ipnet.IP4, ip4, api.TTLAuto, false).
						Return(setter.ResponseUpdated),
				)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			ctx := context.Background()
			conf := config.Default()
			conf.Domains = domains
			conf.TTL = api.TTLAuto
			conf.Proxied = map[domain.Domain]bool{
				domain4_1: false,
				domain4_2: false,
				domain4_3: false,
				domain4_4: false,
			}
			conf.Use1001 = true
			conf.UpdateTimeout = time.Second
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}
			for k := range updater.ShouldDisplayHints {
				updater.ShouldDisplayHints[k] = true
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
			resp := updater.UpdateIPs(ctx, mockPP, conf, mockSetter)
			require.Equal(t, response.Response{
				Ok:               tc.ok,
				NotifierMessages: tc.notifierMessages,
				MonitorMessages:  tc.monitorMessages,
			}, resp)
		})
	}
}

//nolint:funlen,paralleltest // updater.IPv6MessageDisplayed is a global variable
func TestDeleteIPsMultiple(t *testing.T) {
	domain4_1 := domain.FQDN("ip4.hello1")
	domain4_2 := domain.FQDN("ip4.hello2")
	domain4_3 := domain.FQDN("ip4.hello3")
	domain4_4 := domain.FQDN("ip4.hello4")
	domains := map[ipnet.Type][]domain.Domain{
		ipnet.IP4: {domain4_1, domain4_2, domain4_3, domain4_4},
	}

	for name, tc := range map[string]struct {
		ok                bool
		monitorMessages   []string
		notifierMessages  []string
		prepareMockPP     func(m *mocks.MockPP)
		prepareMockSetter func(ppfmt pp.PP, m *mocks.MockSetter)
	}{
		"2yes1no": { //nolint:dupl
			false,
			[]string{"Failed to delete A: ip4.hello2"},
			[]string{"Failed to finish deleting A records of ip4.hello2; those of ip4.hello1 and ip4.hello4 were deleted."}, //nolint:lll
			nil,
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				gomock.InOrder(
					m.EXPECT().Delete(gomock.Any(), ppfmt, domain.FQDN("ip4.hello1"), ipnet.IP4).
						Return(setter.ResponseUpdated),
					m.EXPECT().Delete(gomock.Any(), ppfmt, domain.FQDN("ip4.hello2"), ipnet.IP4).
						Return(setter.ResponseFailed),
					m.EXPECT().Delete(gomock.Any(), ppfmt, domain.FQDN("ip4.hello3"), ipnet.IP4).
						Return(setter.ResponseNoop),
					m.EXPECT().Delete(gomock.Any(), ppfmt, domain.FQDN("ip4.hello4"), ipnet.IP4).
						Return(setter.ResponseUpdated),
				)
			},
		},
		"3yes": { //nolint:dupl
			true,
			[]string{"Deleted A: ip4.hello1, ip4.hello3, ip4.hello4"},
			[]string{"Deleted A records of ip4.hello1, ip4.hello3, and ip4.hello4."},
			nil,
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				gomock.InOrder(
					m.EXPECT().Delete(gomock.Any(), ppfmt, domain.FQDN("ip4.hello1"), ipnet.IP4).
						Return(setter.ResponseUpdated),
					m.EXPECT().Delete(gomock.Any(), ppfmt, domain.FQDN("ip4.hello2"), ipnet.IP4).
						Return(setter.ResponseNoop),
					m.EXPECT().Delete(gomock.Any(), ppfmt, domain.FQDN("ip4.hello3"), ipnet.IP4).
						Return(setter.ResponseUpdated),
					m.EXPECT().Delete(gomock.Any(), ppfmt, domain.FQDN("ip4.hello4"), ipnet.IP4).
						Return(setter.ResponseUpdated),
				)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			ctx := context.Background()
			conf := config.Default()
			conf.Domains = domains
			conf.TTL = api.TTLAuto
			conf.Proxied = map[domain.Domain]bool{
				domain4_1: false,
				domain4_2: false,
				domain4_3: false,
				domain4_4: false,
			}
			conf.Use1001 = true
			conf.UpdateTimeout = time.Second
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}
			for k := range updater.ShouldDisplayHints {
				updater.ShouldDisplayHints[k] = true
			}
			for _, ipnet := range [...]ipnet.Type{ipnet.IP4, ipnet.IP6} {
				conf.Provider[ipnet] = mocks.NewMockProvider(mockCtrl)
			}
			mockSetter := mocks.NewMockSetter(mockCtrl)
			if tc.prepareMockSetter != nil {
				tc.prepareMockSetter(mockPP, mockSetter)
			}
			resp := updater.DeleteIPs(ctx, mockPP, conf, mockSetter)
			require.Equal(t, response.Response{
				Ok:               tc.ok,
				NotifierMessages: tc.notifierMessages,
				MonitorMessages:  tc.monitorMessages,
			}, resp)
		})
	}
}

//nolint:funlen,paralleltest // updater.IPv6MessageDisplayed is a global variable
func TestUpdateIPsUninitializedProbied(t *testing.T) {
	domain4 := domain.FQDN("ip4.hello")
	domains := map[ipnet.Type][]domain.Domain{
		ipnet.IP4: {domain4},
	}

	ip4 := netip.MustParseAddr("127.0.0.1")

	type mockproviders = map[ipnet.Type]func(ppfmt pp.PP, m *mocks.MockProvider)
	provider4 := func(ppfmt pp.PP, m *mocks.MockProvider) {
		m.EXPECT().GetIP(gomock.Any(), ppfmt, ipnet.IP4, true).Return(ip4, true)
	}

	for name, tc := range map[string]struct {
		ok                  bool
		monitorMessages     []string
		notifierMessages    []string
		prepareMockPP       func(m *mocks.MockPP)
		prepareMockProvider mockproviders
		prepareMockSetter   func(ppfmt pp.PP, m *mocks.MockSetter)
	}{
		"ip4only-proxied-nil": {
			true,
			[]string{"Set A (127.0.0.1): ip4.hello"},
			[]string{"Updated A records of ip4.hello with 127.0.0.1."},
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
					Return(setter.ResponseUpdated)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			ctx := context.Background()
			conf := config.Default()
			conf.Domains = domains
			conf.TTL = api.TTLAuto
			conf.Proxied = map[domain.Domain]bool{}
			conf.Use1001 = true
			conf.UpdateTimeout = time.Second
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}
			for k := range updater.ShouldDisplayHints {
				updater.ShouldDisplayHints[k] = true
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
			resp := updater.UpdateIPs(ctx, mockPP, conf, mockSetter)
			require.Equal(t, response.Response{
				Ok:               tc.ok,
				NotifierMessages: tc.notifierMessages,
				MonitorMessages:  tc.monitorMessages,
			}, resp)
		})
	}
}

//nolint:funlen,paralleltest // updater.IPv6MessageDisplayed is a global variable
func TestUpdateIPsHints(t *testing.T) {
	domain4 := domain.FQDN("ip4.hello")
	domain6 := domain.FQDN("ip6.hello")
	domains := map[ipnet.Type][]domain.Domain{
		ipnet.IP4: {domain4},
		ipnet.IP6: {domain6},
	}

	ip4 := netip.MustParseAddr("127.0.0.1")

	type mockproviders = map[ipnet.Type]func(ppfmt pp.PP, m *mocks.MockProvider)
	provider4 := func(ppfmt pp.PP, m *mocks.MockProvider) {
		m.EXPECT().GetIP(gomock.Any(), ppfmt, ipnet.IP4, true).Return(ip4, true)
	}

	for name, tc := range map[string]struct {
		ShouldDisplayHints  map[string]bool
		ok                  bool
		monitorMessages     []string
		notifierMessages    []string
		prepareMockPP       func(m *mocks.MockPP)
		prepareMockProvider mockproviders
		prepareMockSetter   func(ppfmt pp.PP, m *mocks.MockSetter)
	}{
		"ip6fails/again": {
			map[string]bool{"detect-ip4-fail": true, "detect-ip6-fail": false, "update-timeout": true},
			false,
			[]string{"Failed to detect IPv6 address"},
			[]string{"Failed to detect the IPv6 address."},
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
				m.EXPECT().Set(gomock.Any(), ppfmt, domain.FQDN("ip4.hello"), ipnet.IP4, ip4, api.TTLAuto, false).
					Return(setter.ResponseNoop)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			ctx := context.Background()
			conf := config.Default()
			conf.Domains = domains
			conf.TTL = api.TTLAuto
			conf.Proxied = map[domain.Domain]bool{domain4: false, domain6: false}
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
			resp := updater.UpdateIPs(ctx, mockPP, conf, mockSetter)
			require.Equal(t, response.Response{
				Ok:               tc.ok,
				NotifierMessages: tc.notifierMessages,
				MonitorMessages:  tc.monitorMessages,
			}, resp)
		})
	}
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

	for name, tc := range map[string]struct {
		ok                  bool
		monitorMessages     []string
		notifierMessages    []string
		prepareMockPP       func(m *mocks.MockPP)
		prepareMockProvider mockproviders
		prepareMockSetter   func(ppfmt pp.PP, m *mocks.MockSetter)
	}{
		"none": {
			true, []string{}, []string{}, nil, mockproviders{}, nil,
		},
		"ip4only": {
			true,
			[]string{},
			[]string{},
			pp4only,
			mockproviders{ipnet.IP4: provider4},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				m.EXPECT().Set(gomock.Any(), ppfmt, domain.FQDN("ip4.hello"), ipnet.IP4, ip4, api.TTLAuto, false).
					Return(setter.ResponseNoop)
			},
		},
		"ip4only/setfail": {
			false,
			[]string{"Failed to set A (127.0.0.1): ip4.hello"},
			[]string{"Failed to finish updating A records of ip4.hello with 127.0.0.1."},
			pp4only,
			mockproviders{ipnet.IP4: provider4},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				m.EXPECT().Set(gomock.Any(), ppfmt, domain.FQDN("ip4.hello"), ipnet.IP4, ip4, api.TTLAuto, false).
					Return(setter.ResponseFailed)
			},
		},
		"ip6only": {
			true,
			[]string{"Set AAAA (::1): ip6.hello"},
			[]string{"Updated AAAA records of ip6.hello with ::1."},
			pp6only,
			mockproviders{ipnet.IP6: provider6},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				m.EXPECT().Set(gomock.Any(), ppfmt, domain.FQDN("ip6.hello"), ipnet.IP6, ip6, api.TTLAuto, false).
					Return(setter.ResponseUpdated)
			},
		},
		"ip6only/setfail": {
			false,
			[]string{"Failed to set AAAA (::1): ip6.hello"},
			[]string{"Failed to finish updating AAAA records of ip6.hello with ::1."},
			pp6only,
			mockproviders{ipnet.IP6: provider6},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				m.EXPECT().Set(gomock.Any(), ppfmt, domain.FQDN("ip6.hello"), ipnet.IP6, ip6, api.TTLAuto, false).
					Return(setter.ResponseFailed)
			},
		},
		"both": {
			true,
			[]string{},
			[]string{},
			ppBoth,
			mockproviders{ipnet.IP4: provider4, ipnet.IP6: provider6},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				gomock.InOrder(
					m.EXPECT().Set(gomock.Any(), ppfmt, domain.FQDN("ip4.hello"), ipnet.IP4, ip4, api.TTLAuto, false).
						Return(setter.ResponseNoop),
					m.EXPECT().Set(gomock.Any(), ppfmt, domain.FQDN("ip6.hello"), ipnet.IP6, ip6, api.TTLAuto, false).
						Return(setter.ResponseNoop),
				)
			},
		},
		"both/setfail1": {
			false,
			[]string{"Failed to set A (127.0.0.1): ip4.hello"},
			[]string{"Failed to finish updating A records of ip4.hello with 127.0.0.1."},
			ppBoth,
			mockproviders{ipnet.IP4: provider4, ipnet.IP6: provider6},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				gomock.InOrder(
					m.EXPECT().Set(gomock.Any(), ppfmt, domain.FQDN("ip4.hello"), ipnet.IP4, ip4, api.TTLAuto, false).
						Return(setter.ResponseFailed),
					m.EXPECT().Set(gomock.Any(), ppfmt, domain.FQDN("ip6.hello"), ipnet.IP6, ip6, api.TTLAuto, false).
						Return(setter.ResponseNoop),
				)
			},
		},
		"both/setfail2": {
			false,
			[]string{"Failed to set AAAA (::1): ip6.hello"},
			[]string{"Failed to finish updating AAAA records of ip6.hello with ::1."},
			ppBoth,
			mockproviders{ipnet.IP4: provider4, ipnet.IP6: provider6},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				gomock.InOrder(
					m.EXPECT().Set(gomock.Any(), ppfmt, domain.FQDN("ip4.hello"), ipnet.IP4, ip4, api.TTLAuto, false).
						Return(setter.ResponseNoop),
					m.EXPECT().Set(gomock.Any(), ppfmt, domain.FQDN("ip6.hello"), ipnet.IP6, ip6, api.TTLAuto, false).
						Return(setter.ResponseFailed),
				)
			},
		},
		"ip4fails": {
			false,
			[]string{"Failed to detect IPv4 address"},
			[]string{"Failed to detect the IPv4 address."},
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Errorf(pp.EmojiError, "Failed to detect the %s address", "IPv4"),
					m.EXPECT().Infof(pp.EmojiHint, "If your network does not support IPv4, you can disable it with IP4_PROVIDER=none"), //nolint:lll
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
				m.EXPECT().Set(gomock.Any(), ppfmt, domain.FQDN("ip6.hello"), ipnet.IP6, ip6, api.TTLAuto, false).
					Return(setter.ResponseNoop)
			},
		},
		"ip6fails": {
			false,
			[]string{"Failed to detect IPv6 address"},
			[]string{"Failed to detect the IPv6 address."},
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Infof(pp.EmojiInternet, "Detected the %s address: %v", "IPv4", ip4),
					m.EXPECT().Errorf(pp.EmojiError, "Failed to detect the %s address", "IPv6"),
					m.EXPECT().Infof(pp.EmojiHint, "If you are using Docker or Kubernetes, IPv6 often requires additional setups"),     //nolint:lll
					m.EXPECT().Infof(pp.EmojiHint, "Read more about IPv6 networks at https://github.com/favonia/cloudflare-ddns"),      //nolint:lll
					m.EXPECT().Infof(pp.EmojiHint, "If your network does not support IPv6, you can disable it with IP6_PROVIDER=none"), //nolint:lll
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
					Return(setter.ResponseNoop)
			},
		},
		"bothfail": {
			false,
			[]string{"Failed to detect IPv4 address", "Failed to detect IPv6 address"},
			[]string{"Failed to detect the IPv4 address.", "Failed to detect the IPv6 address."},
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Errorf(pp.EmojiError, "Failed to detect the %s address", "IPv4"),
					m.EXPECT().Infof(pp.EmojiHint, "If your network does not support IPv4, you can disable it with IP4_PROVIDER=none"), //nolint:lll
					m.EXPECT().Errorf(pp.EmojiError, "Failed to detect the %s address", "IPv6"),
					m.EXPECT().Infof(pp.EmojiHint, "If you are using Docker or Kubernetes, IPv6 often requires additional setups"),     //nolint:lll
					m.EXPECT().Infof(pp.EmojiHint, "Read more about IPv6 networks at https://github.com/favonia/cloudflare-ddns"),      //nolint:lll
					m.EXPECT().Infof(pp.EmojiHint, "If your network does not support IPv6, you can disable it with IP6_PROVIDER=none"), //nolint:lll
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
		"slow-setting": {
			false,
			[]string{"Failed to set A (127.0.0.1): ip4.hello"},
			[]string{"Failed to finish updating A records of ip4.hello with 127.0.0.1."},
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Infof(pp.EmojiInternet, "Detected the %s address: %v", "IPv4", ip4),
					m.EXPECT().Infof(pp.EmojiHint, "If your network is working but with high latency, consider increasing the value of UPDATE_TIMEOUT"), //nolint:lll
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
							return setter.ResponseFailed
						})
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			ctx := context.Background()
			conf := config.Default()
			conf.Domains = domains
			conf.TTL = api.TTLAuto
			conf.Proxied = map[domain.Domain]bool{domain4: false, domain6: false}
			conf.Use1001 = true
			conf.UpdateTimeout = time.Second
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}
			for k := range updater.ShouldDisplayHints {
				updater.ShouldDisplayHints[k] = true
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
			resp := updater.UpdateIPs(ctx, mockPP, conf, mockSetter)
			require.Equal(t, response.Response{
				Ok:               tc.ok,
				NotifierMessages: tc.notifierMessages,
				MonitorMessages:  tc.monitorMessages,
			}, resp)
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

	for name, tc := range map[string]struct {
		ok                  bool
		monitorMessages     []string
		notifierMessages    []string
		prepareMockPP       func(m *mocks.MockPP)
		prepareMockProvider mockproviders
		prepareMockSetter   func(ppfmt pp.PP, m *mocks.MockSetter)
	}{
		"none": {
			true,
			[]string{},
			[]string{},
			nil,
			mockproviders{},
			nil,
		},
		"ip4only": {
			true,
			[]string{"Deleted A: ip4.hello"},
			[]string{"Deleted A records of ip4.hello."},
			nil,
			mockproviders{ipnet.IP4: true},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				m.EXPECT().Delete(gomock.Any(), ppfmt, domain4, ipnet.IP4).
					Return(setter.ResponseUpdated)
			},
		},
		"ip4only/setfail": {
			false,
			[]string{"Failed to delete A: ip4.hello"},
			[]string{"Failed to finish deleting A records of ip4.hello."},
			nil,
			mockproviders{ipnet.IP4: true},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				m.EXPECT().Delete(gomock.Any(), ppfmt, domain4, ipnet.IP4).Return(setter.ResponseFailed)
			},
		},
		"ip6only": {
			true,
			[]string{"Deleted AAAA: ip6.hello"},
			[]string{"Deleted AAAA records of ip6.hello."},
			nil,
			mockproviders{ipnet.IP6: true},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				m.EXPECT().Delete(gomock.Any(), ppfmt, domain6, ipnet.IP6).Return(setter.ResponseUpdated)
			},
		},
		"ip6only/setfail": {
			false,
			[]string{"Failed to delete AAAA: ip6.hello"},
			[]string{"Failed to finish deleting AAAA records of ip6.hello."},
			nil,
			mockproviders{ipnet.IP6: true},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				m.EXPECT().Delete(gomock.Any(), ppfmt, domain6, ipnet.IP6).Return(setter.ResponseFailed)
			},
		},
		"both": {
			true,
			[]string{"Deleted A: ip4.hello", "Deleted AAAA: ip6.hello"},
			[]string{"Deleted A records of ip4.hello.", "Deleted AAAA records of ip6.hello."},
			nil,
			mockproviders{ipnet.IP4: true, ipnet.IP6: true},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				gomock.InOrder(
					m.EXPECT().Delete(gomock.Any(), ppfmt, domain4, ipnet.IP4).Return(setter.ResponseUpdated),
					m.EXPECT().Delete(gomock.Any(), ppfmt, domain6, ipnet.IP6).Return(setter.ResponseUpdated),
				)
			},
		},
		"both/setfail1": {
			false,
			[]string{"Failed to delete A: ip4.hello"},
			[]string{"Failed to finish deleting A records of ip4.hello."},
			nil,
			mockproviders{ipnet.IP4: true, ipnet.IP6: true},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				gomock.InOrder(
					m.EXPECT().Delete(gomock.Any(), ppfmt, domain4, ipnet.IP4).Return(setter.ResponseFailed),
					m.EXPECT().Delete(gomock.Any(), ppfmt, domain6, ipnet.IP6).Return(setter.ResponseNoop),
				)
			},
		},
		"both/setfail2": {
			false,
			[]string{"Failed to delete AAAA: ip6.hello"},
			[]string{"Failed to finish deleting AAAA records of ip6.hello."},
			nil,
			mockproviders{ipnet.IP4: true, ipnet.IP6: true},
			func(ppfmt pp.PP, m *mocks.MockSetter) {
				gomock.InOrder(
					m.EXPECT().Delete(gomock.Any(), ppfmt, domain4, ipnet.IP4).Return(setter.ResponseNoop),
					m.EXPECT().Delete(gomock.Any(), ppfmt, domain6, ipnet.IP6).Return(setter.ResponseFailed),
				)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			ctx := context.Background()
			conf := config.Default()
			conf.Domains = domains
			conf.TTL = api.TTLAuto
			conf.Proxied = map[domain.Domain]bool{domain4: false, domain6: false}
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}
			for k := range updater.ShouldDisplayHints {
				updater.ShouldDisplayHints[k] = true
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
			resp := updater.DeleteIPs(ctx, mockPP, conf, mockSetter)

			require.Equal(t, response.Response{
				Ok:               tc.ok,
				NotifierMessages: tc.notifierMessages,
				MonitorMessages:  tc.monitorMessages,
			}, resp)
		})
	}
}
