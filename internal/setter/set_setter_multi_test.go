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
