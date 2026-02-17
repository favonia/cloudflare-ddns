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
		prepareMocks func(ctx context.Context, cancel func(), p *mocks.MockPP, m *mocks.MockHandle)
	}{
		{
			name: "0",
			resp: setter.ResponseNoop,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, ipNetwork, domain, params).Return([]api.Record{}, true, true),
					p.EXPECT().Infof(pp.EmojiAlreadyDone, "The %s records of %s were already deleted (cached)", "AAAA", "sub.test.org"),
				)
			},
		},
		{
			name: "0/not-cached",
			resp: setter.ResponseNoop,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, ipNetwork, domain, params).Return([]api.Record{}, false, true),
					p.EXPECT().Infof(pp.EmojiAlreadyDone, "The %s records of %s were already deleted", "AAAA", "sub.test.org"),
				)
			},
		},
		{
			name: "1unmatched",
			resp: setter.ResponseUpdated,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, ipNetwork, domain, params).Return([]api.Record{
						{ID: record1, IP: ip1, RecordParams: params},
					}, true, true),
					h.EXPECT().DeleteRecord(ctx, p, ipNetwork, domain, record1, api.FinalDeletionMode).Return(true),
					p.EXPECT().Noticef(pp.EmojiDeletion, "Deleted a stale %s record of %s (ID: %s)", "AAAA", "sub.test.org", record1),
				)
			},
		},
		{
			name: "1unmatched/delete-fail",
			resp: setter.ResponseFailed,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, ipNetwork, domain, params).Return([]api.Record{
						{ID: record1, IP: ip1, RecordParams: params},
					}, true, true),
					h.EXPECT().DeleteRecord(ctx, p, ipNetwork, domain, record1, api.FinalDeletionMode).Return(false),
					p.EXPECT().Noticef(pp.EmojiError, "Failed to properly delete %s records of %s; records might be inconsistent", "AAAA", "sub.test.org"),
				)
			},
		},
		{
			name: "1unmatched/delete-timeout",
			resp: setter.ResponseFailed,
			prepareMocks: func(ctx context.Context, cancel func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, ipNetwork, domain, params).Return([]api.Record{
						{ID: record1, IP: ip1, RecordParams: params},
					}, true, true),
					h.EXPECT().DeleteRecord(ctx, p, ipNetwork, domain, record1, api.FinalDeletionMode).Do(wrapCancelAsDelete(cancel)).Return(false),
					p.EXPECT().Infof(pp.EmojiTimeout, "Deletion of %s records of %s aborted by timeout or signals; records might be inconsistent", "AAAA", "sub.test.org"),
				)
			},
		},
		{
			name: "impossible-records",
			resp: setter.ResponseUpdated,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, ipNetwork, domain, params).Return([]api.Record{
						{ID: record1, IP: ip1, RecordParams: params},
						{ID: record2, IP: invalidIP, RecordParams: params},
					}, true, true),
					h.EXPECT().DeleteRecord(ctx, p, ipNetwork, domain, record1, api.FinalDeletionMode).Return(true),
					p.EXPECT().Noticef(pp.EmojiDeletion, "Deleted a stale %s record of %s (ID: %s)", "AAAA", "sub.test.org", record1),
					h.EXPECT().DeleteRecord(ctx, p, ipNetwork, domain, record2, api.FinalDeletionMode).Return(true),
					p.EXPECT().Noticef(pp.EmojiDeletion, "Deleted a stale %s record of %s (ID: %s)", "AAAA", "sub.test.org", record2),
				)
			},
		},
		{
			name: "list-fail",
			resp: setter.ResponseFailed,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				h.EXPECT().ListRecords(ctx, p, ipNetwork, domain, params).Return(nil, false, false)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			h := newSetterHarness(t)
			if tc.prepareMocks != nil {
				tc.prepareMocks(h.ctx, h.cancel, h.mockPP, h.mockHandle)
			}

			resp := h.setter.FinalDelete(h.ctx, h.mockPP, ipNetwork, domain, params)
			require.Equal(t, tc.resp, resp)
		})
	}
}
