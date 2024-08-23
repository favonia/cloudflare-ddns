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
	"github.com/favonia/cloudflare-ddns/internal/message"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
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
	use1001 := true
	conf.ShouldWeUse1001 = &use1001
	conf.DetectionTimeout = time.Second
	conf.UpdateTimeout = time.Second
	return conf
}

func TestUpdateIPsMultiple(t *testing.T) {
	t.Parallel()

	domains := map[ipnet.Type][]domain.Domain{
		ipnet.IP4: {domain4_1, domain4_2, domain4_3, domain4_4},
	}

	list1 := api.WAFList{AccountID: "12341234", ListName: "list1"}
	list2 := api.WAFList{AccountID: "xxxxxxxx", ListName: "list2"}
	list3 := api.WAFList{AccountID: "AAAAAAAA", ListName: "list3"}
	list4 := api.WAFList{AccountID: "zzz", ListName: "list4"}
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
		"2yes1no": {
			false,
			[]string{"Failed to set A (127.0.0.1): ip4.hello2", "Failed to set list(s): xxxxxxxx/list2"},
			[]string{
				"Failed to properly update A records of ip4.hello2 with 127.0.0.1; " +
					"those of ip4.hello1 and ip4.hello4 were updated.",
				`Failed to properly update WAF list(s) xxxxxxxx/list2; 12341234/list1 and zzz/list4 were updated.`,
			},
			providerEnablers{ipnet.IP4: true},
			func(p *mocks.MockPP, pv mockProviders, s *mocks.MockSetter) { //nolint:dupl
				gomock.InOrder(
					pv[ipnet.IP4].EXPECT().GetIP(gomock.Any(), p, ipnet.IP4, true).Return(ip4, true),
					p.EXPECT().Infof(pp.EmojiInternet, "Detected the %s address: %v", "IPv4", ip4),
					p.EXPECT().SuppressHint(pp.HintIP4DetectionFails),
					s.EXPECT().Set(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello1"), ip4, api.TTLAuto, false, recordComment).
						Return(setter.ResponseUpdated),
					s.EXPECT().Set(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello2"), ip4, api.TTLAuto, false, recordComment).
						Return(setter.ResponseFailed),
					s.EXPECT().Set(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello3"), ip4, api.TTLAuto, false, recordComment).
						Return(setter.ResponseNoop),
					s.EXPECT().Set(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello4"), ip4, api.TTLAuto, false, recordComment).
						Return(setter.ResponseUpdated),
					s.EXPECT().SetWAFList(gomock.Any(), p, list1, wafListDescription, detected{ipnet.IP4: ip4}, "").
						Return(setter.ResponseUpdated),
					s.EXPECT().SetWAFList(gomock.Any(), p, list2, wafListDescription, detected{ipnet.IP4: ip4}, "").
						Return(setter.ResponseFailed),
					s.EXPECT().SetWAFList(gomock.Any(), p, list3, wafListDescription, detected{ipnet.IP4: ip4}, "").
						Return(setter.ResponseNoop),
					s.EXPECT().SetWAFList(gomock.Any(), p, list4, wafListDescription, detected{ipnet.IP4: ip4}, "").
						Return(setter.ResponseUpdated),
				)
			},
		},
		"3yes": {
			true,
			[]string{
				"Set A (127.0.0.1): ip4.hello1, ip4.hello3, ip4.hello4",
				"Set list(s): 12341234/list1, AAAAAAAA/list3, zzz/list4",
			},
			[]string{
				"Updated A records of ip4.hello1, ip4.hello3, and ip4.hello4 with 127.0.0.1.",
				`Updated WAF list(s) 12341234/list1, AAAAAAAA/list3, and zzz/list4.`,
			},
			providerEnablers{ipnet.IP4: true},
			func(p *mocks.MockPP, pv mockProviders, s *mocks.MockSetter) { //nolint:dupl
				gomock.InOrder(
					pv[ipnet.IP4].EXPECT().GetIP(gomock.Any(), p, ipnet.IP4, true).Return(ip4, true),
					p.EXPECT().Infof(pp.EmojiInternet, "Detected the %s address: %v", "IPv4", ip4),
					p.EXPECT().SuppressHint(pp.HintIP4DetectionFails),
					s.EXPECT().Set(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello1"), ip4, api.TTLAuto, false, recordComment).
						Return(setter.ResponseUpdated),
					s.EXPECT().Set(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello2"), ip4, api.TTLAuto, false, recordComment).
						Return(setter.ResponseNoop),
					s.EXPECT().Set(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello3"), ip4, api.TTLAuto, false, recordComment).
						Return(setter.ResponseUpdated),
					s.EXPECT().Set(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello4"), ip4, api.TTLAuto, false, recordComment).
						Return(setter.ResponseUpdated),
					s.EXPECT().SetWAFList(gomock.Any(), p, list1, wafListDescription, detected{ipnet.IP4: ip4}, "").
						Return(setter.ResponseUpdated),
					s.EXPECT().SetWAFList(gomock.Any(), p, list2, wafListDescription, detected{ipnet.IP4: ip4}, "").
						Return(setter.ResponseNoop),
					s.EXPECT().SetWAFList(gomock.Any(), p, list3, wafListDescription, detected{ipnet.IP4: ip4}, "").
						Return(setter.ResponseUpdated),
					s.EXPECT().SetWAFList(gomock.Any(), p, list4, wafListDescription, detected{ipnet.IP4: ip4}, "").
						Return(setter.ResponseUpdated),
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
			require.Equal(t, message.Message{
				OK:               tc.ok,
				NotifierMessages: tc.notifierMessages,
				MonitorMessages:  tc.monitorMessages,
			}, resp)
		})
	}
}

func TestDeleteIPsMultiple(t *testing.T) {
	t.Parallel()

	domains := map[ipnet.Type][]domain.Domain{
		ipnet.IP4: {domain4_1, domain4_2, domain4_3, domain4_4},
	}

	list1 := api.WAFList{AccountID: "12341234", ListName: "list1"}
	list2 := api.WAFList{AccountID: "xxxxxxxx", ListName: "list2"}
	list3 := api.WAFList{AccountID: "AAAAAAAA", ListName: "list3"}
	list4 := api.WAFList{AccountID: "zzz", ListName: "list4"}
	lists := []api.WAFList{list1, list2, list3, list4}

	for name, tc := range map[string]struct {
		ok               bool
		monitorMessages  []string
		notifierMessages []string
		prepareMocks     func(*mocks.MockPP, *mocks.MockSetter)
	}{
		"2yes1no": { //nolint:dupl
			false,
			[]string{"Failed to delete A: ip4.hello2", "Failed to delete list(s): xxxxxxxx/list2"},
			[]string{
				"Failed to properly delete A records of ip4.hello2; those of ip4.hello1 and ip4.hello4 were deleted.",
				`Failed to properly delete WAF list(s) xxxxxxxx/list2; 12341234/list1 and zzz/list4 were deleted.`,
			},
			func(p *mocks.MockPP, s *mocks.MockSetter) {
				gomock.InOrder(
					s.EXPECT().Delete(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello1")).
						Return(setter.ResponseUpdated),
					s.EXPECT().Delete(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello2")).
						Return(setter.ResponseFailed),
					s.EXPECT().Delete(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello3")).
						Return(setter.ResponseNoop),
					s.EXPECT().Delete(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello4")).
						Return(setter.ResponseUpdated),
					s.EXPECT().DeleteWAFList(gomock.Any(), p, list1).Return(setter.ResponseUpdated),
					s.EXPECT().DeleteWAFList(gomock.Any(), p, list2).Return(setter.ResponseFailed),
					s.EXPECT().DeleteWAFList(gomock.Any(), p, list3).Return(setter.ResponseNoop),
					s.EXPECT().DeleteWAFList(gomock.Any(), p, list4).Return(setter.ResponseUpdated),
				)
			},
		},
		"3yes": { //nolint:dupl
			true,
			[]string{
				"Deleted A: ip4.hello1, ip4.hello3, ip4.hello4",
				"Deleted list(s): 12341234/list1, AAAAAAAA/list3, zzz/list4",
			},
			[]string{
				"Deleted A records of ip4.hello1, ip4.hello3, and ip4.hello4.",
				`Deleted WAF list(s) 12341234/list1, AAAAAAAA/list3, and zzz/list4.`,
			},
			func(p *mocks.MockPP, s *mocks.MockSetter) {
				gomock.InOrder(
					s.EXPECT().Delete(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello1")).
						Return(setter.ResponseUpdated),
					s.EXPECT().Delete(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello2")).
						Return(setter.ResponseNoop),
					s.EXPECT().Delete(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello3")).
						Return(setter.ResponseUpdated),
					s.EXPECT().Delete(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello4")).
						Return(setter.ResponseUpdated),
					s.EXPECT().DeleteWAFList(gomock.Any(), p, list1).Return(setter.ResponseUpdated),
					s.EXPECT().DeleteWAFList(gomock.Any(), p, list2).Return(setter.ResponseNoop),
					s.EXPECT().DeleteWAFList(gomock.Any(), p, list3).Return(setter.ResponseUpdated),
					s.EXPECT().DeleteWAFList(gomock.Any(), p, list4).Return(setter.ResponseUpdated),
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
			resp := updater.DeleteIPs(ctx, mockPP, conf, mockSetter)
			require.Equal(t, message.Message{
				OK:               tc.ok,
				NotifierMessages: tc.notifierMessages,
				MonitorMessages:  tc.monitorMessages,
			}, resp)
		})
	}
}

func TestUpdateIPsUninitializedProxied(t *testing.T) {
	t.Parallel()

	domains := map[ipnet.Type][]domain.Domain{
		ipnet.IP4: {domain4},
	}

	ip4 := netip.MustParseAddr("127.0.0.1")

	for name, tc := range map[string]struct {
		ok               bool
		monitorMessages  []string
		notifierMessages []string
		providerEnablers providerEnablers
		prepareMocks     func(*mocks.MockPP, mockProviders, *mocks.MockSetter)
	}{
		"ip4only-proxied-nil": {
			true,
			[]string{"Set A (127.0.0.1): ip4.hello"},
			[]string{"Updated A records of ip4.hello with 127.0.0.1."},
			providerEnablers{ipnet.IP4: true},
			func(p *mocks.MockPP, pv mockProviders, s *mocks.MockSetter) {
				gomock.InOrder(
					pv[ipnet.IP4].EXPECT().GetIP(gomock.Any(), p, ipnet.IP4, true).Return(ip4, true),
					p.EXPECT().Infof(pp.EmojiInternet, "Detected the %s address: %v", "IPv4", ip4),
					p.EXPECT().SuppressHint(pp.HintIP4DetectionFails),
					p.EXPECT().Noticef(pp.EmojiImpossible,
						"Proxied[%s] not initialized; this should not happen; please report this at %s",
						"ip4.hello", pp.IssueReportingURL,
					),
					s.EXPECT().Set(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello"), ip4, api.TTLAuto, false, recordComment).
						Return(setter.ResponseUpdated),
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
			conf.Proxied = map[domain.Domain]bool{}

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
			require.Equal(t, message.Message{
				OK:               tc.ok,
				NotifierMessages: tc.notifierMessages,
				MonitorMessages:  tc.monitorMessages,
			}, resp)
		})
	}
}

func TestUpdateIPs(t *testing.T) {
	t.Parallel()

	domains := map[ipnet.Type][]domain.Domain{
		ipnet.IP4: {domain4},
		ipnet.IP6: {domain6},
	}
	list := api.WAFList{AccountID: "12341234", ListName: "list"}
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
					pv[ipnet.IP4].EXPECT().GetIP(gomock.Any(), p, ipnet.IP4, true).Return(ip4, true),
					p.EXPECT().Infof(pp.EmojiInternet, "Detected the %s address: %v", "IPv4", ip4),
					p.EXPECT().SuppressHint(pp.HintIP4DetectionFails),
					s.EXPECT().Set(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello"), ip4, api.TTLAuto, false, recordComment).
						Return(setter.ResponseNoop),
					s.EXPECT().SetWAFList(gomock.Any(), p, list, wafListDescription, detectedIPs{ipnet.IP4: ip4}, "").
						Return(setter.ResponseNoop),
				)
			},
		},
		"ip4-only/set-fail": { //nolint:dupl
			false,
			[]string{"Failed to set A (127.0.0.1): ip4.hello", "Failed to set list(s): 12341234/list"},
			[]string{
				"Failed to properly update A records of ip4.hello with 127.0.0.1.",
				`Failed to properly update WAF list(s) 12341234/list.`,
			},
			providerEnablers{ipnet.IP4: true},
			func(p *mocks.MockPP, pv mockProviders, s *mocks.MockSetter) {
				gomock.InOrder(
					pv[ipnet.IP4].EXPECT().GetIP(gomock.Any(), p, ipnet.IP4, true).Return(ip4, true),
					p.EXPECT().Infof(pp.EmojiInternet, "Detected the %s address: %v", "IPv4", ip4),
					p.EXPECT().SuppressHint(pp.HintIP4DetectionFails),
					s.EXPECT().Set(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello"), ip4, api.TTLAuto, false, recordComment).
						Return(setter.ResponseFailed),
					s.EXPECT().SetWAFList(gomock.Any(), p, list, wafListDescription, detectedIPs{ipnet.IP4: ip4}, "").
						Return(setter.ResponseFailed),
				)
			},
		},
		"ip6-only": { //nolint:dupl
			true,
			[]string{"Set AAAA (::1): ip6.hello", "Set list(s): 12341234/list"},
			[]string{
				"Updated AAAA records of ip6.hello with ::1.",
				`Updated WAF list(s) 12341234/list.`,
			},
			providerEnablers{ipnet.IP6: true},
			func(p *mocks.MockPP, pv mockProviders, s *mocks.MockSetter) {
				gomock.InOrder(
					pv[ipnet.IP6].EXPECT().GetIP(gomock.Any(), p, ipnet.IP6, true).Return(ip6, true),
					p.EXPECT().Infof(pp.EmojiInternet, "Detected the %s address: %v", "IPv6", ip6),
					p.EXPECT().SuppressHint(pp.HintIP6DetectionFails),
					s.EXPECT().Set(gomock.Any(), p, ipnet.IP6, domain.FQDN("ip6.hello"), ip6, api.TTLAuto, false, recordComment).
						Return(setter.ResponseUpdated),
					s.EXPECT().SetWAFList(gomock.Any(), p, list, wafListDescription, detectedIPs{ipnet.IP6: ip6}, "").
						Return(setter.ResponseUpdated),
				)
			},
		},
		"ip6-only/set-fail": { //nolint:dupl
			false,
			[]string{"Failed to set AAAA (::1): ip6.hello", "Failed to set list(s): 12341234/list"},
			[]string{
				"Failed to properly update AAAA records of ip6.hello with ::1.",
				`Failed to properly update WAF list(s) 12341234/list.`,
			},
			providerEnablers{ipnet.IP6: true},
			func(p *mocks.MockPP, pv mockProviders, s *mocks.MockSetter) {
				gomock.InOrder(
					pv[ipnet.IP6].EXPECT().GetIP(gomock.Any(), p, ipnet.IP6, true).Return(ip6, true),
					p.EXPECT().Infof(pp.EmojiInternet, "Detected the %s address: %v", "IPv6", ip6),
					p.EXPECT().SuppressHint(pp.HintIP6DetectionFails),
					s.EXPECT().Set(gomock.Any(), p, ipnet.IP6, domain.FQDN("ip6.hello"), ip6, api.TTLAuto, false, recordComment).
						Return(setter.ResponseFailed),
					s.EXPECT().SetWAFList(gomock.Any(), p, list, wafListDescription, detectedIPs{ipnet.IP6: ip6}, "").
						Return(setter.ResponseFailed),
				)
			},
		},
		"dual": {
			true, nil, nil,
			providerEnablers{ipnet.IP4: true, ipnet.IP6: true},
			func(p *mocks.MockPP, pv mockProviders, s *mocks.MockSetter) { //nolint:dupl
				gomock.InOrder(
					pv[ipnet.IP4].EXPECT().GetIP(gomock.Any(), p, ipnet.IP4, true).Return(ip4, true),
					p.EXPECT().Infof(pp.EmojiInternet, "Detected the %s address: %v", "IPv4", ip4),
					p.EXPECT().SuppressHint(pp.HintIP4DetectionFails),
					s.EXPECT().Set(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello"), ip4, api.TTLAuto, false, recordComment).
						Return(setter.ResponseNoop),
					pv[ipnet.IP6].EXPECT().GetIP(gomock.Any(), p, ipnet.IP6, true).Return(ip6, true),
					p.EXPECT().Infof(pp.EmojiInternet, "Detected the %s address: %v", "IPv6", ip6),
					p.EXPECT().SuppressHint(pp.HintIP6DetectionFails),
					s.EXPECT().Set(gomock.Any(), p, ipnet.IP6, domain.FQDN("ip6.hello"), ip6, api.TTLAuto, false, recordComment).
						Return(setter.ResponseNoop),
					s.EXPECT().SetWAFList(gomock.Any(), p, list, wafListDescription,
						detectedIPs{ipnet.IP4: ip4, ipnet.IP6: ip6}, "").
						Return(setter.ResponseNoop),
				)
			},
		},
		"dual/set-fail/1": {
			false,
			[]string{"Failed to set A (127.0.0.1): ip4.hello"},
			[]string{"Failed to properly update A records of ip4.hello with 127.0.0.1."},
			providerEnablers{ipnet.IP4: true, ipnet.IP6: true},
			func(p *mocks.MockPP, pv mockProviders, s *mocks.MockSetter) { //nolint:dupl
				gomock.InOrder(
					pv[ipnet.IP4].EXPECT().GetIP(gomock.Any(), p, ipnet.IP4, true).Return(ip4, true),
					p.EXPECT().Infof(pp.EmojiInternet, "Detected the %s address: %v", "IPv4", ip4),
					p.EXPECT().SuppressHint(pp.HintIP4DetectionFails),
					s.EXPECT().Set(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello"), ip4, api.TTLAuto, false, recordComment).
						Return(setter.ResponseFailed),
					pv[ipnet.IP6].EXPECT().GetIP(gomock.Any(), p, ipnet.IP6, true).Return(ip6, true),
					p.EXPECT().Infof(pp.EmojiInternet, "Detected the %s address: %v", "IPv6", ip6),
					p.EXPECT().SuppressHint(pp.HintIP6DetectionFails),
					s.EXPECT().Set(gomock.Any(), p, ipnet.IP6, domain.FQDN("ip6.hello"), ip6, api.TTLAuto, false, recordComment).
						Return(setter.ResponseNoop),
					s.EXPECT().SetWAFList(gomock.Any(), p, list, wafListDescription,
						detectedIPs{ipnet.IP4: ip4, ipnet.IP6: ip6}, "").
						Return(setter.ResponseNoop),
				)
			},
		},
		"dual/set-fail/2": {
			false,
			[]string{"Failed to set AAAA (::1): ip6.hello"},
			[]string{"Failed to properly update AAAA records of ip6.hello with ::1."},
			providerEnablers{ipnet.IP4: true, ipnet.IP6: true},
			func(p *mocks.MockPP, pv mockProviders, s *mocks.MockSetter) { //nolint:dupl
				gomock.InOrder(
					pv[ipnet.IP4].EXPECT().GetIP(gomock.Any(), p, ipnet.IP4, true).Return(ip4, true),
					p.EXPECT().Infof(pp.EmojiInternet, "Detected the %s address: %v", "IPv4", ip4),
					p.EXPECT().SuppressHint(pp.HintIP4DetectionFails),
					s.EXPECT().Set(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello"), ip4, api.TTLAuto, false, recordComment).
						Return(setter.ResponseNoop),
					pv[ipnet.IP6].EXPECT().GetIP(gomock.Any(), p, ipnet.IP6, true).Return(ip6, true),
					p.EXPECT().Infof(pp.EmojiInternet, "Detected the %s address: %v", "IPv6", ip6),
					p.EXPECT().SuppressHint(pp.HintIP6DetectionFails),
					s.EXPECT().Set(gomock.Any(), p, ipnet.IP6, domain.FQDN("ip6.hello"), ip6, api.TTLAuto, false, recordComment).
						Return(setter.ResponseFailed),
					s.EXPECT().SetWAFList(gomock.Any(), p, list, wafListDescription,
						detectedIPs{ipnet.IP4: ip4, ipnet.IP6: ip6}, "").
						Return(setter.ResponseNoop),
				)
			},
		},
		"dual/set-fail/3": {
			false,
			[]string{`Failed to set list(s): 12341234/list`},
			[]string{`Failed to properly update WAF list(s) 12341234/list.`},
			providerEnablers{ipnet.IP4: true, ipnet.IP6: true},
			func(p *mocks.MockPP, pv mockProviders, s *mocks.MockSetter) { //nolint:dupl
				gomock.InOrder(
					pv[ipnet.IP4].EXPECT().GetIP(gomock.Any(), p, ipnet.IP4, true).Return(ip4, true),
					p.EXPECT().Infof(pp.EmojiInternet, "Detected the %s address: %v", "IPv4", ip4),
					p.EXPECT().SuppressHint(pp.HintIP4DetectionFails),
					s.EXPECT().Set(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello"), ip4, api.TTLAuto, false, recordComment).
						Return(setter.ResponseNoop),
					pv[ipnet.IP6].EXPECT().GetIP(gomock.Any(), p, ipnet.IP6, true).Return(ip6, true),
					p.EXPECT().Infof(pp.EmojiInternet, "Detected the %s address: %v", "IPv6", ip6),
					p.EXPECT().SuppressHint(pp.HintIP6DetectionFails),
					s.EXPECT().Set(gomock.Any(), p, ipnet.IP6, domain.FQDN("ip6.hello"), ip6, api.TTLAuto, false, recordComment).
						Return(setter.ResponseNoop),
					s.EXPECT().SetWAFList(gomock.Any(), p, list, wafListDescription,
						detectedIPs{ipnet.IP4: ip4, ipnet.IP6: ip6}, "").
						Return(setter.ResponseFailed),
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
					pv[ipnet.IP4].EXPECT().GetIP(gomock.Any(), p, ipnet.IP4, true).Return(netip.Addr{}, false),
					p.EXPECT().Noticef(pp.EmojiError, "Failed to detect the %s address", "IPv4"),
					p.EXPECT().Hintf(pp.HintIP4DetectionFails, "If your network does not support IPv4, you can disable it with IP4_PROVIDER=none"), //nolint:lll
					pv[ipnet.IP6].EXPECT().GetIP(gomock.Any(), p, ipnet.IP6, true).Return(ip6, true),
					p.EXPECT().Infof(pp.EmojiInternet, "Detected the %s address: %v", "IPv6", ip6),
					p.EXPECT().SuppressHint(pp.HintIP6DetectionFails),
					s.EXPECT().Set(gomock.Any(), p, ipnet.IP6, domain.FQDN("ip6.hello"), ip6, api.TTLAuto, false, recordComment).
						Return(setter.ResponseNoop),
					s.EXPECT().SetWAFList(gomock.Any(), p, list, wafListDescription,
						detectedIPs{ipnet.IP4: netip.Addr{}, ipnet.IP6: ip6}, ""),
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
					pv[ipnet.IP4].EXPECT().GetIP(gomock.Any(), p, ipnet.IP4, true).Return(ip4, true),
					p.EXPECT().Infof(pp.EmojiInternet, "Detected the %s address: %v", "IPv4", ip4),
					p.EXPECT().SuppressHint(pp.HintIP4DetectionFails),
					s.EXPECT().Set(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello"), ip4, api.TTLAuto, false, recordComment).
						Return(setter.ResponseNoop),
					pv[ipnet.IP6].EXPECT().GetIP(gomock.Any(), p, ipnet.IP6, true).Return(netip.Addr{}, false),
					p.EXPECT().Noticef(pp.EmojiError, "Failed to detect the %s address", "IPv6"),
					p.EXPECT().Hintf(pp.HintIP6DetectionFails, "If you are using Docker or Kubernetes, IPv6 often requires additional steps to set up; read more at %s. If your network does not support IPv6, you can disable it with IP6_PROVIDER=none", pp.ManualURL), //nolint:lll
					s.EXPECT().SetWAFList(gomock.Any(), p, list, wafListDescription,
						detectedIPs{ipnet.IP4: ip4, ipnet.IP6: netip.Addr{}}, ""),
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
					pv[ipnet.IP4].EXPECT().GetIP(gomock.Any(), p, ipnet.IP4, true).Return(netip.Addr{}, false),
					p.EXPECT().Noticef(pp.EmojiError, "Failed to detect the %s address", "IPv4"),
					p.EXPECT().Hintf(pp.HintIP4DetectionFails, "If your network does not support IPv4, you can disable it with IP4_PROVIDER=none"), //nolint:lll
					pv[ipnet.IP6].EXPECT().GetIP(gomock.Any(), p, ipnet.IP6, true).Return(netip.Addr{}, false),
					p.EXPECT().Noticef(pp.EmojiError, "Failed to detect the %s address", "IPv6"),
					p.EXPECT().Hintf(pp.HintIP6DetectionFails, "If you are using Docker or Kubernetes, IPv6 often requires additional steps to set up; read more at %s. If your network does not support IPv6, you can disable it with IP6_PROVIDER=none", pp.ManualURL), //nolint:lll
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
					pv[ipnet.IP4].EXPECT().GetIP(gomock.Any(), p, ipnet.IP4, true).
						DoAndReturn(
							func(context.Context, pp.PP, ipnet.Type, bool) (netip.Addr, bool) {
								time.Sleep(2 * time.Second)
								return netip.Addr{}, false
							},
						),
					p.EXPECT().Noticef(pp.EmojiError, "Failed to detect the %s address", "IPv4"),
					p.EXPECT().Hintf(pp.HintIP4DetectionFails, "If your network does not support IPv4, you can disable it with IP4_PROVIDER=none"),                    //nolint:lll
					p.EXPECT().Hintf(pp.HintDetectionTimeouts, "If your network is experiencing high latency, consider increasing DETECTION_TIMEOUT=%v", time.Second), //nolint:lll
					s.EXPECT().SetWAFList(gomock.Any(), p, list, wafListDescription, detectedIPs{ipnet.IP4: netip.Addr{}}, "").
						Return(setter.ResponseNoop),
				)
			},
		},
		"set-timeout": {
			false,
			[]string{"Failed to set A (127.0.0.1): ip4.hello"},
			[]string{"Failed to properly update A records of ip4.hello with 127.0.0.1."},
			providerEnablers{ipnet.IP4: true},
			func(p *mocks.MockPP, pv mockProviders, s *mocks.MockSetter) {
				gomock.InOrder(
					pv[ipnet.IP4].EXPECT().GetIP(gomock.Any(), p, ipnet.IP4, true).Return(ip4, true),
					p.EXPECT().Infof(pp.EmojiInternet, "Detected the %s address: %v", "IPv4", ip4),
					p.EXPECT().SuppressHint(pp.HintIP4DetectionFails),
					s.EXPECT().Set(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello"), ip4, api.TTLAuto, false, recordComment).
						DoAndReturn(
							func(context.Context, pp.PP, ipnet.Type, domain.Domain, netip.Addr, api.TTL, bool, string) setter.ResponseCode { //nolint:lll
								time.Sleep(2 * time.Second)
								return setter.ResponseFailed
							}),
					p.EXPECT().Hintf(pp.HintUpdateTimeouts, "If your network is experiencing high latency, consider increasing UPDATE_TIMEOUT=%v", time.Second), //nolint:lll
					s.EXPECT().SetWAFList(gomock.Any(), p, list, wafListDescription, detectedIPs{ipnet.IP4: ip4}, "").
						Return(setter.ResponseNoop),
				)
			},
		},
		"set-list-timeout": {
			false,
			[]string{"Failed to set list(s): 12341234/list"},
			[]string{`Failed to properly update WAF list(s) 12341234/list.`},
			providerEnablers{ipnet.IP4: true},
			func(p *mocks.MockPP, pv mockProviders, s *mocks.MockSetter) {
				gomock.InOrder(
					pv[ipnet.IP4].EXPECT().GetIP(gomock.Any(), p, ipnet.IP4, true).Return(ip4, true),
					p.EXPECT().Infof(pp.EmojiInternet, "Detected the %s address: %v", "IPv4", ip4),
					p.EXPECT().SuppressHint(pp.HintIP4DetectionFails),
					s.EXPECT().Set(gomock.Any(), p, ipnet.IP4, domain.FQDN("ip4.hello"), ip4, api.TTLAuto, false, recordComment).
						Return(setter.ResponseNoop),
					s.EXPECT().SetWAFList(gomock.Any(), p, list, wafListDescription, detectedIPs{ipnet.IP4: ip4}, "").
						DoAndReturn(
							func(context.Context, pp.PP, api.WAFList, string, detectedIPs, string) setter.ResponseCode {
								time.Sleep(2 * time.Second)
								return setter.ResponseFailed
							}),
					p.EXPECT().Hintf(pp.HintUpdateTimeouts, "If your network is experiencing high latency, consider increasing UPDATE_TIMEOUT=%v", time.Second), //nolint:lll
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
			require.Equal(t, message.Message{
				OK:               tc.ok,
				NotifierMessages: tc.notifierMessages,
				MonitorMessages:  tc.monitorMessages,
			}, resp)
		})
	}
}

func TestDeleteIPs(t *testing.T) {
	t.Parallel()

	domains := map[ipnet.Type][]domain.Domain{
		ipnet.IP4: {domain4},
		ipnet.IP6: {domain6},
	}
	list := api.WAFList{AccountID: "12341234", ListName: "list"}
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
			[]string{"Deleted A: ip4.hello", "Deleted list(s): 12341234/list"},
			[]string{
				"Deleted A records of ip4.hello.",
				`Deleted WAF list(s) 12341234/list.`,
			},
			mockproviders{ipnet.IP4: true},
			func(ppfmt *mocks.MockPP, s *mocks.MockSetter) {
				gomock.InOrder(
					s.EXPECT().Delete(gomock.Any(), ppfmt, ipnet.IP4, domain4).Return(setter.ResponseUpdated),
					s.EXPECT().DeleteWAFList(gomock.Any(), ppfmt, list).Return(setter.ResponseUpdated),
				)
			},
		},
		"ip4-only/fail": {
			false,
			[]string{"Failed to delete A: ip4.hello", "Failed to delete list(s): 12341234/list"},
			[]string{
				"Failed to properly delete A records of ip4.hello.",
				`Failed to properly delete WAF list(s) 12341234/list.`,
			},
			mockproviders{ipnet.IP4: true},
			func(ppfmt *mocks.MockPP, s *mocks.MockSetter) {
				gomock.InOrder(
					s.EXPECT().Delete(gomock.Any(), ppfmt, ipnet.IP4, domain4).Return(setter.ResponseFailed),
					s.EXPECT().DeleteWAFList(gomock.Any(), ppfmt, list).Return(setter.ResponseFailed),
				)
			},
		},
		"ip6-only": {
			true,
			[]string{"Deleted AAAA: ip6.hello", "Deleted list(s): 12341234/list"},
			[]string{
				"Deleted AAAA records of ip6.hello.",
				`Deleted WAF list(s) 12341234/list.`,
			},
			mockproviders{ipnet.IP6: true},
			func(ppfmt *mocks.MockPP, s *mocks.MockSetter) {
				gomock.InOrder(
					s.EXPECT().Delete(gomock.Any(), ppfmt, ipnet.IP6, domain6).Return(setter.ResponseUpdated),
					s.EXPECT().DeleteWAFList(gomock.Any(), ppfmt, list).Return(setter.ResponseUpdated),
				)
			},
		},
		"ip6-only/fail": {
			false,
			[]string{"Failed to delete AAAA: ip6.hello", "Failed to delete list(s): 12341234/list"},
			[]string{
				"Failed to properly delete AAAA records of ip6.hello.",
				`Failed to properly delete WAF list(s) 12341234/list.`,
			},
			mockproviders{ipnet.IP6: true},
			func(ppfmt *mocks.MockPP, s *mocks.MockSetter) {
				gomock.InOrder(
					s.EXPECT().Delete(gomock.Any(), ppfmt, ipnet.IP6, domain6).Return(setter.ResponseFailed),
					s.EXPECT().DeleteWAFList(gomock.Any(), ppfmt, list).Return(setter.ResponseFailed),
				)
			},
		},
		"dual": {
			true,
			[]string{"Deleted A: ip4.hello", "Deleted AAAA: ip6.hello", "Deleted list(s): 12341234/list"},
			[]string{
				"Deleted A records of ip4.hello.", "Deleted AAAA records of ip6.hello.",
				`Deleted WAF list(s) 12341234/list.`,
			},
			mockproviders{ipnet.IP4: true, ipnet.IP6: true},
			func(ppfmt *mocks.MockPP, s *mocks.MockSetter) {
				gomock.InOrder(
					s.EXPECT().Delete(gomock.Any(), ppfmt, ipnet.IP4, domain4).Return(setter.ResponseUpdated),
					s.EXPECT().Delete(gomock.Any(), ppfmt, ipnet.IP6, domain6).Return(setter.ResponseUpdated),
					s.EXPECT().DeleteWAFList(gomock.Any(), ppfmt, list).Return(setter.ResponseUpdated),
				)
			},
		},
		"dual/fail/1": {
			false,
			[]string{"Failed to delete A: ip4.hello"},
			[]string{"Failed to properly delete A records of ip4.hello."},
			mockproviders{ipnet.IP4: true, ipnet.IP6: true},
			func(ppfmt *mocks.MockPP, s *mocks.MockSetter) {
				gomock.InOrder(
					s.EXPECT().Delete(gomock.Any(), ppfmt, ipnet.IP4, domain4).Return(setter.ResponseFailed),
					s.EXPECT().Delete(gomock.Any(), ppfmt, ipnet.IP6, domain6).Return(setter.ResponseNoop),
					s.EXPECT().DeleteWAFList(gomock.Any(), ppfmt, list).Return(setter.ResponseNoop),
				)
			},
		},
		"dual/fail/2": {
			false,
			[]string{"Failed to delete AAAA: ip6.hello"},
			[]string{"Failed to properly delete AAAA records of ip6.hello."},
			mockproviders{ipnet.IP4: true, ipnet.IP6: true},
			func(ppfmt *mocks.MockPP, s *mocks.MockSetter) {
				gomock.InOrder(
					s.EXPECT().Delete(gomock.Any(), ppfmt, ipnet.IP4, domain4).Return(setter.ResponseNoop),
					s.EXPECT().Delete(gomock.Any(), ppfmt, ipnet.IP6, domain6).Return(setter.ResponseFailed),
					s.EXPECT().DeleteWAFList(gomock.Any(), ppfmt, list).Return(setter.ResponseNoop),
				)
			},
		},
		"dual/fail/3": {
			false,
			[]string{"Failed to delete list(s): 12341234/list"},
			[]string{`Failed to properly delete WAF list(s) 12341234/list.`},
			mockproviders{ipnet.IP4: true, ipnet.IP6: true},
			func(ppfmt *mocks.MockPP, s *mocks.MockSetter) {
				gomock.InOrder(
					s.EXPECT().Delete(gomock.Any(), ppfmt, ipnet.IP4, domain4).Return(setter.ResponseNoop),
					s.EXPECT().Delete(gomock.Any(), ppfmt, ipnet.IP6, domain6).Return(setter.ResponseNoop),
					s.EXPECT().DeleteWAFList(gomock.Any(), ppfmt, list).Return(setter.ResponseFailed),
				)
			},
		},
		"delete-timeout": {
			false,
			[]string{"Failed to delete A: ip4.hello"},
			[]string{"Failed to properly delete A records of ip4.hello."},
			mockproviders{ipnet.IP4: true},
			func(ppfmt *mocks.MockPP, s *mocks.MockSetter) {
				gomock.InOrder(
					s.EXPECT().Delete(gomock.Any(), ppfmt, ipnet.IP4, domain.FQDN("ip4.hello")).
						DoAndReturn(
							func(context.Context, pp.PP, ipnet.Type, domain.Domain) setter.ResponseCode {
								time.Sleep(2 * time.Second)
								return setter.ResponseFailed
							}),
					ppfmt.EXPECT().Hintf(pp.HintUpdateTimeouts, "If your network is experiencing high latency, consider increasing UPDATE_TIMEOUT=%v", time.Second), //nolint:lll
					s.EXPECT().DeleteWAFList(gomock.Any(), ppfmt, list).Return(setter.ResponseNoop),
				)
			},
		},
		"delete-list-timeout": {
			false,
			[]string{"Failed to delete list(s): 12341234/list"},
			[]string{`Failed to properly delete WAF list(s) 12341234/list.`},
			mockproviders{ipnet.IP4: true},
			func(ppfmt *mocks.MockPP, s *mocks.MockSetter) {
				gomock.InOrder(
					s.EXPECT().Delete(gomock.Any(), ppfmt, ipnet.IP4, domain.FQDN("ip4.hello")).
						Return(setter.ResponseNoop),
					s.EXPECT().DeleteWAFList(gomock.Any(), ppfmt, list).
						DoAndReturn(
							func(context.Context, pp.PP, api.WAFList) setter.ResponseCode {
								time.Sleep(2 * time.Second)
								return setter.ResponseFailed
							}),
					ppfmt.EXPECT().Hintf(pp.HintUpdateTimeouts, "If your network is experiencing high latency, consider increasing UPDATE_TIMEOUT=%v", time.Second), //nolint:lll
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

			resp := updater.DeleteIPs(ctx, mockPP, conf, mockSetter)
			require.Equal(t, message.Message{
				OK:               tc.ok,
				NotifierMessages: tc.notifierMessages,
				MonitorMessages:  tc.monitorMessages,
			}, resp)
		})
	}
}
