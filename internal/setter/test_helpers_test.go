package setter_test

import (
	"context"
	"net/netip"
	"regexp"
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
	cancel       context.CancelFunc
	mockPP       *mocks.MockPP
	mockHandle   *mocks.MockHandle
	recordFilter api.ManagedRecordFilter
	setter       setter.Setter
}

type prepareSetterMocks func(ctx context.Context, cancel func(), p *mocks.MockPP, h *mocks.MockHandle)

// setterConfig mirrors setter.New inputs so tests do not depend on positional
// constructor arguments.
type setterConfig struct {
	managedRecordCommentRegex *regexp.Regexp
}

func (c setterConfig) recordFilter() api.ManagedRecordFilter {
	return api.ManagedRecordFilter{CommentRegex: c.managedRecordCommentRegex}
}

type dnsRecordFixture struct {
	domain    domain.Domain
	ipNetwork ipnet.Type
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
		ipNetwork: ipnet.IP6,
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
		},
	}
}

func newSetterHarness(t *testing.T) (context.Context, setterHarness) {
	t.Helper()

	return newSetterHarnessWithConfig(t, setterConfig{}) //nolint:exhaustruct // Zero value means no managed-record filter in this helper.
}

func newSetterHarnessWithConfig(t *testing.T, config setterConfig) (context.Context, setterHarness) {
	t.Helper()

	mockCtrl := gomock.NewController(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	mockPP := mocks.NewMockPP(mockCtrl)
	mockHandle := mocks.NewMockHandle(mockCtrl)

	s, ok := setter.New(mockPP, mockHandle, config.managedRecordCommentRegex)
	require.True(t, ok)

	return ctx, setterHarness{
		cancel:       cancel,
		mockPP:       mockPP,
		mockHandle:   mockHandle,
		recordFilter: config.recordFilter(),
		setter:       s,
	}
}

func (h setterHarness) prepare(ctx context.Context, prepare prepareSetterMocks) {
	if prepare != nil {
		prepare(ctx, h.cancel, h.mockPP, h.mockHandle)
	}
}

func wrapCancelAsDelete(cancel func()) func(context.Context, pp.PP, ipnet.Type, domain.Domain, api.ID, api.DeletionMode) bool {
	return func(context.Context, pp.PP, ipnet.Type, domain.Domain, api.ID, api.DeletionMode) bool {
		cancel()
		return false
	}
}

// DNS record helpers.
func expectRecordList(
	ctx context.Context,
	p *mocks.MockPP,
	h *mocks.MockHandle,
	ipNetwork ipnet.Type,
	domain domain.Domain,
	params api.RecordParams,
	records []api.Record,
	cached bool,
	ok bool,
) any {
	return expectRecordListWithFilter(ctx, p, h, ipNetwork, domain, api.ManagedRecordFilter{CommentRegex: nil}, params, records, cached, ok)
}

func expectRecordListWithFilter(
	ctx context.Context,
	p *mocks.MockPP,
	h *mocks.MockHandle,
	ipNetwork ipnet.Type,
	domain domain.Domain,
	recordFilter api.ManagedRecordFilter,
	params api.RecordParams,
	records []api.Record,
	cached bool,
	ok bool,
) any {
	return h.EXPECT().ListRecords(ctx, p, ipNetwork, domain, recordFilter, params).Return(records, cached, ok)
}

func expectRecordCreate(
	ctx context.Context,
	p *mocks.MockPP,
	h *mocks.MockHandle,
	ipNetwork ipnet.Type,
	domain domain.Domain,
	ip netip.Addr,
	params api.RecordParams,
	id api.ID,
	ok bool,
) any {
	return h.EXPECT().CreateRecord(ctx, p, ipNetwork, domain, ip, params).Return(id, ok)
}

func expectRecordUpdate(
	ctx context.Context,
	p *mocks.MockPP,
	h *mocks.MockHandle,
	ipNetwork ipnet.Type,
	domain domain.Domain,
	id api.ID,
	ip netip.Addr,
	currentParams api.RecordParams,
	expectedParams api.RecordParams,
	ok bool,
) any {
	return h.EXPECT().UpdateRecord(ctx, p, ipNetwork, domain, id, ip, currentParams, expectedParams).Return(ok)
}

func expectRecordDelete(
	ctx context.Context,
	p *mocks.MockPP,
	h *mocks.MockHandle,
	ipNetwork ipnet.Type,
	domain domain.Domain,
	id api.ID,
	mode api.DeletionMode,
	ok bool,
) any {
	return h.EXPECT().DeleteRecord(ctx, p, ipNetwork, domain, id, mode).Return(ok)
}

func expectRecordAddedNotice(p *mocks.MockPP, ipNetwork ipnet.Type, domain domain.Domain, id api.ID) any {
	return p.EXPECT().Noticef(
		pp.EmojiCreation,
		"Added a new %s record of %s (ID: %s)",
		ipNetwork.RecordType(),
		domain.Describe(),
		id,
	)
}

func expectRecordUpdatedNotice(p *mocks.MockPP, ipNetwork ipnet.Type, domain domain.Domain, id api.ID) any {
	return p.EXPECT().Noticef(
		pp.EmojiUpdate,
		"Updated a stale %s record of %s (ID: %s)",
		ipNetwork.RecordType(),
		domain.Describe(),
		id,
	)
}

func expectRecordStaleDeletedNotice(p *mocks.MockPP, ipNetwork ipnet.Type, domain domain.Domain, id api.ID) any {
	return p.EXPECT().Noticef(
		pp.EmojiDeletion,
		"Deleted a stale %s record of %s (ID: %s)",
		ipNetwork.RecordType(),
		domain.Describe(),
		id,
	)
}

func expectRecordDuplicateDeletedNotice(p *mocks.MockPP, ipNetwork ipnet.Type, domain domain.Domain, id api.ID) any {
	return p.EXPECT().Noticef(
		pp.EmojiDeletion,
		"Deleted a duplicate %s record of %s (ID: %s)",
		ipNetwork.RecordType(),
		domain.Describe(),
		id,
	)
}

func expectRecordAlreadyUpdatedInfo(p *mocks.MockPP, ipNetwork ipnet.Type, domain domain.Domain, cached bool) any {
	if cached {
		return p.EXPECT().Infof(
			pp.EmojiAlreadyDone,
			"The %s records of %s are already up to date (cached)",
			ipNetwork.RecordType(),
			domain.Describe(),
		)
	}
	return p.EXPECT().Infof(
		pp.EmojiAlreadyDone,
		"The %s records of %s are already up to date",
		ipNetwork.RecordType(),
		domain.Describe(),
	)
}

func expectRecordAlreadyDeletedInfo(p *mocks.MockPP, ipNetwork ipnet.Type, domain domain.Domain, cached bool) any {
	if cached {
		return p.EXPECT().Infof(
			pp.EmojiAlreadyDone,
			"The %s records of %s were already deleted (cached)",
			ipNetwork.RecordType(),
			domain.Describe(),
		)
	}
	return p.EXPECT().Infof(
		pp.EmojiAlreadyDone,
		"The %s records of %s were already deleted",
		ipNetwork.RecordType(),
		domain.Describe(),
	)
}

func expectRecordSetFailedNotice(p *mocks.MockPP, ipNetwork ipnet.Type, domain domain.Domain) any {
	return p.EXPECT().Noticef(
		pp.EmojiError,
		"Failed to properly update %s records of %s; records might be inconsistent",
		ipNetwork.RecordType(),
		domain.Describe(),
	)
}

func expectRecordFinalDeleteFailedNotice(p *mocks.MockPP, ipNetwork ipnet.Type, domain domain.Domain) any {
	return p.EXPECT().Noticef(
		pp.EmojiError,
		"Failed to properly delete %s records of %s; records might be inconsistent",
		ipNetwork.RecordType(),
		domain.Describe(),
	)
}

func expectRecordDeleteTimeoutInfo(p *mocks.MockPP, ipNetwork ipnet.Type, domain domain.Domain) any {
	return p.EXPECT().Infof(
		pp.EmojiTimeout,
		"Deletion of %s records of %s aborted by timeout or signals; records might be inconsistent",
		ipNetwork.RecordType(),
		domain.Describe(),
	)
}

// WAF list helpers.
type wafListMutationExpectation struct {
	listDescription string
	items           []api.WAFListItem
	alreadyExisting bool
	cached          bool
	createPrefixes  []netip.Prefix
	createComment   string
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
	return m.EXPECT().
		ListWAFListItems(ctx, p, list, api.ManagedWAFListItemFilter{CommentRegex: nil}, listDescription).
		Return(items, alreadyExisting, cached, ok)
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
		CreateWAFListItems(ctx, p, list, want.listDescription, want.createPrefixes, want.createComment).
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
	return m.EXPECT().DeleteWAFListItems(ctx, p, list, listDescription, ids).Return(ok)
}

func wafItemIDs(items []api.WAFListItem) []api.ID {
	ids := make([]api.ID, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	return ids
}
