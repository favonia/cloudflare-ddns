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
}

func TestDNSResultReadFailureUnknown(t *testing.T) {
	t.Parallel()
	ctx, h := newSetterHarness(t)
	f := newDNSRecordFixture()
	h.mockHandle.EXPECT().ListRecords(ctx, h.mockPP, f.ipFamily, f.domain, f.params).Return(nil, false, false)
	result := h.setter.FinalDelete(ctx, h.mockPP, f.ipFamily, f.domain, f.params)
	require.Equal(t, setter.ResponseFailed, result.Code)
	require.Nil(t, result.Before)
	require.Empty(t, result.Effects)
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
				expectRecordSetFailedNotice(h.mockPP, f.ipFamily, f.domain),
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
				expectRecordFinalDeleteFailedNotice(h.mockPP, f.ipFamily, f.domain)
				effects = append(effects, completed)
			}
			result := h.setter.FinalDelete(ctx, h.mockPP, f.ipFamily, f.domain, f.params)
			require.Equal(t, setter.ResponseFailed, result.Code)
			require.Equal(t, effects, result.Effects)
			require.Equal(t, []api.RecordFact{{ID: f.record1, IP: f.ip1}, {ID: f.record2, IP: f.ip2}}, result.Before.Records)
		})
	}
}
