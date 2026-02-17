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

func TestSet(t *testing.T) {
	t.Parallel()

	const (
		domain    = domain.FQDN("sub.test.org")
		ipNetwork = ipnet.IP6
		record1   = api.ID("record1")
		record2   = api.ID("record2")
		record3   = api.ID("record3")
	)
	var (
		ip1    = netip.MustParseAddr("::1")
		ip2    = netip.MustParseAddr("::2")
		params = api.RecordParams{
			TTL:     api.TTLAuto,
			Proxied: false,
			Comment: "hello",
		}
	)

	cases := []struct {
		name         string
		ip           netip.Addr
		resp         setter.ResponseCode
		prepareMocks func(ctx context.Context, cancel func(), p *mocks.MockPP, m *mocks.MockHandle)
	}{
		{
			name: "0",
			ip:   ip1,
			resp: setter.ResponseUpdated,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, ipNetwork, domain, params).Return([]api.Record{}, true, true),
					h.EXPECT().CreateRecord(ctx, p, ipNetwork, domain, ip1, params).Return(record1, true),
					p.EXPECT().Noticef(pp.EmojiCreation, "Added a new %s record of %s (ID: %s)", "AAAA", "sub.test.org", record1),
				)
			},
		},
		{
			name: "0/create-fail",
			ip:   ip1,
			resp: setter.ResponseFailed,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, ipNetwork, domain, params).Return([]api.Record{}, true, true),
					h.EXPECT().CreateRecord(ctx, p, ipNetwork, domain, ip1, params).Return(record1, false),
					p.EXPECT().Noticef(pp.EmojiError, "Failed to properly update %s records of %s; records might be inconsistent", "AAAA", "sub.test.org"),
				)
			},
		},
		{
			name: "1unmatched",
			ip:   ip1,
			resp: setter.ResponseUpdated,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, ipNetwork, domain, params).
						Return([]api.Record{{ID: record1, IP: ip2, RecordParams: params}}, true, true),
					h.EXPECT().UpdateRecord(ctx, p, ipNetwork, domain, record1, ip1, params, params).Return(true),
					p.EXPECT().Noticef(pp.EmojiUpdate, "Updated a stale %s record of %s (ID: %s)", "AAAA", "sub.test.org", record1),
				)
			},
		},
		{
			name: "1unmatched/update-fail",
			ip:   ip1,
			resp: setter.ResponseFailed,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, ipNetwork, domain, params).
						Return([]api.Record{{ID: record1, IP: ip2, RecordParams: params}}, true, true),
					h.EXPECT().UpdateRecord(ctx, p, ipNetwork, domain, record1, ip1, params, params).Return(false),
					p.EXPECT().Noticef(pp.EmojiError, "Failed to properly update %s records of %s; records might be inconsistent", "AAAA", "sub.test.org"),
				)
			},
		},
		{
			name: "1matched",
			ip:   ip1,
			resp: setter.ResponseNoop,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, ipNetwork, domain, params).
						Return([]api.Record{{ID: record1, IP: ip1, RecordParams: params}}, true, true),
					p.EXPECT().Infof(pp.EmojiAlreadyDone,
						"The %s records of %s are already up to date (cached)", "AAAA", "sub.test.org"),
				)
			},
		},
		{
			name: "1matched/not-cached",
			ip:   ip1,
			resp: setter.ResponseNoop,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, ipNetwork, domain, params).Return([]api.Record{
						{ID: record1, IP: ip1, RecordParams: params},
					}, false, true),
					p.EXPECT().Infof(pp.EmojiAlreadyDone, "The %s records of %s are already up to date", "AAAA", "sub.test.org"),
				)
			},
		},
		{
			name: "3matched",
			ip:   ip1,
			resp: setter.ResponseUpdated,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, ipNetwork, domain, params).Return([]api.Record{
						{ID: record1, IP: ip1, RecordParams: params},
						{ID: record2, IP: ip1, RecordParams: params},
						{ID: record3, IP: ip1, RecordParams: params},
					}, true, true),
					h.EXPECT().DeleteRecord(ctx, p, ipNetwork, domain, record2, api.RegularDelitionMode).Return(true),
					p.EXPECT().Noticef(pp.EmojiDeletion, "Deleted a duplicate %s record of %s (ID: %s)", "AAAA", "sub.test.org", record2),
					h.EXPECT().DeleteRecord(ctx, p, ipNetwork, domain, record3, api.RegularDelitionMode).Return(true),
					p.EXPECT().Noticef(pp.EmojiDeletion, "Deleted a duplicate %s record of %s (ID: %s)", "AAAA", "sub.test.org", record3),
				)
			},
		},
		{
			name: "3matched/delete-fail/1",
			ip:   ip1,
			resp: setter.ResponseUpdated,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, ipNetwork, domain, params).Return([]api.Record{
						{ID: record1, IP: ip1, RecordParams: params},
						{ID: record2, IP: ip1, RecordParams: params},
						{ID: record3, IP: ip1, RecordParams: params},
					}, true, true),
					h.EXPECT().DeleteRecord(ctx, p, ipNetwork, domain, record2, api.RegularDelitionMode).Return(false),
					h.EXPECT().DeleteRecord(ctx, p, ipNetwork, domain, record3, api.RegularDelitionMode).Return(true),
					p.EXPECT().Noticef(pp.EmojiDeletion, "Deleted a duplicate %s record of %s (ID: %s)", "AAAA", "sub.test.org", record3),
				)
			},
		},
		{
			name: "3matched/delete-fail/2",
			ip:   ip1,
			resp: setter.ResponseUpdated,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, ipNetwork, domain, params).Return([]api.Record{
						{ID: record1, IP: ip1, RecordParams: params},
						{ID: record2, IP: ip1, RecordParams: params},
						{ID: record3, IP: ip1, RecordParams: params},
					}, true, true),
					h.EXPECT().DeleteRecord(ctx, p, ipNetwork, domain, record2, api.RegularDelitionMode).Return(true),
					p.EXPECT().Noticef(pp.EmojiDeletion, "Deleted a duplicate %s record of %s (ID: %s)", "AAAA", "sub.test.org", record2),
					h.EXPECT().DeleteRecord(ctx, p, ipNetwork, domain, record3, api.RegularDelitionMode).Return(false),
				)
			},
		},
		{
			name: "3matched/delete-timeout",
			ip:   ip1,
			resp: setter.ResponseUpdated,
			prepareMocks: func(ctx context.Context, cancel func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, ipNetwork, domain, params).Return([]api.Record{
						{ID: record1, IP: ip1, RecordParams: params},
						{ID: record2, IP: ip1, RecordParams: params},
					}, true, true),
					h.EXPECT().DeleteRecord(ctx, p, ipNetwork, domain, record2, api.RegularDelitionMode).Do(wrapCancelAsDelete(cancel)).Return(false),
				)
			},
		},
		{
			name: "2unmatched",
			ip:   ip1,
			resp: setter.ResponseUpdated,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, ipNetwork, domain, params).Return([]api.Record{
						{ID: record1, IP: ip2, RecordParams: params},
						{ID: record2, IP: ip2, RecordParams: params},
					}, true, true),
					h.EXPECT().UpdateRecord(ctx, p, ipNetwork, domain, record1, ip1, params, params).Return(true),
					p.EXPECT().Noticef(pp.EmojiUpdate, "Updated a stale %s record of %s (ID: %s)", "AAAA", "sub.test.org", record1),
					h.EXPECT().DeleteRecord(ctx, p, ipNetwork, domain, record2, api.RegularDelitionMode).Return(true),
					p.EXPECT().Noticef(pp.EmojiDeletion, "Deleted a stale %s record of %s (ID: %s)", "AAAA", "sub.test.org", record2),
				)
			},
		},
		{
			name: "2unmatched/delete-timeout",
			ip:   ip1,
			resp: setter.ResponseFailed,
			prepareMocks: func(ctx context.Context, cancel func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, ipNetwork, domain, params).Return([]api.Record{
						{ID: record1, IP: ip2, RecordParams: params},
						{ID: record2, IP: ip2, RecordParams: params},
					}, true, true),
					h.EXPECT().UpdateRecord(ctx, p, ipNetwork, domain, record1, ip1, params, params).Return(true),
					p.EXPECT().Noticef(pp.EmojiUpdate, "Updated a stale %s record of %s (ID: %s)", "AAAA", "sub.test.org", record1),
					h.EXPECT().DeleteRecord(ctx, p, ipNetwork, domain, record2, api.RegularDelitionMode).Do(wrapCancelAsDelete(cancel)).Return(false),
					p.EXPECT().Noticef(pp.EmojiError, "Failed to properly update %s records of %s; records might be inconsistent", "AAAA", "sub.test.org"),
				)
			},
		},
		{
			name: "2unmatched/update-fail",
			ip:   ip1,
			resp: setter.ResponseFailed,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, ipNetwork, domain, params).Return([]api.Record{
						{ID: record1, IP: ip2, RecordParams: params},
						{ID: record2, IP: ip2, RecordParams: params},
					}, true, true),
					h.EXPECT().UpdateRecord(ctx, p, ipNetwork, domain, record1, ip1, params, params).Return(false),
					p.EXPECT().Noticef(pp.EmojiError, "Failed to properly update %s records of %s; records might be inconsistent", "AAAA", "sub.test.org"),
				)
			},
		},
		{
			name: "list-fail",
			ip:   ip1,
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

			resp := h.setter.Set(h.ctx, h.mockPP, ipNetwork, domain, tc.ip, params)
			require.Equal(t, tc.resp, resp)
		})
	}
}
