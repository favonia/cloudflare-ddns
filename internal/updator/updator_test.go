package updator_test

import (
	"context"
	"net"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/updator"
)

//nolint:funlen,maintidx
func TestDo(t *testing.T) {
	t.Parallel()

	type anys = []interface{}

	const (
		domain    = api.FQDN("sub.test.org")
		ipNetwork = ipnet.IP6
		record1   = "record1"
		record2   = "record2"
		record3   = "record3"
		ttl       = api.TTL(100)
		proxied   = true
	)
	var (
		ip1 = net.ParseIP("::1")
		ip2 = net.ParseIP("::2")
	)

	for name, tc := range map[string]struct {
		ip                net.IP
		ok                bool
		prepareMockPP     func(m *mocks.MockPP)
		prepareMockHandle func(ctx context.Context, ppfmt pp.PP, m *mocks.MockHandle)
	}{
		"0-nil": {
			nil,
			true,
			func(m *mocks.MockPP) {
				m.EXPECT().Infof(pp.EmojiAlreadyDone, "The %s records of %q are already up to date", "AAAA", "sub.test.org")
			},
			func(ctx context.Context, ppfmt pp.PP, m *mocks.MockHandle) {
				m.EXPECT().ListRecords(ctx, ppfmt, domain, ipNetwork).Return(map[string]net.IP{}, true)
			},
		},
		"0": {
			ip1,
			true,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiAddRecord, "Added a new %s record of %q (ID: %s)", "AAAA", "sub.test.org", record1)
			},
			func(ctx context.Context, ppfmt pp.PP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().ListRecords(ctx, ppfmt, domain, ipNetwork).Return(map[string]net.IP{}, true),
					m.EXPECT().CreateRecord(ctx, ppfmt, domain, ipNetwork, ip1, ttl, proxied).Return(record1, true),
				)
			},
		},
		"1unmatched": {
			ip1,
			true,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUpdateRecord,
					"Updated a stale %s record of %q (ID: %s)",
					"AAAA",
					"sub.test.org",
					record1,
				)
			},
			func(ctx context.Context, ppfmt pp.PP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().ListRecords(ctx, ppfmt, domain, ipNetwork).Return(map[string]net.IP{record1: ip2}, true),
					m.EXPECT().UpdateRecord(ctx, ppfmt, domain, ipNetwork, record1, ip1).Return(true),
				)
			},
		},
		"1unmatched-updatefail": {
			ip1,
			true,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Noticef(pp.EmojiDelRecord, "Deleted a stale %s record of %q (ID: %s)", "AAAA", "sub.test.org", record1),
					m.EXPECT().Noticef(pp.EmojiAddRecord, "Added a new %s record of %q (ID: %s)", "AAAA", "sub.test.org", record2),
				)
			},
			func(ctx context.Context, ppfmt pp.PP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().ListRecords(ctx, ppfmt, domain, ipNetwork).Return(map[string]net.IP{record1: ip2}, true),
					m.EXPECT().UpdateRecord(ctx, ppfmt, domain, ipNetwork, record1, ip1).Return(false),
					m.EXPECT().DeleteRecord(ctx, ppfmt, domain, ipNetwork, record1).Return(true),
					m.EXPECT().CreateRecord(ctx, ppfmt, domain, ipNetwork, ip1, ttl, proxied).Return(record2, true),
				)
			},
		},
		"1unmatched-nil": {
			nil,
			true,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiDelRecord, "Deleted a stale %s record of %q (ID: %s)", "AAAA", "sub.test.org", record1)
			},
			func(ctx context.Context, ppfmt pp.PP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().ListRecords(ctx, ppfmt, domain, ipNetwork).Return(map[string]net.IP{record1: ip1}, true),
					m.EXPECT().DeleteRecord(ctx, ppfmt, domain, ipNetwork, record1).Return(true),
				)
			},
		},
		"1matched": {
			ip1,
			true,
			func(m *mocks.MockPP) {
				m.EXPECT().Infof(pp.EmojiAlreadyDone, "The %s records of %q are already up to date", "AAAA", "sub.test.org")
			},
			func(ctx context.Context, ppfmt pp.PP, m *mocks.MockHandle) {
				m.EXPECT().ListRecords(ctx, ppfmt, domain, ipNetwork).Return(map[string]net.IP{record1: ip1}, true)
			},
		},
		"2matched": {
			ip1,
			true,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(
					pp.EmojiDelRecord,
					"Deleted a duplicate %s record of %q (ID: %s)",
					"AAAA",
					"sub.test.org",
					record2,
				)
			},
			func(ctx context.Context, ppfmt pp.PP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().ListRecords(ctx, ppfmt, domain, ipNetwork).Return(map[string]net.IP{record1: ip1, record2: ip1}, true),
					m.EXPECT().DeleteRecord(ctx, ppfmt, domain, ipNetwork, record2).Return(true),
				)
			},
		},
		"2matched-deletefail": {
			ip1,
			true,
			nil,
			func(ctx context.Context, ppfmt pp.PP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().ListRecords(ctx, ppfmt, domain, ipNetwork).Return(map[string]net.IP{record1: ip1, record2: ip1}, true),
					m.EXPECT().DeleteRecord(ctx, ppfmt, domain, ipNetwork, record2).Return(false),
				)
			},
		},
		"2unmatched": {
			ip1,
			true,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Noticef(
						pp.EmojiUpdateRecord,
						"Updated a stale %s record of %q (ID: %s)",
						"AAAA",
						"sub.test.org",
						record1,
					),
					m.EXPECT().Noticef(pp.EmojiDelRecord, "Deleted a stale %s record of %q (ID: %s)", "AAAA", "sub.test.org", record2),
				)
			},
			func(ctx context.Context, ppfmt pp.PP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().ListRecords(ctx, ppfmt, domain, ipNetwork).Return(map[string]net.IP{record1: ip2, record2: ip2}, true),
					m.EXPECT().UpdateRecord(ctx, ppfmt, domain, ipNetwork, record1, ip1).Return(true),
					m.EXPECT().DeleteRecord(ctx, ppfmt, domain, ipNetwork, record2).Return(true),
				)
			},
		},
		"2unmatched-updatefail": {
			ip1,
			true,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Noticef(pp.EmojiDelRecord, "Deleted a stale %s record of %q (ID: %s)", "AAAA", "sub.test.org", record1),
					m.EXPECT().Noticef(
						pp.EmojiUpdateRecord,
						"Updated a stale %s record of %q (ID: %s)",
						"AAAA",
						"sub.test.org",
						record2,
					),
				)
			},
			func(ctx context.Context, ppfmt pp.PP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().ListRecords(ctx, ppfmt, domain, ipNetwork).Return(map[string]net.IP{record1: ip2, record2: ip2}, true),
					m.EXPECT().UpdateRecord(ctx, ppfmt, domain, ipNetwork, record1, ip1).Return(false),
					m.EXPECT().DeleteRecord(ctx, ppfmt, domain, ipNetwork, record1).Return(true),
					m.EXPECT().UpdateRecord(ctx, ppfmt, domain, ipNetwork, record2, ip1).Return(true),
				)
			},
		},
		"2unmatched-updatefailtwice": {
			ip1,
			true,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Noticef(pp.EmojiDelRecord, "Deleted a stale %s record of %q (ID: %s)", "AAAA", "sub.test.org", record1),
					m.EXPECT().Noticef(pp.EmojiDelRecord, "Deleted a stale %s record of %q (ID: %s)", "AAAA", "sub.test.org", record2),
					m.EXPECT().Noticef(pp.EmojiAddRecord, "Added a new %s record of %q (ID: %s)", "AAAA", "sub.test.org", record3),
				)
			},
			func(ctx context.Context, ppfmt pp.PP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().ListRecords(ctx, ppfmt, domain, ipNetwork).Return(map[string]net.IP{record1: ip2, record2: ip2}, true),
					m.EXPECT().UpdateRecord(ctx, ppfmt, domain, ipNetwork, record1, ip1).Return(false),
					m.EXPECT().DeleteRecord(ctx, ppfmt, domain, ipNetwork, record1).Return(true),
					m.EXPECT().UpdateRecord(ctx, ppfmt, domain, ipNetwork, record2, ip1).Return(false),
					m.EXPECT().DeleteRecord(ctx, ppfmt, domain, ipNetwork, record2).Return(true),
					m.EXPECT().CreateRecord(ctx, ppfmt, domain, ipNetwork, ip1, ttl, proxied).Return(record3, true),
				)
			},
		},
		//nolint:dupl
		"2unmatched-updatefail-deletefail-updatefail": {
			ip1,
			false,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Noticef(pp.EmojiDelRecord, "Deleted a stale %s record of %q (ID: %s)", "AAAA", "sub.test.org", record2),
					m.EXPECT().Noticef(pp.EmojiAddRecord, "Added a new %s record of %q (ID: %s)", "AAAA", "sub.test.org", record3),
					m.EXPECT().Errorf(pp.EmojiError, "Failed to (fully) update %s records of %q", "AAAA", "sub.test.org"),
				)
			},
			func(ctx context.Context, ppfmt pp.PP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().ListRecords(ctx, ppfmt, domain, ipNetwork).Return(map[string]net.IP{record1: ip2, record2: ip2}, true),
					m.EXPECT().UpdateRecord(ctx, ppfmt, domain, ipNetwork, record1, ip1).Return(false),
					m.EXPECT().DeleteRecord(ctx, ppfmt, domain, ipNetwork, record1).Return(false),
					m.EXPECT().UpdateRecord(ctx, ppfmt, domain, ipNetwork, record2, ip1).Return(false),
					m.EXPECT().DeleteRecord(ctx, ppfmt, domain, ipNetwork, record2).Return(true),
					m.EXPECT().CreateRecord(ctx, ppfmt, domain, ipNetwork, ip1, ttl, proxied).Return(record3, true),
				)
			},
		},
		//nolint:dupl
		"2unmatched-updatefailtwice-createfail": {
			ip1,
			false,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Noticef(pp.EmojiDelRecord, "Deleted a stale %s record of %q (ID: %s)", "AAAA", "sub.test.org", record1),
					m.EXPECT().Noticef(pp.EmojiDelRecord, "Deleted a stale %s record of %q (ID: %s)", "AAAA", "sub.test.org", record2),
					m.EXPECT().Errorf(pp.EmojiError, "Failed to (fully) update %s records of %q", "AAAA", "sub.test.org"),
				)
			},
			func(ctx context.Context, ppfmt pp.PP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().ListRecords(ctx, ppfmt, domain, ipNetwork).Return(map[string]net.IP{record1: ip2, record2: ip2}, true),
					m.EXPECT().UpdateRecord(ctx, ppfmt, domain, ipNetwork, record1, ip1).Return(false),
					m.EXPECT().DeleteRecord(ctx, ppfmt, domain, ipNetwork, record1).Return(true),
					m.EXPECT().UpdateRecord(ctx, ppfmt, domain, ipNetwork, record2, ip1).Return(false),
					m.EXPECT().DeleteRecord(ctx, ppfmt, domain, ipNetwork, record2).Return(true),
					m.EXPECT().CreateRecord(ctx, ppfmt, domain, ipNetwork, ip1, ttl, proxied).Return(record3, false),
				)
			},
		},
		"listfail": {
			ip1,
			false,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiError, "Failed to (fully) update %s records of %q", "AAAA", "sub.test.org")
			},
			func(ctx context.Context, ppfmt pp.PP, m *mocks.MockHandle) {
				m.EXPECT().ListRecords(ctx, ppfmt, domain, ipNetwork).Return(nil, false)
			},
		},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			mockCtrl := gomock.NewController(t)

			ctx := context.Background()

			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}
			mockHandle := mocks.NewMockHandle(mockCtrl)
			if tc.prepareMockHandle != nil {
				tc.prepareMockHandle(ctx, mockPP, mockHandle)
			}

			ok := updator.Do(ctx, mockPP,
				&updator.Args{
					Handle:    mockHandle,
					IPNetwork: ipNetwork,
					IP:        tc.ip,
					Domain:    domain,
					TTL:       ttl,
					Proxied:   proxied,
				})
			require.Equal(t, tc.ok, ok)
		})
	}
}
