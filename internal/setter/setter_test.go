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

func wrapCancelAsCreate(cancel func()) func(context.Context, pp.PP, domain.Domain, ipnet.Type, netip.Addr, api.TTL, bool, string) (string, bool) { //nolint:lll
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
func TestSanityCheck(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		answer bool
	}{
		"true":  {true},
		"false": {false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			mockCtrl := gomock.NewController(t)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			mockPP := mocks.NewMockPP(mockCtrl)
			mockHandle := mocks.NewMockHandle(mockCtrl)

			s, ok := setter.New(mockPP, mockHandle)
			require.True(t, ok)

			mockHandle.EXPECT().SanityCheck(ctx, mockPP).Return(tc.answer)
			require.Equal(t, tc.answer, s.SanityCheck(ctx, mockPP))
		})
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
		ip           netip.Addr
		resp         setter.ResponseCode
		prepareMocks func(ctx context.Context, cancel func(), p *mocks.MockPP, m *mocks.MockHandle)
	}{
		"0": {
			ip1,
			setter.ResponseUpdated,
			func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, domain, ipNetwork).Return(map[string]netip.Addr{}, true, true),
					h.EXPECT().CreateRecord(ctx, p, domain, ipNetwork, ip1, api.TTLAuto, false, "hello").Return(record1, true),
					p.EXPECT().Noticef(pp.EmojiCreation, "Added a new %s record of %q (ID: %q)", "AAAA", "sub.test.org", record1),
				)
			},
		},
		"1unmatched": {
			ip1,
			setter.ResponseUpdated,
			func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, domain, ipNetwork).Return(map[string]netip.Addr{record1: ip2}, true, true),
					h.EXPECT().UpdateRecord(ctx, p, domain, ipNetwork, record1, ip1).Return(true),
					p.EXPECT().Noticef(pp.EmojiUpdate,
						"Updated a stale %s record of %q (ID: %q)",
						"AAAA",
						"sub.test.org",
						record1,
					),
				)
			},
		},
		"1unmatched/update-fail": {
			ip1,
			setter.ResponseUpdated,
			func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, domain, ipNetwork).Return(map[string]netip.Addr{record1: ip2}, true, true),
					h.EXPECT().UpdateRecord(ctx, p, domain, ipNetwork, record1, ip1).Return(false),
					h.EXPECT().DeleteRecord(ctx, p, domain, ipNetwork, record1).Return(true),
					p.EXPECT().Noticef(pp.EmojiDeletion, "Deleted a stale %s record of %q (ID: %q)", "AAAA", "sub.test.org", record1), //nolint:lll
					h.EXPECT().CreateRecord(ctx, p, domain, ipNetwork, ip1, api.TTLAuto, false, "hello").Return(record2, true),
					p.EXPECT().Noticef(pp.EmojiCreation, "Added a new %s record of %q (ID: %q)", "AAAA", "sub.test.org", record2),
				)
			},
		},
		"1unmatched/update-timeout": {
			ip1,
			setter.ResponseFailed,
			func(ctx context.Context, cancel func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, domain, ipNetwork).
						Return(map[string]netip.Addr{record1: ip2}, true, true),
					h.EXPECT().UpdateRecord(ctx, p, domain, ipNetwork, record1, ip1).
						Do(wrapCancelAsUpdate(cancel)).Return(false),
					p.EXPECT().Infof(pp.EmojiBailingOut, "Operation aborted (%v); bailing out . . .", gomock.Any()),
				)
			},
		},
		"1unmatched/delete-timeout": {
			ip1,
			setter.ResponseFailed,
			func(ctx context.Context, cancel func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, domain, ipNetwork).
						Return(map[string]netip.Addr{record1: ip2}, true, true),
					h.EXPECT().UpdateRecord(ctx, p, domain, ipNetwork, record1, ip1).
						Return(false),
					h.EXPECT().DeleteRecord(ctx, p, domain, ipNetwork, record1).
						Do(wrapCancelAsDelete(cancel)).Return(false),
					p.EXPECT().Infof(pp.EmojiBailingOut, "Operation aborted (%v); bailing out . . .", gomock.Any()),
				)
			},
		},
		"1matched": {
			ip1,
			setter.ResponseNoop,
			func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, domain, ipNetwork).Return(map[string]netip.Addr{record1: ip1}, true, true),
					p.EXPECT().Infof(pp.EmojiAlreadyDone,
						"The %s records of %q are already up to date (cached)", "AAAA", "sub.test.org"),
				)
			},
		},
		"1matched/not-cached": {
			ip1,
			setter.ResponseNoop,
			func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, domain, ipNetwork).Return(map[string]netip.Addr{record1: ip1}, false, true),
					p.EXPECT().Infof(pp.EmojiAlreadyDone, "The %s records of %q are already up to date", "AAAA", "sub.test.org"),
				)
			},
		},
		"2matched": {
			ip1,
			setter.ResponseUpdated,
			func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, domain, ipNetwork).
						Return(map[string]netip.Addr{record1: ip1, record2: ip1}, true, true),
					h.EXPECT().DeleteRecord(ctx, p, domain, ipNetwork, record2).Return(true),
					p.EXPECT().Noticef(
						pp.EmojiDeletion,
						"Deleted a duplicate %s record of %q (ID: %q)",
						"AAAA",
						"sub.test.org",
						record2,
					),
				)
			},
		},
		"2matched/delete-fail": {
			ip1,
			setter.ResponseUpdated,
			func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, domain, ipNetwork).
						Return(map[string]netip.Addr{record1: ip1, record2: ip1}, true, true),
					h.EXPECT().DeleteRecord(ctx, p, domain, ipNetwork, record2).Return(false),
				)
			},
		},
		"2matched/delete-timeout": {
			ip1,
			setter.ResponseFailed,
			func(ctx context.Context, cancel func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, domain, ipNetwork).
						Return(map[string]netip.Addr{record1: ip1, record2: ip1}, true, true),
					h.EXPECT().DeleteRecord(ctx, p, domain, ipNetwork, record2).
						Do(wrapCancelAsDelete(cancel)).Return(false),
					p.EXPECT().Infof(pp.EmojiBailingOut, "Operation aborted (%v); bailing out . . .", gomock.Any()),
				)
			},
		},
		"2unmatched": {
			ip1,
			setter.ResponseUpdated,
			func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, domain, ipNetwork).
						Return(map[string]netip.Addr{record1: ip2, record2: ip2}, true, true),
					h.EXPECT().UpdateRecord(ctx, p, domain, ipNetwork, record1, ip1).Return(true),
					p.EXPECT().Noticef(
						pp.EmojiUpdate,
						"Updated a stale %s record of %q (ID: %q)",
						"AAAA",
						"sub.test.org",
						record1,
					),
					h.EXPECT().DeleteRecord(ctx, p, domain, ipNetwork, record2).Return(true),
					p.EXPECT().Noticef(pp.EmojiDeletion,
						"Deleted a stale %s record of %q (ID: %q)",
						"AAAA",
						"sub.test.org",
						record2),
				)
			},
		},
		"2unmatched/delete-timeout": {
			ip1,
			setter.ResponseFailed,
			func(ctx context.Context, cancel func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, domain, ipNetwork).
						Return(map[string]netip.Addr{record1: ip2, record2: ip2}, true, true),
					h.EXPECT().UpdateRecord(ctx, p, domain, ipNetwork, record1, ip1).Return(true),
					p.EXPECT().Noticef(
						pp.EmojiUpdate,
						"Updated a stale %s record of %q (ID: %q)",
						"AAAA",
						"sub.test.org",
						record1,
					),
					h.EXPECT().DeleteRecord(ctx, p, domain, ipNetwork, record2).
						Do(wrapCancelAsDelete(cancel)).Return(false),
					p.EXPECT().Infof(pp.EmojiBailingOut, "Operation aborted (%v); bailing out . . .", gomock.Any()),
				)
			},
		},
		"2unmatched/update-fail": {
			ip1,
			setter.ResponseUpdated,
			func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, domain, ipNetwork).
						Return(map[string]netip.Addr{record1: ip2, record2: ip2}, true, true),
					h.EXPECT().UpdateRecord(ctx, p, domain, ipNetwork, record1, ip1).Return(false),
					h.EXPECT().DeleteRecord(ctx, p, domain, ipNetwork, record1).Return(true),
					p.EXPECT().Noticef(
						pp.EmojiDeletion,
						"Deleted a stale %s record of %q (ID: %q)",
						"AAAA",
						"sub.test.org",
						record1),
					h.EXPECT().UpdateRecord(ctx, p, domain, ipNetwork, record2, ip1).Return(true),
					p.EXPECT().Noticef(
						pp.EmojiUpdate,
						"Updated a stale %s record of %q (ID: %q)",
						"AAAA",
						"sub.test.org",
						record2,
					),
				)
			},
		},
		"2unmatched/update-fail/update-fail": {
			ip1,
			setter.ResponseUpdated,
			func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, domain, ipNetwork).
						Return(map[string]netip.Addr{record1: ip2, record2: ip2}, true, true),
					h.EXPECT().UpdateRecord(ctx, p, domain, ipNetwork, record1, ip1).Return(false),
					h.EXPECT().DeleteRecord(ctx, p, domain, ipNetwork, record1).Return(true),
					p.EXPECT().Noticef(pp.EmojiDeletion, "Deleted a stale %s record of %q (ID: %q)", "AAAA", "sub.test.org", record1), //nolint:lll
					h.EXPECT().UpdateRecord(ctx, p, domain, ipNetwork, record2, ip1).Return(false),
					h.EXPECT().DeleteRecord(ctx, p, domain, ipNetwork, record2).Return(true),
					p.EXPECT().Noticef(pp.EmojiDeletion, "Deleted a stale %s record of %q (ID: %q)", "AAAA", "sub.test.org", record2), //nolint:lll
					h.EXPECT().CreateRecord(ctx, p, domain, ipNetwork, ip1, api.TTLAuto, false, "hello").Return(record3, true),
					p.EXPECT().Noticef(pp.EmojiCreation, "Added a new %s record of %q (ID: %q)", "AAAA", "sub.test.org", record3),
				)
			},
		},
		"2unmatched/update-fail/delete-fail/update-fail": {
			ip1,
			setter.ResponseFailed,
			func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, domain, ipNetwork).Return(map[string]netip.Addr{record1: ip2, record2: ip2}, true, true), //nolint:lll
					h.EXPECT().UpdateRecord(ctx, p, domain, ipNetwork, record1, ip1).Return(false),
					h.EXPECT().DeleteRecord(ctx, p, domain, ipNetwork, record1).Return(false),
					h.EXPECT().UpdateRecord(ctx, p, domain, ipNetwork, record2, ip1).Return(false),
					h.EXPECT().DeleteRecord(ctx, p, domain, ipNetwork, record2).Return(true),
					p.EXPECT().Noticef(pp.EmojiDeletion, "Deleted a stale %s record of %q (ID: %q)", "AAAA", "sub.test.org", record2), //nolint:lll
					h.EXPECT().CreateRecord(ctx, p, domain, ipNetwork, ip1, api.TTLAuto, false, "hello").Return(record3, true),
					p.EXPECT().Noticef(pp.EmojiCreation, "Added a new %s record of %q (ID: %q)", "AAAA", "sub.test.org", record3),                           //nolint:lll
					p.EXPECT().Warningf(pp.EmojiError, "Failed to finish updating %s records of %q; records might be inconsistent", "AAAA", "sub.test.org"), //nolint:lll
				)
			},
		},
		"2unmatched/update-fail/update-fail/create-fail": {
			ip1,
			setter.ResponseFailed,
			func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, domain, ipNetwork).Return(map[string]netip.Addr{record1: ip2, record2: ip2}, true, true), //nolint:lll
					h.EXPECT().UpdateRecord(ctx, p, domain, ipNetwork, record1, ip1).Return(false),
					h.EXPECT().DeleteRecord(ctx, p, domain, ipNetwork, record1).Return(true),
					p.EXPECT().Noticef(pp.EmojiDeletion, "Deleted a stale %s record of %q (ID: %q)", "AAAA", "sub.test.org", record1), //nolint:lll
					h.EXPECT().UpdateRecord(ctx, p, domain, ipNetwork, record2, ip1).Return(false),
					h.EXPECT().DeleteRecord(ctx, p, domain, ipNetwork, record2).Return(true),
					p.EXPECT().Noticef(pp.EmojiDeletion, "Deleted a stale %s record of %q (ID: %q)", "AAAA", "sub.test.org", record2), //nolint:lll
					h.EXPECT().CreateRecord(ctx, p, domain, ipNetwork, ip1, api.TTLAuto, false, "hello").Return(record3, false),
					p.EXPECT().Warningf(pp.EmojiError, "Failed to finish updating %s records of %q; records might be inconsistent", "AAAA", "sub.test.org"), //nolint:lll
				)
			},
		},
		"2unmatched/update-fail/update-fail/create-timeout": {
			ip1,
			setter.ResponseFailed,
			func(ctx context.Context, cancel func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, domain, ipNetwork).
						Return(map[string]netip.Addr{record1: ip2, record2: ip2}, true, true),
					h.EXPECT().UpdateRecord(ctx, p, domain, ipNetwork, record1, ip1).Return(false),
					h.EXPECT().DeleteRecord(ctx, p, domain, ipNetwork, record1).Return(true),
					p.EXPECT().Noticef(pp.EmojiDeletion, "Deleted a stale %s record of %q (ID: %q)", "AAAA", "sub.test.org", record1), //nolint:lll
					h.EXPECT().UpdateRecord(ctx, p, domain, ipNetwork, record2, ip1).Return(false),
					h.EXPECT().DeleteRecord(ctx, p, domain, ipNetwork, record2).Return(true),
					p.EXPECT().Noticef(pp.EmojiDeletion, "Deleted a stale %s record of %q (ID: %q)", "AAAA", "sub.test.org", record2), //nolint:lll
					h.EXPECT().CreateRecord(ctx, p, domain, ipNetwork, ip1, api.TTLAuto, false, "hello").
						Do(wrapCancelAsCreate(cancel)).Return(record3, false),
					p.EXPECT().Infof(pp.EmojiBailingOut, "Operation aborted (%v); bailing out . . .", gomock.Any()),
				)
			},
		},
		"listfail": {
			ip1,
			setter.ResponseFailed,
			func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				h.EXPECT().ListRecords(ctx, p, domain, ipNetwork).Return(nil, false, false)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			mockCtrl := gomock.NewController(t)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			mockPP := mocks.NewMockPP(mockCtrl)
			mockHandle := mocks.NewMockHandle(mockCtrl)
			if tc.prepareMocks != nil {
				tc.prepareMocks(ctx, cancel, mockPP, mockHandle)
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
		resp         setter.ResponseCode
		prepareMocks func(ctx context.Context, cancel func(), p *mocks.MockPP, m *mocks.MockHandle)
	}{
		"0": {
			setter.ResponseNoop,
			func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, domain, ipNetwork).Return(map[string]netip.Addr{}, true, true),
					p.EXPECT().Infof(pp.EmojiAlreadyDone, "The %s records of %q were already deleted (cached)", "AAAA", "sub.test.org"), //nolint:lll
				)
			},
		},
		"0/not-cached": {
			setter.ResponseNoop,
			func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, domain, ipNetwork).Return(map[string]netip.Addr{}, false, true),
					p.EXPECT().Infof(pp.EmojiAlreadyDone, "The %s records of %q were already deleted", "AAAA", "sub.test.org"),
				)
			},
		},
		"1unmatched": {
			setter.ResponseUpdated,
			func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, domain, ipNetwork).Return(map[string]netip.Addr{record1: ip1}, true, true),
					h.EXPECT().DeleteRecord(ctx, p, domain, ipNetwork, record1).Return(true),
					p.EXPECT().Noticef(pp.EmojiDeletion, "Deleted a stale %s record of %q (ID: %q)", "AAAA", "sub.test.org", record1), //nolint:lll
				)
			},
		},
		"1unmatched/delete-fail": {
			setter.ResponseFailed,
			func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, domain, ipNetwork).Return(map[string]netip.Addr{record1: ip1}, true, true),
					h.EXPECT().DeleteRecord(ctx, p, domain, ipNetwork, record1).Return(false),
					p.EXPECT().Warningf(pp.EmojiError, "Failed to finish deleting %s records of %q; records might be inconsistent", "AAAA", "sub.test.org"), //nolint:lll
				)
			},
		},
		"1unmatched/delete-timeout": {
			setter.ResponseFailed,
			func(ctx context.Context, cancel func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, domain, ipNetwork).
						Return(map[string]netip.Addr{record1: ip1}, true, true),
					h.EXPECT().DeleteRecord(ctx, p, domain, ipNetwork, record1).
						Do(wrapCancelAsDelete(cancel)).Return(false),
					p.EXPECT().Infof(pp.EmojiBailingOut, "Operation aborted (%v); bailing out . . .", gomock.Any()),
				)
			},
		},
		"impossible-records": {
			setter.ResponseUpdated,
			func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				gomock.InOrder(
					h.EXPECT().ListRecords(ctx, p, domain, ipNetwork).Return(map[string]netip.Addr{record1: ip1, record2: invalidIP}, true, true), //nolint:lll
					h.EXPECT().DeleteRecord(ctx, p, domain, ipNetwork, record1).Return(true),
					p.EXPECT().Noticef(pp.EmojiDeletion, "Deleted a stale %s record of %q (ID: %q)", "AAAA", "sub.test.org", record1), //nolint:lll
					h.EXPECT().DeleteRecord(ctx, p, domain, ipNetwork, record2).Return(true),
					p.EXPECT().Noticef(pp.EmojiDeletion, "Deleted a stale %s record of %q (ID: %q)", "AAAA", "sub.test.org", record2), //nolint:lll
				)
			},
		},
		"listfail": {
			setter.ResponseFailed,
			func(ctx context.Context, _ func(), p *mocks.MockPP, h *mocks.MockHandle) {
				h.EXPECT().ListRecords(ctx, p, domain, ipNetwork).Return(nil, false, false)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			mockCtrl := gomock.NewController(t)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			mockPP := mocks.NewMockPP(mockCtrl)
			mockHandle := mocks.NewMockHandle(mockCtrl)
			if tc.prepareMocks != nil {
				tc.prepareMocks(ctx, cancel, mockPP, mockHandle)
			}

			s, ok := setter.New(mockPP, mockHandle)
			require.True(t, ok)

			resp := s.Delete(ctx, mockPP, domain, ipNetwork)
			require.Equal(t, tc.resp, resp)
		})
	}
}
