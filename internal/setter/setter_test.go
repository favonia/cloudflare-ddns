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

func wrapCancelAsCreate(cancel func()) func(context.Context, pp.PP, domain.Domain, ipnet.Type, netip.Addr, api.TTL, bool, string) (string, bool) { //nolint:lll,unused
	return func(context.Context, pp.PP, domain.Domain, ipnet.Type, netip.Addr, api.TTL, bool, string) (string, bool) {
		cancel()
		return "", false
	}
}

func wrapCancelAsUpdate(cancel func()) func(context.Context, pp.PP, domain.Domain, ipnet.Type, string, netip.Addr) bool { //nolint:lll
	return func(context.Context, pp.PP, domain.Domain, ipnet.Type, string, netip.Addr) bool {
		cancel()
		return false
	}
}

func wrapCancelAsDelete(cancel func()) func(context.Context, pp.PP, domain.Domain, ipnet.Type, string) bool {
	return func(context.Context, pp.PP, domain.Domain, ipnet.Type, string) bool {
		cancel()
		return false
	}
}

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
		prepareMockPP     func(m *mocks.MockPP)
		prepareMockHandle func(ctx context.Context, cancel func(), ppfmt pp.PP, m *mocks.MockHandle)
	}{
		"0": {
			ip1,
			setter.ResponseUpdated,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiCreateRecord, "Added a new %s record of %q (ID: %q)", "AAAA", "sub.test.org", record1)
			},
			func(ctx context.Context, _ func(), ppfmt pp.PP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().ListRecords(ctx, ppfmt, domain, ipNetwork).Return(map[string]netip.Addr{}, true, true),
					m.EXPECT().CreateRecord(ctx, ppfmt, domain, ipNetwork, ip1, api.TTLAuto, false, "hello").Return(record1, true),
				)
			},
		},
		"1unmatched": {
			ip1,
			setter.ResponseUpdated,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUpdateRecord,
					"Updated a stale %s record of %q (ID: %q)",
					"AAAA",
					"sub.test.org",
					record1,
				)
			},
			func(ctx context.Context, _ func(), ppfmt pp.PP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().ListRecords(ctx, ppfmt, domain, ipNetwork).Return(map[string]netip.Addr{record1: ip2}, true, true),
					m.EXPECT().UpdateRecord(ctx, ppfmt, domain, ipNetwork, record1, ip1).Return(true),
				)
			},
		},
		"1unmatched-updatefail": {
			ip1,
			setter.ResponseUpdated,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Noticef(pp.EmojiDeleteRecord, "Deleted a stale %s record of %q (ID: %q)", "AAAA", "sub.test.org", record1), //nolint:lll
					m.EXPECT().Noticef(pp.EmojiCreateRecord, "Added a new %s record of %q (ID: %q)", "AAAA", "sub.test.org", record2),
				)
			},
			func(ctx context.Context, _ func(), ppfmt pp.PP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().ListRecords(ctx, ppfmt, domain, ipNetwork).Return(map[string]netip.Addr{record1: ip2}, true, true),
					m.EXPECT().UpdateRecord(ctx, ppfmt, domain, ipNetwork, record1, ip1).Return(false),
					m.EXPECT().DeleteRecord(ctx, ppfmt, domain, ipNetwork, record1).Return(true),
					m.EXPECT().CreateRecord(ctx, ppfmt, domain, ipNetwork, ip1, api.TTLAuto, false, "hello").Return(record2, true),
				)
			},
		},
		"1unmatched-updatetimeout": {
			ip1,
			setter.ResponseFailed,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Infof(pp.EmojiBailingOut, "Operation aborted (%v); bailing out . . .", gomock.Any()),
				)
			},
			func(ctx context.Context, cancel func(), ppfmt pp.PP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().ListRecords(ctx, ppfmt, domain, ipNetwork).
						Return(map[string]netip.Addr{record1: ip2}, true, true),
					m.EXPECT().UpdateRecord(ctx, ppfmt, domain, ipNetwork, record1, ip1).
						Do(wrapCancelAsUpdate(cancel)).Return(false),
				)
			},
		},
		"1unmatched-deletetimeout": {
			ip1,
			setter.ResponseFailed,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Infof(pp.EmojiBailingOut, "Operation aborted (%v); bailing out . . .", gomock.Any()),
				)
			},
			func(ctx context.Context, cancel func(), ppfmt pp.PP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().ListRecords(ctx, ppfmt, domain, ipNetwork).
						Return(map[string]netip.Addr{record1: ip2}, true, true),
					m.EXPECT().UpdateRecord(ctx, ppfmt, domain, ipNetwork, record1, ip1).
						Return(false),
					m.EXPECT().DeleteRecord(ctx, ppfmt, domain, ipNetwork, record1).
						Do(wrapCancelAsDelete(cancel)).Return(false),
				)
			},
		},
		"1matched": {
			ip1,
			setter.ResponseNoop,
			func(m *mocks.MockPP) {
				m.EXPECT().Infof(pp.EmojiAlreadyDone,
					"The %s records of %q are already up to date (cached)", "AAAA", "sub.test.org")
			},
			func(ctx context.Context, _ func(), ppfmt pp.PP, m *mocks.MockHandle) {
				m.EXPECT().ListRecords(ctx, ppfmt, domain, ipNetwork).Return(map[string]netip.Addr{record1: ip1}, true, true)
			},
		},
		"1matched/not-cached": {
			ip1,
			setter.ResponseNoop,
			func(m *mocks.MockPP) {
				m.EXPECT().Infof(pp.EmojiAlreadyDone, "The %s records of %q are already up to date", "AAAA", "sub.test.org")
			},
			func(ctx context.Context, _ func(), ppfmt pp.PP, m *mocks.MockHandle) {
				m.EXPECT().ListRecords(ctx, ppfmt, domain, ipNetwork).Return(map[string]netip.Addr{record1: ip1}, false, true)
			},
		},
		"2matched": {
			ip1,
			setter.ResponseUpdated,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(
					pp.EmojiDeleteRecord,
					"Deleted a duplicate %s record of %q (ID: %q)",
					"AAAA",
					"sub.test.org",
					record2,
				)
			},
			func(ctx context.Context, _ func(), ppfmt pp.PP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().ListRecords(ctx, ppfmt, domain, ipNetwork).Return(map[string]netip.Addr{record1: ip1, record2: ip1}, true, true), //nolint:lll
					m.EXPECT().DeleteRecord(ctx, ppfmt, domain, ipNetwork, record2).Return(true),
				)
			},
		},
		"2matched-deletefail": {
			ip1,
			setter.ResponseUpdated,
			nil,
			func(ctx context.Context, _ func(), ppfmt pp.PP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().ListRecords(ctx, ppfmt, domain, ipNetwork).Return(map[string]netip.Addr{record1: ip1, record2: ip1}, true, true), //nolint:lll
					m.EXPECT().DeleteRecord(ctx, ppfmt, domain, ipNetwork, record2).Return(false),
				)
			},
		},
		"2unmatched": {
			ip1,
			setter.ResponseUpdated,
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
			func(ctx context.Context, _ func(), ppfmt pp.PP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().ListRecords(ctx, ppfmt, domain, ipNetwork).Return(map[string]netip.Addr{record1: ip2, record2: ip2}, true, true), //nolint:lll
					m.EXPECT().UpdateRecord(ctx, ppfmt, domain, ipNetwork, record1, ip1).Return(true),
					m.EXPECT().DeleteRecord(ctx, ppfmt, domain, ipNetwork, record2).Return(true),
				)
			},
		},
		"2unmatched-updatefail": {
			ip1,
			setter.ResponseUpdated,
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
			func(ctx context.Context, _ func(), ppfmt pp.PP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().ListRecords(ctx, ppfmt, domain, ipNetwork).Return(map[string]netip.Addr{record1: ip2, record2: ip2}, true, true), //nolint:lll
					m.EXPECT().UpdateRecord(ctx, ppfmt, domain, ipNetwork, record1, ip1).Return(false),
					m.EXPECT().DeleteRecord(ctx, ppfmt, domain, ipNetwork, record1).Return(true),
					m.EXPECT().UpdateRecord(ctx, ppfmt, domain, ipNetwork, record2, ip1).Return(true),
				)
			},
		},
		"2unmatched-updatefailtwice": {
			ip1,
			setter.ResponseUpdated,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Noticef(pp.EmojiDeleteRecord, "Deleted a stale %s record of %q (ID: %q)", "AAAA", "sub.test.org", record1), //nolint:lll
					m.EXPECT().Noticef(pp.EmojiDeleteRecord, "Deleted a stale %s record of %q (ID: %q)", "AAAA", "sub.test.org", record2), //nolint:lll
					m.EXPECT().Noticef(pp.EmojiCreateRecord, "Added a new %s record of %q (ID: %q)", "AAAA", "sub.test.org", record3),
				)
			},
			func(ctx context.Context, _ func(), ppfmt pp.PP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().ListRecords(ctx, ppfmt, domain, ipNetwork).Return(map[string]netip.Addr{record1: ip2, record2: ip2}, true, true), //nolint:lll
					m.EXPECT().UpdateRecord(ctx, ppfmt, domain, ipNetwork, record1, ip1).Return(false),
					m.EXPECT().DeleteRecord(ctx, ppfmt, domain, ipNetwork, record1).Return(true),
					m.EXPECT().UpdateRecord(ctx, ppfmt, domain, ipNetwork, record2, ip1).Return(false),
					m.EXPECT().DeleteRecord(ctx, ppfmt, domain, ipNetwork, record2).Return(true),
					m.EXPECT().CreateRecord(ctx, ppfmt, domain, ipNetwork, ip1, api.TTLAuto, false, "hello").Return(record3, true),
				)
			},
		},
		//nolint:dupl
		"2unmatched-updatefail-deletefail-updatefail": {
			ip1,
			setter.ResponseFailed,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Noticef(pp.EmojiDeleteRecord, "Deleted a stale %s record of %q (ID: %q)", "AAAA", "sub.test.org", record2),                 //nolint:lll
					m.EXPECT().Noticef(pp.EmojiCreateRecord, "Added a new %s record of %q (ID: %q)", "AAAA", "sub.test.org", record3),                     //nolint:lll
					m.EXPECT().Errorf(pp.EmojiError, "Failed to finish updating %s records of %q; records might be inconsistent", "AAAA", "sub.test.org"), //nolint:lll
				)
			},
			func(ctx context.Context, _ func(), ppfmt pp.PP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().ListRecords(ctx, ppfmt, domain, ipNetwork).Return(map[string]netip.Addr{record1: ip2, record2: ip2}, true, true), //nolint:lll
					m.EXPECT().UpdateRecord(ctx, ppfmt, domain, ipNetwork, record1, ip1).Return(false),
					m.EXPECT().DeleteRecord(ctx, ppfmt, domain, ipNetwork, record1).Return(false),
					m.EXPECT().UpdateRecord(ctx, ppfmt, domain, ipNetwork, record2, ip1).Return(false),
					m.EXPECT().DeleteRecord(ctx, ppfmt, domain, ipNetwork, record2).Return(true),
					m.EXPECT().CreateRecord(ctx, ppfmt, domain, ipNetwork, ip1, api.TTLAuto, false, "hello").Return(record3, true),
				)
			},
		},
		//nolint:dupl
		"2unmatched-updatefailtwice-createfail": {
			ip1,
			setter.ResponseFailed,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Noticef(pp.EmojiDeleteRecord, "Deleted a stale %s record of %q (ID: %q)", "AAAA", "sub.test.org", record1),                 //nolint:lll
					m.EXPECT().Noticef(pp.EmojiDeleteRecord, "Deleted a stale %s record of %q (ID: %q)", "AAAA", "sub.test.org", record2),                 //nolint:lll
					m.EXPECT().Errorf(pp.EmojiError, "Failed to finish updating %s records of %q; records might be inconsistent", "AAAA", "sub.test.org"), //nolint:lll
				)
			},
			func(ctx context.Context, _ func(), ppfmt pp.PP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().ListRecords(ctx, ppfmt, domain, ipNetwork).Return(map[string]netip.Addr{record1: ip2, record2: ip2}, true, true), //nolint:lll
					m.EXPECT().UpdateRecord(ctx, ppfmt, domain, ipNetwork, record1, ip1).Return(false),
					m.EXPECT().DeleteRecord(ctx, ppfmt, domain, ipNetwork, record1).Return(true),
					m.EXPECT().UpdateRecord(ctx, ppfmt, domain, ipNetwork, record2, ip1).Return(false),
					m.EXPECT().DeleteRecord(ctx, ppfmt, domain, ipNetwork, record2).Return(true),
					m.EXPECT().CreateRecord(ctx, ppfmt, domain, ipNetwork, ip1, api.TTLAuto, false, "hello").Return(record3, false),
				)
			},
		},
		"listfail": {
			ip1,
			setter.ResponseFailed,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiError, "Failed to retrieve the current %s records of %q", "AAAA", "sub.test.org")
			},
			func(ctx context.Context, _ func(), ppfmt pp.PP, m *mocks.MockHandle) {
				m.EXPECT().ListRecords(ctx, ppfmt, domain, ipNetwork).Return(nil, false, false)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			mockCtrl := gomock.NewController(t)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}
			mockHandle := mocks.NewMockHandle(mockCtrl)
			if tc.prepareMockHandle != nil {
				tc.prepareMockHandle(ctx, cancel, mockPP, mockHandle)
			}

			s, ok := setter.New(mockPP, mockHandle)
			require.True(t, ok)

			resp := s.Set(ctx, mockPP, domain, ipNetwork, tc.ip, api.TTLAuto, false, "hello")
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
			setter.ResponseNoop,
			func(m *mocks.MockPP) {
				m.EXPECT().Infof(pp.EmojiAlreadyDone, "The %s records of %q were already deleted (cached)", "AAAA", "sub.test.org")
			},
			func(ctx context.Context, ppfmt pp.PP, m *mocks.MockHandle) {
				m.EXPECT().ListRecords(ctx, ppfmt, domain, ipNetwork).Return(map[string]netip.Addr{}, true, true)
			},
		},
		"0/not-cached": {
			setter.ResponseNoop,
			func(m *mocks.MockPP) {
				m.EXPECT().Infof(pp.EmojiAlreadyDone, "The %s records of %q were already deleted", "AAAA", "sub.test.org")
			},
			func(ctx context.Context, ppfmt pp.PP, m *mocks.MockHandle) {
				m.EXPECT().ListRecords(ctx, ppfmt, domain, ipNetwork).Return(map[string]netip.Addr{}, false, true)
			},
		},
		"1unmatched": {
			setter.ResponseUpdated,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiDeleteRecord, "Deleted a stale %s record of %q (ID: %q)", "AAAA", "sub.test.org", record1) //nolint:lll
			},
			func(ctx context.Context, ppfmt pp.PP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().ListRecords(ctx, ppfmt, domain, ipNetwork).Return(map[string]netip.Addr{record1: ip1}, true, true),
					m.EXPECT().DeleteRecord(ctx, ppfmt, domain, ipNetwork, record1).Return(true),
				)
			},
		},
		"1unmatched/fail": {
			setter.ResponseFailed,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiError, "Failed to finish deleting %s records of %q; records might be inconsistent", "AAAA", "sub.test.org") //nolint:lll
			},
			func(ctx context.Context, ppfmt pp.PP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().ListRecords(ctx, ppfmt, domain, ipNetwork).Return(map[string]netip.Addr{record1: ip1}, true, true),
					m.EXPECT().DeleteRecord(ctx, ppfmt, domain, ipNetwork, record1).Return(false),
				)
			},
		},
		"impossible-records": {
			setter.ResponseUpdated,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Noticef(pp.EmojiDeleteRecord, "Deleted a stale %s record of %q (ID: %q)", "AAAA", "sub.test.org", record1), //nolint:lll
					m.EXPECT().Noticef(pp.EmojiDeleteRecord, "Deleted a stale %s record of %q (ID: %q)", "AAAA", "sub.test.org", record2), //nolint:lll
				)
			},
			func(ctx context.Context, ppfmt pp.PP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().ListRecords(ctx, ppfmt, domain, ipNetwork).Return(map[string]netip.Addr{record1: ip1, record2: invalidIP}, true, true), //nolint:lll
					m.EXPECT().DeleteRecord(ctx, ppfmt, domain, ipNetwork, record1).Return(true),
					m.EXPECT().DeleteRecord(ctx, ppfmt, domain, ipNetwork, record2).Return(true),
				)
			},
		},
		"listfail": {
			setter.ResponseFailed,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiError, "Failed to retrieve the current %s records of %q", "AAAA", "sub.test.org")
			},
			func(ctx context.Context, ppfmt pp.PP, m *mocks.MockHandle) {
				m.EXPECT().ListRecords(ctx, ppfmt, domain, ipNetwork).Return(nil, false, false)
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
