// vim: nowrap
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
	"github.com/favonia/cloudflare-ddns/internal/monitor"
	"github.com/favonia/cloudflare-ddns/internal/notifier"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/setter"
	"github.com/favonia/cloudflare-ddns/internal/updater"
)

const (
	recordComment      string = "hello record"
	wafListDescription string = "hello list"
)

type (
	providerEnablers = map[ipnet.Type]bool
	mockProviders    = map[ipnet.Type]*mocks.MockProvider
	detectedIPs      = map[ipnet.Type]netip.Addr
)

const (
	domain4   = domain.FQDN("ip4.hello")
	domain4_1 = domain.FQDN("ip4.hello1")
	domain4_2 = domain.FQDN("ip4.hello2")
	domain4_3 = domain.FQDN("ip4.hello3")
	domain4_4 = domain.FQDN("ip4.hello4")
	domain6   = domain.FQDN("ip6.hello")
)

func initConfig() *config.Config {
	conf := config.Default()
	conf.Provider[ipnet.IP4] = nil
	conf.Provider[ipnet.IP6] = nil
	conf.Proxied = map[domain.Domain]bool{
		domain4:   false,
		domain4_1: false,
		domain4_2: false,
		domain4_3: false,
		domain4_4: false,
		domain6:   false,
	}
	conf.RecordComment = recordComment
	conf.WAFListDescription = wafListDescription
	conf.DetectionTimeout = time.Second
	conf.UpdateTimeout = time.Second
	return conf
}

func hintIP6DetectionFails(p *mocks.MockPP) *mocks.PPNoticeOncefCall {
	return p.EXPECT().NoticeOncef(pp.MessageIP6DetectionFails, pp.EmojiHint, "If you are using Docker or Kubernetes, IPv6 might need extra setup. Read more at %s. If your network doesn't support IPv6, you can turn it off by setting IP6_PROVIDER=none", pp.ManualURL)
}

func TestUpdateIPsMultiple(t *testing.T) {
	t.Parallel()

	params := api.RecordParams{
		TTL:     api.TTLAuto,
		Proxied: false,
		Comment: recordComment,
	}

	domains := map[ipnet.Type][]domain.Domain{
		ipnet.IP4: {domain4_1, domain4_2, domain4_3, domain4_4},
	}

	list1 := api.WAFList{AccountID: "12341234", Name: "list1"}
	list2 := api.WAFList{AccountID: "xxxxxxxx", Name: "list2"}
	list3 := api.WAFList{AccountID: "AAAAAAAA", Name: "list3"}
	list4 := api.WAFList{AccountID: "zzz", Name: "list4"}
	lists := []api.WAFList{list1, list2, list3, list4}

	ip4 := netip.MustParseAddr("127.0.0.1")
	type detected = map[ipnet.Type]netip.Addr

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
				"Failed to set A (127.0.0.1) of ip4.hello2",
				"Failed to set list(s) xxxxxxxx/list2",
			},
			[]string{
				"Failed to properly update A records of ip4.hello2 with 127.0.0.1; updating those of ip4.hello1; updated those of ip4.hello4.",
				`Failed to properly update WAF list(s) xxxxxxxx/list2; updating 12341234/list1; updated zzz/list4.`,
			},
			providerEnablers{ipnet.IP4: true},
			func(p *mocks.MockPP, pv mockProviders, s *mocks.MockSetter) {
				gomock.InOrder(
					pv[ipnet.IP4].EXPECT().GetIP(gomock.Any(), p, ipnet.IP4).Return(ip4, true),
					p.EXPECT().Infof(pp.EmojiInternet, "Detected the %s address %v", "IPv4", ip4),
					p.EXPECT().Suppress(pp.MessageIP4DetectionFails),
					s.EXPECT().Set(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello1"), ip4, params).Return(setter.ResponseUpdating),
					s.EXPECT().Set(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello2"), ip4, params).Return(setter.ResponseFailed),
					s.EXPECT().Set(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello3"), ip4, params).Return(setter.ResponseNoop),
					s.EXPECT().Set(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello4"), ip4, params).Return(setter.ResponseUpdated),
					s.EXPECT().SetWAFList(gomock.Any(), p, list1, wafListDescription, detected{ipnet.IP4: ip4}, "").Return(setter.ResponseUpdating),
					s.EXPECT().SetWAFList(gomock.Any(), p, list2, wafListDescription, detected{ipnet.IP4: ip4}, "").Return(setter.ResponseFailed),
					s.EXPECT().SetWAFList(gomock.Any(), p, list3, wafListDescription, detected{ipnet.IP4: ip4}, "").Return(setter.ResponseNoop),
					s.EXPECT().SetWAFList(gomock.Any(), p, list4, wafListDescription, detected{ipnet.IP4: ip4}, "").Return(setter.ResponseUpdated),
				)
			},
		},
		"2yes1doing": {
			true,
			[]string{
				"Set A (127.0.0.1) of ip4.hello1, ip4.hello3, ip4.hello4",
				"Setting list(s) 12341234/list1",
				"Set list(s) AAAAAAAA/list3, zzz/list4",
			},
			[]string{
				"Updated A records of ip4.hello1, ip4.hello3, and ip4.hello4 with 127.0.0.1.",
				`Updating WAF list(s) 12341234/list1; updated AAAAAAAA/list3 and zzz/list4.`,
			},
			providerEnablers{ipnet.IP4: true},
			func(p *mocks.MockPP, pv mockProviders, s *mocks.MockSetter) {
				gomock.InOrder(
					pv[ipnet.IP4].EXPECT().GetIP(gomock.Any(), p, ipnet.IP4).Return(ip4, true),
					p.EXPECT().Infof(pp.EmojiInternet, "Detected the %s address %v", "IPv4", ip4),
					p.EXPECT().Suppress(pp.MessageIP4DetectionFails),
					s.EXPECT().Set(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello1"), ip4, params).Return(setter.ResponseUpdated),
					s.EXPECT().Set(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello2"), ip4, params).Return(setter.ResponseNoop),
					s.EXPECT().Set(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello3"), ip4, params).Return(setter.ResponseUpdated),
					s.EXPECT().Set(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello4"), ip4, params).Return(setter.ResponseUpdated),
					s.EXPECT().SetWAFList(gomock.Any(), p, list1, wafListDescription, detected{ipnet.IP4: ip4}, "").Return(setter.ResponseUpdating),
					s.EXPECT().SetWAFList(gomock.Any(), p, list2, wafListDescription, detected{ipnet.IP4: ip4}, "").Return(setter.ResponseNoop),
					s.EXPECT().SetWAFList(gomock.Any(), p, list3, wafListDescription, detected{ipnet.IP4: ip4}, "").Return(setter.ResponseUpdated),
					s.EXPECT().SetWAFList(gomock.Any(), p, list4, wafListDescription, detected{ipnet.IP4: ip4}, "").Return(setter.ResponseUpdated),
				)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			ctx := context.Background()

			conf := initConfig()
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
				MonitorMessage: monitor.Message{
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
	}

	domains := map[ipnet.Type][]domain.Domain{
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
				"Failed to delete A of ip4.hello2",
				"Failed to clear list(s) xxxxxxxx/list2",
			},
			[]string{
				"Failed to properly delete A records of ip4.hello2; deleting those of ip4.hello1; deleted those of ip4.hello4.",
				`Failed to properly clear WAF list(s) xxxxxxxx/list2; clearing 12341234/list1; cleared zzz/list4.`,
			},
			func(p *mocks.MockPP, s *mocks.MockSetter) {
				gomock.InOrder(
					s.EXPECT().FinalDelete(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello1"), params).Return(setter.ResponseUpdating),
					s.EXPECT().FinalDelete(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello2"), params).Return(setter.ResponseFailed),
					s.EXPECT().FinalDelete(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello3"), params).Return(setter.ResponseNoop),
					s.EXPECT().FinalDelete(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello4"), params).Return(setter.ResponseUpdated),
					s.EXPECT().FinalClearWAFList(gomock.Any(), p, list1, wafListDescription).Return(setter.ResponseUpdating),
					s.EXPECT().FinalClearWAFList(gomock.Any(), p, list2, wafListDescription).Return(setter.ResponseFailed),
					s.EXPECT().FinalClearWAFList(gomock.Any(), p, list3, wafListDescription).Return(setter.ResponseNoop),
					s.EXPECT().FinalClearWAFList(gomock.Any(), p, list4, wafListDescription).Return(setter.ResponseUpdated),
				)
			},
		},
		"3yes": {
			true,
			[]string{
				"Deleted A of ip4.hello1, ip4.hello3, ip4.hello4",
				"Cleared list(s) 12341234/list1, AAAAAAAA/list3, zzz/list4",
			},
			[]string{
				"Deleted A records of ip4.hello1, ip4.hello3, and ip4.hello4.",
				`Cleared WAF list(s) 12341234/list1, AAAAAAAA/list3, and zzz/list4.`,
			},
			func(p *mocks.MockPP, s *mocks.MockSetter) {
				gomock.InOrder(
					s.EXPECT().FinalDelete(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello1"), params).Return(setter.ResponseUpdated),
					s.EXPECT().FinalDelete(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello2"), params).Return(setter.ResponseNoop),
					s.EXPECT().FinalDelete(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello3"), params).Return(setter.ResponseUpdated),
					s.EXPECT().FinalDelete(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello4"), params).Return(setter.ResponseUpdated),
					s.EXPECT().FinalClearWAFList(gomock.Any(), p, list1, wafListDescription).Return(setter.ResponseUpdated),
					s.EXPECT().FinalClearWAFList(gomock.Any(), p, list2, wafListDescription).Return(setter.ResponseNoop),
					s.EXPECT().FinalClearWAFList(gomock.Any(), p, list3, wafListDescription).Return(setter.ResponseUpdated),
					s.EXPECT().FinalClearWAFList(gomock.Any(), p, list4, wafListDescription).Return(setter.ResponseUpdated),
				)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			ctx := context.Background()

			conf := initConfig()
			conf.Domains = domains
			conf.WAFLists = lists

			mockPP := mocks.NewMockPP(mockCtrl)
			for _, ipnet := range [...]ipnet.Type{ipnet.IP4, ipnet.IP6} {
				conf.Provider[ipnet] = mocks.NewMockProvider(mockCtrl)
			}
			mockSetter := mocks.NewMockSetter(mockCtrl)
			if tc.prepareMocks != nil {
				tc.prepareMocks(mockPP, mockSetter)
			}
			resp := updater.FinalDeleteIPs(ctx, mockPP, conf, mockSetter)
			require.Equal(t, updater.Message{
				MonitorMessage: monitor.Message{
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
	}

	domains := map[ipnet.Type][]domain.Domain{
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
					pv[ipnet.IP4].EXPECT().GetIP(gomock.Any(), p, ipnet.IP4).Return(ip4, true),
					p.EXPECT().Infof(pp.EmojiInternet, "Detected the %s address %v", "IPv4", ip4),
					p.EXPECT().Suppress(pp.MessageIP4DetectionFails),
					s.EXPECT().Set(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello"), ip4, params).Return(setter.ResponseNoop),
					s.EXPECT().SetWAFList(gomock.Any(), p, list, wafListDescription, detectedIPs{ipnet.IP4: ip4}, "").Return(setter.ResponseNoop),
				)
			},
		},
		"ip4-only/set-fail": {
			false,
			[]string{"Failed to set A (127.0.0.1) of ip4.hello", "Failed to set list(s) 12341234/list"},
			[]string{"Failed to properly update A records of ip4.hello with 127.0.0.1.", `Failed to properly update WAF list(s) 12341234/list.`},
			providerEnablers{ipnet.IP4: true},
			func(p *mocks.MockPP, pv mockProviders, s *mocks.MockSetter) {
				gomock.InOrder(
					pv[ipnet.IP4].EXPECT().GetIP(gomock.Any(), p, ipnet.IP4).Return(ip4, true),
					p.EXPECT().Infof(pp.EmojiInternet, "Detected the %s address %v", "IPv4", ip4),
					p.EXPECT().Suppress(pp.MessageIP4DetectionFails),
					s.EXPECT().Set(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello"), ip4, params).Return(setter.ResponseFailed),
					s.EXPECT().SetWAFList(gomock.Any(), p, list, wafListDescription, detectedIPs{ipnet.IP4: ip4}, "").Return(setter.ResponseFailed),
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
					pv[ipnet.IP4].EXPECT().GetIP(gomock.Any(), p, ipnet.IP4).Return(ip4, true),
					p.EXPECT().Infof(pp.EmojiInternet, "Detected the %s address %v", "IPv4", ip4),
					p.EXPECT().Suppress(pp.MessageIP4DetectionFails),
					s.EXPECT().Set(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello"), ip4, params).Return(setter.ResponseUpdating),
					s.EXPECT().SetWAFList(gomock.Any(), p, list, wafListDescription, detectedIPs{ipnet.IP4: ip4}, "").Return(setter.ResponseUpdating),
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
					pv[ipnet.IP6].EXPECT().GetIP(gomock.Any(), p, ipnet.IP6).Return(ip6, true),
					p.EXPECT().Infof(pp.EmojiInternet, "Detected the %s address %v", "IPv6", ip6),
					p.EXPECT().Suppress(pp.MessageIP6DetectionFails),
					s.EXPECT().Set(gomock.Any(), p, ipnet.IP6, domain.FQDN("ip6.hello"), ip6, params).Return(setter.ResponseUpdated),
					s.EXPECT().SetWAFList(gomock.Any(), p, list, wafListDescription, detectedIPs{ipnet.IP6: ip6}, "").Return(setter.ResponseUpdated),
				)
			},
		},
		"ip6-only/set-fail": {
			false,
			[]string{"Failed to set AAAA (::1) of ip6.hello", "Failed to set list(s) 12341234/list"},
			[]string{"Failed to properly update AAAA records of ip6.hello with ::1.", `Failed to properly update WAF list(s) 12341234/list.`},
			providerEnablers{ipnet.IP6: true},
			func(p *mocks.MockPP, pv mockProviders, s *mocks.MockSetter) {
				gomock.InOrder(
					pv[ipnet.IP6].EXPECT().GetIP(gomock.Any(), p, ipnet.IP6).Return(ip6, true),
					p.EXPECT().Infof(pp.EmojiInternet, "Detected the %s address %v", "IPv6", ip6),
					p.EXPECT().Suppress(pp.MessageIP6DetectionFails),
					s.EXPECT().Set(gomock.Any(), p, ipnet.IP6, domain.FQDN("ip6.hello"), ip6, params).Return(setter.ResponseFailed),
					s.EXPECT().SetWAFList(gomock.Any(), p, list, wafListDescription, detectedIPs{ipnet.IP6: ip6}, "").Return(setter.ResponseFailed),
				)
			},
		},
		"dual": {
			true, nil, nil,
			providerEnablers{ipnet.IP4: true, ipnet.IP6: true},
			func(p *mocks.MockPP, pv mockProviders, s *mocks.MockSetter) {
				gomock.InOrder(
					pv[ipnet.IP4].EXPECT().GetIP(gomock.Any(), p, ipnet.IP4).Return(ip4, true),
					p.EXPECT().Infof(pp.EmojiInternet, "Detected the %s address %v", "IPv4", ip4),
					p.EXPECT().Suppress(pp.MessageIP4DetectionFails),
					s.EXPECT().Set(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello"), ip4, params).Return(setter.ResponseNoop),
					pv[ipnet.IP6].EXPECT().GetIP(gomock.Any(), p, ipnet.IP6).Return(ip6, true),
					p.EXPECT().Infof(pp.EmojiInternet, "Detected the %s address %v", "IPv6", ip6),
					p.EXPECT().Suppress(pp.MessageIP6DetectionFails),
					s.EXPECT().Set(gomock.Any(), p, ipnet.IP6, domain.FQDN("ip6.hello"), ip6, params).Return(setter.ResponseNoop),
					s.EXPECT().SetWAFList(gomock.Any(), p, list, wafListDescription, detectedIPs{ipnet.IP4: ip4, ipnet.IP6: ip6}, "").Return(setter.ResponseNoop),
				)
			},
		},
		"dual/set-fail/1": {
			false,
			[]string{"Failed to set A (127.0.0.1) of ip4.hello"},
			[]string{"Failed to properly update A records of ip4.hello with 127.0.0.1."},
			providerEnablers{ipnet.IP4: true, ipnet.IP6: true},
			func(p *mocks.MockPP, pv mockProviders, s *mocks.MockSetter) {
				gomock.InOrder(
					pv[ipnet.IP4].EXPECT().GetIP(gomock.Any(), p, ipnet.IP4).Return(ip4, true),
					p.EXPECT().Infof(pp.EmojiInternet, "Detected the %s address %v", "IPv4", ip4),
					p.EXPECT().Suppress(pp.MessageIP4DetectionFails),
					s.EXPECT().Set(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello"), ip4, params).Return(setter.ResponseFailed),
					pv[ipnet.IP6].EXPECT().GetIP(gomock.Any(), p, ipnet.IP6).Return(ip6, true),
					p.EXPECT().Infof(pp.EmojiInternet, "Detected the %s address %v", "IPv6", ip6),
					p.EXPECT().Suppress(pp.MessageIP6DetectionFails),
					s.EXPECT().Set(gomock.Any(), p, ipnet.IP6, domain.FQDN("ip6.hello"), ip6, params).Return(setter.ResponseNoop),
					s.EXPECT().SetWAFList(gomock.Any(), p, list, wafListDescription, detectedIPs{ipnet.IP4: ip4, ipnet.IP6: ip6}, "").Return(setter.ResponseNoop),
				)
			},
		},
		"dual/set-fail/2": {
			false,
			[]string{"Failed to set AAAA (::1) of ip6.hello"},
			[]string{"Failed to properly update AAAA records of ip6.hello with ::1."},
			providerEnablers{ipnet.IP4: true, ipnet.IP6: true},
			func(p *mocks.MockPP, pv mockProviders, s *mocks.MockSetter) {
				gomock.InOrder(
					pv[ipnet.IP4].EXPECT().GetIP(gomock.Any(), p, ipnet.IP4).Return(ip4, true),
					p.EXPECT().Infof(pp.EmojiInternet, "Detected the %s address %v", "IPv4", ip4),
					p.EXPECT().Suppress(pp.MessageIP4DetectionFails),
					s.EXPECT().Set(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello"), ip4, params).Return(setter.ResponseNoop),
					pv[ipnet.IP6].EXPECT().GetIP(gomock.Any(), p, ipnet.IP6).Return(ip6, true),
					p.EXPECT().Infof(pp.EmojiInternet, "Detected the %s address %v", "IPv6", ip6),
					p.EXPECT().Suppress(pp.MessageIP6DetectionFails),
					s.EXPECT().Set(gomock.Any(), p, ipnet.IP6, domain.FQDN("ip6.hello"), ip6, params).Return(setter.ResponseFailed),
					s.EXPECT().SetWAFList(gomock.Any(), p, list, wafListDescription, detectedIPs{ipnet.IP4: ip4, ipnet.IP6: ip6}, "").Return(setter.ResponseNoop),
				)
			},
		},
		"dual/set-fail/3": {
			false,
			[]string{`Failed to set list(s) 12341234/list`},
			[]string{`Failed to properly update WAF list(s) 12341234/list.`},
			providerEnablers{ipnet.IP4: true, ipnet.IP6: true},
			func(p *mocks.MockPP, pv mockProviders, s *mocks.MockSetter) {
				gomock.InOrder(
					pv[ipnet.IP4].EXPECT().GetIP(gomock.Any(), p, ipnet.IP4).Return(ip4, true),
					p.EXPECT().Infof(pp.EmojiInternet, "Detected the %s address %v", "IPv4", ip4),
					p.EXPECT().Suppress(pp.MessageIP4DetectionFails),
					s.EXPECT().Set(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello"), ip4, params).Return(setter.ResponseNoop),
					pv[ipnet.IP6].EXPECT().GetIP(gomock.Any(), p, ipnet.IP6).Return(ip6, true),
					p.EXPECT().Infof(pp.EmojiInternet, "Detected the %s address %v", "IPv6", ip6),
					p.EXPECT().Suppress(pp.MessageIP6DetectionFails),
					s.EXPECT().Set(gomock.Any(), p, ipnet.IP6, domain.FQDN("ip6.hello"), ip6, params).Return(setter.ResponseNoop),
					s.EXPECT().SetWAFList(gomock.Any(), p, list, wafListDescription, detectedIPs{ipnet.IP4: ip4, ipnet.IP6: ip6}, "").Return(setter.ResponseFailed),
				)
			},
		},
		"ip4-detect-fail": {
			false,
			[]string{"Failed to detect IPv4 address"},
			[]string{"Failed to detect the IPv4 address."},
			providerEnablers{ipnet.IP4: true, ipnet.IP6: true},
			func(p *mocks.MockPP, pv mockProviders, s *mocks.MockSetter) {
				gomock.InOrder(
					pv[ipnet.IP4].EXPECT().GetIP(gomock.Any(), p, ipnet.IP4).Return(netip.Addr{}, false),
					p.EXPECT().Noticef(pp.EmojiError, "Failed to detect the %s address", "IPv4"),
					p.EXPECT().NoticeOncef(pp.MessageIP4DetectionFails, pp.EmojiHint, "If your network does not support IPv4, you can disable it with IP4_PROVIDER=none"),
					pv[ipnet.IP6].EXPECT().GetIP(gomock.Any(), p, ipnet.IP6).Return(ip6, true),
					p.EXPECT().Infof(pp.EmojiInternet, "Detected the %s address %v", "IPv6", ip6),
					p.EXPECT().Suppress(pp.MessageIP6DetectionFails),
					s.EXPECT().Set(gomock.Any(), p, ipnet.IP6, domain.FQDN("ip6.hello"), ip6, params).Return(setter.ResponseNoop),
					s.EXPECT().SetWAFList(gomock.Any(), p, list, wafListDescription, detectedIPs{ipnet.IP4: netip.Addr{}, ipnet.IP6: ip6}, ""),
				)
			},
		},
		"ip6-detect-fail": {
			false,
			[]string{"Failed to detect IPv6 address"},
			[]string{"Failed to detect the IPv6 address."},
			providerEnablers{ipnet.IP4: true, ipnet.IP6: true},
			func(p *mocks.MockPP, pv mockProviders, s *mocks.MockSetter) {
				gomock.InOrder(
					pv[ipnet.IP4].EXPECT().GetIP(gomock.Any(), p, ipnet.IP4).Return(ip4, true),
					p.EXPECT().Infof(pp.EmojiInternet, "Detected the %s address %v", "IPv4", ip4),
					p.EXPECT().Suppress(pp.MessageIP4DetectionFails),
					s.EXPECT().Set(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello"), ip4, params).Return(setter.ResponseNoop),
					pv[ipnet.IP6].EXPECT().GetIP(gomock.Any(), p, ipnet.IP6).Return(netip.Addr{}, false),
					p.EXPECT().Noticef(pp.EmojiError, "Failed to detect the %s address", "IPv6"),
					hintIP6DetectionFails(p),
					s.EXPECT().SetWAFList(gomock.Any(), p, list, wafListDescription, detectedIPs{ipnet.IP4: ip4, ipnet.IP6: netip.Addr{}}, ""),
				)
			},
		},
		"dual/detect-fail": {
			false,
			[]string{"Failed to detect IPv4 address", "Failed to detect IPv6 address"},
			[]string{"Failed to detect the IPv4 address.", "Failed to detect the IPv6 address."},
			providerEnablers{ipnet.IP4: true, ipnet.IP6: true},
			func(p *mocks.MockPP, pv mockProviders, _ *mocks.MockSetter) {
				gomock.InOrder(
					pv[ipnet.IP4].EXPECT().GetIP(gomock.Any(), p, ipnet.IP4).Return(netip.Addr{}, false),
					p.EXPECT().Noticef(pp.EmojiError, "Failed to detect the %s address", "IPv4"),
					p.EXPECT().NoticeOncef(pp.MessageIP4DetectionFails, pp.EmojiHint, "If your network does not support IPv4, you can disable it with IP4_PROVIDER=none"),
					pv[ipnet.IP6].EXPECT().GetIP(gomock.Any(), p, ipnet.IP6).Return(netip.Addr{}, false),
					p.EXPECT().Noticef(pp.EmojiError, "Failed to detect the %s address", "IPv6"),
					hintIP6DetectionFails(p),
				)
			},
		},
		"detect-timeout": {
			false,
			[]string{"Failed to detect IPv4 address"},
			[]string{"Failed to detect the IPv4 address."},
			providerEnablers{ipnet.IP4: true},
			func(p *mocks.MockPP, pv mockProviders, s *mocks.MockSetter) {
				gomock.InOrder(
					pv[ipnet.IP4].EXPECT().GetIP(gomock.Any(), p, ipnet.IP4).DoAndReturn(
						func(context.Context, pp.PP, ipnet.Type) (netip.Addr, bool) {
							time.Sleep(2 * time.Second)
							return netip.Addr{}, false
						},
					),
					p.EXPECT().Noticef(pp.EmojiError, "Failed to detect the %s address", "IPv4"),
					p.EXPECT().NoticeOncef(pp.MessageIP4DetectionFails, pp.EmojiHint, "If your network does not support IPv4, you can disable it with IP4_PROVIDER=none"),
					p.EXPECT().NoticeOncef(pp.MessageDetectionTimeouts, pp.EmojiHint, "If your network is experiencing high latency, consider increasing DETECTION_TIMEOUT=%v", time.Second),
					s.EXPECT().SetWAFList(gomock.Any(), p, list, wafListDescription, detectedIPs{ipnet.IP4: netip.Addr{}}, "").Return(setter.ResponseNoop),
				)
			},
		},
		"set-timeout": {
			false,
			[]string{"Failed to set A (127.0.0.1) of ip4.hello"},
			[]string{"Failed to properly update A records of ip4.hello with 127.0.0.1."},
			providerEnablers{ipnet.IP4: true},
			func(p *mocks.MockPP, pv mockProviders, s *mocks.MockSetter) {
				gomock.InOrder(
					pv[ipnet.IP4].EXPECT().GetIP(gomock.Any(), p, ipnet.IP4).Return(ip4, true),
					p.EXPECT().Infof(pp.EmojiInternet, "Detected the %s address %v", "IPv4", ip4),
					p.EXPECT().Suppress(pp.MessageIP4DetectionFails),
					s.EXPECT().Set(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello"), ip4, params).DoAndReturn(
						func(context.Context, pp.PP, ipnet.Type, domain.Domain, netip.Addr, api.RecordParams) setter.ResponseCode {
							time.Sleep(2 * time.Second)
							return setter.ResponseFailed
						}),
					p.EXPECT().NoticeOncef(pp.MessageUpdateTimeouts, pp.EmojiHint, "If your network is experiencing high latency, consider increasing UPDATE_TIMEOUT=%v", time.Second),
					s.EXPECT().SetWAFList(gomock.Any(), p, list, wafListDescription, detectedIPs{ipnet.IP4: ip4}, "").Return(setter.ResponseNoop),
				)
			},
		},
		"set-list-timeout": {
			false,
			[]string{"Failed to set list(s) 12341234/list"},
			[]string{`Failed to properly update WAF list(s) 12341234/list.`},
			providerEnablers{ipnet.IP4: true},
			func(p *mocks.MockPP, pv mockProviders, s *mocks.MockSetter) {
				gomock.InOrder(
					pv[ipnet.IP4].EXPECT().GetIP(gomock.Any(), p, ipnet.IP4).Return(ip4, true),
					p.EXPECT().Infof(pp.EmojiInternet, "Detected the %s address %v", "IPv4", ip4),
					p.EXPECT().Suppress(pp.MessageIP4DetectionFails),
					s.EXPECT().Set(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello"), ip4, params).Return(setter.ResponseNoop),
					s.EXPECT().SetWAFList(gomock.Any(), p, list, wafListDescription, detectedIPs{ipnet.IP4: ip4}, "").DoAndReturn(
						func(context.Context, pp.PP, api.WAFList, string, detectedIPs, string) setter.ResponseCode {
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

			mockCtrl := gomock.NewController(t)
			ctx := context.Background()

			conf := initConfig()
			conf.Domains = domains
			conf.Proxied = map[domain.Domain]bool{domain4: false, domain6: false}
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
				MonitorMessage: monitor.Message{
					OK:    tc.ok,
					Lines: tc.monitorMessages,
				},
				NotifierMessage: notifier.Message(tc.notifierMessages),
			}, resp)
		})
	}
}

func TestFinalDeleteIPs(t *testing.T) {
	t.Parallel()

	params := api.RecordParams{
		TTL:     api.TTLAuto,
		Proxied: false,
		Comment: recordComment,
	}

	domains := map[ipnet.Type][]domain.Domain{
		ipnet.IP4: {domain4},
		ipnet.IP6: {domain6},
	}
	list := api.WAFList{AccountID: "12341234", Name: "list"}
	lists := []api.WAFList{list}

	type mockproviders = map[ipnet.Type]bool

	for name, tc := range map[string]struct {
		ok                  bool
		monitorMessages     []string
		notifierMessages    []string
		prepareMockProvider mockproviders
		prepareMocks        func(*mocks.MockPP, *mocks.MockSetter)
	}{
		"ip4-only": {
			true,
			[]string{"Deleted A of ip4.hello", "Cleared list(s) 12341234/list"},
			[]string{"Deleted A records of ip4.hello.", `Cleared WAF list(s) 12341234/list.`},
			mockproviders{ipnet.IP4: true},
			func(ppfmt *mocks.MockPP, s *mocks.MockSetter) {
				gomock.InOrder(
					s.EXPECT().FinalDelete(gomock.Any(), ppfmt, ipnet.IP4, domain4, params).Return(setter.ResponseUpdated),
					s.EXPECT().FinalClearWAFList(gomock.Any(), ppfmt, list, wafListDescription).Return(setter.ResponseUpdated),
				)
			},
		},
		"ip4-only/fail": {
			false,
			[]string{"Failed to delete A of ip4.hello", "Failed to clear list(s) 12341234/list"},
			[]string{
				"Failed to properly delete A records of ip4.hello.",
				`Failed to properly clear WAF list(s) 12341234/list.`,
			},
			mockproviders{ipnet.IP4: true},
			func(ppfmt *mocks.MockPP, s *mocks.MockSetter) {
				gomock.InOrder(
					s.EXPECT().FinalDelete(gomock.Any(), ppfmt, ipnet.IP4, domain4, params).Return(setter.ResponseFailed),
					s.EXPECT().FinalClearWAFList(gomock.Any(), ppfmt, list, wafListDescription).Return(setter.ResponseFailed),
				)
			},
		},
		"ip4-only/deleting": {
			true,
			[]string{"Deleting A of ip4.hello", "Clearing list(s) 12341234/list"},
			[]string{
				"Deleting A records of ip4.hello.",
				`Clearing WAF list(s) 12341234/list.`,
			},
			mockproviders{ipnet.IP4: true},
			func(ppfmt *mocks.MockPP, s *mocks.MockSetter) {
				gomock.InOrder(
					s.EXPECT().FinalDelete(gomock.Any(), ppfmt, ipnet.IP4, domain4, params).Return(setter.ResponseUpdating),
					s.EXPECT().FinalClearWAFList(gomock.Any(), ppfmt, list, wafListDescription).Return(setter.ResponseUpdating),
				)
			},
		},
		"ip6-only": {
			true,
			[]string{"Deleted AAAA of ip6.hello", "Cleared list(s) 12341234/list"},
			[]string{
				"Deleted AAAA records of ip6.hello.",
				`Cleared WAF list(s) 12341234/list.`,
			},
			mockproviders{ipnet.IP6: true},
			func(ppfmt *mocks.MockPP, s *mocks.MockSetter) {
				gomock.InOrder(
					s.EXPECT().FinalDelete(gomock.Any(), ppfmt, ipnet.IP6, domain6, params).Return(setter.ResponseUpdated),
					s.EXPECT().FinalClearWAFList(gomock.Any(), ppfmt, list, wafListDescription).Return(setter.ResponseUpdated),
				)
			},
		},
		"ip6-only/fail": {
			false,
			[]string{"Failed to delete AAAA of ip6.hello", "Failed to clear list(s) 12341234/list"},
			[]string{
				"Failed to properly delete AAAA records of ip6.hello.",
				`Failed to properly clear WAF list(s) 12341234/list.`,
			},
			mockproviders{ipnet.IP6: true},
			func(ppfmt *mocks.MockPP, s *mocks.MockSetter) {
				gomock.InOrder(
					s.EXPECT().FinalDelete(gomock.Any(), ppfmt, ipnet.IP6, domain6, params).Return(setter.ResponseFailed),
					s.EXPECT().FinalClearWAFList(gomock.Any(), ppfmt, list, wafListDescription).Return(setter.ResponseFailed),
				)
			},
		},
		"dual": {
			true,
			[]string{"Deleted A of ip4.hello", "Deleted AAAA of ip6.hello", "Cleared list(s) 12341234/list"},
			[]string{
				"Deleted A records of ip4.hello.", "Deleted AAAA records of ip6.hello.",
				`Cleared WAF list(s) 12341234/list.`,
			},
			mockproviders{ipnet.IP4: true, ipnet.IP6: true},
			func(ppfmt *mocks.MockPP, s *mocks.MockSetter) {
				gomock.InOrder(
					s.EXPECT().FinalDelete(gomock.Any(), ppfmt, ipnet.IP4, domain4, params).Return(setter.ResponseUpdated),
					s.EXPECT().FinalDelete(gomock.Any(), ppfmt, ipnet.IP6, domain6, params).Return(setter.ResponseUpdated),
					s.EXPECT().FinalClearWAFList(gomock.Any(), ppfmt, list, wafListDescription).Return(setter.ResponseUpdated),
				)
			},
		},
		"dual/fail/1": {
			false,
			[]string{"Failed to delete A of ip4.hello"},
			[]string{"Failed to properly delete A records of ip4.hello."},
			mockproviders{ipnet.IP4: true, ipnet.IP6: true},
			func(ppfmt *mocks.MockPP, s *mocks.MockSetter) {
				gomock.InOrder(
					s.EXPECT().FinalDelete(gomock.Any(), ppfmt, ipnet.IP4, domain4, params).Return(setter.ResponseFailed),
					s.EXPECT().FinalDelete(gomock.Any(), ppfmt, ipnet.IP6, domain6, params).Return(setter.ResponseNoop),
					s.EXPECT().FinalClearWAFList(gomock.Any(), ppfmt, list, wafListDescription).Return(setter.ResponseNoop),
				)
			},
		},
		"dual/fail/2": {
			false,
			[]string{"Failed to delete AAAA of ip6.hello"},
			[]string{"Failed to properly delete AAAA records of ip6.hello."},
			mockproviders{ipnet.IP4: true, ipnet.IP6: true},
			func(ppfmt *mocks.MockPP, s *mocks.MockSetter) {
				gomock.InOrder(
					s.EXPECT().FinalDelete(gomock.Any(), ppfmt, ipnet.IP4, domain4, params).Return(setter.ResponseNoop),
					s.EXPECT().FinalDelete(gomock.Any(), ppfmt, ipnet.IP6, domain6, params).Return(setter.ResponseFailed),
					s.EXPECT().FinalClearWAFList(gomock.Any(), ppfmt, list, wafListDescription).Return(setter.ResponseNoop),
				)
			},
		},
		"dual/fail/3": {
			false,
			[]string{"Failed to clear list(s) 12341234/list"},
			[]string{`Failed to properly clear WAF list(s) 12341234/list.`},
			mockproviders{ipnet.IP4: true, ipnet.IP6: true},
			func(ppfmt *mocks.MockPP, s *mocks.MockSetter) {
				gomock.InOrder(
					s.EXPECT().FinalDelete(gomock.Any(), ppfmt, ipnet.IP4, domain4, params).Return(setter.ResponseNoop),
					s.EXPECT().FinalDelete(gomock.Any(), ppfmt, ipnet.IP6, domain6, params).Return(setter.ResponseNoop),
					s.EXPECT().FinalClearWAFList(gomock.Any(), ppfmt, list, wafListDescription).Return(setter.ResponseFailed),
				)
			},
		},
		"delete-timeout": {
			false,
			[]string{"Failed to delete A of ip4.hello"},
			[]string{"Failed to properly delete A records of ip4.hello."},
			mockproviders{ipnet.IP4: true},
			func(ppfmt *mocks.MockPP, s *mocks.MockSetter) {
				gomock.InOrder(
					s.EXPECT().FinalDelete(gomock.Any(), ppfmt, ipnet.IP4, domain.FQDN("ip4.hello"), params).DoAndReturn(
						func(context.Context, pp.PP, ipnet.Type, domain.Domain, api.RecordParams) setter.ResponseCode {
							time.Sleep(2 * time.Second)
							return setter.ResponseFailed
						}),
					ppfmt.EXPECT().NoticeOncef(pp.MessageUpdateTimeouts, pp.EmojiHint, "If your network is experiencing high latency, consider increasing UPDATE_TIMEOUT=%v", time.Second),
					s.EXPECT().FinalClearWAFList(gomock.Any(), ppfmt, list, wafListDescription).Return(setter.ResponseNoop),
				)
			},
		},
		"delete-list-timeout": {
			false,
			[]string{"Failed to clear list(s) 12341234/list"},
			[]string{`Failed to properly clear WAF list(s) 12341234/list.`},
			mockproviders{ipnet.IP4: true},
			func(ppfmt *mocks.MockPP, s *mocks.MockSetter) {
				gomock.InOrder(
					s.EXPECT().FinalDelete(gomock.Any(), ppfmt, ipnet.IP4, domain.FQDN("ip4.hello"), params).Return(setter.ResponseNoop),
					s.EXPECT().FinalClearWAFList(gomock.Any(), ppfmt, list, wafListDescription).DoAndReturn(
						func(context.Context, pp.PP, api.WAFList, string) setter.ResponseCode {
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

			mockCtrl := gomock.NewController(t)
			ctx := context.Background()

			conf := initConfig()
			conf.Domains = domains
			conf.Proxied = map[domain.Domain]bool{domain4: false, domain6: false}
			conf.WAFLists = lists

			mockPP := mocks.NewMockPP(mockCtrl)
			mockSetter := mocks.NewMockSetter(mockCtrl)
			if tc.prepareMocks != nil {
				tc.prepareMocks(mockPP, mockSetter)
			}

			for _, ipnet := range [...]ipnet.Type{ipnet.IP4, ipnet.IP6} {
				if !tc.prepareMockProvider[ipnet] {
					conf.Provider[ipnet] = nil
					continue
				}

				conf.Provider[ipnet] = mocks.NewMockProvider(mockCtrl)
			}

			resp := updater.FinalDeleteIPs(ctx, mockPP, conf, mockSetter)
			require.Equal(t, updater.Message{
				MonitorMessage: monitor.Message{
					OK:    tc.ok,
					Lines: tc.monitorMessages,
				},
				NotifierMessage: notifier.Message(tc.notifierMessages),
			}, resp)
		})
	}
}
