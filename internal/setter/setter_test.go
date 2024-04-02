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
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/setter"
)

//nolint:funlen
func TestSet(t *testing.T) {
	t.Parallel()

	const (
		domain    = domain.FQDN("sub.test.org")
		ipNetwork = ipnet.IP6
		record1   = "record1"
		record2   = "record2"
		record3   = "record3"
	)
	var (
		ip1 = netip.MustParseAddr("::1")
		ip2 = netip.MustParseAddr("::2")
	)

	for name, tc := range map[string]struct {
		ip                netip.Addr
		resp              setter.ResponseCode
		ttl               api.TTL
		proxied           bool
		prepareMockPP     func(m *mocks.MockPP)
		prepareMockHandle func(ctx context.Context, ppfmt pp.PP, m *mocks.MockHandle)
	}{
		"0/1-false": {
			ip1,
			setter.ResponseUpdatesApplied,
			1,
			false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiCreateRecord, "Added a new %s record of %q (ID: %q)", "AAAA", "sub.test.org", record1)
			},
			func(ctx context.Context, ppfmt pp.PP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().ListRecords(ctx, ppfmt, domain, ipNetwork).Return(map[string]netip.Addr{}, true),
					m.EXPECT().CreateRecord(ctx, ppfmt, domain, ipNetwork, ip1, api.TTL(1), false).Return(record1, true),
				)
			},
		},
		"1unmatched/300-false": {
			ip1,
			setter.ResponseUpdatesApplied,
			300,
			false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUpdateRecord,
					"Updated a stale %s record of %q (ID: %q)",
					"AAAA",
					"sub.test.org",
					record1,
				)
			},
			func(ctx context.Context, ppfmt pp.PP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().ListRecords(ctx, ppfmt, domain, ipNetwork).Return(map[string]netip.Addr{record1: ip2}, true),
					m.EXPECT().UpdateRecord(ctx, ppfmt, domain, ipNetwork, record1, ip1).Return(true),
				)
			},
		},
		"1unmatched-updatefail/300-false": {
			ip1,
			setter.ResponseUpdatesApplied,
			300,
			false,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Noticef(pp.EmojiDeleteRecord, "Deleted a stale %s record of %q (ID: %q)", "AAAA", "sub.test.org", record1), //nolint:lll
					m.EXPECT().Noticef(pp.EmojiCreateRecord, "Added a new %s record of %q (ID: %q)", "AAAA", "sub.test.org", record2),
				)
			},
			func(ctx context.Context, ppfmt pp.PP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().ListRecords(ctx, ppfmt, domain, ipNetwork).Return(map[string]netip.Addr{record1: ip2}, true),
					m.EXPECT().UpdateRecord(ctx, ppfmt, domain, ipNetwork, record1, ip1).Return(false),
					m.EXPECT().DeleteRecord(ctx, ppfmt, domain, ipNetwork, record1).Return(true),
					m.EXPECT().CreateRecord(ctx, ppfmt, domain, ipNetwork, ip1, api.TTL(300), false).Return(record2, true),
				)
			},
		},
		"1matched/300-false": {
			ip1,
			setter.ResponseNoUpdatesNeeded,
			300,
			false,
			func(m *mocks.MockPP) {
				m.EXPECT().Infof(pp.EmojiAlreadyDone, "The %s records of %q are already up to date", "AAAA", "sub.test.org")
			},
			func(ctx context.Context, ppfmt pp.PP, m *mocks.MockHandle) {
				m.EXPECT().ListRecords(ctx, ppfmt, domain, ipNetwork).Return(map[string]netip.Addr{record1: ip1}, true)
			},
		},
		"2matched/300-false": {
			ip1,
			setter.ResponseUpdatesApplied,
			300,
			false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(
					pp.EmojiDeleteRecord,
					"Deleted a duplicate %s record of %q (ID: %q)",
					"AAAA",
					"sub.test.org",
					record2,
				)
			},
			func(ctx context.Context, ppfmt pp.PP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().ListRecords(ctx, ppfmt, domain, ipNetwork).Return(map[string]netip.Addr{record1: ip1, record2: ip1}, true), //nolint:lll
					m.EXPECT().DeleteRecord(ctx, ppfmt, domain, ipNetwork, record2).Return(true),
				)
			},
		},
		"2matched-deletefail/300-false": {
			ip1,
			setter.ResponseUpdatesApplied,
			300,
			false,
			nil,
			func(ctx context.Context, ppfmt pp.PP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().ListRecords(ctx, ppfmt, domain, ipNetwork).Return(map[string]netip.Addr{record1: ip1, record2: ip1}, true), //nolint:lll
					m.EXPECT().DeleteRecord(ctx, ppfmt, domain, ipNetwork, record2).Return(false),
				)
			},
		},
		"2unmatched/300-false": {
			ip1,
			setter.ResponseUpdatesApplied,
			300,
			false,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Noticef(
						pp.EmojiUpdateRecord,
						"Updated a stale %s record of %q (ID: %q)",
						"AAAA",
						"sub.test.org",
						record1,
					),
					m.EXPECT().Noticef(pp.EmojiDeleteRecord,
						"Deleted a stale %s record of %q (ID: %q)",
						"AAAA",
						"sub.test.org",
						record2),
				)
			},
			func(ctx context.Context, ppfmt pp.PP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().ListRecords(ctx, ppfmt, domain, ipNetwork).Return(map[string]netip.Addr{record1: ip2, record2: ip2}, true), //nolint:lll
					m.EXPECT().UpdateRecord(ctx, ppfmt, domain, ipNetwork, record1, ip1).Return(true),
					m.EXPECT().DeleteRecord(ctx, ppfmt, domain, ipNetwork, record2).Return(true),
				)
			},
		},
		"2unmatched-updatefail/300-false": {
			ip1,
			setter.ResponseUpdatesApplied,
			300,
			false,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Noticef(pp.EmojiDeleteRecord, "Deleted a stale %s record of %q (ID: %q)", "AAAA", "sub.test.org", record1), //nolint:lll
					m.EXPECT().Noticef(
						pp.EmojiUpdateRecord,
						"Updated a stale %s record of %q (ID: %q)",
						"AAAA",
						"sub.test.org",
						record2,
					),
				)
			},
			func(ctx context.Context, ppfmt pp.PP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().ListRecords(ctx, ppfmt, domain, ipNetwork).Return(map[string]netip.Addr{record1: ip2, record2: ip2}, true), //nolint:lll
					m.EXPECT().UpdateRecord(ctx, ppfmt, domain, ipNetwork, record1, ip1).Return(false),
					m.EXPECT().DeleteRecord(ctx, ppfmt, domain, ipNetwork, record1).Return(true),
					m.EXPECT().UpdateRecord(ctx, ppfmt, domain, ipNetwork, record2, ip1).Return(true),
				)
			},
		},
		"2unmatched-updatefailtwice/300-false": {
			ip1,
			setter.ResponseUpdatesApplied,
			300,
			false,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Noticef(pp.EmojiDeleteRecord, "Deleted a stale %s record of %q (ID: %q)", "AAAA", "sub.test.org", record1), //nolint:lll
					m.EXPECT().Noticef(pp.EmojiDeleteRecord, "Deleted a stale %s record of %q (ID: %q)", "AAAA", "sub.test.org", record2), //nolint:lll
					m.EXPECT().Noticef(pp.EmojiCreateRecord, "Added a new %s record of %q (ID: %q)", "AAAA", "sub.test.org", record3),
				)
			},
			func(ctx context.Context, ppfmt pp.PP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().ListRecords(ctx, ppfmt, domain, ipNetwork).Return(map[string]netip.Addr{record1: ip2, record2: ip2}, true), //nolint:lll
					m.EXPECT().UpdateRecord(ctx, ppfmt, domain, ipNetwork, record1, ip1).Return(false),
					m.EXPECT().DeleteRecord(ctx, ppfmt, domain, ipNetwork, record1).Return(true),
					m.EXPECT().UpdateRecord(ctx, ppfmt, domain, ipNetwork, record2, ip1).Return(false),
					m.EXPECT().DeleteRecord(ctx, ppfmt, domain, ipNetwork, record2).Return(true),
					m.EXPECT().CreateRecord(ctx, ppfmt, domain, ipNetwork, ip1, api.TTL(300), false).Return(record3, true),
				)
			},
		},
		//nolint:dupl
		"2unmatched-updatefail-deletefail-updatefail/300-false": {
			ip1,
			setter.ResponseUpdatesFailed,
			300,
			false,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Noticef(pp.EmojiDeleteRecord, "Deleted a stale %s record of %q (ID: %q)", "AAAA", "sub.test.org", record2),                      //nolint:lll
					m.EXPECT().Noticef(pp.EmojiCreateRecord, "Added a new %s record of %q (ID: %q)", "AAAA", "sub.test.org", record3),                          //nolint:lll
					m.EXPECT().Errorf(pp.EmojiError, "Failed to complete updating of %s records of %q; records might be inconsistent", "AAAA", "sub.test.org"), //nolint:lll
				)
			},
			func(ctx context.Context, ppfmt pp.PP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().ListRecords(ctx, ppfmt, domain, ipNetwork).Return(map[string]netip.Addr{record1: ip2, record2: ip2}, true), //nolint:lll
					m.EXPECT().UpdateRecord(ctx, ppfmt, domain, ipNetwork, record1, ip1).Return(false),
					m.EXPECT().DeleteRecord(ctx, ppfmt, domain, ipNetwork, record1).Return(false),
					m.EXPECT().UpdateRecord(ctx, ppfmt, domain, ipNetwork, record2, ip1).Return(false),
					m.EXPECT().DeleteRecord(ctx, ppfmt, domain, ipNetwork, record2).Return(true),
					m.EXPECT().CreateRecord(ctx, ppfmt, domain, ipNetwork, ip1, api.TTL(300), false).Return(record3, true),
				)
			},
		},
		//nolint:dupl
		"2unmatched-updatefailtwice-createfail/300-false": {
			ip1,
			setter.ResponseUpdatesFailed,
			300,
			false,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Noticef(pp.EmojiDeleteRecord, "Deleted a stale %s record of %q (ID: %q)", "AAAA", "sub.test.org", record1),                      //nolint:lll
					m.EXPECT().Noticef(pp.EmojiDeleteRecord, "Deleted a stale %s record of %q (ID: %q)", "AAAA", "sub.test.org", record2),                      //nolint:lll
					m.EXPECT().Errorf(pp.EmojiError, "Failed to complete updating of %s records of %q; records might be inconsistent", "AAAA", "sub.test.org"), //nolint:lll
				)
			},
			func(ctx context.Context, ppfmt pp.PP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().ListRecords(ctx, ppfmt, domain, ipNetwork).Return(map[string]netip.Addr{record1: ip2, record2: ip2}, true), //nolint:lll
					m.EXPECT().UpdateRecord(ctx, ppfmt, domain, ipNetwork, record1, ip1).Return(false),
					m.EXPECT().DeleteRecord(ctx, ppfmt, domain, ipNetwork, record1).Return(true),
					m.EXPECT().UpdateRecord(ctx, ppfmt, domain, ipNetwork, record2, ip1).Return(false),
					m.EXPECT().DeleteRecord(ctx, ppfmt, domain, ipNetwork, record2).Return(true),
					m.EXPECT().CreateRecord(ctx, ppfmt, domain, ipNetwork, ip1, api.TTL(300), false).Return(record3, false),
				)
			},
		},
		"listfail/300-false": {
			ip1,
			setter.ResponseUpdatesFailed,
			300,
			false,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiError, "Failed to retrieve the current %s records of %q", "AAAA", "sub.test.org")
			},
			func(ctx context.Context, ppfmt pp.PP, m *mocks.MockHandle) {
				m.EXPECT().ListRecords(ctx, ppfmt, domain, ipNetwork).Return(nil, false)
			},
		},
	} {
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

			s, ok := setter.New(mockPP, mockHandle)
			require.True(t, ok)

			resp := s.Set(ctx, mockPP, domain, ipNetwork, tc.ip, tc.ttl, tc.proxied)
			require.Equal(t, tc.resp, resp)
		})
	}
}

//nolint:funlen
func TestDelete(t *testing.T) {
	t.Parallel()

	const (
		domain    = domain.FQDN("sub.test.org")
		ipNetwork = ipnet.IP6
		record1   = "record1"
		record2   = "record2"
		record3   = "record3"
	)
	var (
		ip1       = netip.MustParseAddr("::1")
		invalidIP = netip.Addr{}
	)

	for name, tc := range map[string]struct {
		resp              setter.ResponseCode
		prepareMockPP     func(m *mocks.MockPP)
		prepareMockHandle func(ctx context.Context, ppfmt pp.PP, m *mocks.MockHandle)
	}{
		"0": {
			setter.ResponseNoUpdatesNeeded,
			func(m *mocks.MockPP) {
				m.EXPECT().Infof(pp.EmojiAlreadyDone, "The %s records of %q were already deleted", "AAAA", "sub.test.org")
			},
			func(ctx context.Context, ppfmt pp.PP, m *mocks.MockHandle) {
				m.EXPECT().ListRecords(ctx, ppfmt, domain, ipNetwork).Return(map[string]netip.Addr{}, true)
			},
		},
		"1unmatched": {
			setter.ResponseUpdatesApplied,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiDeleteRecord, "Deleted a stale %s record of %q (ID: %q)", "AAAA", "sub.test.org", record1) //nolint:lll
			},
			func(ctx context.Context, ppfmt pp.PP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().ListRecords(ctx, ppfmt, domain, ipNetwork).Return(map[string]netip.Addr{record1: ip1}, true),
					m.EXPECT().DeleteRecord(ctx, ppfmt, domain, ipNetwork, record1).Return(true),
				)
			},
		},
		"1unmatched/fail": {
			setter.ResponseUpdatesFailed,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiError, "Failed to complete deleting of %s records of %q; records might be inconsistent", "AAAA", "sub.test.org") //nolint:lll
			},
			func(ctx context.Context, ppfmt pp.PP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().ListRecords(ctx, ppfmt, domain, ipNetwork).Return(map[string]netip.Addr{record1: ip1}, true),
					m.EXPECT().DeleteRecord(ctx, ppfmt, domain, ipNetwork, record1).Return(false),
				)
			},
		},
		"impossible-records": {
			setter.ResponseUpdatesApplied,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Noticef(pp.EmojiDeleteRecord, "Deleted a stale %s record of %q (ID: %q)", "AAAA", "sub.test.org", record1), //nolint:lll
					m.EXPECT().Noticef(pp.EmojiDeleteRecord, "Deleted a stale %s record of %q (ID: %q)", "AAAA", "sub.test.org", record2), //nolint:lll
				)
			},
			func(ctx context.Context, ppfmt pp.PP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().ListRecords(ctx, ppfmt, domain, ipNetwork).Return(map[string]netip.Addr{record1: ip1, record2: invalidIP}, true), //nolint:lll
					m.EXPECT().DeleteRecord(ctx, ppfmt, domain, ipNetwork, record1).Return(true),
					m.EXPECT().DeleteRecord(ctx, ppfmt, domain, ipNetwork, record2).Return(true),
				)
			},
		},
		"listfail": {
			setter.ResponseUpdatesFailed,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiError, "Failed to retrieve the current %s records of %q", "AAAA", "sub.test.org")
			},
			func(ctx context.Context, ppfmt pp.PP, m *mocks.MockHandle) {
				m.EXPECT().ListRecords(ctx, ppfmt, domain, ipNetwork).Return(nil, false)
			},
		},
	} {
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

			s, ok := setter.New(mockPP, mockHandle)
			require.True(t, ok)

			resp := s.Delete(ctx, mockPP, domain, ipNetwork)
			require.Equal(t, tc.resp, resp)
		})
	}
}
