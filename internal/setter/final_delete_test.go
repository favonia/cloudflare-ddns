package setter_test

// vim: nowrap

import (
	"context"
	"net/netip"
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

func TestFinalDelete(t *testing.T) {
	t.Parallel()

	const (
		domain    = domain.FQDN("sub.test.org")
		ipNetwork = ipnet.IP6
		record1   = api.ID("record1")
		record2   = api.ID("record2")
		record3   = api.ID("record3")
	)
	var (
		ip1       = netip.MustParseAddr("::1")
		invalidIP = netip.Addr{}
		params    = api.RecordParams{
			TTL:     api.TTLAuto,
			Proxied: false,
			Comment: "hello",
		}
	)

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
					h.EXPECT().ListRecords(ctx, p, ipNetwork, domain, params).Return([]api.Record{}, true, true),
					p.EXPECT().Infof(pp.EmojiAlreadyDone, "The %s records of %s were already deleted (cached)", "AAAA", "sub.test.org"),
				)
			},
		},
		{
			name: "no-records/list-records/response-noop-uncached",
			resp: setter.ResponseNoop,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, ipNetwork, domain, params).Return([]api.Record{}, false, true),
					p.EXPECT().Infof(pp.EmojiAlreadyDone, "The %s records of %s were already deleted", "AAAA", "sub.test.org"),
				)
			},
		},
		{
			name: "single-record/delete-record/response-updated",
			resp: setter.ResponseUpdated,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, ipNetwork, domain, params).Return([]api.Record{
						dnsRecord(record1, ip1, params),
					}, true, true),
					h.EXPECT().DeleteRecord(ctx, p, ipNetwork, domain, record1, api.FinalDeletionMode).Return(true),
					p.EXPECT().Noticef(pp.EmojiDeletion, "Deleted a stale %s record of %s (ID: %s)", "AAAA", "sub.test.org", record1),
				)
			},
		},
		{
			name: "single-record/delete-record/response-failed",
			resp: setter.ResponseFailed,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, ipNetwork, domain, params).Return([]api.Record{
						dnsRecord(record1, ip1, params),
					}, true, true),
					h.EXPECT().DeleteRecord(ctx, p, ipNetwork, domain, record1, api.FinalDeletionMode).Return(false),
					p.EXPECT().Noticef(pp.EmojiError, "Failed to properly delete %s records of %s; records might be inconsistent", "AAAA", "sub.test.org"),
				)
			},
		},
		{
			name: "single-record/delete-record-timeout/response-failed",
			resp: setter.ResponseFailed,
			prepareMocks: func(ctx context.Context, cancel func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, ipNetwork, domain, params).Return([]api.Record{
						dnsRecord(record1, ip1, params),
					}, true, true),
					h.EXPECT().DeleteRecord(ctx, p, ipNetwork, domain, record1, api.FinalDeletionMode).Do(wrapCancelAsDelete(cancel)).Return(false),
					p.EXPECT().Infof(pp.EmojiTimeout, "Deletion of %s records of %s aborted by timeout or signals; records might be inconsistent", "AAAA", "sub.test.org"),
				)
			},
		},
		{
			name: "mixed-valid-and-invalid-records/delete-all-records/response-updated",
			resp: setter.ResponseUpdated,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, ipNetwork, domain, params).Return([]api.Record{
						dnsRecord(record1, ip1, params),
						dnsRecord(record2, invalidIP, params),
					}, true, true),
					h.EXPECT().DeleteRecord(ctx, p, ipNetwork, domain, record1, api.FinalDeletionMode).Return(true),
					p.EXPECT().Noticef(pp.EmojiDeletion, "Deleted a stale %s record of %s (ID: %s)", "AAAA", "sub.test.org", record1),
					h.EXPECT().DeleteRecord(ctx, p, ipNetwork, domain, record2, api.FinalDeletionMode).Return(true),
					p.EXPECT().Noticef(pp.EmojiDeletion, "Deleted a stale %s record of %s (ID: %s)", "AAAA", "sub.test.org", record2),
				)
			},
		},
		{
			name: "records-unknown/list-records/response-failed",
			resp: setter.ResponseFailed,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				h.EXPECT().ListRecords(ctx, p, ipNetwork, domain, params).Return(nil, false, false)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx, h := newSetterHarness(t)
			h.prepare(ctx, tc.prepareMocks)

			resp := h.setter.FinalDelete(ctx, h.mockPP, ipNetwork, domain, params)
			require.Equal(t, tc.resp, resp)
		})
	}
}
