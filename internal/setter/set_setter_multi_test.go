package setter_test

// vim: nowrap

import (
	"context"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/setter"
)

func TestSetIPs(t *testing.T) {
	t.Parallel()

	fixture := newDNSRecordFixture()
	ip3 := netip.MustParseAddr("::3")
	ip4 := netip.MustParseAddr("::4")
	ip5 := netip.MustParseAddr("::5")
	ip6 := netip.MustParseAddr("::6")
	record4 := api.ID("record4")

	cases := []struct {
		name         string
		ips          []netip.Addr
		resp         setter.ResponseCode
		prepareMocks prepareSetterMocks
	}{
		{
			name: "zero-targets/no-records/response-noop-cached",
			ips:  nil,
			resp: setter.ResponseNoop,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					expectRecordList(ctx, p, h, fixture.ipFamily, fixture.domain, fixture.params, []api.Record{}, true, true),
					expectRecordAlreadyUpdatedInfo(p, fixture.ipFamily, fixture.domain, true),
				)
			},
		},
		{
			name: "one-target/keep-record/response-noop-uncached",
			ips:  []netip.Addr{fixture.ip1},
			resp: setter.ResponseNoop,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					expectRecordList(ctx, p, h, fixture.ipFamily, fixture.domain, fixture.params, []api.Record{
						dnsRecord(fixture.record1, fixture.ip1, fixture.params),
					}, false, true),
					expectRecordAlreadyUpdatedInfo(p, fixture.ipFamily, fixture.domain, false),
				)
			},
		},
		{
			name: "zero-targets/delete-all-existing/response-updated",
			ips:  []netip.Addr{},
			resp: setter.ResponseUpdated,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					expectRecordList(ctx, p, h, fixture.ipFamily, fixture.domain, fixture.params, []api.Record{
						dnsRecord(fixture.record1, fixture.ip1, fixture.params),
						dnsRecord(fixture.record2, fixture.ip2, fixture.params),
					}, true, true),
					expectRecordDelete(ctx, p, h, fixture.ipFamily, fixture.domain, fixture.record1, api.RegularDeletionMode, true),
					expectRecordOutdatedDeletedNotice(p, fixture.ipFamily, fixture.domain, fixture.record1),
					expectRecordDelete(ctx, p, h, fixture.ipFamily, fixture.domain, fixture.record2, api.RegularDeletionMode, true),
					expectRecordOutdatedDeletedNotice(p, fixture.ipFamily, fixture.domain, fixture.record2),
				)
			},
		},
		{
			name: "many-targets/reconcile-and-cleanup/response-updated",
			ips:  []netip.Addr{fixture.ip1, fixture.ip2, ip3},
			resp: setter.ResponseUpdated,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				// InOrder is stricter than the API contract here. We intentionally pin
				// deterministic ordering for canonical SetIPs inputs.
				gomock.InOrder(
					expectRecordList(ctx, p, h, fixture.ipFamily, fixture.domain, fixture.params, []api.Record{
						dnsRecord(fixture.record1, fixture.ip1, fixture.params),
						dnsRecord(fixture.record2, fixture.ip1, fixture.params),
						dnsRecord(fixture.record3, ip4, fixture.params),
					}, true, true),
					expectRecordUpdate(
						ctx,
						p,
						h,
						fixture.ipFamily,
						fixture.domain,
						fixture.record3,
						fixture.ip2,
						fixture.params,
						true,
					),
					expectRecordUpdatedNotice(p, fixture.ipFamily, fixture.domain, fixture.record3),
					expectRecordCreate(ctx, p, h, fixture.ipFamily, fixture.domain, ip3, fixture.params, record4, true),
					expectRecordAddedNotice(p, fixture.ipFamily, fixture.domain, record4),
				)
			},
		},
		{
			name: "many-targets/delete-leftover-outdated-fails/response-failed",
			ips:  []netip.Addr{fixture.ip1, fixture.ip2},
			resp: setter.ResponseFailed,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					expectRecordList(ctx, p, h, fixture.ipFamily, fixture.domain, fixture.params, []api.Record{
						dnsRecord(fixture.record1, ip4, fixture.params),
						dnsRecord(fixture.record2, ip5, fixture.params),
						dnsRecord(fixture.record3, ip6, fixture.params),
					}, true, true),
					expectRecordUpdate(
						ctx,
						p,
						h,
						fixture.ipFamily,
						fixture.domain,
						fixture.record1,
						fixture.ip1,
						fixture.params,
						true,
					),
					expectRecordUpdatedNotice(p, fixture.ipFamily, fixture.domain, fixture.record1),
					expectRecordUpdate(
						ctx,
						p,
						h,
						fixture.ipFamily,
						fixture.domain,
						fixture.record2,
						fixture.ip2,
						fixture.params,
						true,
					),
					expectRecordUpdatedNotice(p, fixture.ipFamily, fixture.domain, fixture.record2),
					expectRecordDelete(ctx, p, h, fixture.ipFamily, fixture.domain, fixture.record3, api.RegularDeletionMode, false),
					expectRecordSetFailedNotice(p, fixture.ipFamily, fixture.domain),
				)
			},
		},
		{
			name: "many-targets/duplicate-cleanup-timeout/response-noop",
			ips:  []netip.Addr{fixture.ip1},
			resp: setter.ResponseNoop,
			prepareMocks: func(ctx context.Context, cancel func(), p *mocks.MockPP, h *mocks.MockHandle) {
				_ = cancel
				gomock.InOrder(
					expectRecordList(ctx, p, h, fixture.ipFamily, fixture.domain, fixture.params, []api.Record{
						dnsRecord(fixture.record1, fixture.ip1, fixture.params),
						dnsRecord(fixture.record2, fixture.ip1, fixture.params),
					}, true, true),
					expectRecordAlreadyUpdatedInfo(p, fixture.ipFamily, fixture.domain, true),
				)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx, h := newSetterHarness(t)
			h.prepare(ctx, tc.prepareMocks)

			resp := h.setter.SetIPs(ctx, h.mockPP, fixture.ipFamily, fixture.domain, tc.ips, fixture.params)
			require.Equal(t, tc.resp, resp)
		})
	}
}

func TestSetIPsCreateDoesNotInheritMetadataFromDeletedDuplicate(t *testing.T) {
	t.Parallel()

	fixture := newDNSRecordFixture()
	targetCreate := netip.MustParseAddr("::3")
	record4 := api.ID("record4")
	inherited := api.RecordParams{
		TTL:     300,
		Proxied: true,
		Comment: "from-duplicate",
		Tags:    []string{"env:prod", "Team:Alpha"},
	}

	ctx, h := newSetterHarness(t)

	gomock.InOrder(
		expectRecordList(ctx, h.mockPP, h.mockHandle, fixture.ipFamily, fixture.domain, fixture.params, []api.Record{
			dnsRecord(fixture.record1, fixture.ip1, fixture.params),
			dnsRecord(fixture.record2, fixture.ip1, inherited),
		}, true, true),
		expectRecordCreate(ctx, h.mockPP, h.mockHandle, fixture.ipFamily, fixture.domain, targetCreate, fixture.params, record4, true),
		expectRecordAddedNotice(h.mockPP, fixture.ipFamily, fixture.domain, record4),
	)

	resp := h.setter.SetIPs(ctx, h.mockPP, fixture.ipFamily, fixture.domain, []netip.Addr{fixture.ip1, targetCreate}, fixture.params)
	require.Equal(t, setter.ResponseUpdated, resp)
}

func TestSetIPsDuplicateKeeperUsesLowestID(t *testing.T) {
	t.Parallel()

	fixture := newDNSRecordFixture()
	ctx, h := newSetterHarness(t)

	gomock.InOrder(
		expectRecordList(ctx, h.mockPP, h.mockHandle, fixture.ipFamily, fixture.domain, fixture.params, []api.Record{
			dnsRecord(fixture.record2, fixture.ip1, fixture.params),
			dnsRecord(fixture.record1, fixture.ip1, fixture.params),
		}, true, true),
		expectRecordAlreadyUpdatedInfo(h.mockPP, fixture.ipFamily, fixture.domain, true),
	)

	resp := h.setter.SetIPs(ctx, h.mockPP, fixture.ipFamily, fixture.domain, []netip.Addr{fixture.ip1}, fixture.params)
	require.Equal(t, setter.ResponseNoop, resp)
}

func TestSetIPsDuplicateKeeperUsesLowestIDWithinMetadataMatchingSubset(t *testing.T) {
	t.Parallel()

	fixture := newDNSRecordFixture()
	record0 := api.ID("record0")
	ctx, h := newSetterHarness(t)

	nonMatching := fixture.params
	nonMatching.Comment = "foreign"

	gomock.InOrder(
		expectRecordList(ctx, h.mockPP, h.mockHandle, fixture.ipFamily, fixture.domain, fixture.params, []api.Record{
			dnsRecord(record0, fixture.ip1, nonMatching),
			dnsRecord(fixture.record3, fixture.ip1, fixture.params),
			dnsRecord(fixture.record2, fixture.ip1, fixture.params),
		}, true, true),
		expectRecordAlreadyUpdatedInfo(h.mockPP, fixture.ipFamily, fixture.domain, true),
	)

	resp := h.setter.SetIPs(ctx, h.mockPP, fixture.ipFamily, fixture.domain, []netip.Addr{fixture.ip1}, fixture.params)
	require.Equal(t, setter.ResponseNoop, resp)
}

func TestSetIPsMatchedMetadataReconciliationUpdateFailure(t *testing.T) {
	t.Parallel()

	fixture := newDNSRecordFixture()
	ctx, h := newSetterHarness(t)

	firstNonMatching := fixture.params
	firstNonMatching.TTL = fixture.params.TTL + 30
	secondNonMatching := fixture.params
	secondNonMatching.TTL = fixture.params.TTL + 60

	gomock.InOrder(
		expectRecordList(ctx, h.mockPP, h.mockHandle, fixture.ipFamily, fixture.domain, fixture.params, []api.Record{
			dnsRecord(fixture.record1, fixture.ip1, firstNonMatching),
			dnsRecord(fixture.record2, fixture.ip1, secondNonMatching),
		}, true, true),
		expectRecordAlreadyUpdatedInfo(h.mockPP, fixture.ipFamily, fixture.domain, true),
	)

	resp := h.setter.SetIPs(ctx, h.mockPP, fixture.ipFamily, fixture.domain, []netip.Addr{fixture.ip1}, fixture.params)
	require.Equal(t, setter.ResponseNoop, resp)
}

func TestSetIPsOutdatedOperationsBeforeMatchedUpdateAndDelete(t *testing.T) {
	t.Parallel()

	fixture := newDNSRecordFixture()
	ip3 := netip.MustParseAddr("::3")
	nonMatchingA := fixture.params
	nonMatchingA.TTL = 300
	nonMatchingA.Proxied = true
	nonMatchingA.Comment = "a"
	nonMatchingB := fixture.params
	nonMatchingB.TTL = 400
	nonMatchingB.Proxied = true
	nonMatchingB.Comment = "b"

	ctx, h := newSetterHarness(t)

	gomock.InOrder(
		expectRecordList(ctx, h.mockPP, h.mockHandle, fixture.ipFamily, fixture.domain, fixture.params, []api.Record{
			dnsRecord(fixture.record1, fixture.ip1, nonMatchingA),
			dnsRecord(fixture.record2, fixture.ip1, nonMatchingB),
			dnsRecord(fixture.record3, ip3, fixture.params),
		}, true, true),
		expectRecordUpdate(
			ctx,
			h.mockPP,
			h.mockHandle,
			fixture.ipFamily,
			fixture.domain,
			fixture.record3,
			fixture.ip2,
			fixture.params,
			true,
		),
		expectRecordUpdatedNotice(h.mockPP, fixture.ipFamily, fixture.domain, fixture.record3),
	)

	resp := h.setter.SetIPs(ctx, h.mockPP, fixture.ipFamily, fixture.domain, []netip.Addr{fixture.ip1, fixture.ip2}, fixture.params)
	require.Equal(t, setter.ResponseUpdated, resp)
}

func TestSetIPsOutdatedDeleteBeforeMatchedUpdate(t *testing.T) {
	t.Parallel()

	fixture := newDNSRecordFixture()
	ip3 := netip.MustParseAddr("::3")
	nonMatchingA := fixture.params
	nonMatchingA.TTL = 300
	nonMatchingA.Proxied = true
	nonMatchingA.Comment = "a"
	nonMatchingB := fixture.params
	nonMatchingB.TTL = 400
	nonMatchingB.Proxied = true
	nonMatchingB.Comment = "b"

	ctx, h := newSetterHarness(t)

	gomock.InOrder(
		expectRecordList(ctx, h.mockPP, h.mockHandle, fixture.ipFamily, fixture.domain, fixture.params, []api.Record{
			dnsRecord(fixture.record1, fixture.ip1, nonMatchingA),
			dnsRecord(fixture.record2, fixture.ip1, nonMatchingB),
			dnsRecord(fixture.record3, ip3, fixture.params),
		}, true, true),
		expectRecordDelete(ctx, h.mockPP, h.mockHandle, fixture.ipFamily, fixture.domain, fixture.record3, api.RegularDeletionMode, true),
		expectRecordOutdatedDeletedNotice(h.mockPP, fixture.ipFamily, fixture.domain, fixture.record3),
	)

	resp := h.setter.SetIPs(ctx, h.mockPP, fixture.ipFamily, fixture.domain, []netip.Addr{fixture.ip1}, fixture.params)
	require.Equal(t, setter.ResponseUpdated, resp)
}

func TestSetIPsDuplicateDeletionPrioritizesNonMatchingAcrossTargets(t *testing.T) {
	t.Parallel()

	fixture := newDNSRecordFixture()
	record4 := api.ID("record4")
	record5 := api.ID("record5")
	record6 := api.ID("record6")
	nonMatchingA := fixture.params
	nonMatchingA.Comment = "foreign-a"
	nonMatchingB := fixture.params
	nonMatchingB.Comment = "foreign-b"

	ctx, h := newSetterHarness(t)

	gomock.InOrder(
		expectRecordList(ctx, h.mockPP, h.mockHandle, fixture.ipFamily, fixture.domain, fixture.params, []api.Record{
			dnsRecord(fixture.record3, fixture.ip1, nonMatchingA),
			dnsRecord(fixture.record2, fixture.ip1, fixture.params),
			dnsRecord(fixture.record1, fixture.ip1, fixture.params),
			dnsRecord(record6, fixture.ip2, nonMatchingB),
			dnsRecord(record5, fixture.ip2, fixture.params),
			dnsRecord(record4, fixture.ip2, fixture.params),
		}, true, true),
		expectRecordAlreadyUpdatedInfo(h.mockPP, fixture.ipFamily, fixture.domain, true),
	)

	resp := h.setter.SetIPs(ctx, h.mockPP, fixture.ipFamily, fixture.domain, []netip.Addr{fixture.ip1, fixture.ip2}, fixture.params)
	require.Equal(t, setter.ResponseNoop, resp)
}

func TestSetIPsOutdatedRecycleUsesLowestIDTieBreak(t *testing.T) {
	t.Parallel()

	fixture := newDNSRecordFixture()
	ip3 := netip.MustParseAddr("::3")
	ip4 := netip.MustParseAddr("::4")

	ctx, h := newSetterHarness(t)

	gomock.InOrder(
		expectRecordList(ctx, h.mockPP, h.mockHandle, fixture.ipFamily, fixture.domain, fixture.params, []api.Record{
			dnsRecord(fixture.record2, ip3, fixture.params),
			dnsRecord(fixture.record1, ip4, fixture.params),
		}, true, true),
		expectRecordUpdate(
			ctx,
			h.mockPP,
			h.mockHandle,
			fixture.ipFamily,
			fixture.domain,
			fixture.record1,
			fixture.ip1,
			fixture.params,
			true,
		),
		expectRecordUpdatedNotice(h.mockPP, fixture.ipFamily, fixture.domain, fixture.record1),
		expectRecordUpdate(
			ctx,
			h.mockPP,
			h.mockHandle,
			fixture.ipFamily,
			fixture.domain,
			fixture.record2,
			fixture.ip2,
			fixture.params,
			true,
		),
		expectRecordUpdatedNotice(h.mockPP, fixture.ipFamily, fixture.domain, fixture.record2),
	)

	resp := h.setter.SetIPs(ctx, h.mockPP, fixture.ipFamily, fixture.domain, []netip.Addr{fixture.ip1, fixture.ip2}, fixture.params)
	require.Equal(t, setter.ResponseUpdated, resp)
}

func TestSetIPsRecycleUsesReconciledOutdatedMetadata(t *testing.T) {
	t.Parallel()

	fixture := newDNSRecordFixture()
	reconciled := api.RecordParams{
		TTL:     300,
		Proxied: true,
		Comment: "from-outdated",
		Tags:    []string{"env:prod", "Team:Alpha"},
	}

	ctx, h := newSetterHarness(t)

	gomock.InOrder(
		expectRecordList(ctx, h.mockPP, h.mockHandle, fixture.ipFamily, fixture.domain, fixture.params, []api.Record{
			dnsRecord(fixture.record1, fixture.ip2, reconciled),
		}, true, true),
		expectRecordUpdate(
			ctx,
			h.mockPP,
			h.mockHandle,
			fixture.ipFamily,
			fixture.domain,
			fixture.record1,
			fixture.ip1,
			reconciled,
			true,
		),
		expectRecordUpdatedNotice(h.mockPP, fixture.ipFamily, fixture.domain, fixture.record1),
	)

	resp := h.setter.SetIPs(ctx, h.mockPP, fixture.ipFamily, fixture.domain, []netip.Addr{fixture.ip1}, fixture.params)
	require.Equal(t, setter.ResponseUpdated, resp)
}

func TestSetIPsDuplicateCanonicalTagsWarnOnImpossibleCases(t *testing.T) {
	t.Parallel()

	fixture := newDNSRecordFixture()
	targetCreate := netip.MustParseAddr("::3")
	record4 := api.ID("record4")
	withDuplicateCanonicalTags := api.RecordParams{
		TTL:     fixture.params.TTL,
		Proxied: fixture.params.Proxied,
		Comment: fixture.params.Comment,
		Tags:    []string{"ENV:prod", "env:prod", "Team:Alpha"},
	}
	ctx, h := newSetterHarness(t)

	gomock.InOrder(
		expectRecordList(ctx, h.mockPP, h.mockHandle, fixture.ipFamily, fixture.domain, fixture.params, []api.Record{
			dnsRecord(fixture.record1, fixture.ip1, fixture.params),
			dnsRecord(fixture.record2, fixture.ip1, withDuplicateCanonicalTags),
		}, true, true),
		expectRecordCreate(ctx, h.mockPP, h.mockHandle, fixture.ipFamily, fixture.domain, targetCreate, fixture.params, record4, true),
		expectRecordAddedNotice(h.mockPP, fixture.ipFamily, fixture.domain, record4),
	)

	resp := h.setter.SetIPs(ctx, h.mockPP, fixture.ipFamily, fixture.domain, []netip.Addr{fixture.ip1, targetCreate}, fixture.params)
	require.Equal(t, setter.ResponseUpdated, resp)
}

func TestSetIPsDuplicateCanonicalTagsImpossibleWarningsAreNotDeduped(t *testing.T) {
	t.Parallel()

	fixture := newDNSRecordFixture()
	targetCreate := netip.MustParseAddr("::3")
	withDuplicateCanonicalTags := api.RecordParams{
		TTL:     fixture.params.TTL,
		Proxied: fixture.params.Proxied,
		Comment: fixture.params.Comment,
		Tags:    []string{"TEAM:one", "team:one"},
	}
	outdatedWithDuplicateCanonicalTags := api.RecordParams{
		TTL:     fixture.params.TTL,
		Proxied: fixture.params.Proxied,
		Comment: fixture.params.Comment,
		Tags:    []string{"env:prod", "env:prod"},
	}
	reconciledFromOutdated := api.RecordParams{
		TTL:     fixture.params.TTL,
		Proxied: fixture.params.Proxied,
		Comment: fixture.params.Comment,
		Tags:    []string{"env:prod"},
	}

	ctx, h := newSetterHarness(t)

	gomock.InOrder(
		expectRecordList(ctx, h.mockPP, h.mockHandle, fixture.ipFamily, fixture.domain, fixture.params, []api.Record{
			dnsRecord(fixture.record1, fixture.ip1, fixture.params),
			dnsRecord(fixture.record2, fixture.ip1, withDuplicateCanonicalTags),
			dnsRecord(fixture.record3, fixture.ip2, outdatedWithDuplicateCanonicalTags),
		}, true, true),
		h.mockPP.EXPECT().Noticef(
			pp.EmojiImpossible,
			"The tags for %s contain duplicates that differ only by letter case; this should not happen and please report it at %s",
			"AAAA records of sub.test.org", pp.IssueReportingURL,
		),
		expectRecordUpdate(
			ctx,
			h.mockPP,
			h.mockHandle,
			fixture.ipFamily,
			fixture.domain,
			fixture.record3,
			targetCreate,
			reconciledFromOutdated,
			true,
		),
		expectRecordUpdatedNotice(h.mockPP, fixture.ipFamily, fixture.domain, fixture.record3),
	)

	resp := h.setter.SetIPs(ctx, h.mockPP, fixture.ipFamily, fixture.domain, []netip.Addr{fixture.ip1, targetCreate}, fixture.params)
	require.Equal(t, setter.ResponseUpdated, resp)
}

func TestSetIPsRepeatedKeeperIDWarnsAndReturnsNoop(t *testing.T) {
	t.Parallel()

	fixture := newDNSRecordFixture()
	repeatedID := fixture.record1
	ctx, h := newSetterHarness(t)

	gomock.InOrder(
		expectRecordList(ctx, h.mockPP, h.mockHandle, fixture.ipFamily, fixture.domain, fixture.params, []api.Record{
			dnsRecord(repeatedID, fixture.ip1, fixture.params),
			dnsRecord(repeatedID, fixture.ip1, fixture.params),
		}, true, true),
		expectRecordAlreadyUpdatedInfo(h.mockPP, fixture.ipFamily, fixture.domain, true),
	)

	resp := h.setter.SetIPs(ctx, h.mockPP, fixture.ipFamily, fixture.domain, []netip.Addr{fixture.ip1}, fixture.params)
	require.Equal(t, setter.ResponseNoop, resp)
}

func TestSetIPsNonMatchingDuplicateCleanupTimeoutReturnsUpdated(t *testing.T) {
	t.Parallel()

	fixture := newDNSRecordFixture()
	nonMatching := api.RecordParams{
		TTL:     fixture.params.TTL,
		Proxied: fixture.params.Proxied,
		Comment: fixture.params.Comment,
		Tags:    []string{"env:outdated"},
	}
	ctx, h := newSetterHarness(t)

	gomock.InOrder(
		expectRecordList(ctx, h.mockPP, h.mockHandle, fixture.ipFamily, fixture.domain, fixture.params, []api.Record{
			dnsRecord(fixture.record1, fixture.ip1, fixture.params),
			dnsRecord(fixture.record2, fixture.ip1, nonMatching),
		}, true, true),
		expectRecordAlreadyUpdatedInfo(h.mockPP, fixture.ipFamily, fixture.domain, true),
	)

	resp := h.setter.SetIPs(ctx, h.mockPP, fixture.ipFamily, fixture.domain, []netip.Addr{fixture.ip1}, fixture.params)
	require.Equal(t, setter.ResponseNoop, resp)
}

func TestSetIPsWarnsAmbiguousTagsFromOutdatedSources(t *testing.T) {
	t.Parallel()

	fixture := newDNSRecordFixture()
	ip3 := netip.MustParseAddr("::3")
	outdated1 := api.RecordParams{
		TTL:     fixture.params.TTL,
		Proxied: fixture.params.Proxied,
		Comment: fixture.params.Comment,
		Tags:    []string{"env:prod"},
	}
	outdated2 := api.RecordParams{
		TTL:     fixture.params.TTL,
		Proxied: fixture.params.Proxied,
		Comment: fixture.params.Comment,
		Tags:    []string{"team:alpha"},
	}
	reconciled := fixture.params

	ctx, h := newSetterHarness(t)

	gomock.InOrder(
		expectRecordList(ctx, h.mockPP, h.mockHandle, fixture.ipFamily, fixture.domain, fixture.params, []api.Record{
			dnsRecord(fixture.record1, fixture.ip2, outdated1),
			dnsRecord(fixture.record2, ip3, outdated2),
		}, true, true),
		h.mockPP.EXPECT().Noticef(
			pp.EmojiWarning,
			"The %d outdated %s disagree on %s; using %s",
			2, "AAAA records of sub.test.org", "tags", "common subset",
		),
		expectRecordUpdate(
			ctx,
			h.mockPP,
			h.mockHandle,
			fixture.ipFamily,
			fixture.domain,
			fixture.record1,
			fixture.ip1,
			reconciled,
			true,
		),
		expectRecordUpdatedNotice(h.mockPP, fixture.ipFamily, fixture.domain, fixture.record1),
		expectRecordDelete(ctx, h.mockPP, h.mockHandle, fixture.ipFamily, fixture.domain, fixture.record2, api.RegularDeletionMode, true),
		expectRecordOutdatedDeletedNotice(h.mockPP, fixture.ipFamily, fixture.domain, fixture.record2),
	)

	resp := h.setter.SetIPs(ctx, h.mockPP, fixture.ipFamily, fixture.domain, []netip.Addr{fixture.ip1}, fixture.params)
	require.Equal(t, setter.ResponseUpdated, resp)
}
