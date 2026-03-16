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

func TestSetIPsSingleton(t *testing.T) {
	t.Parallel()

	fixture := newDNSRecordFixture()

	cases := []struct {
		name         string
		ip           netip.Addr
		resp         setter.ResponseCode
		prepareMocks prepareSetterMocks
	}{
		{
			name: "no-records/create-record/response-updated",
			ip:   fixture.ip1,
			resp: setter.ResponseUpdated,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					expectRecordList(ctx, p, h, fixture.ipFamily, fixture.domain, fixture.params, []api.Record{}, true, true),
					expectRecordCreate(
						ctx, p, h, fixture.ipFamily, fixture.domain, fixture.ip1, fixture.params, fixture.record1, true),
					expectRecordAddedNotice(p, fixture.ipFamily, fixture.domain, fixture.record1),
				)
			},
		},
		{
			name: "no-records/create-record/response-failed",
			ip:   fixture.ip1,
			resp: setter.ResponseFailed,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					expectRecordList(ctx, p, h, fixture.ipFamily, fixture.domain, fixture.params, []api.Record{}, true, true),
					expectRecordCreate(
						ctx, p, h, fixture.ipFamily, fixture.domain, fixture.ip1, fixture.params, fixture.record1, false),
					expectRecordSetFailedNotice(p, fixture.ipFamily, fixture.domain),
				)
			},
		},
		{
			name: "single-outdated-record/update-record/response-updated",
			ip:   fixture.ip1,
			resp: setter.ResponseUpdated,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					expectRecordList(ctx, p, h, fixture.ipFamily, fixture.domain, fixture.params, []api.Record{
						dnsRecord(fixture.record1, fixture.ip2, fixture.params),
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
				)
			},
		},
		{
			name: "single-outdated-record/update-record/response-failed",
			ip:   fixture.ip1,
			resp: setter.ResponseFailed,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					expectRecordList(ctx, p, h, fixture.ipFamily, fixture.domain, fixture.params, []api.Record{
						dnsRecord(fixture.record1, fixture.ip2, fixture.params),
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
						false,
					),
					expectRecordSetFailedNotice(p, fixture.ipFamily, fixture.domain),
				)
			},
		},
		{
			name: "single-matching-record/keep-record/response-noop-cached",
			ip:   fixture.ip1,
			resp: setter.ResponseNoop,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					expectRecordList(ctx, p, h, fixture.ipFamily, fixture.domain, fixture.params, []api.Record{
						dnsRecord(fixture.record1, fixture.ip1, fixture.params),
					}, true, true),
					expectRecordAlreadyUpdatedInfo(p, fixture.ipFamily, fixture.domain, true),
				)
			},
		},
		{
			name: "single-matching-record/keep-record/response-noop-uncached",
			ip:   fixture.ip1,
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
			name: "multiple-matching-records/preserve-duplicates/response-noop",
			ip:   fixture.ip1,
			resp: setter.ResponseNoop,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					expectRecordList(ctx, p, h, fixture.ipFamily, fixture.domain, fixture.params, []api.Record{
						dnsRecord(fixture.record1, fixture.ip1, fixture.params),
						dnsRecord(fixture.record2, fixture.ip1, fixture.params),
						dnsRecord(fixture.record3, fixture.ip1, fixture.params),
					}, true, true),
					expectRecordAlreadyUpdatedInfo(p, fixture.ipFamily, fixture.domain, true),
				)
			},
		},
		{
			name: "multiple-matching-records/delete-first-duplicate/response-noop",
			ip:   fixture.ip1,
			resp: setter.ResponseNoop,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					expectRecordList(ctx, p, h, fixture.ipFamily, fixture.domain, fixture.params, []api.Record{
						dnsRecord(fixture.record1, fixture.ip1, fixture.params),
						dnsRecord(fixture.record2, fixture.ip1, fixture.params),
						dnsRecord(fixture.record3, fixture.ip1, fixture.params),
					}, true, true),
					expectRecordAlreadyUpdatedInfo(p, fixture.ipFamily, fixture.domain, true),
				)
			},
		},
		{
			name: "multiple-matching-records/delete-second-duplicate/response-noop",
			ip:   fixture.ip1,
			resp: setter.ResponseNoop,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					expectRecordList(ctx, p, h, fixture.ipFamily, fixture.domain, fixture.params, []api.Record{
						dnsRecord(fixture.record1, fixture.ip1, fixture.params),
						dnsRecord(fixture.record2, fixture.ip1, fixture.params),
						dnsRecord(fixture.record3, fixture.ip1, fixture.params),
					}, true, true),
					expectRecordAlreadyUpdatedInfo(p, fixture.ipFamily, fixture.domain, true),
				)
			},
		},
		{
			name: "multiple-matching-records/delete-duplicate-timeout/response-noop",
			ip:   fixture.ip1,
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
		{
			name: "multiple-outdated-records/update-and-delete/response-updated",
			ip:   fixture.ip1,
			resp: setter.ResponseUpdated,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				// InOrder is stricter than the API contract here: either outdated record can be recycled first.
				// We intentionally pin singleton SetIPs's deterministic top-to-bottom traversal so this test can
				// detect processing-order regressions; looser gomock checks allow too many orders.
				gomock.InOrder(
					expectRecordList(ctx, p, h, fixture.ipFamily, fixture.domain, fixture.params, []api.Record{
						dnsRecord(fixture.record1, fixture.ip2, fixture.params),
						dnsRecord(fixture.record2, fixture.ip2, fixture.params),
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
					expectRecordDelete(
						ctx, p, h, fixture.ipFamily, fixture.domain, fixture.record2, api.RegularDeletionMode, true),
					expectRecordOutdatedDeletedNotice(p, fixture.ipFamily, fixture.domain, fixture.record2),
				)
			},
		},
		{
			name: "multiple-outdated-records/delete-after-update-timeout/response-failed",
			ip:   fixture.ip1,
			resp: setter.ResponseFailed,
			prepareMocks: func(ctx context.Context, cancel func(), p *mocks.MockPP, h *mocks.MockHandle) {
				// InOrder is stricter than the API contract here: either outdated record can be recycled first.
				// We intentionally pin singleton SetIPs's deterministic top-to-bottom traversal so this test can
				// detect processing-order regressions; looser gomock checks allow too many orders.
				gomock.InOrder(
					expectRecordList(ctx, p, h, fixture.ipFamily, fixture.domain, fixture.params, []api.Record{
						dnsRecord(fixture.record1, fixture.ip2, fixture.params),
						dnsRecord(fixture.record2, fixture.ip2, fixture.params),
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
					h.EXPECT().DeleteRecord(
						ctx, p, fixture.ipFamily, fixture.domain, fixture.record2, api.RegularDeletionMode).
						Do(wrapCancelAsDelete(cancel)).Return(false),
					expectRecordSetFailedNotice(p, fixture.ipFamily, fixture.domain),
				)
			},
		},
		{
			name: "multiple-outdated-records/update-first-record/response-failed",
			ip:   fixture.ip1,
			resp: setter.ResponseFailed,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				// InOrder is stricter than the API contract here: either outdated record can be recycled first.
				// We intentionally pin singleton SetIPs's deterministic top-to-bottom traversal so this test can
				// detect processing-order regressions; looser gomock checks allow too many orders.
				gomock.InOrder(
					expectRecordList(ctx, p, h, fixture.ipFamily, fixture.domain, fixture.params, []api.Record{
						dnsRecord(fixture.record1, fixture.ip2, fixture.params),
						dnsRecord(fixture.record2, fixture.ip2, fixture.params),
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
						false,
					),
					expectRecordSetFailedNotice(p, fixture.ipFamily, fixture.domain),
				)
			},
		},
		{
			name: "records-unknown/list-records/response-failed",
			ip:   fixture.ip1,
			resp: setter.ResponseFailed,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				expectRecordList(ctx, p, h, fixture.ipFamily, fixture.domain, fixture.params, nil, false, false)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx, h := newSetterHarness(t)
			h.prepare(ctx, tc.prepareMocks)

			resp := h.setter.SetIPs(ctx, h.mockPP, fixture.ipFamily, fixture.domain, []netip.Addr{tc.ip}, fixture.params)
			require.Equal(t, tc.resp, resp)
		})
	}
}
