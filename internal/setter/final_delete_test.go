package setter_test

// vim: nowrap

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/setter"
)

func TestFinalDelete(t *testing.T) {
	t.Parallel()

	fixture := newDNSRecordFixture()

	cases := []struct {
		name         string
		resp         setter.ResponseCode
		prepareMocks prepareSetterMocks
	}{
		{
			name: "no-records/list-records/response-noop-cached",
			resp: setter.ResponseNoop,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					expectRecordList(ctx, p, h, fixture.ipNetwork, fixture.domain, fixture.params, []api.Record{}, true, true),
					expectRecordAlreadyDeletedInfo(p, fixture.ipNetwork, fixture.domain, true),
				)
			},
		},
		{
			name: "no-records/list-records/response-noop-uncached",
			resp: setter.ResponseNoop,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					expectRecordList(ctx, p, h, fixture.ipNetwork, fixture.domain, fixture.params, []api.Record{}, false, true),
					expectRecordAlreadyDeletedInfo(p, fixture.ipNetwork, fixture.domain, false),
				)
			},
		},
		{
			name: "single-record/delete-record/response-updated",
			resp: setter.ResponseUpdated,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					expectRecordList(ctx, p, h, fixture.ipNetwork, fixture.domain, fixture.params, []api.Record{
						dnsRecord(fixture.record1, fixture.ip1, fixture.params),
					}, true, true),
					expectRecordDelete(
						ctx, p, h, fixture.ipNetwork, fixture.domain, fixture.record1, api.FinalDeletionMode, true),
					expectRecordStaleDeletedNotice(p, fixture.ipNetwork, fixture.domain, fixture.record1),
				)
			},
		},
		{
			name: "single-record/delete-record/response-failed",
			resp: setter.ResponseFailed,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					expectRecordList(ctx, p, h, fixture.ipNetwork, fixture.domain, fixture.params, []api.Record{
						dnsRecord(fixture.record1, fixture.ip1, fixture.params),
					}, true, true),
					expectRecordDelete(
						ctx, p, h, fixture.ipNetwork, fixture.domain, fixture.record1, api.FinalDeletionMode, false),
					expectRecordFinalDeleteFailedNotice(p, fixture.ipNetwork, fixture.domain),
				)
			},
		},
		{
			name: "single-record/delete-record-timeout/response-failed",
			resp: setter.ResponseFailed,
			prepareMocks: func(ctx context.Context, cancel func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					expectRecordList(ctx, p, h, fixture.ipNetwork, fixture.domain, fixture.params, []api.Record{
						dnsRecord(fixture.record1, fixture.ip1, fixture.params),
					}, true, true),
					h.EXPECT().DeleteRecord(
						ctx, p, fixture.ipNetwork, fixture.domain, fixture.record1, api.FinalDeletionMode).
						Do(wrapCancelAsDelete(cancel)).Return(false),
					expectRecordDeleteTimeoutInfo(p, fixture.ipNetwork, fixture.domain),
				)
			},
		},
		{
			name: "mixed-valid-and-invalid-records/delete-all-records/response-updated",
			resp: setter.ResponseUpdated,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				// InOrder is stricter than the API contract here: deleting these records in either
				// order is valid. We intentionally pin FinalDelete's deterministic top-to-bottom
				// traversal so this test can detect processing-order regressions; looser gomock
				// checks allow too many orders.
				gomock.InOrder(
					expectRecordList(ctx, p, h, fixture.ipNetwork, fixture.domain, fixture.params, []api.Record{
						dnsRecord(fixture.record1, fixture.ip1, fixture.params),
						dnsRecord(fixture.record2, fixture.invalidIP, fixture.params),
					}, true, true),
					expectRecordDelete(
						ctx, p, h, fixture.ipNetwork, fixture.domain, fixture.record1, api.FinalDeletionMode, true),
					expectRecordStaleDeletedNotice(p, fixture.ipNetwork, fixture.domain, fixture.record1),
					expectRecordDelete(
						ctx, p, h, fixture.ipNetwork, fixture.domain, fixture.record2, api.FinalDeletionMode, true),
					expectRecordStaleDeletedNotice(p, fixture.ipNetwork, fixture.domain, fixture.record2),
				)
			},
		},
		{
			name: "records-unknown/list-records/response-failed",
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

			resp := h.setter.FinalDelete(ctx, h.mockPP, fixture.ipNetwork, fixture.domain, fixture.params)
			require.Equal(t, tc.resp, resp)
		})
	}
}
