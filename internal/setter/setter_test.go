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

func wrapCancelAsDelete(cancel func()) func(context.Context, pp.PP, ipnet.Type, domain.Domain, api.ID, bool) bool {
	return func(context.Context, pp.PP, ipnet.Type, domain.Domain, api.ID, bool) bool {
		cancel()
		return false
	}
}

//nolint:funlen
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
		ip1 = netip.MustParseAddr("::1")
		ip2 = netip.MustParseAddr("::2")
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
					h.EXPECT().ListRecords(ctx, p, ipNetwork, domain).Return([]api.Record{}, true, true),
					h.EXPECT().CreateRecord(ctx, p, ipNetwork, domain, ip1, api.TTLAuto, false, "hello").Return(record1, true),
					p.EXPECT().Noticef(pp.EmojiCreation, "Added a new %s record of %q (ID: %s)", "AAAA", "sub.test.org", record1),
				)
			},
		},
		"0/create-fail": {
			ip1,
			setter.ResponseFailed,
			func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, ipNetwork, domain).Return([]api.Record{}, true, true),
					h.EXPECT().CreateRecord(ctx, p, ipNetwork, domain, ip1, api.TTLAuto, false, "hello").Return(record1, false),
					p.EXPECT().Noticef(pp.EmojiError, "Failed to properly update %s records of %q; records might be inconsistent", "AAAA", "sub.test.org"), //nolint:lll
				)
			},
		},
		"1unmatched": {
			ip1,
			setter.ResponseUpdated,
			func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, ipNetwork, domain).Return([]api.Record{{ID: record1, IP: ip2}}, true, true),
					h.EXPECT().UpdateRecord(ctx, p, ipNetwork, domain, record1, ip1).Return(true),
					p.EXPECT().Noticef(pp.EmojiUpdate,
						"Updated a stale %s record of %q (ID: %s)",
						"AAAA",
						"sub.test.org",
						record1,
					),
				)
			},
		},
		"1unmatched/update-fail": {
			ip1,
			setter.ResponseFailed,
			func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, ipNetwork, domain).Return([]api.Record{{ID: record1, IP: ip2}}, true, true),
					h.EXPECT().UpdateRecord(ctx, p, ipNetwork, domain, record1, ip1).Return(false),
					p.EXPECT().Noticef(pp.EmojiError, "Failed to properly update %s records of %q; records might be inconsistent", "AAAA", "sub.test.org"), //nolint:lll
				)
			},
		},
		"1matched": {
			ip1,
			setter.ResponseNoop,
			func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, ipNetwork, domain).Return([]api.Record{{ID: record1, IP: ip1}}, true, true),
					p.EXPECT().Infof(pp.EmojiAlreadyDone,
						"The %s records of %q are already up to date (cached)", "AAAA", "sub.test.org"),
				)
			},
		},
		"1matched/not-cached": {
			ip1,
			setter.ResponseNoop,
			func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, ipNetwork, domain).Return([]api.Record{{ID: record1, IP: ip1}}, false, true),
					p.EXPECT().Infof(pp.EmojiAlreadyDone, "The %s records of %q are already up to date", "AAAA", "sub.test.org"),
				)
			},
		},
		"3matched": {
			ip1,
			setter.ResponseUpdated,
			func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, ipNetwork, domain).
						Return([]api.Record{{ID: record1, IP: ip1}, {ID: record2, IP: ip1}, {ID: record3, IP: ip1}}, true, true),
					h.EXPECT().DeleteRecord(ctx, p, ipNetwork, domain, record2, false).Return(true),
					p.EXPECT().Noticef(
						pp.EmojiDeletion,
						"Deleted a duplicate %s record of %q (ID: %s)",
						"AAAA",
						"sub.test.org",
						record2,
					),
					h.EXPECT().DeleteRecord(ctx, p, ipNetwork, domain, record3, false).Return(true),
					p.EXPECT().Noticef(
						pp.EmojiDeletion,
						"Deleted a duplicate %s record of %q (ID: %s)",
						"AAAA",
						"sub.test.org",
						record3,
					),
				)
			},
		},
		"3matched/delete-fail/1": {
			ip1,
			setter.ResponseUpdated,
			func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, ipNetwork, domain).
						Return([]api.Record{{ID: record1, IP: ip1}, {ID: record2, IP: ip1}, {ID: record3, IP: ip1}}, true, true),
					h.EXPECT().DeleteRecord(ctx, p, ipNetwork, domain, record2, false).Return(false),
					h.EXPECT().DeleteRecord(ctx, p, ipNetwork, domain, record3, false).Return(true),
					p.EXPECT().Noticef(
						pp.EmojiDeletion,
						"Deleted a duplicate %s record of %q (ID: %s)",
						"AAAA",
						"sub.test.org",
						record3,
					),
				)
			},
		},
		"3matched/delete-fail/2": {
			ip1,
			setter.ResponseUpdated,
			func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, ipNetwork, domain).
						Return([]api.Record{{ID: record1, IP: ip1}, {ID: record2, IP: ip1}, {ID: record3, IP: ip1}}, true, true),
					h.EXPECT().DeleteRecord(ctx, p, ipNetwork, domain, record2, false).Return(true),
					p.EXPECT().Noticef(
						pp.EmojiDeletion,
						"Deleted a duplicate %s record of %q (ID: %s)",
						"AAAA",
						"sub.test.org",
						record2,
					),
					h.EXPECT().DeleteRecord(ctx, p, ipNetwork, domain, record3, false).Return(false),
				)
			},
		},
		"3matched/delete-timeout": {
			ip1,
			setter.ResponseUpdated,
			func(ctx context.Context, cancel func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, ipNetwork, domain).
						Return([]api.Record{{ID: record1, IP: ip1}, {ID: record2, IP: ip1}}, true, true),
					h.EXPECT().DeleteRecord(ctx, p, ipNetwork, domain, record2, false).
						Do(wrapCancelAsDelete(cancel)).Return(false),
				)
			},
		},
		"2unmatched": {
			ip1,
			setter.ResponseUpdated,
			func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, ipNetwork, domain).
						Return([]api.Record{{ID: record1, IP: ip2}, {ID: record2, IP: ip2}}, true, true),
					h.EXPECT().UpdateRecord(ctx, p, ipNetwork, domain, record1, ip1).Return(true),
					p.EXPECT().Noticef(
						pp.EmojiUpdate,
						"Updated a stale %s record of %q (ID: %s)",
						"AAAA",
						"sub.test.org",
						record1,
					),
					h.EXPECT().DeleteRecord(ctx, p, ipNetwork, domain, record2, false).Return(true),
					p.EXPECT().Noticef(pp.EmojiDeletion,
						"Deleted a stale %s record of %q (ID: %s)",
						"AAAA",
						"sub.test.org",
						record2),
				)
			},
		},
		"2unmatched/delete-timeout": {
			ip1,
			setter.ResponseFailed,
			func(ctx context.Context, cancel func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, ipNetwork, domain).
						Return([]api.Record{{ID: record1, IP: ip2}, {ID: record2, IP: ip2}}, true, true),
					h.EXPECT().UpdateRecord(ctx, p, ipNetwork, domain, record1, ip1).Return(true),
					p.EXPECT().Noticef(
						pp.EmojiUpdate,
						"Updated a stale %s record of %q (ID: %s)",
						"AAAA",
						"sub.test.org",
						record1,
					),
					h.EXPECT().DeleteRecord(ctx, p, ipNetwork, domain, record2, false).
						Do(wrapCancelAsDelete(cancel)).Return(false),
					p.EXPECT().Noticef(pp.EmojiError, "Failed to properly update %s records of %q; records might be inconsistent", "AAAA", "sub.test.org"), //nolint:lll
				)
			},
		},
		"2unmatched/update-fail": {
			ip1,
			setter.ResponseFailed,
			func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, ipNetwork, domain).
						Return([]api.Record{{ID: record1, IP: ip2}, {ID: record2, IP: ip2}}, true, true),
					h.EXPECT().UpdateRecord(ctx, p, ipNetwork, domain, record1, ip1).Return(false),
					p.EXPECT().Noticef(pp.EmojiError, "Failed to properly update %s records of %q; records might be inconsistent", "AAAA", "sub.test.org"), //nolint:lll
				)
			},
		},
		"list-fail": {
			ip1,
			setter.ResponseFailed,
			func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				h.EXPECT().ListRecords(ctx, p, ipNetwork, domain).Return(nil, false, false)
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

			resp := s.Set(ctx, mockPP, ipNetwork, domain, tc.ip, api.TTLAuto, false, "hello")
			require.Equal(t, tc.resp, resp)
		})
	}
}

//nolint:funlen
func TestDelete(t *testing.T) {
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
	)

	for name, tc := range map[string]struct {
		resp         setter.ResponseCode
		prepareMocks func(ctx context.Context, cancel func(), p *mocks.MockPP, m *mocks.MockHandle)
	}{
		"0": {
			setter.ResponseNoop,
			func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, ipNetwork, domain).Return([]api.Record{}, true, true),
					p.EXPECT().Infof(pp.EmojiAlreadyDone, "The %s records of %q were already deleted (cached)", "AAAA", "sub.test.org"), //nolint:lll
				)
			},
		},
		"0/not-cached": {
			setter.ResponseNoop,
			func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, ipNetwork, domain).Return([]api.Record{}, false, true),
					p.EXPECT().Infof(pp.EmojiAlreadyDone, "The %s records of %q were already deleted", "AAAA", "sub.test.org"),
				)
			},
		},
		"1unmatched": {
			setter.ResponseUpdated,
			func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, ipNetwork, domain).Return([]api.Record{{ID: record1, IP: ip1}}, true, true),
					h.EXPECT().DeleteRecord(ctx, p, ipNetwork, domain, record1, true).Return(true),
					p.EXPECT().Noticef(pp.EmojiDeletion, "Deleted a stale %s record of %q (ID: %s)", "AAAA", "sub.test.org", record1), //nolint:lll
				)
			},
		},
		"1unmatched/delete-fail": {
			setter.ResponseFailed,
			func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, ipNetwork, domain).Return([]api.Record{{ID: record1, IP: ip1}}, true, true),
					h.EXPECT().DeleteRecord(ctx, p, ipNetwork, domain, record1, true).Return(false),
					p.EXPECT().Noticef(pp.EmojiError, "Failed to properly delete %s records of %q; records might be inconsistent", "AAAA", "sub.test.org"), //nolint:lll
				)
			},
		},
		"1unmatched/delete-timeout": {
			setter.ResponseFailed,
			func(ctx context.Context, cancel func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, ipNetwork, domain).
						Return([]api.Record{{ID: record1, IP: ip1}}, true, true),
					h.EXPECT().DeleteRecord(ctx, p, ipNetwork, domain, record1, true).
						Do(wrapCancelAsDelete(cancel)).Return(false),
					p.EXPECT().Infof(pp.EmojiTimeout,
						"Deletion of %s records of %q aborted by timeout or signals; records might be inconsistent",
						"AAAA", "sub.test.org"),
				)
			},
		},
		"impossible-records": {
			setter.ResponseUpdated,
			func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, ipNetwork, domain).Return([]api.Record{{ID: record1, IP: ip1}, {ID: record2, IP: invalidIP}}, true, true), //nolint:lll
					h.EXPECT().DeleteRecord(ctx, p, ipNetwork, domain, record1, true).Return(true),
					p.EXPECT().Noticef(pp.EmojiDeletion, "Deleted a stale %s record of %q (ID: %s)", "AAAA", "sub.test.org", record1), //nolint:lll
					h.EXPECT().DeleteRecord(ctx, p, ipNetwork, domain, record2, true).Return(true),
					p.EXPECT().Noticef(pp.EmojiDeletion, "Deleted a stale %s record of %q (ID: %s)", "AAAA", "sub.test.org", record2), //nolint:lll
				)
			},
		},
		"listfail": {
			setter.ResponseFailed,
			func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				h.EXPECT().ListRecords(ctx, p, ipNetwork, domain).Return(nil, false, false)
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

			resp := s.Delete(ctx, mockPP, ipNetwork, domain)
			require.Equal(t, tc.resp, resp)
		})
	}
}

func TestSetWAFList(t *testing.T) {
	t.Parallel()

	const listName = "list"
	const listID = api.ID("list1")
	const listDescription = "My List"
	wafList := api.WAFList{AccountID: "account", ListName: listName}

	var (
		ip4           = netip.MustParseAddr("10.0.0.1")
		ip6           = netip.MustParseAddr("2001:db8::1111")
		prefix4       = api.WAFListItem{Prefix: netip.MustParsePrefix("10.0.0.1/32"), ID: "pre4"}
		prefix6       = api.WAFListItem{Prefix: netip.MustParsePrefix("2001:0db8::/64"), ID: "pre6"}
		prefix4range1 = api.WAFListItem{Prefix: netip.MustParsePrefix("10.0.0.0/16"), ID: "ip4-16"}
		prefix4range2 = api.WAFListItem{Prefix: netip.MustParsePrefix("10.0.0.0/20"), ID: "ip4-20"}
		prefix4range3 = api.WAFListItem{Prefix: netip.MustParsePrefix("10.0.0.0/24"), ID: "ip4-24"}
		prefix4wrong1 = api.WAFListItem{Prefix: netip.MustParsePrefix("20.0.0.0/16"), ID: "ip4-16"}
		prefix4wrong2 = api.WAFListItem{Prefix: netip.MustParsePrefix("20.0.0.0/20"), ID: "ip4-20"}
		prefix4wrong3 = api.WAFListItem{Prefix: netip.MustParsePrefix("20.0.0.0/24"), ID: "ip4-24"}
		prefix6range1 = api.WAFListItem{Prefix: netip.MustParsePrefix("2001:db8::/32"), ID: "ip6-32"}
		prefix6range2 = api.WAFListItem{Prefix: netip.MustParsePrefix("2001:db8::/40"), ID: "ip6-40"}
		prefix6range3 = api.WAFListItem{Prefix: netip.MustParsePrefix("2001:db8::/48"), ID: "ip6-48"}
		prefix6wrong1 = api.WAFListItem{Prefix: netip.MustParsePrefix("4001:db8::/32"), ID: "ip6-32"}
		prefix6wrong2 = api.WAFListItem{Prefix: netip.MustParsePrefix("4001:db8::/40"), ID: "ip6-40"}
		prefix6wrong3 = api.WAFListItem{Prefix: netip.MustParsePrefix("4001:db8::/48"), ID: "ip6-48"}
	)

	type items = []api.WAFListItem
	type ipmap = map[ipnet.Type]netip.Addr

	for name, tc := range map[string]struct {
		detected     ipmap
		resp         setter.ResponseCode
		prepareMocks func(ctx context.Context, cancel func(), p *mocks.MockPP, m *mocks.MockHandle)
	}{
		"ensure-fail": {
			ipmap{ipnet.IP4: ip4, ipnet.IP6: ip6},
			setter.ResponseFailed,
			func(ctx context.Context, _ func(), p *mocks.MockPP, m *mocks.MockHandle) {
				m.EXPECT().EnsureWAFList(ctx, p, wafList, listDescription).Return(listID, false, false)
			},
		},
		"created": {
			ipmap{ipnet.IP4: ip4, ipnet.IP6: ip6},
			setter.ResponseUpdated,
			func(ctx context.Context, _ func(), p *mocks.MockPP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().EnsureWAFList(ctx, p, wafList, listDescription).Return(listID, false, true),
					p.EXPECT().Noticef(pp.EmojiCreation, "Created a new list named %q (ID: %s)", listName, listID),
					m.EXPECT().ListWAFListItems(ctx, p, wafList).Return(items{}, false, true),
					m.EXPECT().CreateWAFListItems(ctx, p,
						wafList, []netip.Prefix{prefix4.Prefix, prefix6.Prefix}, "").Return(true),
					p.EXPECT().Noticef(pp.EmojiCreation,
						"Added %s to the list %q (ID: %s)", "10.0.0.1", listName, listID),
					p.EXPECT().Noticef(pp.EmojiCreation,
						"Added %s to the list %q (ID: %s)", "2001:db8::/64", listName, listID),
					m.EXPECT().DeleteWAFListItems(ctx, p, wafList, []api.ID{}).Return(true),
				)
			},
		},
		"list-fail": {
			ipmap{ipnet.IP4: ip4, ipnet.IP6: ip6},
			setter.ResponseFailed,
			func(ctx context.Context, _ func(), p *mocks.MockPP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().EnsureWAFList(ctx, p, wafList, listDescription).Return(listID, true, true),
					m.EXPECT().ListWAFListItems(ctx, p, wafList).Return(nil, false, false),
				)
			},
		},
		"skip-unknown": {
			ipmap{ipnet.IP4: netip.Addr{}, ipnet.IP6: ip6},
			setter.ResponseNoop,
			func(ctx context.Context, _ func(), p *mocks.MockPP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().EnsureWAFList(ctx, p, wafList, listDescription).Return(listID, true, true),
					m.EXPECT().ListWAFListItems(ctx, p, wafList).
						Return(items{
							prefix4wrong2,
							prefix6range1,
							prefix4wrong1,
							prefix4wrong3,
						}, true, true),
					p.EXPECT().Infof(pp.EmojiAlreadyDone,
						"The list %q (ID: %s) is already up to date (cached)", listName, listID),
				)
			},
		},
		"noop": {
			ipmap{ipnet.IP4: ip4, ipnet.IP6: ip6},
			setter.ResponseNoop,
			func(ctx context.Context, _ func(), p *mocks.MockPP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().EnsureWAFList(ctx, p, wafList, listDescription).Return(listID, true, true),
					m.EXPECT().ListWAFListItems(ctx, p, wafList).Return(items{prefix4, prefix6}, false, true),
					p.EXPECT().Infof(pp.EmojiAlreadyDone,
						"The list %q (ID: %s) is already up to date", listName, listID),
				)
			},
		},
		"noop/cached": {
			ipmap{ipnet.IP4: ip4, ipnet.IP6: ip6},
			setter.ResponseNoop,
			func(ctx context.Context, _ func(), p *mocks.MockPP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().EnsureWAFList(ctx, p, wafList, listDescription).Return(listID, true, true),
					m.EXPECT().ListWAFListItems(ctx, p, wafList).Return(items{prefix4, prefix6}, true, true),
					p.EXPECT().Infof(pp.EmojiAlreadyDone,
						"The list %q (ID: %s) is already up to date (cached)", listName, listID),
				)
			},
		},
		"test1": {
			ipmap{ipnet.IP4: ip4, ipnet.IP6: ip6},
			setter.ResponseUpdated,
			func(ctx context.Context, _ func(), p *mocks.MockPP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().EnsureWAFList(ctx, p, wafList, listDescription).Return(listID, true, true),
					m.EXPECT().ListWAFListItems(ctx, p, wafList).
						Return(items{
							prefix6range1,
							prefix4wrong2,
							prefix6range2,
							prefix6range3,
							prefix4range2,
							prefix4range3,
							prefix6wrong2,
							prefix6wrong3,
							prefix4wrong3,
							prefix4range1,
							prefix4wrong1,
							prefix6wrong1,
						}, false, true),
					m.EXPECT().CreateWAFListItems(ctx, p, wafList, nil, "").Return(true),
					m.EXPECT().DeleteWAFListItems(ctx, p, wafList,
						gomock.InAnyOrder([]api.ID{
							prefix4wrong2.ID,
							prefix6wrong2.ID,
							prefix6wrong3.ID,
							prefix4wrong3.ID,
							prefix4wrong1.ID,
							prefix6wrong1.ID,
						})).Return(true),
					p.EXPECT().Noticef(pp.EmojiDeletion,
						"Deleted %s from the list %q (ID: %s)", "20.0.0.0/20", listName, listID),
					p.EXPECT().Noticef(pp.EmojiDeletion,
						"Deleted %s from the list %q (ID: %s)", "20.0.0.0/24", listName, listID),
					p.EXPECT().Noticef(pp.EmojiDeletion,
						"Deleted %s from the list %q (ID: %s)", "20.0.0.0/16", listName, listID),
					p.EXPECT().Noticef(pp.EmojiDeletion,
						"Deleted %s from the list %q (ID: %s)", "4001:db8::/40", listName, listID),
					p.EXPECT().Noticef(pp.EmojiDeletion,
						"Deleted %s from the list %q (ID: %s)", "4001:db8::/48", listName, listID),
					p.EXPECT().Noticef(pp.EmojiDeletion,
						"Deleted %s from the list %q (ID: %s)", "4001:db8::/32", listName, listID),
				)
			},
		},
		"test2": {
			ipmap{ipnet.IP4: ip4, ipnet.IP6: ip6},
			setter.ResponseUpdated,
			func(ctx context.Context, _ func(), p *mocks.MockPP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().EnsureWAFList(ctx, p, wafList, listDescription).Return(listID, true, true),
					m.EXPECT().ListWAFListItems(ctx, p, wafList).
						Return(items{
							prefix4wrong2,
							prefix6wrong2,
							prefix6wrong3,
							prefix4wrong3,
							prefix4wrong1,
							prefix6wrong1,
						}, false, true),
					m.EXPECT().CreateWAFListItems(ctx, p, wafList,
						[]netip.Prefix{prefix4.Prefix, prefix6.Prefix}, "").Return(true),
					p.EXPECT().Noticef(pp.EmojiCreation,
						"Added %s to the list %q (ID: %s)", "10.0.0.1", listName, listID),
					p.EXPECT().Noticef(pp.EmojiCreation,
						"Added %s to the list %q (ID: %s)", "2001:db8::/64", listName, listID),
					m.EXPECT().DeleteWAFListItems(ctx, p, wafList,
						gomock.InAnyOrder([]api.ID{
							prefix4wrong2.ID,
							prefix6wrong2.ID,
							prefix6wrong3.ID,
							prefix4wrong3.ID,
							prefix4wrong1.ID,
							prefix6wrong1.ID,
						})).Return(true),
					p.EXPECT().Noticef(pp.EmojiDeletion,
						"Deleted %s from the list %q (ID: %s)", "20.0.0.0/20", listName, listID),
					p.EXPECT().Noticef(pp.EmojiDeletion,
						"Deleted %s from the list %q (ID: %s)", "20.0.0.0/24", listName, listID),
					p.EXPECT().Noticef(pp.EmojiDeletion,
						"Deleted %s from the list %q (ID: %s)", "20.0.0.0/16", listName, listID),
					p.EXPECT().Noticef(pp.EmojiDeletion,
						"Deleted %s from the list %q (ID: %s)", "4001:db8::/40", listName, listID),
					p.EXPECT().Noticef(pp.EmojiDeletion,
						"Deleted %s from the list %q (ID: %s)", "4001:db8::/48", listName, listID),
					p.EXPECT().Noticef(pp.EmojiDeletion,
						"Deleted %s from the list %q (ID: %s)", "4001:db8::/32", listName, listID),
				)
			},
		},
		"create-fail": {
			ipmap{ipnet.IP4: ip4, ipnet.IP6: ip6},
			setter.ResponseFailed,
			func(ctx context.Context, _ func(), p *mocks.MockPP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().EnsureWAFList(ctx, p, wafList, listDescription).Return(listID, true, true),
					m.EXPECT().ListWAFListItems(ctx, p, wafList).Return(items{}, false, true),
					m.EXPECT().CreateWAFListItems(ctx, p, wafList,
						[]netip.Prefix{prefix4.Prefix, prefix6.Prefix}, "").Return(false),
					p.EXPECT().Noticef(pp.EmojiError,
						"Failed to properly update the list %q (ID: %s); its content may be inconsistent", listName, listID),
				)
			},
		},
		"delete-fail": {
			ipmap{ipnet.IP4: ip4, ipnet.IP6: ip6},
			setter.ResponseFailed,
			func(ctx context.Context, _ func(), p *mocks.MockPP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().EnsureWAFList(ctx, p, wafList, listDescription).Return(listID, true, true),
					m.EXPECT().ListWAFListItems(ctx, p, wafList).
						Return(items{
							prefix6range1,
							prefix4wrong2,
							prefix6range2,
							prefix6range3,
							prefix4range2,
							prefix4range3,
							prefix6wrong2,
							prefix6wrong3,
							prefix4wrong3,
							prefix4range1,
							prefix4wrong1,
							prefix6wrong1,
						}, false, true),
					m.EXPECT().CreateWAFListItems(ctx, p, wafList, nil, "").Return(true),
					m.EXPECT().DeleteWAFListItems(ctx, p, wafList,
						gomock.InAnyOrder([]api.ID{
							prefix4wrong2.ID,
							prefix6wrong2.ID,
							prefix6wrong3.ID,
							prefix4wrong3.ID,
							prefix4wrong1.ID,
							prefix6wrong1.ID,
						})).Return(false),
					p.EXPECT().Noticef(pp.EmojiError,
						"Failed to properly update the list %q (ID: %s); its content may be inconsistent", listName, listID),
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

			resp := s.SetWAFList(ctx, mockPP, wafList, listDescription, tc.detected, "")
			require.Equal(t, tc.resp, resp)
		})
	}
}

func TestClearWAFList(t *testing.T) {
	t.Parallel()

	const listName = "list"
	wafList := api.WAFList{AccountID: "account", ListName: listName}

	for name, tc := range map[string]struct {
		resp         setter.ResponseCode
		prepareMocks func(ctx context.Context, cancel func(), p *mocks.MockPP, m *mocks.MockHandle)
	}{
		"deleted": {
			setter.ResponseUpdated,
			func(ctx context.Context, _ func(), p *mocks.MockPP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().ClearWAFListAsync(ctx, p, wafList, true).Return(true, true),
					p.EXPECT().Noticef(pp.EmojiDeletion, "The list %q was deleted", listName),
				)
			},
		},
		"cleared": {
			setter.ResponseUpdating,
			func(ctx context.Context, _ func(), p *mocks.MockPP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().ClearWAFListAsync(ctx, p, wafList, true).Return(false, true),
					p.EXPECT().Noticef(pp.EmojiClear, "The list %q is being cleared (asynchronously)", listName),
				)
			},
		},
		"delete-fail/clear-fail": {
			setter.ResponseFailed,
			func(ctx context.Context, _ func(), p *mocks.MockPP, m *mocks.MockHandle) {
				m.EXPECT().ClearWAFListAsync(ctx, p, wafList, true).Return(false, false)
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

			resp := s.ClearWAFList(ctx, mockPP, wafList)
			require.Equal(t, tc.resp, resp)
		})
	}
}
