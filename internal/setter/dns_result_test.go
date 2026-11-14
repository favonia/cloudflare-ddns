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
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/setter"
)

func TestDNSResultBeforeSurvivesMutation(t *testing.T) {
	t.Parallel()
	ctx, h := newSetterHarness(t)
	f := newDNSRecordFixture()
	rs := []api.Record{dnsRecord(f.record1, f.ip1, f.params)}
	h.mockHandle.EXPECT().ListRecords(ctx, h.mockPP, f.ipFamily, f.domain, f.params).Return(rs, true, true)
	h.mockHandle.EXPECT().UpdateRecord(ctx, h.mockPP, f.ipFamily, f.domain, f.record1, f.ip2, f.params).DoAndReturn(func(context.Context, pp.PP, ipnet.Family, domain.Domain, api.ID, netip.Addr, api.RecordParams) api.RecordMutation {
		rs[0].IP = f.ip2
		return api.RecordMutation{Status: api.MutationCompleted, Kind: api.RecordUpdate, Record: api.RecordFact{ID: f.record1, IP: f.ip2}, Membership: api.MembershipManaged}
	})
	h.mockPP.EXPECT().Noticef(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
	result := h.setter.SetIPs(ctx, h.mockPP, f.ipFamily, f.domain, []netip.Addr{f.ip2}, f.params)
	require.Equal(t, setter.ResponseUpdated, result.Code)
	require.Equal(t, &setter.DNSSnapshot{Cached: true, Records: []api.RecordFact{{ID: f.record1, IP: f.ip1}}}, result.Before)
	require.Len(t, result.Effects, 1)
	requireDNSProgress(t, &setter.DNSProgress{Satisfied: nil, Operations: []setter.DNSOperation{{Kind: api.RecordUpdate, Record: api.RecordFact{ID: f.record1, IP: f.ip2}, Status: setter.DNSOperationCompleted, Membership: api.MembershipManaged}}}, result.Progress)
	clear(rs)
	require.Equal(t, f.ip2, result.Progress.Operations[0].Record.IP)
}

func TestDNSResultReadFailureUnknown(t *testing.T) {
	t.Parallel()
	for _, cleanup := range []bool{false, true} {
		t.Run(map[bool]string{false: "update", true: "cleanup"}[cleanup], func(t *testing.T) {
			t.Parallel()
			ctx, h := newSetterHarness(t)
			f := newDNSRecordFixture()
			h.mockHandle.EXPECT().ListRecords(ctx, h.mockPP, f.ipFamily, f.domain, f.params).Return(nil, false, false)
			var result setter.DNSResult
			if cleanup {
				result = h.setter.FinalDelete(ctx, h.mockPP, f.ipFamily, f.domain, f.params)
			} else {
				result = h.setter.SetIPs(ctx, h.mockPP, f.ipFamily, f.domain, []netip.Addr{f.ip1}, f.params)
			}
			require.Equal(t, setter.ResponseFailed, result.Code)
			require.Nil(t, result.Before)
			require.Nil(t, result.Progress)
			require.Empty(t, result.Effects)
		})
	}
}

func TestDNSResultPartialEffects(t *testing.T) {
	t.Parallel()
	for _, status := range []api.MutationStatus{api.MutationUnconfirmed, api.MutationNoEffect} {
		t.Run(map[api.MutationStatus]string{api.MutationUnconfirmed: "unconfirmed", api.MutationNoEffect: "prerequisite"}[status], func(t *testing.T) {
			t.Parallel()
			ctx, h := newSetterHarness(t)
			f := newDNSRecordFixture()
			third := netip.MustParseAddr("::3")
			h.mockHandle.EXPECT().ListRecords(ctx, h.mockPP, f.ipFamily, f.domain, f.params).Return([]api.Record{}, false, true)
			first := api.RecordMutation{Status: api.MutationCompleted, Kind: api.RecordCreate, Record: api.RecordFact{ID: f.record1, IP: f.ip1}, Membership: api.MembershipUnmanaged}
			failed := api.RecordMutation{Status: status, Kind: api.RecordCreate, Record: api.RecordFact{ID: "", IP: f.ip2}, Membership: api.MembershipUnknown}
			gomock.InOrder(
				h.mockHandle.EXPECT().CreateRecord(ctx, h.mockPP, f.ipFamily, f.domain, f.ip1, f.params).Return(first),
				h.mockPP.EXPECT().Noticef(pp.EmojiCreation, "Added a new %s record for %s (ID: %s)", f.ipFamily.RecordType(), f.domain.Describe(), f.record1),
				h.mockHandle.EXPECT().CreateRecord(ctx, h.mockPP, f.ipFamily, f.domain, f.ip2, f.params).Return(failed),
			)
			result := h.setter.SetIPs(ctx, h.mockPP, f.ipFamily, f.domain, []netip.Addr{f.ip1, f.ip2, third}, f.params)
			require.Equal(t, setter.ResponseFailed, result.Code)
			require.NotNil(t, result.Before)
			require.NotNil(t, result.Before.Records)
			require.Empty(t, result.Before.Records)
			expected := []api.RecordMutation{first}
			if status != api.MutationNoEffect {
				expected = append(expected, failed)
			}
			require.Equal(t, expected, result.Effects)
			failedStatus := setter.DNSOperationUnconfirmed
			if status == api.MutationNoEffect {
				failedStatus = setter.DNSOperationBlocked
			}
			requireDNSProgress(t, &setter.DNSProgress{Satisfied: nil, Operations: []setter.DNSOperation{
				{Kind: api.RecordCreate, Record: api.RecordFact{ID: f.record1, IP: f.ip1}, Status: setter.DNSOperationCompleted, Membership: api.MembershipUnmanaged},
				{Kind: api.RecordCreate, Record: api.RecordFact{ID: "", IP: f.ip2}, Status: failedStatus, Membership: api.MembershipUnknown},
				{Kind: api.RecordCreate, Record: api.RecordFact{ID: "", IP: third}, Status: setter.DNSOperationNotAttempted, Membership: api.MembershipUnknown},
			}}, result.Progress)
		})
	}
}

func TestDNSResultCleanupEffects(t *testing.T) {
	t.Parallel()
	for _, cancel := range []bool{false, true} {
		t.Run(map[bool]string{false: "continues", true: "cancellation"}[cancel], func(t *testing.T) {
			t.Parallel()
			ctx, h := newSetterHarness(t)
			f := newDNSRecordFixture()
			rs := []api.Record{dnsRecord(f.record1, f.ip1, f.params), dnsRecord(f.record2, f.ip2, f.params)}
			h.mockHandle.EXPECT().ListRecords(ctx, h.mockPP, f.ipFamily, f.domain, f.params).Return(rs, true, true)
			failed := api.RecordMutation{Status: api.MutationUnconfirmed, Kind: api.RecordDelete, Record: api.RecordFact{ID: f.record1, IP: netip.Addr{}}, Membership: api.MembershipUnknown}
			h.mockHandle.EXPECT().DeleteRecord(ctx, h.mockPP, f.ipFamily, f.domain, f.record1, api.FinalDeletionMode).DoAndReturn(func(context.Context, pp.PP, ipnet.Family, domain.Domain, api.ID, api.DeletionMode) api.RecordMutation {
				if cancel {
					h.cancel()
				}
				return failed
			})
			effects := []api.RecordMutation{failed}
			if cancel {
				expectRecordDeleteTimeoutInfo(h.mockPP, f.ipFamily, f.domain)
			} else {
				completed := api.RecordMutation{Status: api.MutationCompleted, Kind: api.RecordDelete, Record: api.RecordFact{ID: f.record2, IP: netip.Addr{}}, Membership: api.MembershipUnknown}
				h.mockHandle.EXPECT().DeleteRecord(ctx, h.mockPP, f.ipFamily, f.domain, f.record2, api.FinalDeletionMode).DoAndReturn(func(context.Context, pp.PP, ipnet.Family, domain.Domain, api.ID, api.DeletionMode) api.RecordMutation {
					clear(rs)
					return completed
				})
				expectRecordOutdatedDeletedNotice(h.mockPP, f.ipFamily, f.domain, f.record2)
				effects = append(effects, completed)
			}
			result := h.setter.FinalDelete(ctx, h.mockPP, f.ipFamily, f.domain, f.params)
			require.Equal(t, setter.ResponseFailed, result.Code)
			require.Equal(t, effects, result.Effects)
			secondStatus := setter.DNSOperationCompleted
			if cancel {
				secondStatus = setter.DNSOperationNotAttempted
			}
			requireDNSProgress(t, &setter.DNSProgress{Satisfied: nil, Operations: []setter.DNSOperation{
				{Kind: api.RecordDelete, Record: api.RecordFact{ID: f.record1, IP: f.ip1}, Status: setter.DNSOperationUnconfirmed, Membership: api.MembershipUnknown},
				{Kind: api.RecordDelete, Record: api.RecordFact{ID: f.record2, IP: f.ip2}, Status: secondStatus, Membership: api.MembershipUnknown},
			}}, result.Progress)
			require.Equal(t, []api.RecordFact{{ID: f.record1, IP: f.ip1}, {ID: f.record2, IP: f.ip2}}, result.Before.Records)
		})
	}
}

// These cases catch conflating read-time coverage with pending write operations.
func TestDNSResultSatisfiedBesideFailure(t *testing.T) {
	t.Parallel()
	ctx, h := newSetterHarness(t)
	f := newDNSRecordFixture()
	third := netip.MustParseAddr("::3")
	rs := []api.Record{dnsRecord(f.record1, f.ip1, f.params)}
	targets := []netip.Addr{f.ip1, f.ip2, third}
	failed := api.RecordMutation{Status: api.MutationUnconfirmed, Kind: api.RecordCreate, Record: api.RecordFact{ID: "", IP: f.ip2}, Membership: api.MembershipManaged}
	gomock.InOrder(
		h.mockHandle.EXPECT().ListRecords(ctx, h.mockPP, f.ipFamily, f.domain, f.params).Return(rs, true, true),
		h.mockHandle.EXPECT().CreateRecord(ctx, h.mockPP, f.ipFamily, f.domain, f.ip2, f.params).Return(failed),
	)
	result := h.setter.SetIPs(ctx, h.mockPP, f.ipFamily, f.domain, targets, f.params)
	clear(rs)
	clear(targets)
	require.Equal(t, setter.ResponseFailed, result.Code)
	requireDNSProgress(t, &setter.DNSProgress{Satisfied: []netip.Addr{f.ip1}, Operations: []setter.DNSOperation{
		{Kind: api.RecordCreate, Record: api.RecordFact{ID: "", IP: f.ip2}, Status: setter.DNSOperationUnconfirmed, Membership: api.MembershipManaged},
		{Kind: api.RecordCreate, Record: api.RecordFact{ID: "", IP: third}, Status: setter.DNSOperationNotAttempted, Membership: api.MembershipUnknown},
	}}, result.Progress)
	require.Equal(t, []api.RecordMutation{failed}, result.Effects)
	require.Equal(t, &setter.DNSSnapshot{Cached: true, Records: []api.RecordFact{{ID: f.record1, IP: f.ip1}}}, result.Before)
}

func TestDNSResultKnownNoopProgress(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"satisfied", "empty-update", "empty-delete"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			ctx, h := newSetterHarness(t)
			f := newDNSRecordFixture()
			var rs []api.Record
			var targets []netip.Addr
			if mode == "satisfied" {
				// Reverse source order must not affect target coverage order.
				rs = []api.Record{dnsRecord(f.record2, f.ip2, f.params), dnsRecord(f.record1, f.ip1, f.params)}
				targets = []netip.Addr{f.ip1, f.ip2}
			}
			h.mockHandle.EXPECT().ListRecords(ctx, h.mockPP, f.ipFamily, f.domain, f.params).Return(rs, false, true)
			var result setter.DNSResult
			if mode == "empty-delete" {
				expectRecordAlreadyDeletedInfo(h.mockPP, f.ipFamily, f.domain, false)
				result = h.setter.FinalDelete(ctx, h.mockPP, f.ipFamily, f.domain, f.params)
			} else {
				expectRecordAlreadyUpdatedInfo(h.mockPP, f.ipFamily, f.domain, false)
				result = h.setter.SetIPs(ctx, h.mockPP, f.ipFamily, f.domain, targets, f.params)
			}
			require.Equal(t, setter.ResponseNoop, result.Code)
			require.NotNil(t, result.Before)
			require.NotNil(t, result.Progress)
			require.Equal(t, targets, result.Progress.Satisfied)
			require.Empty(t, result.Progress.Operations)
			require.Empty(t, result.Effects)
		})
	}
}

// A successful write can be followed by a deletion failure. Shared addresses
// remain distinct record operations; API-borrowed input can change on any call.
func TestDNSResultWritesThenDuplicateDeletionFailure(t *testing.T) {
	t.Parallel()
	ctx, h := newSetterHarness(t)
	f := newDNSRecordFixture()
	third := netip.MustParseAddr("::3")
	rs := []api.Record{
		dnsRecord("r1", f.ip1, f.params),
		dnsRecord("r2", f.ip2, f.params),
		dnsRecord("r3", f.ip2, f.params),
		dnsRecord("r4", f.ip2, f.params),
	}
	updated := api.RecordMutation{Status: api.MutationCompleted, Kind: api.RecordUpdate, Record: api.RecordFact{ID: "r1", IP: third}, Membership: api.MembershipManaged}
	deleted := api.RecordMutation{Status: api.MutationCompleted, Kind: api.RecordDelete, Record: api.RecordFact{ID: "r2", IP: netip.Addr{}}, Membership: api.MembershipUnknown}
	failed := api.RecordMutation{Status: api.MutationUnconfirmed, Kind: api.RecordDelete, Record: api.RecordFact{ID: "r3", IP: netip.Addr{}}, Membership: api.MembershipUnknown}
	gomock.InOrder(
		h.mockHandle.EXPECT().ListRecords(ctx, h.mockPP, f.ipFamily, f.domain, f.params).Return(rs, true, true),
		h.mockHandle.EXPECT().UpdateRecord(ctx, h.mockPP, f.ipFamily, f.domain, api.ID("r1"), third, f.params).DoAndReturn(func(context.Context, pp.PP, ipnet.Family, domain.Domain, api.ID, netip.Addr, api.RecordParams) api.RecordMutation {
			clear(rs)
			return updated
		}),
		expectRecordUpdatedNotice(h.mockPP, f.ipFamily, f.domain, "r1"),
		h.mockHandle.EXPECT().DeleteRecord(ctx, h.mockPP, f.ipFamily, f.domain, api.ID("r2"), api.RegularDeletionMode).Return(deleted),
		expectRecordOutdatedDeletedNotice(h.mockPP, f.ipFamily, f.domain, "r2"),
		h.mockHandle.EXPECT().DeleteRecord(ctx, h.mockPP, f.ipFamily, f.domain, api.ID("r3"), api.RegularDeletionMode).Return(failed),
	)
	result := h.setter.SetIPs(ctx, h.mockPP, f.ipFamily, f.domain, []netip.Addr{third}, f.params)
	require.Equal(t, setter.ResponseFailed, result.Code)
	require.Equal(t, []api.RecordMutation{updated, deleted, failed}, result.Effects)
	requireDNSProgress(t, &setter.DNSProgress{Satisfied: nil, Operations: []setter.DNSOperation{
		{Kind: api.RecordUpdate, Record: api.RecordFact{ID: "r1", IP: third}, Status: setter.DNSOperationCompleted, Membership: api.MembershipManaged},
		{Kind: api.RecordDelete, Record: api.RecordFact{ID: "r2", IP: f.ip2}, Status: setter.DNSOperationCompleted, Membership: api.MembershipUnknown},
		{Kind: api.RecordDelete, Record: api.RecordFact{ID: "r3", IP: f.ip2}, Status: setter.DNSOperationUnconfirmed, Membership: api.MembershipUnknown},
		{Kind: api.RecordDelete, Record: api.RecordFact{ID: "r4", IP: f.ip2}, Status: setter.DNSOperationNotAttempted, Membership: api.MembershipUnknown},
	}}, result.Progress)
	require.Equal(t, []api.RecordFact{{ID: "r1", IP: f.ip1}, {ID: "r2", IP: f.ip2}, {ID: "r3", IP: f.ip2}, {ID: "r4", IP: f.ip2}}, result.Before.Records)
}

// Progress slice nilness carries no meaning; compare nonempty slices in order.
func requireDNSProgress(t *testing.T, want, got *setter.DNSProgress) {
	t.Helper()
	require.NotNil(t, got)
	if len(want.Satisfied) == 0 {
		require.Empty(t, got.Satisfied)
	} else {
		require.Equal(t, want.Satisfied, got.Satisfied)
	}
	if len(want.Operations) == 0 {
		require.Empty(t, got.Operations)
	} else {
		require.Equal(t, want.Operations, got.Operations)
	}
}

func TestDNSResultDeletionFailureRetainsDistinctSuffix(t *testing.T) {
	t.Parallel()
	ctx, h := newSetterHarness(t)
	f := newDNSRecordFixture()
	third := netip.MustParseAddr("::3")
	fourth := netip.MustParseAddr("::4")
	target := netip.MustParseAddr("::9")
	rs := []api.Record{dnsRecord("r1", f.ip1, f.params), dnsRecord("r2", f.ip2, f.params), dnsRecord("r3", third, f.params), dnsRecord("r4", fourth, f.params)}
	updated := api.RecordMutation{Status: api.MutationCompleted, Kind: api.RecordUpdate, Record: api.RecordFact{ID: "r1", IP: target}, Membership: api.MembershipManaged}
	failed := api.RecordMutation{Status: api.MutationUnconfirmed, Kind: api.RecordDelete, Record: api.RecordFact{ID: "r2", IP: netip.Addr{}}, Membership: api.MembershipUnknown}
	gomock.InOrder(
		h.mockHandle.EXPECT().ListRecords(ctx, h.mockPP, f.ipFamily, f.domain, f.params).Return(rs, false, true),
		h.mockHandle.EXPECT().UpdateRecord(ctx, h.mockPP, f.ipFamily, f.domain, api.ID("r1"), target, f.params).Return(updated),
		expectRecordUpdatedNotice(h.mockPP, f.ipFamily, f.domain, "r1"),
		h.mockHandle.EXPECT().DeleteRecord(ctx, h.mockPP, f.ipFamily, f.domain, api.ID("r2"), api.RegularDeletionMode).Return(failed),
	)
	result := h.setter.SetIPs(ctx, h.mockPP, f.ipFamily, f.domain, []netip.Addr{target}, f.params)
	require.Equal(t, setter.ResponseFailed, result.Code)
	require.Equal(t, []api.RecordMutation{updated, failed}, result.Effects)
	requireDNSProgress(t, &setter.DNSProgress{Satisfied: nil, Operations: []setter.DNSOperation{
		{Kind: api.RecordUpdate, Record: api.RecordFact{ID: "r1", IP: target}, Status: setter.DNSOperationCompleted, Membership: api.MembershipManaged},
		{Kind: api.RecordDelete, Record: api.RecordFact{ID: "r2", IP: f.ip2}, Status: setter.DNSOperationUnconfirmed, Membership: api.MembershipUnknown},
		{Kind: api.RecordDelete, Record: api.RecordFact{ID: "r3", IP: third}, Status: setter.DNSOperationNotAttempted, Membership: api.MembershipUnknown},
		{Kind: api.RecordDelete, Record: api.RecordFact{ID: "r4", IP: fourth}, Status: setter.DNSOperationNotAttempted, Membership: api.MembershipUnknown},
	}}, result.Progress)
}

// ListRecords does not promise unique or nonempty IDs. Progress must retain
// each occurrence's address rather than reconstructing it through an ID map.
func TestDNSResultDeletionProgressPreservesConflictingIDs(t *testing.T) {
	t.Parallel()
	for _, id := range []api.ID{"shared", ""} {
		t.Run(string(id), func(t *testing.T) {
			t.Parallel()
			ctx, h := newSetterHarness(t)
			f := newDNSRecordFixture()
			rs := []api.Record{dnsRecord(id, f.ip1, f.params), dnsRecord(id, f.ip2, f.params)}
			completed := api.RecordMutation{Status: api.MutationCompleted, Kind: api.RecordDelete, Record: api.RecordFact{ID: id, IP: netip.Addr{}}, Membership: api.MembershipUnknown}
			failed := api.RecordMutation{Status: api.MutationUnconfirmed, Kind: api.RecordDelete, Record: api.RecordFact{ID: id, IP: netip.Addr{}}, Membership: api.MembershipUnknown}
			gomock.InOrder(
				h.mockHandle.EXPECT().ListRecords(ctx, h.mockPP, f.ipFamily, f.domain, f.params).Return(rs, true, true),
				h.mockHandle.EXPECT().DeleteRecord(ctx, h.mockPP, f.ipFamily, f.domain, id, api.RegularDeletionMode).DoAndReturn(func(context.Context, pp.PP, ipnet.Family, domain.Domain, api.ID, api.DeletionMode) api.RecordMutation {
					clear(rs)
					return completed
				}),
				expectRecordOutdatedDeletedNotice(h.mockPP, f.ipFamily, f.domain, id),
				h.mockHandle.EXPECT().DeleteRecord(ctx, h.mockPP, f.ipFamily, f.domain, id, api.RegularDeletionMode).Return(failed),
			)
			result := h.setter.SetIPs(ctx, h.mockPP, f.ipFamily, f.domain, nil, f.params)
			require.Equal(t, setter.ResponseFailed, result.Code)
			require.Equal(t, []api.RecordMutation{completed, failed}, result.Effects)
			requireDNSProgress(t, &setter.DNSProgress{Satisfied: nil, Operations: []setter.DNSOperation{
				{Kind: api.RecordDelete, Record: api.RecordFact{ID: id, IP: f.ip1}, Status: setter.DNSOperationCompleted, Membership: api.MembershipUnknown},
				{Kind: api.RecordDelete, Record: api.RecordFact{ID: id, IP: f.ip2}, Status: setter.DNSOperationUnconfirmed, Membership: api.MembershipUnknown},
			}}, result.Progress)
		})
	}
}
