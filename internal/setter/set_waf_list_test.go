package setter_test

// vim: nowrap

import (
	"context"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/setter"
)

func TestSetWAFList(t *testing.T) {
	t.Parallel()

	const listName = "list"
	const listDescription = "My List"
	wafList := api.WAFList{AccountID: "account", Name: listName}
	wafListDescribed := "account/list"

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

	cases := []struct {
		name         string
		detected     ipmap
		resp         setter.ResponseCode
		prepareMocks func(ctx context.Context, cancel func(), p *mocks.MockPP, m *mocks.MockHandle)
	}{
		{
			name:     "created",
			detected: ipmap{ipnet.IP4: ip4, ipnet.IP6: ip6},
			resp:     setter.ResponseUpdated,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().ListWAFListItems(ctx, p, wafList, listDescription).Return(items{}, false, false, true),
					p.EXPECT().Noticef(pp.EmojiCreation, "Created a new list %s", wafListDescribed),
					m.EXPECT().CreateWAFListItems(ctx, p, wafList, listDescription, []netip.Prefix{prefix4.Prefix, prefix6.Prefix}, "").Return(true),
					p.EXPECT().Noticef(pp.EmojiCreation, "Added %s to the list %s", "10.0.0.1", wafListDescribed),
					p.EXPECT().Noticef(pp.EmojiCreation, "Added %s to the list %s", "2001:db8::/64", wafListDescribed),
					m.EXPECT().DeleteWAFListItems(ctx, p, wafList, listDescription, []api.ID{}).Return(true),
				)
			},
		},
		{
			name:     "list-fail",
			detected: ipmap{ipnet.IP4: ip4, ipnet.IP6: ip6},
			resp:     setter.ResponseFailed,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, m *mocks.MockHandle) {
				m.EXPECT().ListWAFListItems(ctx, p, wafList, listDescription).Return(nil, false, false, false)
			},
		},
		{
			name:     "skip-unknown",
			detected: ipmap{ipnet.IP4: netip.Addr{}, ipnet.IP6: ip6},
			resp:     setter.ResponseNoop,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().ListWAFListItems(ctx, p, wafList, listDescription).Return(items{
						prefix4wrong2,
						prefix6range1,
						prefix4wrong1,
						prefix4wrong3,
					}, true, true, true),
					p.EXPECT().Infof(pp.EmojiAlreadyDone, "The list %s is already up to date (cached)", wafListDescribed),
				)
			},
		},
		{
			name:     "noop",
			detected: ipmap{ipnet.IP4: ip4, ipnet.IP6: ip6},
			resp:     setter.ResponseNoop,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().ListWAFListItems(ctx, p, wafList, listDescription).Return(items{prefix4, prefix6}, true, false, true),
					p.EXPECT().Infof(pp.EmojiAlreadyDone, "The list %s is already up to date", wafListDescribed),
				)
			},
		},
		{
			name:     "noop/cached",
			detected: ipmap{ipnet.IP4: ip4, ipnet.IP6: ip6},
			resp:     setter.ResponseNoop,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().ListWAFListItems(ctx, p, wafList, listDescription).Return(items{prefix4, prefix6}, true, true, true),
					p.EXPECT().Infof(pp.EmojiAlreadyDone, "The list %s is already up to date (cached)", wafListDescribed),
				)
			},
		},
		{
			name:     "delete-only-wrong-prefixes",
			detected: ipmap{ipnet.IP4: ip4, ipnet.IP6: ip6},
			resp:     setter.ResponseUpdated,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().ListWAFListItems(ctx, p, wafList, listDescription).Return(items{
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
					}, true, false, true),
					m.EXPECT().CreateWAFListItems(ctx, p, wafList, listDescription, nil, "").Return(true),
					m.EXPECT().DeleteWAFListItems(ctx, p, wafList, listDescription, gomock.InAnyOrder([]api.ID{
						prefix4wrong2.ID,
						prefix6wrong2.ID,
						prefix6wrong3.ID,
						prefix4wrong3.ID,
						prefix4wrong1.ID,
						prefix6wrong1.ID,
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
		{
			name:     "create-and-delete-prefixes",
			detected: ipmap{ipnet.IP4: ip4, ipnet.IP6: ip6},
			resp:     setter.ResponseUpdated,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().ListWAFListItems(ctx, p, wafList, listDescription).Return(items{
						prefix4wrong2,
						prefix6wrong2,
						prefix6wrong3,
						prefix4wrong3,
						prefix4wrong1,
						prefix6wrong1,
					}, true, false, true),
					m.EXPECT().CreateWAFListItems(ctx, p, wafList, listDescription, []netip.Prefix{prefix4.Prefix, prefix6.Prefix}, "").Return(true),
					p.EXPECT().Noticef(pp.EmojiCreation, "Added %s to the list %s", "10.0.0.1", wafListDescribed),
					p.EXPECT().Noticef(pp.EmojiCreation, "Added %s to the list %s", "2001:db8::/64", wafListDescribed),
					m.EXPECT().DeleteWAFListItems(ctx, p, wafList, listDescription, gomock.InAnyOrder([]api.ID{
						prefix4wrong2.ID,
						prefix6wrong2.ID,
						prefix6wrong3.ID,
						prefix4wrong3.ID,
						prefix4wrong1.ID,
						prefix6wrong1.ID,
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
		{
			name:     "create-fail",
			detected: ipmap{ipnet.IP4: ip4, ipnet.IP6: ip6},
			resp:     setter.ResponseFailed,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().ListWAFListItems(ctx, p, wafList, listDescription).Return(items{}, true, false, true),
					m.EXPECT().CreateWAFListItems(ctx, p, wafList, listDescription, []netip.Prefix{prefix4.Prefix, prefix6.Prefix}, "").Return(false),
					p.EXPECT().Noticef(pp.EmojiError, "Failed to properly update the list %s; its content may be inconsistent", wafListDescribed),
				)
			},
		},
		{
			name:     "delete-fail",
			detected: ipmap{ipnet.IP4: ip4, ipnet.IP6: ip6},
			resp:     setter.ResponseFailed,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().ListWAFListItems(ctx, p, wafList, listDescription).Return(items{
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
					}, true, false, true),
					m.EXPECT().CreateWAFListItems(ctx, p, wafList, listDescription, nil, "").Return(true),
					m.EXPECT().DeleteWAFListItems(ctx, p, wafList, listDescription, gomock.InAnyOrder([]api.ID{
						prefix4wrong2.ID,
						prefix6wrong2.ID,
						prefix6wrong3.ID,
						prefix4wrong3.ID,
						prefix4wrong1.ID,
						prefix6wrong1.ID,
					})).Return(false),
					p.EXPECT().Noticef(pp.EmojiError, "Failed to properly update the list %s; its content may be inconsistent", wafListDescribed),
				)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			h := newSetterHarness(t)
			if tc.prepareMocks != nil {
				tc.prepareMocks(h.ctx, h.cancel, h.mockPP, h.mockHandle)
			}

			resp := h.setter.SetWAFList(h.ctx, h.mockPP, wafList, listDescription, tc.detected, "")
			require.Equal(t, tc.resp, resp)
		})
	}
}
