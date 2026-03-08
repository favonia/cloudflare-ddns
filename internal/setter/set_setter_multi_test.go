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
					expectRecordList(ctx, p, h, fixture.ipNetwork, fixture.domain, fixture.params, []api.Record{}, true, true),
					expectRecordAlreadyUpdatedInfo(p, fixture.ipNetwork, fixture.domain, true),
				)
			},
		},
		{
			name: "one-target/keep-record/response-noop-uncached",
			ips:  []netip.Addr{fixture.ip1},
			resp: setter.ResponseNoop,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					expectRecordList(ctx, p, h, fixture.ipNetwork, fixture.domain, fixture.params, []api.Record{
						dnsRecord(fixture.record1, fixture.ip1, fixture.params),
					}, false, true),
					expectRecordAlreadyUpdatedInfo(p, fixture.ipNetwork, fixture.domain, false),
				)
			},
		},
		{
			name: "zero-targets/delete-all-existing/response-updated",
			ips:  []netip.Addr{},
			resp: setter.ResponseUpdated,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					expectRecordList(ctx, p, h, fixture.ipNetwork, fixture.domain, fixture.params, []api.Record{
						dnsRecord(fixture.record1, fixture.ip1, fixture.params),
						dnsRecord(fixture.record2, fixture.ip2, fixture.params),
					}, true, true),
					expectRecordDelete(ctx, p, h, fixture.ipNetwork, fixture.domain, fixture.record1, api.RegularDelitionMode, true),
					expectRecordStaleDeletedNotice(p, fixture.ipNetwork, fixture.domain, fixture.record1),
					expectRecordDelete(ctx, p, h, fixture.ipNetwork, fixture.domain, fixture.record2, api.RegularDelitionMode, true),
					expectRecordStaleDeletedNotice(p, fixture.ipNetwork, fixture.domain, fixture.record2),
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
					expectRecordList(ctx, p, h, fixture.ipNetwork, fixture.domain, fixture.params, []api.Record{
						dnsRecord(fixture.record1, fixture.ip1, fixture.params),
						dnsRecord(fixture.record2, fixture.ip1, fixture.params),
						dnsRecord(fixture.record3, ip4, fixture.params),
					}, true, true),
					expectRecordUpdate(
						ctx,
						p,
						h,
						fixture.ipNetwork,
						fixture.domain,
						fixture.record3,
						fixture.ip2,
						fixture.params,
						fixture.params,
						true,
					),
					expectRecordUpdatedNotice(p, fixture.ipNetwork, fixture.domain, fixture.record3),
					expectRecordCreate(ctx, p, h, fixture.ipNetwork, fixture.domain, ip3, fixture.params, record4, true),
					expectRecordAddedNotice(p, fixture.ipNetwork, fixture.domain, record4),
					expectRecordDelete(ctx, p, h, fixture.ipNetwork, fixture.domain, fixture.record2, api.RegularDelitionMode, true),
					expectRecordDuplicateDeletedNotice(p, fixture.ipNetwork, fixture.domain, fixture.record2),
				)
			},
		},
		{
			name: "many-targets/delete-leftover-stale-fails/response-failed",
			ips:  []netip.Addr{fixture.ip1, fixture.ip2},
			resp: setter.ResponseFailed,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					expectRecordList(ctx, p, h, fixture.ipNetwork, fixture.domain, fixture.params, []api.Record{
						dnsRecord(fixture.record1, ip4, fixture.params),
						dnsRecord(fixture.record2, ip5, fixture.params),
						dnsRecord(fixture.record3, ip6, fixture.params),
					}, true, true),
					expectRecordUpdate(
						ctx,
						p,
						h,
						fixture.ipNetwork,
						fixture.domain,
						fixture.record1,
						fixture.ip1,
						fixture.params,
						fixture.params,
						true,
					),
					expectRecordUpdatedNotice(p, fixture.ipNetwork, fixture.domain, fixture.record1),
					expectRecordUpdate(
						ctx,
						p,
						h,
						fixture.ipNetwork,
						fixture.domain,
						fixture.record2,
						fixture.ip2,
						fixture.params,
						fixture.params,
						true,
					),
					expectRecordUpdatedNotice(p, fixture.ipNetwork, fixture.domain, fixture.record2),
					expectRecordDelete(ctx, p, h, fixture.ipNetwork, fixture.domain, fixture.record3, api.RegularDelitionMode, false),
					expectRecordSetFailedNotice(p, fixture.ipNetwork, fixture.domain),
				)
			},
		},
		{
			name: "many-targets/duplicate-cleanup-timeout/response-updated",
			ips:  []netip.Addr{fixture.ip1},
			resp: setter.ResponseUpdated,
			prepareMocks: func(ctx context.Context, cancel func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					expectRecordList(ctx, p, h, fixture.ipNetwork, fixture.domain, fixture.params, []api.Record{
						dnsRecord(fixture.record1, fixture.ip1, fixture.params),
						dnsRecord(fixture.record2, fixture.ip1, fixture.params),
					}, true, true),
					h.EXPECT().DeleteRecord(
						ctx, p, fixture.ipNetwork, fixture.domain, fixture.record2, api.RegularDelitionMode).
						Do(wrapCancelAsDelete(cancel)).Return(false),
				)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx, h := newSetterHarness(t)
			h.prepare(ctx, tc.prepareMocks)

			resp := h.setter.SetIPs(ctx, h.mockPP, fixture.ipNetwork, fixture.domain, tc.ips, fixture.params)
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
		expectRecordList(ctx, h.mockPP, h.mockHandle, fixture.ipNetwork, fixture.domain, fixture.params, []api.Record{
			dnsRecord(fixture.record1, fixture.ip1, fixture.params),
			dnsRecord(fixture.record2, fixture.ip1, inherited),
		}, true, true),
		expectRecordCreate(ctx, h.mockPP, h.mockHandle, fixture.ipNetwork, fixture.domain, targetCreate, fixture.params, record4, true),
		expectRecordAddedNotice(h.mockPP, fixture.ipNetwork, fixture.domain, record4),
		h.mockPP.EXPECT().Noticef(
			pp.EmojiWarning,
			"Metadata reconciliation for %s field %q is ambiguous across %d candidates; using %s",
			"AAAA records of sub.test.org", "tags", 2, "common subset",
		),
		h.mockPP.EXPECT().Noticef(
			pp.EmojiWarning,
			"Metadata reconciliation for %s field %q is ambiguous across %d candidates; using %s",
			"AAAA records of sub.test.org", "ttl", 2, "configured value",
		),
		h.mockPP.EXPECT().Noticef(
			pp.EmojiWarning,
			"Metadata reconciliation for %s field %q is ambiguous across %d candidates; using %s",
			"AAAA records of sub.test.org", "proxied", 2, "configured value",
		),
		h.mockPP.EXPECT().Noticef(
			pp.EmojiWarning,
			"Metadata reconciliation for %s field %q is ambiguous across %d candidates; using %s",
			"AAAA records of sub.test.org", "comment", 2, "configured value",
		),
		expectRecordDelete(ctx, h.mockPP, h.mockHandle, fixture.ipNetwork, fixture.domain, fixture.record2, api.RegularDelitionMode, true),
		expectRecordDuplicateDeletedNotice(h.mockPP, fixture.ipNetwork, fixture.domain, fixture.record2),
	)

	resp := h.setter.SetIPs(ctx, h.mockPP, fixture.ipNetwork, fixture.domain, []netip.Addr{fixture.ip1, targetCreate}, fixture.params)
	require.Equal(t, setter.ResponseUpdated, resp)
}

func TestSetIPsDuplicateKeeperUsesLowestID(t *testing.T) {
	t.Parallel()

	fixture := newDNSRecordFixture()
	ctx, h := newSetterHarness(t)

	gomock.InOrder(
		expectRecordList(ctx, h.mockPP, h.mockHandle, fixture.ipNetwork, fixture.domain, fixture.params, []api.Record{
			dnsRecord(fixture.record2, fixture.ip1, fixture.params),
			dnsRecord(fixture.record1, fixture.ip1, fixture.params),
		}, true, true),
		expectRecordDelete(ctx, h.mockPP, h.mockHandle, fixture.ipNetwork, fixture.domain, fixture.record2, api.RegularDelitionMode, true),
		expectRecordDuplicateDeletedNotice(h.mockPP, fixture.ipNetwork, fixture.domain, fixture.record2),
	)

	resp := h.setter.SetIPs(ctx, h.mockPP, fixture.ipNetwork, fixture.domain, []netip.Addr{fixture.ip1}, fixture.params)
	require.Equal(t, setter.ResponseUpdated, resp)
}

func TestSetIPsDuplicateKeeperUsesLowestIDWithinMetadataMatchingSubset(t *testing.T) {
	t.Parallel()

	fixture := newDNSRecordFixture()
	record0 := api.ID("record0")
	ctx, h := newSetterHarness(t)

	nonMatching := fixture.params
	nonMatching.Comment = "foreign"

	gomock.InOrder(
		expectRecordList(ctx, h.mockPP, h.mockHandle, fixture.ipNetwork, fixture.domain, fixture.params, []api.Record{
			dnsRecord(record0, fixture.ip1, nonMatching),
			dnsRecord(fixture.record3, fixture.ip1, fixture.params),
			dnsRecord(fixture.record2, fixture.ip1, fixture.params),
		}, true, true),
		h.mockPP.EXPECT().Noticef(
			pp.EmojiWarning,
			"Metadata reconciliation for %s field %q is ambiguous across %d candidates; using %s",
			"AAAA records of sub.test.org", "comment", 3, "configured value",
		),
		expectRecordDelete(ctx, h.mockPP, h.mockHandle, fixture.ipNetwork, fixture.domain, record0, api.RegularDelitionMode, true),
		expectRecordDuplicateDeletedNotice(h.mockPP, fixture.ipNetwork, fixture.domain, record0),
		expectRecordDelete(ctx, h.mockPP, h.mockHandle, fixture.ipNetwork, fixture.domain, fixture.record3, api.RegularDelitionMode, true),
		expectRecordDuplicateDeletedNotice(h.mockPP, fixture.ipNetwork, fixture.domain, fixture.record3),
	)

	resp := h.setter.SetIPs(ctx, h.mockPP, fixture.ipNetwork, fixture.domain, []netip.Addr{fixture.ip1}, fixture.params)
	require.Equal(t, setter.ResponseUpdated, resp)
}

func TestSetIPsStaleOperationsBeforeMatchedUpdateAndDelete(t *testing.T) {
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
		expectRecordList(ctx, h.mockPP, h.mockHandle, fixture.ipNetwork, fixture.domain, fixture.params, []api.Record{
			dnsRecord(fixture.record1, fixture.ip1, nonMatchingA),
			dnsRecord(fixture.record2, fixture.ip1, nonMatchingB),
			dnsRecord(fixture.record3, ip3, fixture.params),
		}, true, true),
		expectRecordUpdate(
			ctx,
			h.mockPP,
			h.mockHandle,
			fixture.ipNetwork,
			fixture.domain,
			fixture.record3,
			fixture.ip2,
			fixture.params,
			fixture.params,
			true,
		),
		expectRecordUpdatedNotice(h.mockPP, fixture.ipNetwork, fixture.domain, fixture.record3),
		h.mockPP.EXPECT().Noticef(
			pp.EmojiWarning,
			"Metadata reconciliation for %s field %q is ambiguous across %d candidates; using %s",
			"AAAA records of sub.test.org", "ttl", 2, "configured value",
		),
		h.mockPP.EXPECT().Noticef(
			pp.EmojiWarning,
			"Metadata reconciliation for %s field %q is ambiguous across %d candidates; using %s",
			"AAAA records of sub.test.org", "comment", 2, "configured value",
		),
		expectRecordUpdate(
			ctx,
			h.mockPP,
			h.mockHandle,
			fixture.ipNetwork,
			fixture.domain,
			fixture.record1,
			fixture.ip1,
			nonMatchingA,
			api.RecordParams{
				TTL:     fixture.params.TTL,
				Proxied: true,
				Comment: fixture.params.Comment,
				Tags:    fixture.params.Tags,
			},
			true,
		),
		expectRecordMatchedUpdatedNotice(h.mockPP, fixture.ipNetwork, fixture.domain, fixture.record1),
		expectRecordDelete(ctx, h.mockPP, h.mockHandle, fixture.ipNetwork, fixture.domain, fixture.record2, api.RegularDelitionMode, true),
		expectRecordDuplicateDeletedNotice(h.mockPP, fixture.ipNetwork, fixture.domain, fixture.record2),
	)

	resp := h.setter.SetIPs(ctx, h.mockPP, fixture.ipNetwork, fixture.domain, []netip.Addr{fixture.ip1, fixture.ip2}, fixture.params)
	require.Equal(t, setter.ResponseUpdated, resp)
}

func TestSetIPsStaleDeleteBeforeMatchedUpdate(t *testing.T) {
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
		expectRecordList(ctx, h.mockPP, h.mockHandle, fixture.ipNetwork, fixture.domain, fixture.params, []api.Record{
			dnsRecord(fixture.record1, fixture.ip1, nonMatchingA),
			dnsRecord(fixture.record2, fixture.ip1, nonMatchingB),
			dnsRecord(fixture.record3, ip3, fixture.params),
		}, true, true),
		expectRecordDelete(ctx, h.mockPP, h.mockHandle, fixture.ipNetwork, fixture.domain, fixture.record3, api.RegularDelitionMode, true),
		expectRecordStaleDeletedNotice(h.mockPP, fixture.ipNetwork, fixture.domain, fixture.record3),
		h.mockPP.EXPECT().Noticef(
			pp.EmojiWarning,
			"Metadata reconciliation for %s field %q is ambiguous across %d candidates; using %s",
			"AAAA records of sub.test.org", "ttl", 2, "configured value",
		),
		h.mockPP.EXPECT().Noticef(
			pp.EmojiWarning,
			"Metadata reconciliation for %s field %q is ambiguous across %d candidates; using %s",
			"AAAA records of sub.test.org", "comment", 2, "configured value",
		),
		expectRecordUpdate(
			ctx,
			h.mockPP,
			h.mockHandle,
			fixture.ipNetwork,
			fixture.domain,
			fixture.record1,
			fixture.ip1,
			nonMatchingA,
			api.RecordParams{
				TTL:     fixture.params.TTL,
				Proxied: true,
				Comment: fixture.params.Comment,
				Tags:    fixture.params.Tags,
			},
			true,
		),
		expectRecordMatchedUpdatedNotice(h.mockPP, fixture.ipNetwork, fixture.domain, fixture.record1),
		expectRecordDelete(ctx, h.mockPP, h.mockHandle, fixture.ipNetwork, fixture.domain, fixture.record2, api.RegularDelitionMode, true),
		expectRecordDuplicateDeletedNotice(h.mockPP, fixture.ipNetwork, fixture.domain, fixture.record2),
	)

	resp := h.setter.SetIPs(ctx, h.mockPP, fixture.ipNetwork, fixture.domain, []netip.Addr{fixture.ip1}, fixture.params)
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
		expectRecordList(ctx, h.mockPP, h.mockHandle, fixture.ipNetwork, fixture.domain, fixture.params, []api.Record{
			dnsRecord(fixture.record3, fixture.ip1, nonMatchingA),
			dnsRecord(fixture.record2, fixture.ip1, fixture.params),
			dnsRecord(fixture.record1, fixture.ip1, fixture.params),
			dnsRecord(record6, fixture.ip2, nonMatchingB),
			dnsRecord(record5, fixture.ip2, fixture.params),
			dnsRecord(record4, fixture.ip2, fixture.params),
		}, true, true),
		h.mockPP.EXPECT().Noticef(
			pp.EmojiWarning,
			"Metadata reconciliation for %s field %q is ambiguous across %d candidates; using %s",
			"AAAA records of sub.test.org", "comment", 3, "configured value",
		),
		expectRecordDelete(ctx, h.mockPP, h.mockHandle, fixture.ipNetwork, fixture.domain, fixture.record3, api.RegularDelitionMode, true),
		expectRecordDuplicateDeletedNotice(h.mockPP, fixture.ipNetwork, fixture.domain, fixture.record3),
		expectRecordDelete(ctx, h.mockPP, h.mockHandle, fixture.ipNetwork, fixture.domain, record6, api.RegularDelitionMode, true),
		expectRecordDuplicateDeletedNotice(h.mockPP, fixture.ipNetwork, fixture.domain, record6),
		expectRecordDelete(ctx, h.mockPP, h.mockHandle, fixture.ipNetwork, fixture.domain, fixture.record2, api.RegularDelitionMode, true),
		expectRecordDuplicateDeletedNotice(h.mockPP, fixture.ipNetwork, fixture.domain, fixture.record2),
		expectRecordDelete(ctx, h.mockPP, h.mockHandle, fixture.ipNetwork, fixture.domain, record5, api.RegularDelitionMode, true),
		expectRecordDuplicateDeletedNotice(h.mockPP, fixture.ipNetwork, fixture.domain, record5),
	)

	resp := h.setter.SetIPs(ctx, h.mockPP, fixture.ipNetwork, fixture.domain, []netip.Addr{fixture.ip1, fixture.ip2}, fixture.params)
	require.Equal(t, setter.ResponseUpdated, resp)
}

func TestSetIPsStaleRecycleUsesLowestIDTieBreak(t *testing.T) {
	t.Parallel()

	fixture := newDNSRecordFixture()
	ip3 := netip.MustParseAddr("::3")
	ip4 := netip.MustParseAddr("::4")

	ctx, h := newSetterHarness(t)

	gomock.InOrder(
		expectRecordList(ctx, h.mockPP, h.mockHandle, fixture.ipNetwork, fixture.domain, fixture.params, []api.Record{
			dnsRecord(fixture.record2, ip3, fixture.params),
			dnsRecord(fixture.record1, ip4, fixture.params),
		}, true, true),
		expectRecordUpdate(
			ctx,
			h.mockPP,
			h.mockHandle,
			fixture.ipNetwork,
			fixture.domain,
			fixture.record1,
			fixture.ip1,
			fixture.params,
			fixture.params,
			true,
		),
		expectRecordUpdatedNotice(h.mockPP, fixture.ipNetwork, fixture.domain, fixture.record1),
		expectRecordUpdate(
			ctx,
			h.mockPP,
			h.mockHandle,
			fixture.ipNetwork,
			fixture.domain,
			fixture.record2,
			fixture.ip2,
			fixture.params,
			fixture.params,
			true,
		),
		expectRecordUpdatedNotice(h.mockPP, fixture.ipNetwork, fixture.domain, fixture.record2),
	)

	resp := h.setter.SetIPs(ctx, h.mockPP, fixture.ipNetwork, fixture.domain, []netip.Addr{fixture.ip1, fixture.ip2}, fixture.params)
	require.Equal(t, setter.ResponseUpdated, resp)
}

func TestSetIPsRecycleUsesReconciledStaleMetadata(t *testing.T) {
	t.Parallel()

	fixture := newDNSRecordFixture()
	reconciled := api.RecordParams{
		TTL:     300,
		Proxied: true,
		Comment: "from-stale",
		Tags:    []string{"env:prod", "Team:Alpha"},
	}

	ctx, h := newSetterHarness(t)

	gomock.InOrder(
		expectRecordList(ctx, h.mockPP, h.mockHandle, fixture.ipNetwork, fixture.domain, fixture.params, []api.Record{
			dnsRecord(fixture.record1, fixture.ip2, reconciled),
		}, true, true),
		expectRecordUpdate(
			ctx,
			h.mockPP,
			h.mockHandle,
			fixture.ipNetwork,
			fixture.domain,
			fixture.record1,
			fixture.ip1,
			reconciled,
			reconciled,
			true,
		),
		expectRecordUpdatedNotice(h.mockPP, fixture.ipNetwork, fixture.domain, fixture.record1),
	)

	resp := h.setter.SetIPs(ctx, h.mockPP, fixture.ipNetwork, fixture.domain, []netip.Addr{fixture.ip1}, fixture.params)
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
		expectRecordList(ctx, h.mockPP, h.mockHandle, fixture.ipNetwork, fixture.domain, fixture.params, []api.Record{
			dnsRecord(fixture.record1, fixture.ip1, fixture.params),
			dnsRecord(fixture.record2, fixture.ip1, withDuplicateCanonicalTags),
		}, true, true),
		expectRecordCreate(ctx, h.mockPP, h.mockHandle, fixture.ipNetwork, fixture.domain, targetCreate, fixture.params, record4, true),
		expectRecordAddedNotice(h.mockPP, fixture.ipNetwork, fixture.domain, record4),
		h.mockPP.EXPECT().Noticef(
			pp.EmojiImpossible,
			"Found duplicate canonical tags in metadata reconciliation for %s field %q; this should not happen and please report it at %s",
			"AAAA records of sub.test.org", "tags", pp.IssueReportingURL,
		),
		h.mockPP.EXPECT().Noticef(
			pp.EmojiWarning,
			"Metadata reconciliation for %s field %q is ambiguous across %d candidates; using %s",
			"AAAA records of sub.test.org", "tags", 2, "common subset",
		),
		expectRecordDelete(ctx, h.mockPP, h.mockHandle, fixture.ipNetwork, fixture.domain, fixture.record2, api.RegularDelitionMode, true),
		expectRecordDuplicateDeletedNotice(h.mockPP, fixture.ipNetwork, fixture.domain, fixture.record2),
	)

	resp := h.setter.SetIPs(ctx, h.mockPP, fixture.ipNetwork, fixture.domain, []netip.Addr{fixture.ip1, targetCreate}, fixture.params)
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
	staleWithDuplicateCanonicalTags := api.RecordParams{
		TTL:     fixture.params.TTL,
		Proxied: fixture.params.Proxied,
		Comment: fixture.params.Comment,
		Tags:    []string{"env:prod", "env:prod"},
	}
	reconciledFromStale := api.RecordParams{
		TTL:     fixture.params.TTL,
		Proxied: fixture.params.Proxied,
		Comment: fixture.params.Comment,
		Tags:    []string{"env:prod"},
	}

	ctx, h := newSetterHarness(t)

	gomock.InOrder(
		expectRecordList(ctx, h.mockPP, h.mockHandle, fixture.ipNetwork, fixture.domain, fixture.params, []api.Record{
			dnsRecord(fixture.record1, fixture.ip1, fixture.params),
			dnsRecord(fixture.record2, fixture.ip1, withDuplicateCanonicalTags),
			dnsRecord(fixture.record3, fixture.ip2, staleWithDuplicateCanonicalTags),
		}, true, true),
		h.mockPP.EXPECT().Noticef(
			pp.EmojiImpossible,
			"Found duplicate canonical tags in metadata reconciliation for %s field %q; this should not happen and please report it at %s",
			"AAAA records of sub.test.org", "tags", pp.IssueReportingURL,
		),
		expectRecordUpdate(
			ctx,
			h.mockPP,
			h.mockHandle,
			fixture.ipNetwork,
			fixture.domain,
			fixture.record3,
			targetCreate,
			staleWithDuplicateCanonicalTags,
			reconciledFromStale,
			true,
		),
		expectRecordUpdatedNotice(h.mockPP, fixture.ipNetwork, fixture.domain, fixture.record3),
		h.mockPP.EXPECT().Noticef(
			pp.EmojiImpossible,
			"Found duplicate canonical tags in metadata reconciliation for %s field %q; this should not happen and please report it at %s",
			"AAAA records of sub.test.org", "tags", pp.IssueReportingURL,
		),
		h.mockPP.EXPECT().Noticef(
			pp.EmojiWarning,
			"Metadata reconciliation for %s field %q is ambiguous across %d candidates; using %s",
			"AAAA records of sub.test.org", "tags", 2, "common subset",
		),
		expectRecordDelete(ctx, h.mockPP, h.mockHandle, fixture.ipNetwork, fixture.domain, fixture.record2, api.RegularDelitionMode, true),
		expectRecordDuplicateDeletedNotice(h.mockPP, fixture.ipNetwork, fixture.domain, fixture.record2),
	)

	resp := h.setter.SetIPs(ctx, h.mockPP, fixture.ipNetwork, fixture.domain, []netip.Addr{fixture.ip1, targetCreate}, fixture.params)
	require.Equal(t, setter.ResponseUpdated, resp)
}

func TestSetIPsRepeatedKeeperIDWarnsAndReturnsNoop(t *testing.T) {
	t.Parallel()

	fixture := newDNSRecordFixture()
	repeatedID := fixture.record1
	ctx, h := newSetterHarness(t)

	gomock.InOrder(
		expectRecordList(ctx, h.mockPP, h.mockHandle, fixture.ipNetwork, fixture.domain, fixture.params, []api.Record{
			dnsRecord(repeatedID, fixture.ip1, fixture.params),
			dnsRecord(repeatedID, fixture.ip1, fixture.params),
		}, true, true),
		h.mockPP.EXPECT().Noticef(
			pp.EmojiImpossible,
			"Found repeated managed record ID %s among %s records of %s; skipping duplicate deletion for this impossible case",
			repeatedID,
			fixture.ipNetwork.RecordType(),
			fixture.domain.Describe(),
		),
	)

	resp := h.setter.SetIPs(ctx, h.mockPP, fixture.ipNetwork, fixture.domain, []netip.Addr{fixture.ip1}, fixture.params)
	require.Equal(t, setter.ResponseNoop, resp)
}

func TestSetIPsWarnsAmbiguousTagsFromStaleSources(t *testing.T) {
	t.Parallel()

	fixture := newDNSRecordFixture()
	ip3 := netip.MustParseAddr("::3")
	stale1 := api.RecordParams{
		TTL:     fixture.params.TTL,
		Proxied: fixture.params.Proxied,
		Comment: fixture.params.Comment,
		Tags:    []string{"env:prod"},
	}
	stale2 := api.RecordParams{
		TTL:     fixture.params.TTL,
		Proxied: fixture.params.Proxied,
		Comment: fixture.params.Comment,
		Tags:    []string{"team:alpha"},
	}
	reconciled := fixture.params

	ctx, h := newSetterHarness(t)

	gomock.InOrder(
		expectRecordList(ctx, h.mockPP, h.mockHandle, fixture.ipNetwork, fixture.domain, fixture.params, []api.Record{
			dnsRecord(fixture.record1, fixture.ip2, stale1),
			dnsRecord(fixture.record2, ip3, stale2),
		}, true, true),
		h.mockPP.EXPECT().Noticef(
			pp.EmojiWarning,
			"Metadata reconciliation for %s field %q is ambiguous across %d candidates; using %s",
			"AAAA records of sub.test.org", "tags", 2, "common subset",
		),
		expectRecordUpdate(
			ctx,
			h.mockPP,
			h.mockHandle,
			fixture.ipNetwork,
			fixture.domain,
			fixture.record1,
			fixture.ip1,
			stale1,
			reconciled,
			true,
		),
		expectRecordUpdatedNotice(h.mockPP, fixture.ipNetwork, fixture.domain, fixture.record1),
		expectRecordDelete(ctx, h.mockPP, h.mockHandle, fixture.ipNetwork, fixture.domain, fixture.record2, api.RegularDelitionMode, true),
		expectRecordStaleDeletedNotice(h.mockPP, fixture.ipNetwork, fixture.domain, fixture.record2),
	)

	resp := h.setter.SetIPs(ctx, h.mockPP, fixture.ipNetwork, fixture.domain, []netip.Addr{fixture.ip1}, fixture.params)
	require.Equal(t, setter.ResponseUpdated, resp)
}
