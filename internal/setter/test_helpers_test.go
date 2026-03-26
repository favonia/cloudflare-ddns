package setter_test

import (
	"context"
	"net/netip"
	"slices"
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

type setterHarness struct {
	cancel     context.CancelFunc
	mockPP     *mocks.MockPP
	mockHandle *mocks.MockHandle
	setter     setter.Setter
}

type prepareSetterMocks func(ctx context.Context, cancel func(), p *mocks.MockPP, h *mocks.MockHandle)

type dnsRecordFixture struct {
	domain    domain.Domain
	ipFamily  ipnet.Family
	record1   api.ID
	record2   api.ID
	record3   api.ID
	ip1       netip.Addr
	ip2       netip.Addr
	invalidIP netip.Addr
	params    api.RecordParams
}

func newDNSRecordFixture() dnsRecordFixture {
	return dnsRecordFixture{
		domain:    domain.FQDN("sub.test.org"),
		ipFamily:  ipnet.IP6,
		record1:   api.ID("record1"),
		record2:   api.ID("record2"),
		record3:   api.ID("record3"),
		ip1:       netip.MustParseAddr("::1"),
		ip2:       netip.MustParseAddr("::2"),
		invalidIP: netip.Addr{},
		params: api.RecordParams{
			TTL:     api.TTLAuto,
			Proxied: false,
			Comment: "hello",
			Tags:    nil,
		},
	}
}

func newSetterHarness(t *testing.T) (context.Context, setterHarness) {
	t.Helper()

	mockCtrl := gomock.NewController(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	mockPP := mocks.NewMockPP(mockCtrl)
	mockHandle := mocks.NewMockHandle(mockCtrl)

	s, ok := setter.New(mockPP, mockHandle)
	require.True(t, ok)

	return ctx, setterHarness{
		cancel:     cancel,
		mockPP:     mockPP,
		mockHandle: mockHandle,
		setter:     s,
	}
}

func (h setterHarness) prepare(ctx context.Context, prepare prepareSetterMocks) {
	if prepare != nil {
		prepare(ctx, h.cancel, h.mockPP, h.mockHandle)
	}
}

func wrapCancelAsDelete(cancel func()) func(context.Context, pp.PP, ipnet.Family, domain.Domain, api.ID, api.DeletionMode) bool {
	return func(context.Context, pp.PP, ipnet.Family, domain.Domain, api.ID, api.DeletionMode) bool {
		cancel()
		return false
	}
}

// DNS record helpers.
func expectRecordList(
	ctx context.Context,
	p *mocks.MockPP,
	h *mocks.MockHandle,
	ipFamily ipnet.Family,
	domain domain.Domain,
	params api.RecordParams,
	records []api.Record,
	cached bool,
	ok bool,
) any {
	return h.EXPECT().ListRecords(ctx, p, ipFamily, domain, params).Return(records, cached, ok)
}

func expectRecordCreate(
	ctx context.Context,
	p *mocks.MockPP,
	h *mocks.MockHandle,
	ipFamily ipnet.Family,
	domain domain.Domain,
	ip netip.Addr,
	params api.RecordParams,
	id api.ID,
	ok bool,
) any {
	return h.EXPECT().CreateRecord(ctx, p, ipFamily, domain, ip, params).Return(id, ok)
}

func expectRecordUpdate(
	ctx context.Context,
	p *mocks.MockPP,
	h *mocks.MockHandle,
	ipFamily ipnet.Family,
	domain domain.Domain,
	id api.ID,
	ip netip.Addr,
	desiredParams api.RecordParams,
	ok bool,
) any {
	return h.EXPECT().UpdateRecord(ctx, p, ipFamily, domain, id, ip, desiredParams).Return(ok)
}

func expectRecordDelete(
	ctx context.Context,
	p *mocks.MockPP,
	h *mocks.MockHandle,
	ipFamily ipnet.Family,
	domain domain.Domain,
	id api.ID,
	mode api.DeletionMode,
	ok bool,
) any {
	return h.EXPECT().DeleteRecord(ctx, p, ipFamily, domain, id, mode).Return(ok)
}

func expectRecordAddedNotice(p *mocks.MockPP, ipFamily ipnet.Family, domain domain.Domain, id api.ID) any {
	return p.EXPECT().Noticef(
		pp.EmojiCreation,
		"Added a new %s record for %s (ID: %s)",
		ipFamily.RecordType(),
		domain.Describe(),
		id,
	)
}

func expectRecordUpdatedNotice(p *mocks.MockPP, ipFamily ipnet.Family, domain domain.Domain, id api.ID) any {
	return p.EXPECT().Noticef(
		pp.EmojiUpdate,
		"Updated an outdated %s record for %s (ID: %s)",
		ipFamily.RecordType(),
		domain.Describe(),
		id,
	)
}

func expectRecordOutdatedDeletedNotice(p *mocks.MockPP, ipFamily ipnet.Family, domain domain.Domain, id api.ID) any {
	return p.EXPECT().Noticef(
		pp.EmojiDeletion,
		"Deleted an outdated %s record for %s (ID: %s)",
		ipFamily.RecordType(),
		domain.Describe(),
		id,
	)
}

func expectRecordAlreadyUpdatedInfo(p *mocks.MockPP, ipFamily ipnet.Family, domain domain.Domain, cached bool) any {
	if cached {
		return p.EXPECT().Infof(
			pp.EmojiAlreadyDone,
			"The %s records for %s are already up to date (cached)",
			ipFamily.RecordType(),
			domain.Describe(),
		)
	}
	return p.EXPECT().Infof(
		pp.EmojiAlreadyDone,
		"The %s records for %s are already up to date",
		ipFamily.RecordType(),
		domain.Describe(),
	)
}

func expectRecordAlreadyDeletedInfo(p *mocks.MockPP, ipFamily ipnet.Family, domain domain.Domain, cached bool) any {
	if cached {
		return p.EXPECT().Infof(
			pp.EmojiAlreadyDone,
			"The %s records for %s were already deleted (cached)",
			ipFamily.RecordType(),
			domain.Describe(),
		)
	}
	return p.EXPECT().Infof(
		pp.EmojiAlreadyDone,
		"The %s records for %s were already deleted",
		ipFamily.RecordType(),
		domain.Describe(),
	)
}

func expectRecordSetFailedNotice(p *mocks.MockPP, ipFamily ipnet.Family, domain domain.Domain) any {
	return p.EXPECT().Noticef(
		pp.EmojiError,
		"Could not confirm update of %s records for %s; the records might be inconsistent",
		ipFamily.RecordType(),
		domain.Describe(),
	)
}

func expectRecordFinalDeleteFailedNotice(p *mocks.MockPP, ipFamily ipnet.Family, domain domain.Domain) any {
	return p.EXPECT().Noticef(
		pp.EmojiError,
		"Could not confirm deletion of %s records for %s; the records might be inconsistent",
		ipFamily.RecordType(),
		domain.Describe(),
	)
}

func expectRecordDeleteTimeoutInfo(p *mocks.MockPP, ipFamily ipnet.Family, domain domain.Domain) any {
	return p.EXPECT().Infof(
		pp.EmojiTimeout,
		"Deletion of %s records for %s was aborted by a timeout or signal; the records might be inconsistent",
		ipFamily.RecordType(),
		domain.Describe(),
	)
}

// WAF list helpers.
type wafListMutationExpectation struct {
	listDescription string
	items           []api.WAFListItem
	alreadyExisting bool
	cached          bool
	createItems     []api.WAFListCreateItem
	createPrefixes  []netip.Prefix
	createComment   string
	createOK        bool
	deleteItems     []api.WAFListItem
	deleteOK        bool
}

//nolint:unparam // This helper keeps repeated noop fixtures concise despite constant local test inputs.
func wafListNoopExpectation(
	listDescription, fallbackItemComment string, alreadyExisting, cached bool,
) wafListMutationExpectation {
	return wafListMutationExpectation{
		listDescription: listDescription,
		items:           nil,
		alreadyExisting: alreadyExisting,
		cached:          cached,
		createItems:     nil,
		createPrefixes:  nil,
		createComment:   fallbackItemComment,
		createOK:        false,
		deleteItems:     nil,
		deleteOK:        false,
	}
}

func expectWAFListRead(
	ctx context.Context,
	p *mocks.MockPP,
	m *mocks.MockHandle,
	list api.WAFList,
	listDescription string,
	fallbackItemComment string,
	items []api.WAFListItem,
	alreadyExisting bool,
	cached bool,
	ok bool,
) any {
	return m.EXPECT().ListWAFListItems(ctx, p, list, listDescription, fallbackItemComment).Return(items, alreadyExisting, cached, ok)
}

func expectWAFListNoop(
	ctx context.Context,
	p *mocks.MockPP,
	m *mocks.MockHandle,
	list api.WAFList,
	want wafListMutationExpectation,
	items []api.WAFListItem,
) {
	calls := []any{
		expectWAFListRead(ctx, p, m, list, want.listDescription, want.createComment, items, want.alreadyExisting, want.cached, true),
	}
	if !want.alreadyExisting {
		calls = append(calls, expectWAFListCreatedNotice(p, list))
	}
	calls = append(calls, expectWAFListNoopNotice(p, list, want.cached))
	gomock.InOrder(calls...)
}

func expectWAFListMutation(
	ctx context.Context,
	p *mocks.MockPP,
	m *mocks.MockHandle,
	list api.WAFList,
	want wafListMutationExpectation,
) {
	createItems := slices.Clone(want.createItems)
	if len(createItems) == 0 {
		for _, prefix := range want.createPrefixes {
			createItems = append(createItems, api.WAFListCreateItem{
				Prefix:  prefix,
				Comment: want.createComment,
			})
		}
	}

	calls := []any{
		expectWAFListRead(ctx, p, m, list, want.listDescription, want.createComment, want.items, want.alreadyExisting, want.cached, true),
	}
	if !want.alreadyExisting {
		calls = append(calls, expectWAFListCreatedNotice(p, list))
	}

	if len(createItems) > 0 || !want.createOK {
		calls = append(calls, m.EXPECT().
			CreateWAFListItems(ctx, p, list, want.listDescription, createItems).
			Return(want.createOK))
		if !want.createOK {
			calls = append(calls, expectWAFListErrorNotice(p, list))
			gomock.InOrder(calls...)
			return
		}
		calls = append(calls, expectWAFCreateNotices(p, list, wafListCreatePrefixes(createItems))...)
	}
	calls = append(calls, expectWAFListDelete(ctx, p, m, list, want.listDescription, wafItemIDs(want.deleteItems), want.deleteOK))
	if !want.deleteOK {
		calls = append(calls, expectWAFListErrorNotice(p, list))
		gomock.InOrder(calls...)
		return
	}

	calls = append(calls, expectWAFDeleteNotices(p, list, want.deleteItems)...)
	gomock.InOrder(calls...)
}

func wafListCreatePrefixes(items []api.WAFListCreateItem) []netip.Prefix {
	prefixes := make([]netip.Prefix, 0, len(items))
	for _, item := range items {
		prefixes = append(prefixes, item.Prefix)
	}
	return prefixes
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
		"Could not confirm update of the list %s; its content may be inconsistent",
		list.Describe(),
	)
}

func expectWAFCreateNotices(p *mocks.MockPP, list api.WAFList, prefixes []netip.Prefix) []any {
	calls := make([]any, 0, len(prefixes))
	for _, prefix := range prefixes {
		calls = append(calls, p.EXPECT().Noticef(
			pp.EmojiCreation,
			"Added %s to the list %s",
			prefix.Masked().String(),
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
			item.Prefix.Masked().String(),
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
	return m.EXPECT().DeleteWAFListItems(ctx, p, list, listDescription, ids).Return(ok)
}

func wafItemIDs(items []api.WAFListItem) []api.ID {
	ids := make([]api.ID, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	return ids
}
