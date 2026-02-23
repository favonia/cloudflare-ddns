package setter_test

// vim: nowrap

import (
	"context"
	"net/netip"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/setter"
)

func TestSetWAFList(t *testing.T) {
	t.Parallel()

	const listName = "list"
	const listDescription = "My List"
	wafList := api.WAFList{AccountID: "account", Name: listName}

	var (
		ip4  = netip.MustParseAddr("10.0.0.1")
		ip4b = netip.MustParseAddr("10.0.1.2")
		ip6  = netip.MustParseAddr("2001:db8::1111")
		ip6b = netip.MustParseAddr("2001:db9:1::2222")

		prefix4 = wafItem("10.0.0.1/32", "pre4")
		prefix6 = wafItem("2001:0db8::/64", "pre6")

		prefix4range1 = wafItem("10.0.0.0/16", "ip4-16")
		prefix4range2 = wafItem("10.0.0.0/20", "ip4-20")
		prefix4range3 = wafItem("10.0.0.0/24", "ip4-24")
		prefix4wrong1 = wafItem("20.0.0.0/16", "ip4-16")
		prefix4wrong2 = wafItem("20.0.0.0/20", "ip4-20")
		prefix4wrong3 = wafItem("20.0.0.0/24", "ip4-24")

		prefix6range1  = wafItem("2001:db8::/32", "ip6-32")
		prefix6range2  = wafItem("2001:db8::/40", "ip6-40")
		prefix6range3  = wafItem("2001:db8::/48", "ip6-48")
		prefix6target2 = wafItem("2001:db9:1::/64", "ip6-target2")
		prefix6wrong1  = wafItem("4001:db8::/32", "ip6-32")
		prefix6wrong2  = wafItem("4001:db8::/40", "ip6-40")
		prefix6wrong3  = wafItem("4001:db8::/48", "ip6-48")
	)

	type items = []api.WAFListItem
	type ipmap = map[ipnet.Type][]netip.Addr

	targetPrefixes := []netip.Prefix{prefix4.Prefix, prefix6.Prefix}

	skipUnknownItems := items{
		prefix4wrong2,
		prefix6range1,
		prefix4wrong1,
		prefix4wrong3,
	}
	mixedItems := items{
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
	}
	wrongItems := items{
		prefix4wrong2,
		prefix4wrong3,
		prefix4wrong1,
		prefix6wrong2,
		prefix6wrong3,
		prefix6wrong1,
	}
	sortedWrongItems := items{
		prefix4wrong1,
		prefix4wrong2,
		prefix4wrong3,
		prefix6wrong1,
		prefix6wrong2,
		prefix6wrong3,
	}

	cases := []struct {
		name         string
		detected     ipmap
		resp         setter.ResponseCode
		prepareMocks prepareSetterMocks
	}{
		{
			name:     "list-missing/create-list-and-prefixes/response-updated",
			detected: detected(ip4, ip6),
			resp:     setter.ResponseUpdated,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, m *mocks.MockHandle) {
				expectWAFListMutation(ctx, p, m, wafList, wafListMutationExpectation{
					listDescription: listDescription,
					items:           items{},
					alreadyExisting: false,
					cached:          false,
					createPrefixes:  targetPrefixes,
					createOK:        true,
					deleteItems:     items{},
					deleteOK:        true,
				})
			},
		},
		{
			name:     "list-state-unknown/list-items/response-failed",
			detected: detected(ip4, ip6),
			resp:     setter.ResponseFailed,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, m *mocks.MockHandle) {
				expectWAFListRead(ctx, p, m, wafList, listDescription, nil, false, false, false)
			},
		},
		{
			name:     "ipv4-detection-failed/skip-ipv4-updates/response-noop",
			detected: detected(netip.Addr{}, ip6),
			resp:     setter.ResponseNoop,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, m *mocks.MockHandle) {
				expectWAFListNoop(ctx, p, m, wafList, listDescription, skipUnknownItems, true, true)
			},
		},
		{
			name:     "list-already-up-to-date/report-noop/response-noop",
			detected: detected(ip4, ip6),
			resp:     setter.ResponseNoop,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, m *mocks.MockHandle) {
				expectWAFListNoop(ctx, p, m, wafList, listDescription, items{prefix4, prefix6}, true, false)
			},
		},
		{
			name:     "list-already-up-to-date-cached/report-noop/response-noop",
			detected: detected(ip4, ip6),
			resp:     setter.ResponseNoop,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, m *mocks.MockHandle) {
				expectWAFListNoop(ctx, p, m, wafList, listDescription, items{prefix4, prefix6}, true, true)
			},
		},
		{
			name:     "mixed-covered-and-wrong-prefixes/delete-wrong-prefixes/response-updated",
			detected: detected(ip4, ip6),
			resp:     setter.ResponseUpdated,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, m *mocks.MockHandle) {
				expectWAFListMutation(ctx, p, m, wafList, wafListMutationExpectation{
					listDescription: listDescription,
					items:           mixedItems,
					alreadyExisting: true,
					cached:          false,
					createPrefixes:  nil,
					createOK:        true,
					deleteItems:     sortedWrongItems,
					deleteOK:        true,
				})
			},
		},
		{
			name:     "wrong-prefixes-only/create-and-delete-prefixes/response-updated",
			detected: detected(ip4, ip6),
			resp:     setter.ResponseUpdated,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, m *mocks.MockHandle) {
				expectWAFListMutation(ctx, p, m, wafList, wafListMutationExpectation{
					listDescription: listDescription,
					items:           wrongItems,
					alreadyExisting: true,
					cached:          false,
					createPrefixes:  targetPrefixes,
					createOK:        true,
					deleteItems:     sortedWrongItems,
					deleteOK:        true,
				})
			},
		},
		{
			name:     "empty-list/create-prefixes/response-failed",
			detected: detected(ip4, ip6),
			resp:     setter.ResponseFailed,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, m *mocks.MockHandle) {
				expectWAFListMutation(ctx, p, m, wafList, wafListMutationExpectation{
					listDescription: listDescription,
					items:           items{},
					alreadyExisting: true,
					cached:          false,
					createPrefixes:  targetPrefixes,
					createOK:        false,
					deleteItems:     nil,
					deleteOK:        false,
				})
			},
		},
		{
			name: "multi-target/keep-covering-prefixes-fill-uncovered-and-delete-unmatched/response-updated",
			detected: detectedSets(
				[]netip.Addr{ip4, ip4b},
				[]netip.Addr{ip6, ip6b},
			),
			resp: setter.ResponseUpdated,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, m *mocks.MockHandle) {
				expectWAFListMutation(ctx, p, m, wafList, wafListMutationExpectation{
					listDescription: listDescription,
					items: items{
						prefix6range1,
						prefix4range2,
						prefix4wrong2,
						prefix6wrong1,
					},
					alreadyExisting: true,
					cached:          false,
					createPrefixes:  []netip.Prefix{prefix6target2.Prefix},
					createOK:        true,
					deleteItems:     []api.WAFListItem{prefix4wrong2, prefix6wrong1},
					deleteOK:        true,
				})
			},
		},
		{
			name:     "mixed-covered-and-wrong-prefixes/delete-wrong-prefixes/response-failed",
			detected: detected(ip4, ip6),
			resp:     setter.ResponseFailed,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, m *mocks.MockHandle) {
				expectWAFListMutation(ctx, p, m, wafList, wafListMutationExpectation{
					listDescription: listDescription,
					items:           mixedItems,
					alreadyExisting: true,
					cached:          false,
					createPrefixes:  nil,
					createOK:        true,
					deleteItems:     sortedWrongItems,
					deleteOK:        false,
				})
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx, h := newSetterHarness(t)
			h.prepare(ctx, tc.prepareMocks)

			resp := h.setter.SetWAFList(ctx, h.mockPP, wafList, listDescription, tc.detected, "")
			require.Equal(t, tc.resp, resp)
		})
	}
}

func TestSetWAFListMutationPlanOrderInvariant(t *testing.T) {
	t.Parallel()

	const listName = "list"
	const listDescription = "My List"
	wafList := api.WAFList{AccountID: "account", Name: listName}

	var (
		ip4 = netip.MustParseAddr("10.0.0.1")
		ip6 = netip.MustParseAddr("2001:db8::1111")

		target4 = wafItem("10.0.0.1/32", "target4")
		target6 = wafItem("2001:0db8::/64", "target6")

		cover4 = wafItem("10.0.0.0/20", "cover4")
		cover6 = wafItem("2001:db8::/40", "cover6")

		wrong4a = wafItem("20.0.0.0/16", "wrong4a")
		wrong4b = wafItem("20.0.0.0/24", "wrong4b")
		wrong6a = wafItem("4001:db8::/32", "wrong6a")
		wrong6b = wafItem("4001:db8::/48", "wrong6b")
	)

	type itemOrderer struct {
		name  string
		order func([]api.WAFListItem) []api.WAFListItem
	}

	scenarios := []struct {
		name         string
		items        []api.WAFListItem
		wantCreate   []netip.Prefix
		wantDeleteID []api.ID
	}{
		{
			name: "mixed-covered-and-wrong-prefixes/mutate-list/response-updated",
			items: []api.WAFListItem{
				cover6,
				wrong4a,
				wrong6a,
				cover4,
				wrong6b,
				wrong4b,
			},
			wantCreate: nil,
			wantDeleteID: []api.ID{
				wrong4a.ID,
				wrong4b.ID,
				wrong6a.ID,
				wrong6b.ID,
			},
		},
		{
			name: "wrong-prefixes-only/mutate-list/response-updated",
			items: []api.WAFListItem{
				wrong6a,
				wrong4a,
				wrong6b,
				wrong4b,
			},
			wantCreate: []netip.Prefix{target4.Prefix, target6.Prefix},
			wantDeleteID: []api.ID{
				wrong4a.ID,
				wrong4b.ID,
				wrong6a.ID,
				wrong6b.ID,
			},
		},
	}

	itemOrders := []itemOrderer{
		{
			name:  "input-order-original/run-mutation/response-updated",
			order: slices.Clone[[]api.WAFListItem, api.WAFListItem],
		},
		{
			name: "input-order-reversed/run-mutation/response-updated",
			order: func(items []api.WAFListItem) []api.WAFListItem {
				reversed := slices.Clone(items)
				slices.Reverse(reversed)
				return reversed
			},
		},
		{
			name: "input-order-rotated-left/run-mutation/response-updated",
			order: func(items []api.WAFListItem) []api.WAFListItem {
				return rotateItems(items, 1)
			},
		},
		{
			name: "input-order-rotated-right/run-mutation/response-updated",
			order: func(items []api.WAFListItem) []api.WAFListItem {
				return rotateItems(items, -2)
			},
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			t.Parallel()

			for _, itemOrder := range itemOrders {
				t.Run(itemOrder.name, func(t *testing.T) {
					t.Parallel()

					ctx, h := newSetterHarness(t)
					permutedItems := itemOrder.order(scenario.items)
					detectedIPs := detected(ip4, ip6)

					readCall := h.mockHandle.EXPECT().
						ListWAFListItems(ctx, h.mockPP, wafList, listDescription).
						Return(permutedItems, true, false, true)
					createCall := h.mockHandle.EXPECT().
						CreateWAFListItems(ctx, h.mockPP, wafList, listDescription, scenario.wantCreate, "").
						Return(true)
					deleteCall := h.mockHandle.EXPECT().
						DeleteWAFListItems(ctx, h.mockPP, wafList, listDescription, scenario.wantDeleteID).
						Return(true)
					gomock.InOrder(readCall, createCall, deleteCall)

					h.mockPP.EXPECT().Noticef(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
					h.mockPP.EXPECT().Noticef(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

					resp := h.setter.SetWAFList(ctx, h.mockPP, wafList, listDescription, detectedIPs, "")
					require.Equal(t, setter.ResponseUpdated, resp)
				})
			}
		})
	}
}

func rotateItems[T any](items []T, shift int) []T {
	if len(items) == 0 {
		return nil
	}

	shift %= len(items)
	if shift < 0 {
		shift += len(items)
	}

	rotated := make([]T, 0, len(items))
	rotated = append(rotated, items[shift:]...)
	rotated = append(rotated, items[:shift]...)
	return rotated
}
