// vim: nowrap
package setter_test

import (
	"context"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/setter"
)

func wrapCancelAsDelete(cancel func()) func(context.Context, pp.PP, ipnet.Type, domain.Domain, api.ID, api.DeletionMode) bool {
	return func(context.Context, pp.PP, ipnet.Type, domain.Domain, api.ID, api.DeletionMode) bool {
		cancel()
		return false
	}
}

func TestSet(t *testing.T) {
	t.Parallel()

	const (
		domain    = domain.FQDN("sub.test.org")
		ipNetwork = ipnet.IP6
		record1   = api.ID("record1")
		record2   = api.ID("record2")
		record3   = api.ID("record3")
	)
	var (
		ip1    = netip.MustParseAddr("::1")
		ip2    = netip.MustParseAddr("::2")
		params = api.RecordParams{
			TTL:     api.TTLAuto,
			Proxied: false,
			Comment: "hello",
		}
	)

	for name, tc := range map[string]struct {
		ip           netip.Addr
		resp         setter.ResponseCode
		prepareMocks func(ctx context.Context, cancel func(), p *mocks.MockPP, m *mocks.MockHandle)
	}{
		"0": {
			ip1,
			setter.ResponseUpdated,
			func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, ipNetwork, domain, params).Return([]api.Record{}, true, true),
					h.EXPECT().CreateRecord(ctx, p, ipNetwork, domain, ip1, params).Return(record1, true),
					p.EXPECT().Noticef(pp.EmojiCreation, "Added a new %s record of %s (ID: %s)", "AAAA", "sub.test.org", record1),
				)
			},
		},
		"0/create-fail": {
			ip1,
			setter.ResponseFailed,
			func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, ipNetwork, domain, params).Return([]api.Record{}, true, true),
					h.EXPECT().CreateRecord(ctx, p, ipNetwork, domain, ip1, params).Return(record1, false),
					p.EXPECT().Noticef(pp.EmojiError, "Failed to properly update %s records of %s; records might be inconsistent", "AAAA", "sub.test.org"),
				)
			},
		},
		"1unmatched": {
			ip1,
			setter.ResponseUpdated,
			func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, ipNetwork, domain, params).
						Return([]api.Record{{ID: record1, IP: ip2, RecordParams: params}}, true, true),
					h.EXPECT().UpdateRecord(ctx, p, ipNetwork, domain, record1, ip1, params, params).Return(true),
					p.EXPECT().Noticef(pp.EmojiUpdate, "Updated a stale %s record of %s (ID: %s)", "AAAA", "sub.test.org", record1),
				)
			},
		},
		"1unmatched/update-fail": {
			ip1,
			setter.ResponseFailed,
			func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, ipNetwork, domain, params).
						Return([]api.Record{{ID: record1, IP: ip2, RecordParams: params}}, true, true),
					h.EXPECT().UpdateRecord(ctx, p, ipNetwork, domain, record1, ip1, params, params).Return(false),
					p.EXPECT().Noticef(pp.EmojiError, "Failed to properly update %s records of %s; records might be inconsistent", "AAAA", "sub.test.org"),
				)
			},
		},
		"1matched": {
			ip1,
			setter.ResponseNoop,
			func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, ipNetwork, domain, params).
						Return([]api.Record{{ID: record1, IP: ip1, RecordParams: params}}, true, true),
					p.EXPECT().Infof(pp.EmojiAlreadyDone,
						"The %s records of %s are already up to date (cached)", "AAAA", "sub.test.org"),
				)
			},
		},
		"1matched/not-cached": {
			ip1,
			setter.ResponseNoop,
			func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, ipNetwork, domain, params).Return([]api.Record{
						{ID: record1, IP: ip1, RecordParams: params},
					}, false, true),
					p.EXPECT().Infof(pp.EmojiAlreadyDone, "The %s records of %s are already up to date", "AAAA", "sub.test.org"),
				)
			},
		},
		"3matched": {
			ip1,
			setter.ResponseUpdated,
			func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, ipNetwork, domain, params).Return([]api.Record{
						{ID: record1, IP: ip1, RecordParams: params},
						{ID: record2, IP: ip1, RecordParams: params},
						{ID: record3, IP: ip1, RecordParams: params},
					}, true, true),
					h.EXPECT().DeleteRecord(ctx, p, ipNetwork, domain, record2, api.RegularDelitionMode).Return(true),
					p.EXPECT().Noticef(pp.EmojiDeletion, "Deleted a duplicate %s record of %s (ID: %s)", "AAAA", "sub.test.org", record2),
					h.EXPECT().DeleteRecord(ctx, p, ipNetwork, domain, record3, api.RegularDelitionMode).Return(true),
					p.EXPECT().Noticef(pp.EmojiDeletion, "Deleted a duplicate %s record of %s (ID: %s)", "AAAA", "sub.test.org", record3),
				)
			},
		},
		"3matched/delete-fail/1": {
			ip1,
			setter.ResponseUpdated,
			func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, ipNetwork, domain, params).Return([]api.Record{
						{ID: record1, IP: ip1, RecordParams: params},
						{ID: record2, IP: ip1, RecordParams: params},
						{ID: record3, IP: ip1, RecordParams: params},
					}, true, true),
					h.EXPECT().DeleteRecord(ctx, p, ipNetwork, domain, record2, api.RegularDelitionMode).Return(false),
					h.EXPECT().DeleteRecord(ctx, p, ipNetwork, domain, record3, api.RegularDelitionMode).Return(true),
					p.EXPECT().Noticef(pp.EmojiDeletion, "Deleted a duplicate %s record of %s (ID: %s)", "AAAA", "sub.test.org", record3),
				)
			},
		},
		"3matched/delete-fail/2": {
			ip1,
			setter.ResponseUpdated,
			func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, ipNetwork, domain, params).Return([]api.Record{
						{ID: record1, IP: ip1, RecordParams: params},
						{ID: record2, IP: ip1, RecordParams: params},
						{ID: record3, IP: ip1, RecordParams: params},
					}, true, true),
					h.EXPECT().DeleteRecord(ctx, p, ipNetwork, domain, record2, api.RegularDelitionMode).Return(true),
					p.EXPECT().Noticef(pp.EmojiDeletion, "Deleted a duplicate %s record of %s (ID: %s)", "AAAA", "sub.test.org", record2),
					h.EXPECT().DeleteRecord(ctx, p, ipNetwork, domain, record3, api.RegularDelitionMode).Return(false),
				)
			},
		},
		"3matched/delete-timeout": {
			ip1,
			setter.ResponseUpdated,
			func(ctx context.Context, cancel func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, ipNetwork, domain, params).Return([]api.Record{
						{ID: record1, IP: ip1, RecordParams: params},
						{ID: record2, IP: ip1, RecordParams: params},
					}, true, true),
					h.EXPECT().DeleteRecord(ctx, p, ipNetwork, domain, record2, api.RegularDelitionMode).Do(wrapCancelAsDelete(cancel)).Return(false),
				)
			},
		},
		"2unmatched": {
			ip1,
			setter.ResponseUpdated,
			func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, ipNetwork, domain, params).Return([]api.Record{
						{ID: record1, IP: ip2, RecordParams: params},
						{ID: record2, IP: ip2, RecordParams: params},
					}, true, true),
					h.EXPECT().UpdateRecord(ctx, p, ipNetwork, domain, record1, ip1, params, params).Return(true),
					p.EXPECT().Noticef(pp.EmojiUpdate, "Updated a stale %s record of %s (ID: %s)", "AAAA", "sub.test.org", record1),
					h.EXPECT().DeleteRecord(ctx, p, ipNetwork, domain, record2, api.RegularDelitionMode).Return(true),
					p.EXPECT().Noticef(pp.EmojiDeletion, "Deleted a stale %s record of %s (ID: %s)", "AAAA", "sub.test.org", record2),
				)
			},
		},
		"2unmatched/delete-timeout": {
			ip1,
			setter.ResponseFailed,
			func(ctx context.Context, cancel func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, ipNetwork, domain, params).Return([]api.Record{
						{ID: record1, IP: ip2, RecordParams: params},
						{ID: record2, IP: ip2, RecordParams: params},
					}, true, true),
					h.EXPECT().UpdateRecord(ctx, p, ipNetwork, domain, record1, ip1, params, params).Return(true),
					p.EXPECT().Noticef(pp.EmojiUpdate, "Updated a stale %s record of %s (ID: %s)", "AAAA", "sub.test.org", record1),
					h.EXPECT().DeleteRecord(ctx, p, ipNetwork, domain, record2, api.RegularDelitionMode).Do(wrapCancelAsDelete(cancel)).Return(false),
					p.EXPECT().Noticef(pp.EmojiError, "Failed to properly update %s records of %s; records might be inconsistent", "AAAA", "sub.test.org"),
				)
			},
		},
		"2unmatched/update-fail": {
			ip1,
			setter.ResponseFailed,
			func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, ipNetwork, domain, params).Return([]api.Record{
						{ID: record1, IP: ip2, RecordParams: params},
						{ID: record2, IP: ip2, RecordParams: params},
					}, true, true),
					h.EXPECT().UpdateRecord(ctx, p, ipNetwork, domain, record1, ip1, params, params).Return(false),
					p.EXPECT().Noticef(pp.EmojiError, "Failed to properly update %s records of %s; records might be inconsistent", "AAAA", "sub.test.org"),
				)
			},
		},
		"list-fail": {
			ip1,
			setter.ResponseFailed,
			func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				h.EXPECT().ListRecords(ctx, p, ipNetwork, domain, params).Return(nil, false, false)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			mockCtrl := gomock.NewController(t)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			mockPP := mocks.NewMockPP(mockCtrl)
			mockHandle := mocks.NewMockHandle(mockCtrl)
			if tc.prepareMocks != nil {
				tc.prepareMocks(ctx, cancel, mockPP, mockHandle)
			}

			s, ok := setter.New(mockPP, mockHandle)
			require.True(t, ok)

			resp := s.Set(ctx, mockPP, ipNetwork, domain, tc.ip, params)
			require.Equal(t, tc.resp, resp)
		})
	}
}

func TestFinalDelete(t *testing.T) {
	t.Parallel()

	const (
		domain    = domain.FQDN("sub.test.org")
		ipNetwork = ipnet.IP6
		record1   = api.ID("record1")
		record2   = api.ID("record2")
		record3   = api.ID("record3")
	)
	var (
		ip1       = netip.MustParseAddr("::1")
		invalidIP = netip.Addr{}
		params    = api.RecordParams{
			TTL:     api.TTLAuto,
			Proxied: false,
			Comment: "hello",
		}
	)

	for name, tc := range map[string]struct {
		resp         setter.ResponseCode
		prepareMocks func(ctx context.Context, cancel func(), p *mocks.MockPP, m *mocks.MockHandle)
	}{
		"0": {
			setter.ResponseNoop,
			func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, ipNetwork, domain, params).Return([]api.Record{}, true, true),
					p.EXPECT().Infof(pp.EmojiAlreadyDone, "The %s records of %s were already deleted (cached)", "AAAA", "sub.test.org"),
				)
			},
		},
		"0/not-cached": {
			setter.ResponseNoop,
			func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, ipNetwork, domain, params).Return([]api.Record{}, false, true),
					p.EXPECT().Infof(pp.EmojiAlreadyDone, "The %s records of %s were already deleted", "AAAA", "sub.test.org"),
				)
			},
		},
		"1unmatched": {
			setter.ResponseUpdated,
			func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, ipNetwork, domain, params).Return([]api.Record{
						{ID: record1, IP: ip1, RecordParams: params},
					}, true, true),
					h.EXPECT().DeleteRecord(ctx, p, ipNetwork, domain, record1, api.FinalDeletionMode).Return(true),
					p.EXPECT().Noticef(pp.EmojiDeletion, "Deleted a stale %s record of %s (ID: %s)", "AAAA", "sub.test.org", record1),
				)
			},
		},
		"1unmatched/delete-fail": {
			setter.ResponseFailed,
			func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, ipNetwork, domain, params).Return([]api.Record{
						{ID: record1, IP: ip1, RecordParams: params},
					}, true, true),
					h.EXPECT().DeleteRecord(ctx, p, ipNetwork, domain, record1, api.FinalDeletionMode).Return(false),
					p.EXPECT().Noticef(pp.EmojiError, "Failed to properly delete %s records of %s; records might be inconsistent", "AAAA", "sub.test.org"),
				)
			},
		},
		"1unmatched/delete-timeout": {
			setter.ResponseFailed,
			func(ctx context.Context, cancel func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, ipNetwork, domain, params).Return([]api.Record{
						{ID: record1, IP: ip1, RecordParams: params},
					}, true, true),
					h.EXPECT().DeleteRecord(ctx, p, ipNetwork, domain, record1, api.FinalDeletionMode).Do(wrapCancelAsDelete(cancel)).Return(false),
					p.EXPECT().Infof(pp.EmojiTimeout, "Deletion of %s records of %s aborted by timeout or signals; records might be inconsistent", "AAAA", "sub.test.org"),
				)
			},
		},
		"impossible-records": {
			setter.ResponseUpdated,
			func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, ipNetwork, domain, params).Return([]api.Record{
						{ID: record1, IP: ip1, RecordParams: params},
						{ID: record2, IP: invalidIP, RecordParams: params},
					}, true, true),
					h.EXPECT().DeleteRecord(ctx, p, ipNetwork, domain, record1, api.FinalDeletionMode).Return(true),
					p.EXPECT().Noticef(pp.EmojiDeletion, "Deleted a stale %s record of %s (ID: %s)", "AAAA", "sub.test.org", record1),
					h.EXPECT().DeleteRecord(ctx, p, ipNetwork, domain, record2, api.FinalDeletionMode).Return(true),
					p.EXPECT().Noticef(pp.EmojiDeletion, "Deleted a stale %s record of %s (ID: %s)", "AAAA", "sub.test.org", record2),
				)
			},
		},
		"listfail": {
			setter.ResponseFailed,
			func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				h.EXPECT().ListRecords(ctx, p, ipNetwork, domain, params).Return(nil, false, false)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			mockCtrl := gomock.NewController(t)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			mockPP := mocks.NewMockPP(mockCtrl)
			mockHandle := mocks.NewMockHandle(mockCtrl)
			if tc.prepareMocks != nil {
				tc.prepareMocks(ctx, cancel, mockPP, mockHandle)
			}

			s, ok := setter.New(mockPP, mockHandle)
			require.True(t, ok)

			resp := s.FinalDelete(ctx, mockPP, ipNetwork, domain, params)
			require.Equal(t, tc.resp, resp)
		})
	}
}

func TestSetWAFList(t *testing.T) {
	t.Parallel()

	const listName = "list"
	const listDescription = "My List"
	wafList := api.WAFList{AccountID: "account", Name: listName}
	wafListDescribed := "account/list"

	var (
		invalid     = netip.PrefixFrom(netip.Addr{}, -1)
		prefix4     = netip.MustParsePrefix("10.0.0.1/32")
		prefix6     = netip.MustParsePrefix("2001:0db8::/56")
		item4       = api.WAFListItem{Prefix: netip.MustParsePrefix("10.0.0.1/32"), ID: "pre4"}
		item6       = api.WAFListItem{Prefix: netip.MustParsePrefix("2001:0db8::/56"), ID: "pre6"}
		item4range1 = api.WAFListItem{Prefix: netip.MustParsePrefix("10.0.0.0/16"), ID: "ip4-16"}
		item4range2 = api.WAFListItem{Prefix: netip.MustParsePrefix("10.0.0.0/20"), ID: "ip4-20"}
		item4range3 = api.WAFListItem{Prefix: netip.MustParsePrefix("10.0.0.0/24"), ID: "ip4-24"}
		item4wrong1 = api.WAFListItem{Prefix: netip.MustParsePrefix("20.0.0.0/16"), ID: "ip4-16"}
		item4wrong2 = api.WAFListItem{Prefix: netip.MustParsePrefix("20.0.0.0/20"), ID: "ip4-20"}
		item4wrong3 = api.WAFListItem{Prefix: netip.MustParsePrefix("20.0.0.0/24"), ID: "ip4-24"}
		item6range1 = api.WAFListItem{Prefix: netip.MustParsePrefix("2001:db8::/32"), ID: "ip6-32"}
		item6range2 = api.WAFListItem{Prefix: netip.MustParsePrefix("2001:db8::/40"), ID: "ip6-40"}
		item6range3 = api.WAFListItem{Prefix: netip.MustParsePrefix("2001:db8::/48"), ID: "ip6-48"}
		item6range4 = api.WAFListItem{Prefix: netip.MustParsePrefix("2001:db8::/64"), ID: "ip6-64"}
		item6wrong1 = api.WAFListItem{Prefix: netip.MustParsePrefix("4001:db8::/32"), ID: "ip6-32"}
		item6wrong2 = api.WAFListItem{Prefix: netip.MustParsePrefix("4001:db8::/40"), ID: "ip6-40"}
		item6wrong3 = api.WAFListItem{Prefix: netip.MustParsePrefix("4001:db8::/48"), ID: "ip6-48"}
	)

	type items = []api.WAFListItem
	type prefixmap = map[ipnet.Type]netip.Prefix

	for name, tc := range map[string]struct {
		detectedPrefix prefixmap
		resp           setter.ResponseCode
		prepareMocks   func(ctx context.Context, cancel func(), p *mocks.MockPP, m *mocks.MockHandle)
	}{
		"created": {
			prefixmap{ipnet.IP4: prefix4, ipnet.IP6: prefix6},
			setter.ResponseUpdated,
			func(ctx context.Context, _ func(), p *mocks.MockPP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().ListWAFListItems(ctx, p, wafList, listDescription).Return(items{}, false, false, true),
					p.EXPECT().Noticef(pp.EmojiCreation, "Created a new list %s", wafListDescribed),
					m.EXPECT().CreateWAFListItems(ctx, p, wafList, listDescription, []netip.Prefix{item4.Prefix, item6.Prefix}, "").Return(true),
					p.EXPECT().Noticef(pp.EmojiCreation, "Added %s to the list %s", "10.0.0.1", wafListDescribed),
					p.EXPECT().Noticef(pp.EmojiCreation, "Added %s to the list %s", "2001:db8::/56", wafListDescribed),
					m.EXPECT().DeleteWAFListItems(ctx, p, wafList, listDescription, []api.ID{}).Return(true),
				)
			},
		},
		"list-fail": {
			prefixmap{ipnet.IP4: prefix4, ipnet.IP6: prefix6},
			setter.ResponseFailed,
			func(ctx context.Context, _ func(), p *mocks.MockPP, m *mocks.MockHandle) {
				m.EXPECT().ListWAFListItems(ctx, p, wafList, listDescription).Return(nil, false, false, false)
			},
		},
		"skip-unknown": {
			prefixmap{ipnet.IP4: invalid, ipnet.IP6: prefix6},
			setter.ResponseNoop,
			func(ctx context.Context, _ func(), p *mocks.MockPP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().ListWAFListItems(ctx, p, wafList, listDescription).Return(items{
						item4wrong2,
						item6range1,
						item4wrong1,
						item4wrong3,
					}, true, true, true),
					p.EXPECT().Infof(pp.EmojiAlreadyDone, "The list %s is already up to date (cached)", wafListDescribed),
				)
			},
		},
		"noop": {
			prefixmap{ipnet.IP4: invalid, ipnet.IP6: prefix6},
			setter.ResponseNoop,
			func(ctx context.Context, _ func(), p *mocks.MockPP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().ListWAFListItems(ctx, p, wafList, listDescription).Return(items{item4, item6}, true, false, true),
					p.EXPECT().Infof(pp.EmojiAlreadyDone, "The list %s is already up to date", wafListDescribed),
				)
			},
		},
		"noop/cached": {
			prefixmap{ipnet.IP4: invalid, ipnet.IP6: prefix6},
			setter.ResponseNoop,
			func(ctx context.Context, _ func(), p *mocks.MockPP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().ListWAFListItems(ctx, p, wafList, listDescription).Return(items{item4, item6}, true, true, true),
					p.EXPECT().Infof(pp.EmojiAlreadyDone, "The list %s is already up to date (cached)", wafListDescribed),
				)
			},
		},
		"test1": {
			prefixmap{ipnet.IP4: prefix4, ipnet.IP6: prefix6},
			setter.ResponseUpdated,
			func(ctx context.Context, _ func(), p *mocks.MockPP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().ListWAFListItems(ctx, p, wafList, listDescription).Return(items{
						item6range1,
						item4wrong2,
						item6range2,
						item6range3,
						item6range4,
						item4range2,
						item4range3,
						item6wrong2,
						item6wrong3,
						item4wrong3,
						item4range1,
						item4wrong1,
						item6wrong1,
					}, true, false, true),
					m.EXPECT().CreateWAFListItems(ctx, p, wafList, listDescription, nil, "").Return(true),
					m.EXPECT().DeleteWAFListItems(ctx, p, wafList, listDescription, gomock.InAnyOrder([]api.ID{
						item4wrong2.ID,
						item6wrong2.ID,
						item6wrong3.ID,
						item4wrong3.ID,
						item4wrong1.ID,
						item6wrong1.ID,
					})).Return(true),
					p.EXPECT().Noticef(pp.EmojiDeletion, "Deleted %s from the list %s", "20.0.0.0/20", wafListDescribed),
					p.EXPECT().Noticef(pp.EmojiDeletion, "Deleted %s from the list %s", "20.0.0.0/24", wafListDescribed),
					p.EXPECT().Noticef(pp.EmojiDeletion, "Deleted %s from the list %s", "20.0.0.0/16", wafListDescribed),
					p.EXPECT().Noticef(pp.EmojiDeletion, "Deleted %s from the list %s", "4001:db8::/40", wafListDescribed),
					p.EXPECT().Noticef(pp.EmojiDeletion, "Deleted %s from the list %s", "4001:db8::/48", wafListDescribed),
					p.EXPECT().Noticef(pp.EmojiDeletion, "Deleted %s from the list %s", "4001:db8::/32", wafListDescribed),
				)
			},
		},
		"test2": {
			prefixmap{ipnet.IP4: prefix4, ipnet.IP6: prefix6},
			setter.ResponseUpdated,
			func(ctx context.Context, _ func(), p *mocks.MockPP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().ListWAFListItems(ctx, p, wafList, listDescription).Return(items{
						item4wrong2,
						item6wrong2,
						item6wrong3,
						item4wrong3,
						item4wrong1,
						item6wrong1,
					}, true, false, true),
					m.EXPECT().CreateWAFListItems(ctx, p, wafList, listDescription, []netip.Prefix{item4.Prefix, item6.Prefix}, "").Return(true),
					p.EXPECT().Noticef(pp.EmojiCreation, "Added %s to the list %s", "10.0.0.1", wafListDescribed),
					p.EXPECT().Noticef(pp.EmojiCreation, "Added %s to the list %s", "2001:db8::/56", wafListDescribed),
					m.EXPECT().DeleteWAFListItems(ctx, p, wafList, listDescription, gomock.InAnyOrder([]api.ID{
						item4wrong2.ID,
						item6wrong2.ID,
						item6wrong3.ID,
						item4wrong3.ID,
						item4wrong1.ID,
						item6wrong1.ID,
					})).Return(true),
					p.EXPECT().Noticef(pp.EmojiDeletion, "Deleted %s from the list %s", "20.0.0.0/20", wafListDescribed),
					p.EXPECT().Noticef(pp.EmojiDeletion, "Deleted %s from the list %s", "20.0.0.0/24", wafListDescribed),
					p.EXPECT().Noticef(pp.EmojiDeletion, "Deleted %s from the list %s", "20.0.0.0/16", wafListDescribed),
					p.EXPECT().Noticef(pp.EmojiDeletion, "Deleted %s from the list %s", "4001:db8::/40", wafListDescribed),
					p.EXPECT().Noticef(pp.EmojiDeletion, "Deleted %s from the list %s", "4001:db8::/48", wafListDescribed),
					p.EXPECT().Noticef(pp.EmojiDeletion, "Deleted %s from the list %s", "4001:db8::/32", wafListDescribed),
				)
			},
		},
		"create-fail": {
			prefixmap{ipnet.IP4: prefix4, ipnet.IP6: prefix6},
			setter.ResponseFailed,
			func(ctx context.Context, _ func(), p *mocks.MockPP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().ListWAFListItems(ctx, p, wafList, listDescription).Return(items{}, true, false, true),
					m.EXPECT().CreateWAFListItems(ctx, p, wafList, listDescription, []netip.Prefix{item4.Prefix, item6.Prefix}, "").Return(false),
					p.EXPECT().Noticef(pp.EmojiError, "Failed to properly update the list %s; its content may be inconsistent", wafListDescribed),
				)
			},
		},
		"delete-fail": {
			prefixmap{ipnet.IP4: prefix4, ipnet.IP6: prefix6},
			setter.ResponseFailed,
			func(ctx context.Context, _ func(), p *mocks.MockPP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().ListWAFListItems(ctx, p, wafList, listDescription).Return(items{
						item6range1,
						item4wrong2,
						item6range2,
						item6range3,
						item6range4,
						item4range2,
						item4range3,
						item6wrong2,
						item6wrong3,
						item4wrong3,
						item4range1,
						item4wrong1,
						item6wrong1,
					}, true, false, true),
					m.EXPECT().CreateWAFListItems(ctx, p, wafList, listDescription, nil, "").Return(true),
					m.EXPECT().DeleteWAFListItems(ctx, p, wafList, listDescription, gomock.InAnyOrder([]api.ID{
						item4wrong2.ID,
						item6wrong2.ID,
						item6wrong3.ID,
						item4wrong3.ID,
						item4wrong1.ID,
						item6wrong1.ID,
					})).Return(false),
					p.EXPECT().Noticef(pp.EmojiError, "Failed to properly update the list %s; its content may be inconsistent", wafListDescribed),
				)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			mockCtrl := gomock.NewController(t)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			mockPP := mocks.NewMockPP(mockCtrl)
			mockHandle := mocks.NewMockHandle(mockCtrl)
			if tc.prepareMocks != nil {
				tc.prepareMocks(ctx, cancel, mockPP, mockHandle)
			}

			s, ok := setter.New(mockPP, mockHandle)
			require.True(t, ok)

			resp := s.SetWAFList(ctx, mockPP, wafList, listDescription, tc.detectedPrefix, "")
			require.Equal(t, tc.resp, resp)
		})
	}
}

func TestFinalClearWAFListAsync(t *testing.T) {
	t.Parallel()

	const listName = "list"
	const listDescription = "My List"
	wafList := api.WAFList{AccountID: "account", Name: listName}

	for name, tc := range map[string]struct {
		resp         setter.ResponseCode
		prepareMocks func(ctx context.Context, cancel func(), p *mocks.MockPP, m *mocks.MockHandle)
	}{
		"deleted": {
			setter.ResponseUpdated,
			func(ctx context.Context, _ func(), p *mocks.MockPP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().FinalClearWAFListAsync(ctx, p, wafList, listDescription).Return(true, true),
					p.EXPECT().Noticef(pp.EmojiDeletion, "The list %s was deleted", wafList.Describe()),
				)
			},
		},
		"cleared": {
			setter.ResponseUpdating,
			func(ctx context.Context, _ func(), p *mocks.MockPP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().FinalClearWAFListAsync(ctx, p, wafList, listDescription).Return(false, true),
					p.EXPECT().Noticef(pp.EmojiClear, "The list %s is being cleared (asynchronously)", wafList.Describe()),
				)
			},
		},
		"delete-fail/clear-fail": {
			setter.ResponseFailed,
			func(ctx context.Context, _ func(), p *mocks.MockPP, m *mocks.MockHandle) {
				m.EXPECT().FinalClearWAFListAsync(ctx, p, wafList, listDescription).Return(false, false)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			mockCtrl := gomock.NewController(t)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			mockPP := mocks.NewMockPP(mockCtrl)
			mockHandle := mocks.NewMockHandle(mockCtrl)
			if tc.prepareMocks != nil {
				tc.prepareMocks(ctx, cancel, mockPP, mockHandle)
			}

			s, ok := setter.New(mockPP, mockHandle)
			require.True(t, ok)

			resp := s.FinalClearWAFList(ctx, mockPP, wafList, listDescription)
			require.Equal(t, tc.resp, resp)
		})
	}
}
