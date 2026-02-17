package setter_test

import (
	"context"
	"net/netip"

	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

type wafListMutationExpectation struct {
	listDescription string
	items           []api.WAFListItem
	alreadyExisting bool
	cached          bool
	createPrefixes  []netip.Prefix
	createOK        bool
	deleteItems     []api.WAFListItem
	deleteOK        bool
}

func expectWAFListRead(
	ctx context.Context,
	p *mocks.MockPP,
	m *mocks.MockHandle,
	list api.WAFList,
	listDescription string,
	items []api.WAFListItem,
	alreadyExisting bool,
	cached bool,
	ok bool,
) any {
	return m.EXPECT().ListWAFListItems(ctx, p, list, listDescription).Return(items, alreadyExisting, cached, ok)
}

func expectWAFListNoop(
	ctx context.Context,
	p *mocks.MockPP,
	m *mocks.MockHandle,
	list api.WAFList,
	listDescription string,
	items []api.WAFListItem,
	alreadyExisting bool,
	cached bool,
) {
	calls := []any{
		expectWAFListRead(ctx, p, m, list, listDescription, items, alreadyExisting, cached, true),
	}
	if !alreadyExisting {
		calls = append(calls, expectWAFListCreatedNotice(p, list))
	}
	calls = append(calls, expectWAFListNoopNotice(p, list, cached))
	gomock.InOrder(calls...)
}

func expectWAFListMutation(
	ctx context.Context,
	p *mocks.MockPP,
	m *mocks.MockHandle,
	list api.WAFList,
	want wafListMutationExpectation,
) {
	calls := []any{
		expectWAFListRead(ctx, p, m, list, want.listDescription, want.items, want.alreadyExisting, want.cached, true),
	}
	if !want.alreadyExisting {
		calls = append(calls, expectWAFListCreatedNotice(p, list))
	}

	calls = append(calls, m.EXPECT().
		CreateWAFListItems(ctx, p, list, want.listDescription, want.createPrefixes, "").
		Return(want.createOK))
	if !want.createOK {
		calls = append(calls, expectWAFListErrorNotice(p, list))
		gomock.InOrder(calls...)
		return
	}

	calls = append(calls, expectWAFCreateNotices(p, list, want.createPrefixes)...)
	calls = append(calls, expectWAFListDelete(ctx, p, m, list, want.listDescription, wafItemIDs(want.deleteItems), want.deleteOK))
	if !want.deleteOK {
		calls = append(calls, expectWAFListErrorNotice(p, list))
		gomock.InOrder(calls...)
		return
	}

	calls = append(calls, expectWAFDeleteNotices(p, list, want.deleteItems)...)
	gomock.InOrder(calls...)
}

func expectWAFListCreatedNotice(p *mocks.MockPP, list api.WAFList) any {
	return p.EXPECT().Noticef(pp.EmojiCreation, "Created a new list %s", list.Describe())
}

func expectWAFListNoopNotice(p *mocks.MockPP, list api.WAFList, cached bool) any {
	if cached {
		return p.EXPECT().Infof(pp.EmojiAlreadyDone, "The list %s is already up to date (cached)", list.Describe())
	}
	return p.EXPECT().Infof(pp.EmojiAlreadyDone, "The list %s is already up to date", list.Describe())
}

func expectWAFListErrorNotice(p *mocks.MockPP, list api.WAFList) any {
	return p.EXPECT().Noticef(
		pp.EmojiError,
		"Failed to properly update the list %s; its content may be inconsistent",
		list.Describe(),
	)
}

func expectWAFCreateNotices(p *mocks.MockPP, list api.WAFList, prefixes []netip.Prefix) []any {
	calls := make([]any, 0, len(prefixes))
	for _, prefix := range prefixes {
		calls = append(calls, p.EXPECT().Noticef(
			pp.EmojiCreation,
			"Added %s to the list %s",
			ipnet.DescribePrefixOrIP(prefix),
			list.Describe(),
		))
	}
	return calls
}

func expectWAFDeleteNotices(p *mocks.MockPP, list api.WAFList, items []api.WAFListItem) []any {
	calls := make([]any, 0, len(items))
	for _, item := range items {
		calls = append(calls, p.EXPECT().Noticef(
			pp.EmojiDeletion,
			"Deleted %s from the list %s",
			ipnet.DescribePrefixOrIP(item.Prefix),
			list.Describe(),
		))
	}
	return calls
}

func expectWAFListDelete(
	ctx context.Context,
	p *mocks.MockPP,
	m *mocks.MockHandle,
	list api.WAFList,
	listDescription string,
	ids []api.ID,
	ok bool,
) any {
	if len(ids) == 0 {
		return m.EXPECT().DeleteWAFListItems(ctx, p, list, listDescription, []api.ID{}).Return(ok)
	}
	return m.EXPECT().DeleteWAFListItems(ctx, p, list, listDescription, gomock.InAnyOrder(ids)).Return(ok)
}

func wafItemIDs(items []api.WAFListItem) []api.ID {
	ids := make([]api.ID, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	return ids
}
