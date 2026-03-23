// vim: nowrap
package updater_test

import (
	"context"
	"maps"
	"net/netip"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/config"
	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/heartbeat"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/notifier"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider"
	"github.com/favonia/cloudflare-ddns/internal/setter"
	"github.com/favonia/cloudflare-ddns/internal/updater"
)

const (
	recordComment      string = "hello record"
	wafListDescription string = "hello list"
	wafItemComment     string = "hello waf item"
)

type (
	providerEnablers = map[ipnet.Family]bool
	mockProviders    = map[ipnet.Family]*mocks.MockProvider
	familyTargets    = map[ipnet.Family]setter.WAFTargets
	cleanupFamilies  = map[ipnet.Family]bool
)

const (
	domain4   = domain.FQDN("ip4.hello")
	domain4_1 = domain.FQDN("ip4.hello1")
	domain4_2 = domain.FQDN("ip4.hello2")
	domain4_3 = domain.FQDN("ip4.hello3")
	domain4_4 = domain.FQDN("ip4.hello4")
	domain6   = domain.FQDN("ip6.hello")
)

func initUpdateConfig() *config.UpdateConfig {
	conf := &config.UpdateConfig{} //nolint:exhaustruct // Tests build only the runtime fields updater behavior depends on.
	conf.TTL = api.TTLAuto
	conf.RecordComment = recordComment
	conf.WAFListDescription = wafListDescription
	conf.WAFListItemComment = wafItemComment
	conf.DefaultPrefixLen = map[ipnet.Family]int{
		ipnet.IP4: 32,
		ipnet.IP6: 64,
	}
	conf.DetectionTimeout = time.Second
	conf.UpdateTimeout = time.Second
	conf.Provider = map[ipnet.Family]provider.Provider{
		ipnet.IP4: nil,
		ipnet.IP6: nil,
	}
	conf.Domains = map[ipnet.Family][]domain.Domain{
		ipnet.IP4: nil,
		ipnet.IP6: nil,
	}
	conf.Proxied = map[domain.Domain]bool{
		domain4:   false,
		domain4_1: false,
		domain4_2: false,
		domain4_3: false,
		domain4_4: false,
		domain6:   false,
	}
	return conf
}

func hintIP6DetectionFails(p *mocks.MockPP) *mocks.MockPPNoticeOncefCall {
	return p.EXPECT().NoticeOncef(pp.MessageIP6DetectionFails, pp.EmojiHint, "If you are using Docker or Kubernetes, IPv6 might need extra setup. Read more at %s. If your network doesn't support IPv6, you can stop managing it by setting IP6_PROVIDER=none", pp.ManualURL)
}

func detectionResult(ipFamily ipnet.Family, ips []netip.Addr) provider.DetectionResult {
	prefixLen := 32
	if ipFamily == ipnet.IP6 {
		prefixLen = 64
	}

	rawEntries := make([]ipnet.RawEntry, 0, len(ips))
	for _, ip := range ips {
		rawEntries = append(rawEntries, ipnet.RawEntryFrom(ip, prefixLen))
	}
	return provider.NewKnownDetectionResult(rawEntries)
}

func wafTargets(ip4, ip6 []netip.Addr) familyTargets {
	result := familyTargets{}
	if ip4 != nil {
		prefixes := make([]netip.Prefix, 0, len(ip4))
		for _, ip := range ip4 {
			prefixes = append(prefixes, netip.PrefixFrom(ip, 32).Masked())
		}
		result[ipnet.IP4] = setter.NewAvailableWAFTargets(prefixes)
	}
	if ip6 != nil {
		prefixes := make([]netip.Prefix, 0, len(ip6))
		for _, ip := range ip6 {
			prefixes = append(prefixes, netip.PrefixFrom(ip, 64).Masked())
		}
		result[ipnet.IP6] = setter.NewAvailableWAFTargets(prefixes)
	}
	return result
}

func withUnavailableTargets(base familyTargets, families ...ipnet.Family) familyTargets {
	cloned := make(familyTargets, len(base))
	maps.Copy(cloned, base)
	for _, ipFamily := range families {
		cloned[ipFamily] = setter.NewUnavailableWAFTargets()
	}
	return cloned
}

func runUpdateIPsScenario(
	t *testing.T,
	domains map[ipnet.Family][]domain.Domain,
	lists []api.WAFList,
	enabled providerEnablers,
	prepareMocks func(*mocks.MockPP, mockProviders, *mocks.MockSetter),
) updater.Message {
	t.Helper()

	mockCtrl := gomock.NewController(t)
	ctx := context.Background()

	conf := initUpdateConfig()
	conf.Domains = domains
	conf.WAFLists = lists

	mockPP := mocks.NewMockPP(mockCtrl)
	mockProviders := make(mockProviders)
	for ipnet := range enabled {
		mockProvider := mocks.NewMockProvider(mockCtrl)
		conf.Provider[ipnet] = mockProvider
		mockProviders[ipnet] = mockProvider
	}
	mockSetter := mocks.NewMockSetter(mockCtrl)
	if prepareMocks != nil {
		prepareMocks(mockPP, mockProviders, mockSetter)
	}

	return updater.UpdateIPs(ctx, mockPP, conf, mockSetter)
}

func runFinalDeleteIPsScenario(
	t *testing.T,
	domains map[ipnet.Family][]domain.Domain,
	lists []api.WAFList,
	enabled providerEnablers,
	prepareMocks func(*mocks.MockPP, *mocks.MockSetter),
) updater.Message {
	t.Helper()

	mockCtrl := gomock.NewController(t)
	ctx := context.Background()

	conf := initUpdateConfig()
	conf.Domains = domains
	conf.WAFLists = lists

	mockPP := mocks.NewMockPP(mockCtrl)
	mockSetter := mocks.NewMockSetter(mockCtrl)
	if prepareMocks != nil {
		prepareMocks(mockPP, mockSetter)
	}

	for _, ipnet := range [...]ipnet.Family{ipnet.IP4, ipnet.IP6} {
		if !enabled[ipnet] {
			conf.Provider[ipnet] = nil
			continue
		}

		conf.Provider[ipnet] = mocks.NewMockProvider(mockCtrl)
	}

	return updater.FinalDeleteIPs(ctx, mockPP, conf, mockSetter)
}

func TestUpdateIPsMultiple(t *testing.T) {
	t.Parallel()

	params := api.RecordParams{
		TTL:     api.TTLAuto,
		Proxied: false,
		Comment: recordComment,
		Tags:    nil,
	}

	domains := map[ipnet.Family][]domain.Domain{
		ipnet.IP4: {domain4_1, domain4_2, domain4_3, domain4_4},
	}

	list1 := api.WAFList{AccountID: "12341234", Name: "list1"}
	list2 := api.WAFList{AccountID: "xxxxxxxx", Name: "list2"}
	list3 := api.WAFList{AccountID: "AAAAAAAA", Name: "list3"}
	list4 := api.WAFList{AccountID: "zzz", Name: "list4"}
	lists := []api.WAFList{list1, list2, list3, list4}

	ip4a := netip.MustParseAddr("127.0.0.1")
	ip4b := netip.MustParseAddr("127.0.0.2")
	ip4Targets := []netip.Addr{ip4a, ip4b}

	for name, tc := range map[string]struct {
		ok               bool
		monitorMessages  []string
		notifierMessages []string
		providerEnablers providerEnablers
		prepareMocks     func(*mocks.MockPP, mockProviders, *mocks.MockSetter)
	}{
		"1yes1doing1no": {
			false,
			[]string{
				"Could not confirm update of A (127.0.0.1, 127.0.0.2) for ip4.hello2",
				"Could not confirm update of WAF list(s) xxxxxxxx/list2",
			},
			[]string{
				"Could not confirm update of A records of ip4.hello2 with 127.0.0.1 and 127.0.0.2; updating those of ip4.hello1; updated those of ip4.hello4.",
				`Could not confirm update of WAF list(s) xxxxxxxx/list2; updating 12341234/list1; updated zzz/list4.`,
			},
			providerEnablers{ipnet.IP4: true},
			func(p *mocks.MockPP, pv mockProviders, s *mocks.MockSetter) {
				gomock.InOrder(
					pv[ipnet.IP4].EXPECT().GetRawData(gomock.Any(), p, ipnet.IP4, 32).Return(detectionResult(ipnet.IP4, ip4Targets)),
					p.EXPECT().Infof(pp.EmojiInternet, "Detected %d %s addresses: %s", 2, "IPv4", "127.0.0.1, 127.0.0.2"),
					p.EXPECT().Suppress(pp.MessageIP4DetectionFails),
					s.EXPECT().SetIPs(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello1"), ip4Targets, params).Return(setter.ResponseUpdating),
					s.EXPECT().SetIPs(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello2"), ip4Targets, params).Return(setter.ResponseFailed),
					s.EXPECT().SetIPs(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello3"), ip4Targets, params).Return(setter.ResponseNoop),
					s.EXPECT().SetIPs(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello4"), ip4Targets, params).Return(setter.ResponseUpdated),
					s.EXPECT().SetWAFList(gomock.Any(), p, list1, wafListDescription, wafTargets(ip4Targets, nil), wafItemComment).Return(setter.ResponseUpdating),
					s.EXPECT().SetWAFList(gomock.Any(), p, list2, wafListDescription, wafTargets(ip4Targets, nil), wafItemComment).Return(setter.ResponseFailed),
					s.EXPECT().SetWAFList(gomock.Any(), p, list3, wafListDescription, wafTargets(ip4Targets, nil), wafItemComment).Return(setter.ResponseNoop),
					s.EXPECT().SetWAFList(gomock.Any(), p, list4, wafListDescription, wafTargets(ip4Targets, nil), wafItemComment).Return(setter.ResponseUpdated),
				)
			},
		},
		"2yes1doing": {
			true,
			[]string{
				"Set A (127.0.0.1, 127.0.0.2) of ip4.hello1, ip4.hello3, ip4.hello4",
				"Setting list(s) 12341234/list1",
				"Set list(s) AAAAAAAA/list3, zzz/list4",
			},
			[]string{
				"Updated A records of ip4.hello1, ip4.hello3, and ip4.hello4 with 127.0.0.1 and 127.0.0.2.",
				`Updating WAF list(s) 12341234/list1; updated AAAAAAAA/list3 and zzz/list4.`,
			},
			providerEnablers{ipnet.IP4: true},
			func(p *mocks.MockPP, pv mockProviders, s *mocks.MockSetter) {
				gomock.InOrder(
					pv[ipnet.IP4].EXPECT().GetRawData(gomock.Any(), p, ipnet.IP4, 32).Return(detectionResult(ipnet.IP4, ip4Targets)),
					p.EXPECT().Infof(pp.EmojiInternet, "Detected %d %s addresses: %s", 2, "IPv4", "127.0.0.1, 127.0.0.2"),
					p.EXPECT().Suppress(pp.MessageIP4DetectionFails),
					s.EXPECT().SetIPs(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello1"), ip4Targets, params).Return(setter.ResponseUpdated),
					s.EXPECT().SetIPs(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello2"), ip4Targets, params).Return(setter.ResponseNoop),
					s.EXPECT().SetIPs(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello3"), ip4Targets, params).Return(setter.ResponseUpdated),
					s.EXPECT().SetIPs(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello4"), ip4Targets, params).Return(setter.ResponseUpdated),
					s.EXPECT().SetWAFList(gomock.Any(), p, list1, wafListDescription, wafTargets(ip4Targets, nil), wafItemComment).Return(setter.ResponseUpdating),
					s.EXPECT().SetWAFList(gomock.Any(), p, list2, wafListDescription, wafTargets(ip4Targets, nil), wafItemComment).Return(setter.ResponseNoop),
					s.EXPECT().SetWAFList(gomock.Any(), p, list3, wafListDescription, wafTargets(ip4Targets, nil), wafItemComment).Return(setter.ResponseUpdated),
					s.EXPECT().SetWAFList(gomock.Any(), p, list4, wafListDescription, wafTargets(ip4Targets, nil), wafItemComment).Return(setter.ResponseUpdated),
				)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			ctx := context.Background()

			conf := initUpdateConfig()
			conf.Domains = domains
			conf.WAFLists = lists

			mockPP := mocks.NewMockPP(mockCtrl)
			mockProviders := make(mockProviders)
			for ipnet := range tc.providerEnablers {
				mockProvider := mocks.NewMockProvider(mockCtrl)
				conf.Provider[ipnet] = mockProvider
				mockProviders[ipnet] = mockProvider
			}
			mockSetter := mocks.NewMockSetter(mockCtrl)
			if tc.prepareMocks != nil {
				tc.prepareMocks(mockPP, mockProviders, mockSetter)
			}

			resp := updater.UpdateIPs(ctx, mockPP, conf, mockSetter)
			require.Equal(t, updater.Message{
				HeartbeatMessage: heartbeat.Message{
					OK:    tc.ok,
					Lines: tc.monitorMessages,
				},
				NotifierMessage: notifier.Message(tc.notifierMessages),
			}, resp)
		})
	}
}

func TestFinalDeleteIPsMultiple(t *testing.T) {
	t.Parallel()

	params := api.RecordParams{
		TTL:     api.TTLAuto,
		Proxied: false,
		Comment: recordComment,
		Tags:    nil,
	}

	domains := map[ipnet.Family][]domain.Domain{
		ipnet.IP4: {domain4_1, domain4_2, domain4_3, domain4_4},
	}

	list1 := api.WAFList{AccountID: "12341234", Name: "list1"}
	list2 := api.WAFList{AccountID: "xxxxxxxx", Name: "list2"}
	list3 := api.WAFList{AccountID: "AAAAAAAA", Name: "list3"}
	list4 := api.WAFList{AccountID: "zzz", Name: "list4"}
	lists := []api.WAFList{list1, list2, list3, list4}

	for name, tc := range map[string]struct {
		ok               bool
		monitorMessages  []string
		notifierMessages []string
		prepareMocks     func(*mocks.MockPP, *mocks.MockSetter)
	}{
		"1yes1doing1no": {
			false,
			[]string{
				"Could not confirm deletion of A of ip4.hello2",
				"Could not confirm cleanup of WAF list(s) xxxxxxxx/list2",
			},
			[]string{
				"Could not confirm deletion of A records of ip4.hello2; deleting those of ip4.hello1; deleted those of ip4.hello4.",
				`Could not confirm cleanup of WAF list(s) xxxxxxxx/list2; cleaning 12341234/list1; cleaned zzz/list4.`,
			},
			func(p *mocks.MockPP, s *mocks.MockSetter) {
				gomock.InOrder(
					s.EXPECT().FinalDelete(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello1"), params).Return(setter.ResponseUpdating),
					s.EXPECT().FinalDelete(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello2"), params).Return(setter.ResponseFailed),
					s.EXPECT().FinalDelete(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello3"), params).Return(setter.ResponseNoop),
					s.EXPECT().FinalDelete(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello4"), params).Return(setter.ResponseUpdated),
					s.EXPECT().FinalClearWAFList(gomock.Any(), p, list1, wafListDescription, gomock.Any()).Return(setter.ResponseUpdating),
					s.EXPECT().FinalClearWAFList(gomock.Any(), p, list2, wafListDescription, gomock.Any()).Return(setter.ResponseFailed),
					s.EXPECT().FinalClearWAFList(gomock.Any(), p, list3, wafListDescription, gomock.Any()).Return(setter.ResponseNoop),
					s.EXPECT().FinalClearWAFList(gomock.Any(), p, list4, wafListDescription, gomock.Any()).Return(setter.ResponseUpdated),
				)
			},
		},
		"3yes": {
			true,
			[]string{
				"Deleted A of ip4.hello1, ip4.hello3, ip4.hello4",
				"Cleaned WAF list(s) 12341234/list1, AAAAAAAA/list3, zzz/list4",
			},
			[]string{
				"Deleted A records of ip4.hello1, ip4.hello3, and ip4.hello4.",
				`Cleaned WAF list(s) 12341234/list1, AAAAAAAA/list3, and zzz/list4.`,
			},
			func(p *mocks.MockPP, s *mocks.MockSetter) {
				gomock.InOrder(
					s.EXPECT().FinalDelete(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello1"), params).Return(setter.ResponseUpdated),
					s.EXPECT().FinalDelete(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello2"), params).Return(setter.ResponseNoop),
					s.EXPECT().FinalDelete(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello3"), params).Return(setter.ResponseUpdated),
					s.EXPECT().FinalDelete(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello4"), params).Return(setter.ResponseUpdated),
					s.EXPECT().FinalClearWAFList(gomock.Any(), p, list1, wafListDescription, gomock.Any()).Return(setter.ResponseUpdated),
					s.EXPECT().FinalClearWAFList(gomock.Any(), p, list2, wafListDescription, gomock.Any()).Return(setter.ResponseNoop),
					s.EXPECT().FinalClearWAFList(gomock.Any(), p, list3, wafListDescription, gomock.Any()).Return(setter.ResponseUpdated),
					s.EXPECT().FinalClearWAFList(gomock.Any(), p, list4, wafListDescription, gomock.Any()).Return(setter.ResponseUpdated),
				)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			ctx := context.Background()

			conf := initUpdateConfig()
			conf.Domains = domains
			conf.WAFLists = lists

			mockPP := mocks.NewMockPP(mockCtrl)
			for _, ipnet := range [...]ipnet.Family{ipnet.IP4, ipnet.IP6} {
				conf.Provider[ipnet] = mocks.NewMockProvider(mockCtrl)
			}
			mockSetter := mocks.NewMockSetter(mockCtrl)
			if tc.prepareMocks != nil {
				tc.prepareMocks(mockPP, mockSetter)
			}
			resp := updater.FinalDeleteIPs(ctx, mockPP, conf, mockSetter)
			require.Equal(t, updater.Message{
				HeartbeatMessage: heartbeat.Message{
					OK:    tc.ok,
					Lines: tc.monitorMessages,
				},
				NotifierMessage: notifier.Message(tc.notifierMessages),
			}, resp)
		})
	}
}

func TestUpdateIPs(t *testing.T) {
	t.Parallel()

	params := api.RecordParams{
		TTL:     api.TTLAuto,
		Proxied: false,
		Comment: recordComment,
		Tags:    nil,
	}

	domains := map[ipnet.Family][]domain.Domain{
		ipnet.IP4: {domain4},
		ipnet.IP6: {domain6},
	}
	list := api.WAFList{AccountID: "12341234", Name: "list"}
	lists := []api.WAFList{list}

	ip4 := netip.MustParseAddr("127.0.0.1")
	ip6 := netip.MustParseAddr("::1")

	for name, tc := range map[string]struct {
		ok               bool
		monitorMessages  []string
		notifierMessages []string
		providerEnablers providerEnablers
		prepareMocks     func(*mocks.MockPP, mockProviders, *mocks.MockSetter)
	}{
		"ip4-only": {
			true, nil, nil,
			providerEnablers{ipnet.IP4: true},
			func(p *mocks.MockPP, pv mockProviders, s *mocks.MockSetter) {
				gomock.InOrder(
					pv[ipnet.IP4].EXPECT().GetRawData(gomock.Any(), p, ipnet.IP4, 32).Return(detectionResult(ipnet.IP4, []netip.Addr{ip4})),
					p.EXPECT().Infof(pp.EmojiInternet, "Detected the %s address %v", "IPv4", ip4),
					p.EXPECT().Suppress(pp.MessageIP4DetectionFails),
					s.EXPECT().SetIPs(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello"), []netip.Addr{ip4}, params).Return(setter.ResponseNoop),
					s.EXPECT().SetWAFList(gomock.Any(), p, list, wafListDescription, wafTargets([]netip.Addr{ip4}, nil), wafItemComment).Return(setter.ResponseNoop),
				)
			},
		},
		"ip4-only/clear": {
			true,
			[]string{"Cleared A of ip4.hello", "Set list(s) 12341234/list"},
			[]string{"Cleared A records of ip4.hello.", `Updated WAF list(s) 12341234/list.`},
			providerEnablers{ipnet.IP4: true},
			func(p *mocks.MockPP, pv mockProviders, s *mocks.MockSetter) {
				gomock.InOrder(
					pv[ipnet.IP4].EXPECT().GetRawData(gomock.Any(), p, ipnet.IP4, 32).Return(detectionResult(ipnet.IP4, []netip.Addr{})),
					p.EXPECT().Infof(pp.EmojiInternet, "The desired %s raw data set is empty", "IPv4"),
					p.EXPECT().Suppress(pp.MessageIP4DetectionFails),
					s.EXPECT().SetIPs(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello"), []netip.Addr{}, params).Return(setter.ResponseUpdated),
					s.EXPECT().SetWAFList(gomock.Any(), p, list, wafListDescription, wafTargets([]netip.Addr{}, nil), wafItemComment).Return(setter.ResponseUpdated),
				)
			},
		},
		"ip4-only/set-fail": {
			false,
			[]string{"Could not confirm update of A (127.0.0.1) for ip4.hello", "Could not confirm update of WAF list(s) 12341234/list"},
			[]string{"Could not confirm update of A records of ip4.hello with 127.0.0.1.", `Could not confirm update of WAF list(s) 12341234/list.`},
			providerEnablers{ipnet.IP4: true},
			func(p *mocks.MockPP, pv mockProviders, s *mocks.MockSetter) {
				gomock.InOrder(
					pv[ipnet.IP4].EXPECT().GetRawData(gomock.Any(), p, ipnet.IP4, 32).Return(detectionResult(ipnet.IP4, []netip.Addr{ip4})),
					p.EXPECT().Infof(pp.EmojiInternet, "Detected the %s address %v", "IPv4", ip4),
					p.EXPECT().Suppress(pp.MessageIP4DetectionFails),
					s.EXPECT().SetIPs(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello"), []netip.Addr{ip4}, params).Return(setter.ResponseFailed),
					s.EXPECT().SetWAFList(gomock.Any(), p, list, wafListDescription, wafTargets([]netip.Addr{ip4}, nil), wafItemComment).Return(setter.ResponseFailed),
				)
			},
		},
		"ip4-only/setting": {
			true,
			[]string{"Setting A (127.0.0.1) of ip4.hello", "Setting list(s) 12341234/list"},
			[]string{"Updating A records of ip4.hello with 127.0.0.1.", `Updating WAF list(s) 12341234/list.`},
			providerEnablers{ipnet.IP4: true},
			func(p *mocks.MockPP, pv mockProviders, s *mocks.MockSetter) {
				gomock.InOrder(
					pv[ipnet.IP4].EXPECT().GetRawData(gomock.Any(), p, ipnet.IP4, 32).Return(detectionResult(ipnet.IP4, []netip.Addr{ip4})),
					p.EXPECT().Infof(pp.EmojiInternet, "Detected the %s address %v", "IPv4", ip4),
					p.EXPECT().Suppress(pp.MessageIP4DetectionFails),
					s.EXPECT().SetIPs(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello"), []netip.Addr{ip4}, params).Return(setter.ResponseUpdating),
					s.EXPECT().SetWAFList(gomock.Any(), p, list, wafListDescription, wafTargets([]netip.Addr{ip4}, nil), wafItemComment).Return(setter.ResponseUpdating),
				)
			},
		},
		"ip6-only": {
			true,
			[]string{"Set AAAA (::1) of ip6.hello", "Set list(s) 12341234/list"},
			[]string{"Updated AAAA records of ip6.hello with ::1.", `Updated WAF list(s) 12341234/list.`},
			providerEnablers{ipnet.IP6: true},
			func(p *mocks.MockPP, pv mockProviders, s *mocks.MockSetter) {
				gomock.InOrder(
					pv[ipnet.IP6].EXPECT().GetRawData(gomock.Any(), p, ipnet.IP6, 64).Return(detectionResult(ipnet.IP6, []netip.Addr{ip6})),
					p.EXPECT().Infof(pp.EmojiInternet, "Detected the %s address %v", "IPv6", ip6),
					p.EXPECT().Suppress(pp.MessageIP6DetectionFails),
					s.EXPECT().SetIPs(gomock.Any(), p, ipnet.IP6, domain.FQDN("ip6.hello"), []netip.Addr{ip6}, params).Return(setter.ResponseUpdated),
					s.EXPECT().SetWAFList(gomock.Any(), p, list, wafListDescription, wafTargets(nil, []netip.Addr{ip6}), wafItemComment).Return(setter.ResponseUpdated),
				)
			},
		},
		"ip6-only/set-fail": {
			false,
			[]string{"Could not confirm update of AAAA (::1) for ip6.hello", "Could not confirm update of WAF list(s) 12341234/list"},
			[]string{"Could not confirm update of AAAA records of ip6.hello with ::1.", `Could not confirm update of WAF list(s) 12341234/list.`},
			providerEnablers{ipnet.IP6: true},
			func(p *mocks.MockPP, pv mockProviders, s *mocks.MockSetter) {
				gomock.InOrder(
					pv[ipnet.IP6].EXPECT().GetRawData(gomock.Any(), p, ipnet.IP6, 64).Return(detectionResult(ipnet.IP6, []netip.Addr{ip6})),
					p.EXPECT().Infof(pp.EmojiInternet, "Detected the %s address %v", "IPv6", ip6),
					p.EXPECT().Suppress(pp.MessageIP6DetectionFails),
					s.EXPECT().SetIPs(gomock.Any(), p, ipnet.IP6, domain.FQDN("ip6.hello"), []netip.Addr{ip6}, params).Return(setter.ResponseFailed),
					s.EXPECT().SetWAFList(gomock.Any(), p, list, wafListDescription, wafTargets(nil, []netip.Addr{ip6}), wafItemComment).Return(setter.ResponseFailed),
				)
			},
		},
		"dual": {
			true, nil, nil,
			providerEnablers{ipnet.IP4: true, ipnet.IP6: true},
			func(p *mocks.MockPP, pv mockProviders, s *mocks.MockSetter) {
				gomock.InOrder(
					pv[ipnet.IP4].EXPECT().GetRawData(gomock.Any(), p, ipnet.IP4, 32).Return(detectionResult(ipnet.IP4, []netip.Addr{ip4})),
					p.EXPECT().Infof(pp.EmojiInternet, "Detected the %s address %v", "IPv4", ip4),
					p.EXPECT().Suppress(pp.MessageIP4DetectionFails),
					s.EXPECT().SetIPs(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello"), []netip.Addr{ip4}, params).Return(setter.ResponseNoop),
					pv[ipnet.IP6].EXPECT().GetRawData(gomock.Any(), p, ipnet.IP6, 64).Return(detectionResult(ipnet.IP6, []netip.Addr{ip6})),
					p.EXPECT().Infof(pp.EmojiInternet, "Detected the %s address %v", "IPv6", ip6),
					p.EXPECT().Suppress(pp.MessageIP6DetectionFails),
					s.EXPECT().SetIPs(gomock.Any(), p, ipnet.IP6, domain.FQDN("ip6.hello"), []netip.Addr{ip6}, params).Return(setter.ResponseNoop),
					s.EXPECT().SetWAFList(gomock.Any(), p, list, wafListDescription, wafTargets([]netip.Addr{ip4}, []netip.Addr{ip6}), wafItemComment).Return(setter.ResponseNoop),
				)
			},
		},
		"dual/set-fail/1": {
			false,
			[]string{"Could not confirm update of A (127.0.0.1) for ip4.hello"},
			[]string{"Could not confirm update of A records of ip4.hello with 127.0.0.1."},
			providerEnablers{ipnet.IP4: true, ipnet.IP6: true},
			func(p *mocks.MockPP, pv mockProviders, s *mocks.MockSetter) {
				gomock.InOrder(
					pv[ipnet.IP4].EXPECT().GetRawData(gomock.Any(), p, ipnet.IP4, 32).Return(detectionResult(ipnet.IP4, []netip.Addr{ip4})),
					p.EXPECT().Infof(pp.EmojiInternet, "Detected the %s address %v", "IPv4", ip4),
					p.EXPECT().Suppress(pp.MessageIP4DetectionFails),
					s.EXPECT().SetIPs(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello"), []netip.Addr{ip4}, params).Return(setter.ResponseFailed),
					pv[ipnet.IP6].EXPECT().GetRawData(gomock.Any(), p, ipnet.IP6, 64).Return(detectionResult(ipnet.IP6, []netip.Addr{ip6})),
					p.EXPECT().Infof(pp.EmojiInternet, "Detected the %s address %v", "IPv6", ip6),
					p.EXPECT().Suppress(pp.MessageIP6DetectionFails),
					s.EXPECT().SetIPs(gomock.Any(), p, ipnet.IP6, domain.FQDN("ip6.hello"), []netip.Addr{ip6}, params).Return(setter.ResponseNoop),
					s.EXPECT().SetWAFList(gomock.Any(), p, list, wafListDescription, wafTargets([]netip.Addr{ip4}, []netip.Addr{ip6}), wafItemComment).Return(setter.ResponseNoop),
				)
			},
		},
		"dual/set-fail/2": {
			false,
			[]string{"Could not confirm update of AAAA (::1) for ip6.hello"},
			[]string{"Could not confirm update of AAAA records of ip6.hello with ::1."},
			providerEnablers{ipnet.IP4: true, ipnet.IP6: true},
			func(p *mocks.MockPP, pv mockProviders, s *mocks.MockSetter) {
				gomock.InOrder(
					pv[ipnet.IP4].EXPECT().GetRawData(gomock.Any(), p, ipnet.IP4, 32).Return(detectionResult(ipnet.IP4, []netip.Addr{ip4})),
					p.EXPECT().Infof(pp.EmojiInternet, "Detected the %s address %v", "IPv4", ip4),
					p.EXPECT().Suppress(pp.MessageIP4DetectionFails),
					s.EXPECT().SetIPs(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello"), []netip.Addr{ip4}, params).Return(setter.ResponseNoop),
					pv[ipnet.IP6].EXPECT().GetRawData(gomock.Any(), p, ipnet.IP6, 64).Return(detectionResult(ipnet.IP6, []netip.Addr{ip6})),
					p.EXPECT().Infof(pp.EmojiInternet, "Detected the %s address %v", "IPv6", ip6),
					p.EXPECT().Suppress(pp.MessageIP6DetectionFails),
					s.EXPECT().SetIPs(gomock.Any(), p, ipnet.IP6, domain.FQDN("ip6.hello"), []netip.Addr{ip6}, params).Return(setter.ResponseFailed),
					s.EXPECT().SetWAFList(gomock.Any(), p, list, wafListDescription, wafTargets([]netip.Addr{ip4}, []netip.Addr{ip6}), wafItemComment).Return(setter.ResponseNoop),
				)
			},
		},
		"dual/set-fail/3": {
			false,
			[]string{`Could not confirm update of WAF list(s) 12341234/list`},
			[]string{`Could not confirm update of WAF list(s) 12341234/list.`},
			providerEnablers{ipnet.IP4: true, ipnet.IP6: true},
			func(p *mocks.MockPP, pv mockProviders, s *mocks.MockSetter) {
				gomock.InOrder(
					pv[ipnet.IP4].EXPECT().GetRawData(gomock.Any(), p, ipnet.IP4, 32).Return(detectionResult(ipnet.IP4, []netip.Addr{ip4})),
					p.EXPECT().Infof(pp.EmojiInternet, "Detected the %s address %v", "IPv4", ip4),
					p.EXPECT().Suppress(pp.MessageIP4DetectionFails),
					s.EXPECT().SetIPs(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello"), []netip.Addr{ip4}, params).Return(setter.ResponseNoop),
					pv[ipnet.IP6].EXPECT().GetRawData(gomock.Any(), p, ipnet.IP6, 64).Return(detectionResult(ipnet.IP6, []netip.Addr{ip6})),
					p.EXPECT().Infof(pp.EmojiInternet, "Detected the %s address %v", "IPv6", ip6),
					p.EXPECT().Suppress(pp.MessageIP6DetectionFails),
					s.EXPECT().SetIPs(gomock.Any(), p, ipnet.IP6, domain.FQDN("ip6.hello"), []netip.Addr{ip6}, params).Return(setter.ResponseNoop),
					s.EXPECT().SetWAFList(gomock.Any(), p, list, wafListDescription, wafTargets([]netip.Addr{ip4}, []netip.Addr{ip6}), wafItemComment).Return(setter.ResponseFailed),
				)
			},
		},
		"ip4-detect-fail": {
			false,
			[]string{"Failed to detect any IPv4 addresses"},
			[]string{"Failed to detect any IPv4 addresses."},
			providerEnablers{ipnet.IP4: true, ipnet.IP6: true},
			func(p *mocks.MockPP, pv mockProviders, s *mocks.MockSetter) {
				gomock.InOrder(
					pv[ipnet.IP4].EXPECT().GetRawData(gomock.Any(), p, ipnet.IP4, 32).Return(provider.NewUnavailableDetectionResult()),
					p.EXPECT().Noticef(pp.EmojiError, "Failed to detect valid %s addresses; will try again", "IPv4"),
					p.EXPECT().NoticeOncef(pp.MessageIP4DetectionFails, pp.EmojiHint, "If your network does not support IPv4, you can stop managing it with IP4_PROVIDER=none"),
					pv[ipnet.IP6].EXPECT().GetRawData(gomock.Any(), p, ipnet.IP6, 64).Return(detectionResult(ipnet.IP6, []netip.Addr{ip6})),
					p.EXPECT().Infof(pp.EmojiInternet, "Detected the %s address %v", "IPv6", ip6),
					p.EXPECT().Suppress(pp.MessageIP6DetectionFails),
					s.EXPECT().SetIPs(gomock.Any(), p, ipnet.IP6, domain.FQDN("ip6.hello"), []netip.Addr{ip6}, params).Return(setter.ResponseNoop),
					s.EXPECT().SetWAFList(gomock.Any(), p, list, wafListDescription, withUnavailableTargets(wafTargets(nil, []netip.Addr{ip6}), ipnet.IP4), wafItemComment),
				)
			},
		},
		"ip6-detect-fail": {
			false,
			[]string{"Failed to detect any IPv6 addresses"},
			[]string{"Failed to detect any IPv6 addresses."},
			providerEnablers{ipnet.IP4: true, ipnet.IP6: true},
			func(p *mocks.MockPP, pv mockProviders, s *mocks.MockSetter) {
				gomock.InOrder(
					pv[ipnet.IP4].EXPECT().GetRawData(gomock.Any(), p, ipnet.IP4, 32).Return(detectionResult(ipnet.IP4, []netip.Addr{ip4})),
					p.EXPECT().Infof(pp.EmojiInternet, "Detected the %s address %v", "IPv4", ip4),
					p.EXPECT().Suppress(pp.MessageIP4DetectionFails),
					s.EXPECT().SetIPs(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello"), []netip.Addr{ip4}, params).Return(setter.ResponseNoop),
					pv[ipnet.IP6].EXPECT().GetRawData(gomock.Any(), p, ipnet.IP6, 64).Return(provider.NewUnavailableDetectionResult()),
					p.EXPECT().Noticef(pp.EmojiError, "Failed to detect valid %s addresses; will try again", "IPv6"),
					hintIP6DetectionFails(p),
					s.EXPECT().SetWAFList(gomock.Any(), p, list, wafListDescription, withUnavailableTargets(wafTargets([]netip.Addr{ip4}, nil), ipnet.IP6), wafItemComment),
				)
			},
		},
		"dual/detect-fail": {
			false,
			[]string{"Failed to detect any IPv4 addresses", "Failed to detect any IPv6 addresses"},
			[]string{"Failed to detect any IPv4 addresses.", "Failed to detect any IPv6 addresses."},
			providerEnablers{ipnet.IP4: true, ipnet.IP6: true},
			func(p *mocks.MockPP, pv mockProviders, _ *mocks.MockSetter) {
				gomock.InOrder(
					pv[ipnet.IP4].EXPECT().GetRawData(gomock.Any(), p, ipnet.IP4, 32).Return(provider.NewUnavailableDetectionResult()),
					p.EXPECT().Noticef(pp.EmojiError, "Failed to detect valid %s addresses; will try again", "IPv4"),
					p.EXPECT().NoticeOncef(pp.MessageIP4DetectionFails, pp.EmojiHint, "If your network does not support IPv4, you can stop managing it with IP4_PROVIDER=none"),
					pv[ipnet.IP6].EXPECT().GetRawData(gomock.Any(), p, ipnet.IP6, 64).Return(provider.NewUnavailableDetectionResult()),
					p.EXPECT().Noticef(pp.EmojiError, "Failed to detect valid %s addresses; will try again", "IPv6"),
					hintIP6DetectionFails(p),
				)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			resp := runUpdateIPsScenario(t, domains, lists, tc.providerEnablers, tc.prepareMocks)
			require.Equal(t, updater.Message{
				HeartbeatMessage: heartbeat.Message{
					OK:    tc.ok,
					Lines: tc.monitorMessages,
				},
				NotifierMessage: notifier.Message(tc.notifierMessages),
			}, resp)
		})
	}
}

func TestUpdateIPsTimeouts(t *testing.T) {
	t.Parallel()

	params := api.RecordParams{
		TTL:     api.TTLAuto,
		Proxied: false,
		Comment: recordComment,
		Tags:    nil,
	}

	domains := map[ipnet.Family][]domain.Domain{
		ipnet.IP4: {domain4},
		ipnet.IP6: {domain6},
	}
	list := api.WAFList{AccountID: "12341234", Name: "list"}
	lists := []api.WAFList{list}

	ip4 := netip.MustParseAddr("127.0.0.1")

	for name, tc := range map[string]struct {
		ok               bool
		monitorMessages  []string
		notifierMessages []string
		providerEnablers providerEnablers
		prepareMocks     func(*mocks.MockPP, mockProviders, *mocks.MockSetter)
	}{
		"detect-timeout": {
			false,
			[]string{"Failed to detect any IPv4 addresses"},
			[]string{"Failed to detect any IPv4 addresses."},
			providerEnablers{ipnet.IP4: true},
			func(p *mocks.MockPP, pv mockProviders, _ *mocks.MockSetter) {
				gomock.InOrder(
					pv[ipnet.IP4].EXPECT().GetRawData(gomock.Any(), p, ipnet.IP4, 32).DoAndReturn(
						func(context.Context, pp.PP, ipnet.Family, int) provider.DetectionResult {
							time.Sleep(2 * time.Second)
							return provider.NewUnavailableDetectionResult()
						},
					),
					p.EXPECT().Noticef(pp.EmojiError, "Failed to detect valid %s addresses; will try again", "IPv4"),
					p.EXPECT().NoticeOncef(pp.MessageIP4DetectionFails, pp.EmojiHint, "If your network does not support IPv4, you can stop managing it with IP4_PROVIDER=none"),
					p.EXPECT().NoticeOncef(pp.MessageDetectionTimeouts, pp.EmojiHint, "If your network is experiencing high latency, consider increasing DETECTION_TIMEOUT=%v", time.Second),
				)
			},
		},
		"set-timeout": {
			false,
			[]string{"Could not confirm update of A (127.0.0.1) for ip4.hello"},
			[]string{"Could not confirm update of A records of ip4.hello with 127.0.0.1."},
			providerEnablers{ipnet.IP4: true},
			func(p *mocks.MockPP, pv mockProviders, s *mocks.MockSetter) {
				gomock.InOrder(
					pv[ipnet.IP4].EXPECT().GetRawData(gomock.Any(), p, ipnet.IP4, 32).Return(detectionResult(ipnet.IP4, []netip.Addr{ip4})),
					p.EXPECT().Infof(pp.EmojiInternet, "Detected the %s address %v", "IPv4", ip4),
					p.EXPECT().Suppress(pp.MessageIP4DetectionFails),
					s.EXPECT().SetIPs(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello"), []netip.Addr{ip4}, params).DoAndReturn(
						func(context.Context, pp.PP, ipnet.Family, domain.Domain, []netip.Addr, api.RecordParams) setter.ResponseCode {
							time.Sleep(2 * time.Second)
							return setter.ResponseFailed
						}),
					p.EXPECT().NoticeOncef(pp.MessageUpdateTimeouts, pp.EmojiHint, "If your network is experiencing high latency, consider increasing UPDATE_TIMEOUT=%v", time.Second),
					s.EXPECT().SetWAFList(gomock.Any(), p, list, wafListDescription, wafTargets([]netip.Addr{ip4}, nil), wafItemComment).Return(setter.ResponseNoop),
				)
			},
		},
		"set-list-timeout": {
			false,
			[]string{"Could not confirm update of WAF list(s) 12341234/list"},
			[]string{`Could not confirm update of WAF list(s) 12341234/list.`},
			providerEnablers{ipnet.IP4: true},
			func(p *mocks.MockPP, pv mockProviders, s *mocks.MockSetter) {
				gomock.InOrder(
					pv[ipnet.IP4].EXPECT().GetRawData(gomock.Any(), p, ipnet.IP4, 32).Return(detectionResult(ipnet.IP4, []netip.Addr{ip4})),
					p.EXPECT().Infof(pp.EmojiInternet, "Detected the %s address %v", "IPv4", ip4),
					p.EXPECT().Suppress(pp.MessageIP4DetectionFails),
					s.EXPECT().SetIPs(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello"), []netip.Addr{ip4}, params).Return(setter.ResponseNoop),
					s.EXPECT().SetWAFList(gomock.Any(), p, list, wafListDescription, wafTargets([]netip.Addr{ip4}, nil), wafItemComment).DoAndReturn(
						func(context.Context, pp.PP, api.WAFList, string, familyTargets, string) setter.ResponseCode {
							time.Sleep(2 * time.Second)
							return setter.ResponseFailed
						}),
					p.EXPECT().NoticeOncef(pp.MessageUpdateTimeouts, pp.EmojiHint, "If your network is experiencing high latency, consider increasing UPDATE_TIMEOUT=%v", time.Second),
				)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			synctest.Test(t, func(t *testing.T) {
				resp := runUpdateIPsScenario(t, domains, lists, tc.providerEnablers, tc.prepareMocks)
				require.Equal(t, updater.Message{
					HeartbeatMessage: heartbeat.Message{
						OK:    tc.ok,
						Lines: tc.monitorMessages,
					},
					NotifierMessage: notifier.Message(tc.notifierMessages),
				}, resp)
			})
		})
	}
}

func TestFinalDeleteIPs(t *testing.T) {
	t.Parallel()

	params := api.RecordParams{
		TTL:     api.TTLAuto,
		Proxied: false,
		Comment: recordComment,
		Tags:    nil,
	}

	domains := map[ipnet.Family][]domain.Domain{
		ipnet.IP4: {domain4},
		ipnet.IP6: {domain6},
	}
	list := api.WAFList{AccountID: "12341234", Name: "list"}
	lists := []api.WAFList{list}

	for name, tc := range map[string]struct {
		ok               bool
		monitorMessages  []string
		notifierMessages []string
		providerEnablers providerEnablers
		prepareMocks     func(*mocks.MockPP, *mocks.MockSetter)
	}{
		"ip4-only": {
			true,
			[]string{"Deleted A of ip4.hello", "Cleaned WAF list(s) 12341234/list"},
			[]string{"Deleted A records of ip4.hello.", `Cleaned WAF list(s) 12341234/list.`},
			providerEnablers{ipnet.IP4: true},
			func(ppfmt *mocks.MockPP, s *mocks.MockSetter) {
				gomock.InOrder(
					s.EXPECT().FinalDelete(gomock.Any(), ppfmt, ipnet.IP4, domain4, params).Return(setter.ResponseUpdated),
					s.EXPECT().FinalClearWAFList(gomock.Any(), ppfmt, list, wafListDescription, gomock.Any()).Return(setter.ResponseUpdated),
				)
			},
		},
		"ip4-only/fail": {
			false,
			[]string{"Could not confirm deletion of A of ip4.hello", "Could not confirm cleanup of WAF list(s) 12341234/list"},
			[]string{
				"Could not confirm deletion of A records of ip4.hello.",
				`Could not confirm cleanup of WAF list(s) 12341234/list.`,
			},
			providerEnablers{ipnet.IP4: true},
			func(ppfmt *mocks.MockPP, s *mocks.MockSetter) {
				gomock.InOrder(
					s.EXPECT().FinalDelete(gomock.Any(), ppfmt, ipnet.IP4, domain4, params).Return(setter.ResponseFailed),
					s.EXPECT().FinalClearWAFList(gomock.Any(), ppfmt, list, wafListDescription, gomock.Any()).Return(setter.ResponseFailed),
				)
			},
		},
		"ip4-only/deleting": {
			true,
			[]string{"Deleting A of ip4.hello", "Cleaning WAF list(s) 12341234/list"},
			[]string{
				"Deleting A records of ip4.hello.",
				`Cleaning WAF list(s) 12341234/list.`,
			},
			providerEnablers{ipnet.IP4: true},
			func(ppfmt *mocks.MockPP, s *mocks.MockSetter) {
				gomock.InOrder(
					s.EXPECT().FinalDelete(gomock.Any(), ppfmt, ipnet.IP4, domain4, params).Return(setter.ResponseUpdating),
					s.EXPECT().FinalClearWAFList(gomock.Any(), ppfmt, list, wafListDescription, gomock.Any()).Return(setter.ResponseUpdating),
				)
			},
		},
		"ip6-only": {
			true,
			[]string{"Deleted AAAA of ip6.hello", "Cleaned WAF list(s) 12341234/list"},
			[]string{
				"Deleted AAAA records of ip6.hello.",
				`Cleaned WAF list(s) 12341234/list.`,
			},
			providerEnablers{ipnet.IP6: true},
			func(ppfmt *mocks.MockPP, s *mocks.MockSetter) {
				gomock.InOrder(
					s.EXPECT().FinalDelete(gomock.Any(), ppfmt, ipnet.IP6, domain6, params).Return(setter.ResponseUpdated),
					s.EXPECT().FinalClearWAFList(gomock.Any(), ppfmt, list, wafListDescription, gomock.Any()).Return(setter.ResponseUpdated),
				)
			},
		},
		"ip6-only/fail": {
			false,
			[]string{"Could not confirm deletion of AAAA of ip6.hello", "Could not confirm cleanup of WAF list(s) 12341234/list"},
			[]string{
				"Could not confirm deletion of AAAA records of ip6.hello.",
				`Could not confirm cleanup of WAF list(s) 12341234/list.`,
			},
			providerEnablers{ipnet.IP6: true},
			func(ppfmt *mocks.MockPP, s *mocks.MockSetter) {
				gomock.InOrder(
					s.EXPECT().FinalDelete(gomock.Any(), ppfmt, ipnet.IP6, domain6, params).Return(setter.ResponseFailed),
					s.EXPECT().FinalClearWAFList(gomock.Any(), ppfmt, list, wafListDescription, gomock.Any()).Return(setter.ResponseFailed),
				)
			},
		},
		"dual": {
			true,
			[]string{"Deleted A of ip4.hello", "Deleted AAAA of ip6.hello", "Cleaned WAF list(s) 12341234/list"},
			[]string{
				"Deleted A records of ip4.hello.", "Deleted AAAA records of ip6.hello.",
				`Cleaned WAF list(s) 12341234/list.`,
			},
			providerEnablers{ipnet.IP4: true, ipnet.IP6: true},
			func(ppfmt *mocks.MockPP, s *mocks.MockSetter) {
				gomock.InOrder(
					s.EXPECT().FinalDelete(gomock.Any(), ppfmt, ipnet.IP4, domain4, params).Return(setter.ResponseUpdated),
					s.EXPECT().FinalDelete(gomock.Any(), ppfmt, ipnet.IP6, domain6, params).Return(setter.ResponseUpdated),
					s.EXPECT().FinalClearWAFList(gomock.Any(), ppfmt, list, wafListDescription, gomock.Any()).Return(setter.ResponseUpdated),
				)
			},
		},
		"dual/fail/1": {
			false,
			[]string{"Could not confirm deletion of A of ip4.hello"},
			[]string{"Could not confirm deletion of A records of ip4.hello."},
			providerEnablers{ipnet.IP4: true, ipnet.IP6: true},
			func(ppfmt *mocks.MockPP, s *mocks.MockSetter) {
				gomock.InOrder(
					s.EXPECT().FinalDelete(gomock.Any(), ppfmt, ipnet.IP4, domain4, params).Return(setter.ResponseFailed),
					s.EXPECT().FinalDelete(gomock.Any(), ppfmt, ipnet.IP6, domain6, params).Return(setter.ResponseNoop),
					s.EXPECT().FinalClearWAFList(gomock.Any(), ppfmt, list, wafListDescription, gomock.Any()).Return(setter.ResponseNoop),
				)
			},
		},
		"dual/fail/2": {
			false,
			[]string{"Could not confirm deletion of AAAA of ip6.hello"},
			[]string{"Could not confirm deletion of AAAA records of ip6.hello."},
			providerEnablers{ipnet.IP4: true, ipnet.IP6: true},
			func(ppfmt *mocks.MockPP, s *mocks.MockSetter) {
				gomock.InOrder(
					s.EXPECT().FinalDelete(gomock.Any(), ppfmt, ipnet.IP4, domain4, params).Return(setter.ResponseNoop),
					s.EXPECT().FinalDelete(gomock.Any(), ppfmt, ipnet.IP6, domain6, params).Return(setter.ResponseFailed),
					s.EXPECT().FinalClearWAFList(gomock.Any(), ppfmt, list, wafListDescription, gomock.Any()).Return(setter.ResponseNoop),
				)
			},
		},
		"dual/fail/3": {
			false,
			[]string{"Could not confirm cleanup of WAF list(s) 12341234/list"},
			[]string{`Could not confirm cleanup of WAF list(s) 12341234/list.`},
			providerEnablers{ipnet.IP4: true, ipnet.IP6: true},
			func(ppfmt *mocks.MockPP, s *mocks.MockSetter) {
				gomock.InOrder(
					s.EXPECT().FinalDelete(gomock.Any(), ppfmt, ipnet.IP4, domain4, params).Return(setter.ResponseNoop),
					s.EXPECT().FinalDelete(gomock.Any(), ppfmt, ipnet.IP6, domain6, params).Return(setter.ResponseNoop),
					s.EXPECT().FinalClearWAFList(gomock.Any(), ppfmt, list, wafListDescription, gomock.Any()).Return(setter.ResponseFailed),
				)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			resp := runFinalDeleteIPsScenario(t, domains, lists, tc.providerEnablers, tc.prepareMocks)
			require.Equal(t, updater.Message{
				HeartbeatMessage: heartbeat.Message{
					OK:    tc.ok,
					Lines: tc.monitorMessages,
				},
				NotifierMessage: notifier.Message(tc.notifierMessages),
			}, resp)
		})
	}
}

func TestFinalDeleteIPsTimeouts(t *testing.T) {
	t.Parallel()

	params := api.RecordParams{
		TTL:     api.TTLAuto,
		Proxied: false,
		Comment: recordComment,
		Tags:    nil,
	}

	domains := map[ipnet.Family][]domain.Domain{
		ipnet.IP4: {domain4},
		ipnet.IP6: {domain6},
	}
	list := api.WAFList{AccountID: "12341234", Name: "list"}
	lists := []api.WAFList{list}

	for name, tc := range map[string]struct {
		ok               bool
		monitorMessages  []string
		notifierMessages []string
		providerEnablers providerEnablers
		prepareMocks     func(*mocks.MockPP, *mocks.MockSetter)
	}{
		"delete-timeout": {
			false,
			[]string{"Could not confirm deletion of A of ip4.hello"},
			[]string{"Could not confirm deletion of A records of ip4.hello."},
			providerEnablers{ipnet.IP4: true},
			func(ppfmt *mocks.MockPP, s *mocks.MockSetter) {
				gomock.InOrder(
					s.EXPECT().FinalDelete(gomock.Any(), ppfmt, ipnet.IP4, domain.FQDN("ip4.hello"), params).DoAndReturn(
						func(context.Context, pp.PP, ipnet.Family, domain.Domain, api.RecordParams) setter.ResponseCode {
							time.Sleep(2 * time.Second)
							return setter.ResponseFailed
						}),
					ppfmt.EXPECT().NoticeOncef(pp.MessageUpdateTimeouts, pp.EmojiHint, "If your network is experiencing high latency, consider increasing UPDATE_TIMEOUT=%v", time.Second),
					s.EXPECT().FinalClearWAFList(gomock.Any(), ppfmt, list, wafListDescription, gomock.Any()).Return(setter.ResponseNoop),
				)
			},
		},
		"delete-list-timeout": {
			false,
			[]string{"Could not confirm cleanup of WAF list(s) 12341234/list"},
			[]string{`Could not confirm cleanup of WAF list(s) 12341234/list.`},
			providerEnablers{ipnet.IP4: true},
			func(ppfmt *mocks.MockPP, s *mocks.MockSetter) {
				gomock.InOrder(
					s.EXPECT().FinalDelete(gomock.Any(), ppfmt, ipnet.IP4, domain.FQDN("ip4.hello"), params).Return(setter.ResponseNoop),
					s.EXPECT().FinalClearWAFList(gomock.Any(), ppfmt, list, wafListDescription, gomock.Any()).DoAndReturn(
						func(context.Context, pp.PP, api.WAFList, string, cleanupFamilies) setter.ResponseCode {
							time.Sleep(2 * time.Second)
							return setter.ResponseFailed
						}),
					ppfmt.EXPECT().NoticeOncef(pp.MessageUpdateTimeouts, pp.EmojiHint, "If your network is experiencing high latency, consider increasing UPDATE_TIMEOUT=%v", time.Second),
				)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			synctest.Test(t, func(t *testing.T) {
				resp := runFinalDeleteIPsScenario(t, domains, lists, tc.providerEnablers, tc.prepareMocks)
				require.Equal(t, updater.Message{
					HeartbeatMessage: heartbeat.Message{
						OK:    tc.ok,
						Lines: tc.monitorMessages,
					},
					NotifierMessage: notifier.Message(tc.notifierMessages),
				}, resp)
			})
		})
	}
}
