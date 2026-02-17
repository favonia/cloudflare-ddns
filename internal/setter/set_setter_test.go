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
			name: "no-records/create-record/response-updated",
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
			name: "no-records/create-record/response-failed",
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
			name: "single-stale-record/update-record/response-updated",
			ip:   ip1,
			resp: setter.ResponseUpdated,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, ipNetwork, domain, params).
						Return([]api.Record{dnsRecord(record1, ip2, params)}, true, true),
					h.EXPECT().UpdateRecord(ctx, p, ipNetwork, domain, record1, ip1, params, params).Return(true),
					p.EXPECT().Noticef(pp.EmojiUpdate, "Updated a stale %s record of %s (ID: %s)", "AAAA", "sub.test.org", record1),
				)
			},
		},
		{
			name: "single-stale-record/update-record/response-failed",
			ip:   ip1,
			resp: setter.ResponseFailed,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, ipNetwork, domain, params).
						Return([]api.Record{dnsRecord(record1, ip2, params)}, true, true),
					h.EXPECT().UpdateRecord(ctx, p, ipNetwork, domain, record1, ip1, params, params).Return(false),
					p.EXPECT().Noticef(pp.EmojiError, "Failed to properly update %s records of %s; records might be inconsistent", "AAAA", "sub.test.org"),
				)
			},
		},
		{
			name: "single-matching-record/keep-record/response-noop-cached",
			ip:   ip1,
			resp: setter.ResponseNoop,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, ipNetwork, domain, params).
						Return([]api.Record{dnsRecord(record1, ip1, params)}, true, true),
					p.EXPECT().Infof(pp.EmojiAlreadyDone,
						"The %s records of %s are already up to date (cached)", "AAAA", "sub.test.org"),
				)
			},
		},
		{
			name: "single-matching-record/keep-record/response-noop-uncached",
			ip:   ip1,
			resp: setter.ResponseNoop,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, ipNetwork, domain, params).Return([]api.Record{
						dnsRecord(record1, ip1, params),
					}, false, true),
					p.EXPECT().Infof(pp.EmojiAlreadyDone, "The %s records of %s are already up to date", "AAAA", "sub.test.org"),
				)
			},
		},
		{
			name: "multiple-matching-records/delete-duplicates/response-updated",
			ip:   ip1,
			resp: setter.ResponseUpdated,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, ipNetwork, domain, params).Return([]api.Record{
						dnsRecord(record1, ip1, params),
						dnsRecord(record2, ip1, params),
						dnsRecord(record3, ip1, params),
					}, true, true),
					h.EXPECT().DeleteRecord(ctx, p, ipNetwork, domain, record2, api.RegularDelitionMode).Return(true),
					p.EXPECT().Noticef(pp.EmojiDeletion, "Deleted a duplicate %s record of %s (ID: %s)", "AAAA", "sub.test.org", record2),
					h.EXPECT().DeleteRecord(ctx, p, ipNetwork, domain, record3, api.RegularDelitionMode).Return(true),
					p.EXPECT().Noticef(pp.EmojiDeletion, "Deleted a duplicate %s record of %s (ID: %s)", "AAAA", "sub.test.org", record3),
				)
			},
		},
		{
			name: "multiple-matching-records/delete-first-duplicate/response-updated",
			ip:   ip1,
			resp: setter.ResponseUpdated,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, ipNetwork, domain, params).Return([]api.Record{
						dnsRecord(record1, ip1, params),
						dnsRecord(record2, ip1, params),
						dnsRecord(record3, ip1, params),
					}, true, true),
					h.EXPECT().DeleteRecord(ctx, p, ipNetwork, domain, record2, api.RegularDelitionMode).Return(false),
					h.EXPECT().DeleteRecord(ctx, p, ipNetwork, domain, record3, api.RegularDelitionMode).Return(true),
					p.EXPECT().Noticef(pp.EmojiDeletion, "Deleted a duplicate %s record of %s (ID: %s)", "AAAA", "sub.test.org", record3),
				)
			},
		},
		{
			name: "multiple-matching-records/delete-second-duplicate/response-updated",
			ip:   ip1,
			resp: setter.ResponseUpdated,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, ipNetwork, domain, params).Return([]api.Record{
						dnsRecord(record1, ip1, params),
						dnsRecord(record2, ip1, params),
						dnsRecord(record3, ip1, params),
					}, true, true),
					h.EXPECT().DeleteRecord(ctx, p, ipNetwork, domain, record2, api.RegularDelitionMode).Return(true),
					p.EXPECT().Noticef(pp.EmojiDeletion, "Deleted a duplicate %s record of %s (ID: %s)", "AAAA", "sub.test.org", record2),
					h.EXPECT().DeleteRecord(ctx, p, ipNetwork, domain, record3, api.RegularDelitionMode).Return(false),
				)
			},
		},
		{
			name: "multiple-matching-records/delete-duplicate-timeout/response-updated",
			ip:   ip1,
			resp: setter.ResponseUpdated,
			prepareMocks: func(ctx context.Context, cancel func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, ipNetwork, domain, params).Return([]api.Record{
						dnsRecord(record1, ip1, params),
						dnsRecord(record2, ip1, params),
					}, true, true),
					h.EXPECT().DeleteRecord(ctx, p, ipNetwork, domain, record2, api.RegularDelitionMode).Do(wrapCancelAsDelete(cancel)).Return(false),
				)
			},
		},
		{
			name: "multiple-stale-records/update-and-delete/response-updated",
			ip:   ip1,
			resp: setter.ResponseUpdated,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, ipNetwork, domain, params).Return([]api.Record{
						dnsRecord(record1, ip2, params),
						dnsRecord(record2, ip2, params),
					}, true, true),
					h.EXPECT().UpdateRecord(ctx, p, ipNetwork, domain, record1, ip1, params, params).Return(true),
					p.EXPECT().Noticef(pp.EmojiUpdate, "Updated a stale %s record of %s (ID: %s)", "AAAA", "sub.test.org", record1),
					h.EXPECT().DeleteRecord(ctx, p, ipNetwork, domain, record2, api.RegularDelitionMode).Return(true),
					p.EXPECT().Noticef(pp.EmojiDeletion, "Deleted a stale %s record of %s (ID: %s)", "AAAA", "sub.test.org", record2),
				)
			},
		},
		{
			name: "multiple-stale-records/delete-after-update-timeout/response-failed",
			ip:   ip1,
			resp: setter.ResponseFailed,
			prepareMocks: func(ctx context.Context, cancel func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, ipNetwork, domain, params).Return([]api.Record{
						dnsRecord(record1, ip2, params),
						dnsRecord(record2, ip2, params),
					}, true, true),
					h.EXPECT().UpdateRecord(ctx, p, ipNetwork, domain, record1, ip1, params, params).Return(true),
					p.EXPECT().Noticef(pp.EmojiUpdate, "Updated a stale %s record of %s (ID: %s)", "AAAA", "sub.test.org", record1),
					h.EXPECT().DeleteRecord(ctx, p, ipNetwork, domain, record2, api.RegularDelitionMode).Do(wrapCancelAsDelete(cancel)).Return(false),
					p.EXPECT().Noticef(pp.EmojiError, "Failed to properly update %s records of %s; records might be inconsistent", "AAAA", "sub.test.org"),
				)
			},
		},
		{
			name: "multiple-stale-records/update-first-record/response-failed",
			ip:   ip1,
			resp: setter.ResponseFailed,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, ipNetwork, domain, params).Return([]api.Record{
						dnsRecord(record1, ip2, params),
						dnsRecord(record2, ip2, params),
					}, true, true),
					h.EXPECT().UpdateRecord(ctx, p, ipNetwork, domain, record1, ip1, params, params).Return(false),
					p.EXPECT().Noticef(pp.EmojiError, "Failed to properly update %s records of %s; records might be inconsistent", "AAAA", "sub.test.org"),
				)
			},
		},
		{
			name: "records-unknown/list-records/response-failed",
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

			ctx, h := newSetterHarness(t)
			if tc.prepareMocks != nil {
				tc.prepareMocks(ctx, h.cancel, h.mockPP, h.mockHandle)
			}

			resp := h.setter.Set(ctx, h.mockPP, ipNetwork, domain, tc.ip, params)
			require.Equal(t, tc.resp, resp)
		})
	}
}
