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

func TestSet(t *testing.T) {
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
					expectRecordList(ctx, p, h, fixture.ipNetwork, fixture.domain, fixture.params, []api.Record{}, true, true),
					expectRecordCreate(
						ctx, p, h, fixture.ipNetwork, fixture.domain, fixture.ip1, fixture.params, fixture.record1, true),
					expectRecordAddedNotice(p, fixture.ipNetwork, fixture.domain, fixture.record1),
				)
			},
		},
		{
			name: "no-records/create-record/response-failed",
			ip:   fixture.ip1,
			resp: setter.ResponseFailed,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					expectRecordList(ctx, p, h, fixture.ipNetwork, fixture.domain, fixture.params, []api.Record{}, true, true),
					expectRecordCreate(
						ctx, p, h, fixture.ipNetwork, fixture.domain, fixture.ip1, fixture.params, fixture.record1, false),
					expectRecordSetFailedNotice(p, fixture.ipNetwork, fixture.domain),
				)
			},
		},
		{
			name: "single-stale-record/update-record/response-updated",
			ip:   fixture.ip1,
			resp: setter.ResponseUpdated,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					expectRecordList(ctx, p, h, fixture.ipNetwork, fixture.domain, fixture.params, []api.Record{
						dnsRecord(fixture.record1, fixture.ip2, fixture.params),
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
				)
			},
		},
		{
			name: "single-stale-record/update-record/response-failed",
			ip:   fixture.ip1,
			resp: setter.ResponseFailed,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					expectRecordList(ctx, p, h, fixture.ipNetwork, fixture.domain, fixture.params, []api.Record{
						dnsRecord(fixture.record1, fixture.ip2, fixture.params),
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
						false,
					),
					expectRecordSetFailedNotice(p, fixture.ipNetwork, fixture.domain),
				)
			},
		},
		{
			name: "single-matching-record/keep-record/response-noop-cached",
			ip:   fixture.ip1,
			resp: setter.ResponseNoop,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					expectRecordList(ctx, p, h, fixture.ipNetwork, fixture.domain, fixture.params, []api.Record{
						dnsRecord(fixture.record1, fixture.ip1, fixture.params),
					}, true, true),
					expectRecordAlreadyUpdatedInfo(p, fixture.ipNetwork, fixture.domain, true),
				)
			},
		},
		{
			name: "single-matching-record/keep-record/response-noop-uncached",
			ip:   fixture.ip1,
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
			name: "multiple-matching-records/delete-duplicates/response-updated",
			ip:   fixture.ip1,
			resp: setter.ResponseUpdated,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				// InOrder is stricter than the API contract here: either duplicate can be deleted first.
				// We intentionally pin Set's deterministic top-to-bottom traversal so this test can
				// detect processing-order regressions; looser gomock checks allow too many orders.
				gomock.InOrder(
					expectRecordList(ctx, p, h, fixture.ipNetwork, fixture.domain, fixture.params, []api.Record{
						dnsRecord(fixture.record1, fixture.ip1, fixture.params),
						dnsRecord(fixture.record2, fixture.ip1, fixture.params),
						dnsRecord(fixture.record3, fixture.ip1, fixture.params),
					}, true, true),
					expectRecordDelete(
						ctx, p, h, fixture.ipNetwork, fixture.domain, fixture.record2, api.RegularDelitionMode, true),
					expectRecordDuplicateDeletedNotice(p, fixture.ipNetwork, fixture.domain, fixture.record2),
					expectRecordDelete(
						ctx, p, h, fixture.ipNetwork, fixture.domain, fixture.record3, api.RegularDelitionMode, true),
					expectRecordDuplicateDeletedNotice(p, fixture.ipNetwork, fixture.domain, fixture.record3),
				)
			},
		},
		{
			name: "multiple-matching-records/delete-first-duplicate/response-updated",
			ip:   fixture.ip1,
			resp: setter.ResponseUpdated,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				// InOrder is stricter than the API contract here: either duplicate can be deleted first.
				// We intentionally pin Set's deterministic top-to-bottom traversal so this test can
				// detect processing-order regressions; looser gomock checks allow too many orders.
				gomock.InOrder(
					expectRecordList(ctx, p, h, fixture.ipNetwork, fixture.domain, fixture.params, []api.Record{
						dnsRecord(fixture.record1, fixture.ip1, fixture.params),
						dnsRecord(fixture.record2, fixture.ip1, fixture.params),
						dnsRecord(fixture.record3, fixture.ip1, fixture.params),
					}, true, true),
					expectRecordDelete(
						ctx, p, h, fixture.ipNetwork, fixture.domain, fixture.record2, api.RegularDelitionMode, false),
					expectRecordDelete(
						ctx, p, h, fixture.ipNetwork, fixture.domain, fixture.record3, api.RegularDelitionMode, true),
					expectRecordDuplicateDeletedNotice(p, fixture.ipNetwork, fixture.domain, fixture.record3),
				)
			},
		},
		{
			name: "multiple-matching-records/delete-second-duplicate/response-updated",
			ip:   fixture.ip1,
			resp: setter.ResponseUpdated,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				// InOrder is stricter than the API contract here: either duplicate can be deleted first.
				// We intentionally pin Set's deterministic top-to-bottom traversal so this test can
				// detect processing-order regressions; looser gomock checks allow too many orders.
				gomock.InOrder(
					expectRecordList(ctx, p, h, fixture.ipNetwork, fixture.domain, fixture.params, []api.Record{
						dnsRecord(fixture.record1, fixture.ip1, fixture.params),
						dnsRecord(fixture.record2, fixture.ip1, fixture.params),
						dnsRecord(fixture.record3, fixture.ip1, fixture.params),
					}, true, true),
					expectRecordDelete(
						ctx, p, h, fixture.ipNetwork, fixture.domain, fixture.record2, api.RegularDelitionMode, true),
					expectRecordDuplicateDeletedNotice(p, fixture.ipNetwork, fixture.domain, fixture.record2),
					expectRecordDelete(
						ctx, p, h, fixture.ipNetwork, fixture.domain, fixture.record3, api.RegularDelitionMode, false),
				)
			},
		},
		{
			name: "multiple-matching-records/delete-duplicate-timeout/response-updated",
			ip:   fixture.ip1,
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
		{
			name: "multiple-stale-records/update-and-delete/response-updated",
			ip:   fixture.ip1,
			resp: setter.ResponseUpdated,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				// InOrder is stricter than the API contract here: either stale record can be recycled first.
				// We intentionally pin Set's deterministic top-to-bottom traversal so this test can
				// detect processing-order regressions; looser gomock checks allow too many orders.
				gomock.InOrder(
					expectRecordList(ctx, p, h, fixture.ipNetwork, fixture.domain, fixture.params, []api.Record{
						dnsRecord(fixture.record1, fixture.ip2, fixture.params),
						dnsRecord(fixture.record2, fixture.ip2, fixture.params),
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
					expectRecordDelete(
						ctx, p, h, fixture.ipNetwork, fixture.domain, fixture.record2, api.RegularDelitionMode, true),
					expectRecordStaleDeletedNotice(p, fixture.ipNetwork, fixture.domain, fixture.record2),
				)
			},
		},
		{
			name: "multiple-stale-records/delete-after-update-timeout/response-failed",
			ip:   fixture.ip1,
			resp: setter.ResponseFailed,
			prepareMocks: func(ctx context.Context, cancel func(), p *mocks.MockPP, h *mocks.MockHandle) {
				// InOrder is stricter than the API contract here: either stale record can be recycled first.
				// We intentionally pin Set's deterministic top-to-bottom traversal so this test can
				// detect processing-order regressions; looser gomock checks allow too many orders.
				gomock.InOrder(
					expectRecordList(ctx, p, h, fixture.ipNetwork, fixture.domain, fixture.params, []api.Record{
						dnsRecord(fixture.record1, fixture.ip2, fixture.params),
						dnsRecord(fixture.record2, fixture.ip2, fixture.params),
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
					h.EXPECT().DeleteRecord(
						ctx, p, fixture.ipNetwork, fixture.domain, fixture.record2, api.RegularDelitionMode).
						Do(wrapCancelAsDelete(cancel)).Return(false),
					expectRecordSetFailedNotice(p, fixture.ipNetwork, fixture.domain),
				)
			},
		},
		{
			name: "multiple-stale-records/update-first-record/response-failed",
			ip:   fixture.ip1,
			resp: setter.ResponseFailed,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				// InOrder is stricter than the API contract here: either stale record can be recycled first.
				// We intentionally pin Set's deterministic top-to-bottom traversal so this test can
				// detect processing-order regressions; looser gomock checks allow too many orders.
				gomock.InOrder(
					expectRecordList(ctx, p, h, fixture.ipNetwork, fixture.domain, fixture.params, []api.Record{
						dnsRecord(fixture.record1, fixture.ip2, fixture.params),
						dnsRecord(fixture.record2, fixture.ip2, fixture.params),
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
						false,
					),
					expectRecordSetFailedNotice(p, fixture.ipNetwork, fixture.domain),
				)
			},
		},
		{
			name: "records-unknown/list-records/response-failed",
			ip:   fixture.ip1,
			resp: setter.ResponseFailed,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				expectRecordList(ctx, p, h, fixture.ipNetwork, fixture.domain, fixture.params, nil, false, false)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx, h := newSetterHarness(t)
			h.prepare(ctx, tc.prepareMocks)

			resp := h.setter.Set(ctx, h.mockPP, fixture.ipNetwork, fixture.domain, tc.ip, fixture.params)
			require.Equal(t, tc.resp, resp)
		})
	}
}
