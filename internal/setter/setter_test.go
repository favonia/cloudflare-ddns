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
					p.EXPECT().Noticef(pp.EmojiCreation, "Added a new %s record of %q (ID: %s)", "AAAA", "sub.test.org", record1),
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
						"Updated a stale %s record of %q (ID: %s)",
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
					p.EXPECT().Noticef(pp.EmojiDeletion, "Deleted a stale %s record of %q (ID: %s)", "AAAA", "sub.test.org", record1), //nolint:lll
					h.EXPECT().CreateRecord(ctx, p, domain, ipNetwork, ip1, api.TTLAuto, false, "hello").Return(record2, true),
					p.EXPECT().Noticef(pp.EmojiCreation, "Added a new %s record of %q (ID: %s)", "AAAA", "sub.test.org", record2),
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
						"Deleted a duplicate %s record of %q (ID: %s)",
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
						"Updated a stale %s record of %q (ID: %s)",
						"AAAA",
						"sub.test.org",
						record1,
					),
					h.EXPECT().DeleteRecord(ctx, p, domain, ipNetwork, record2).Return(true),
					p.EXPECT().Noticef(pp.EmojiDeletion,
						"Deleted a stale %s record of %q (ID: %s)",
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
						"Updated a stale %s record of %q (ID: %s)",
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
						"Deleted a stale %s record of %q (ID: %s)",
						"AAAA",
						"sub.test.org",
						record1),
					h.EXPECT().UpdateRecord(ctx, p, domain, ipNetwork, record2, ip1).Return(true),
					p.EXPECT().Noticef(
						pp.EmojiUpdate,
						"Updated a stale %s record of %q (ID: %s)",
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
					p.EXPECT().Noticef(pp.EmojiDeletion, "Deleted a stale %s record of %q (ID: %s)", "AAAA", "sub.test.org", record1), //nolint:lll
					h.EXPECT().UpdateRecord(ctx, p, domain, ipNetwork, record2, ip1).Return(false),
					h.EXPECT().DeleteRecord(ctx, p, domain, ipNetwork, record2).Return(true),
					p.EXPECT().Noticef(pp.EmojiDeletion, "Deleted a stale %s record of %q (ID: %s)", "AAAA", "sub.test.org", record2), //nolint:lll
					h.EXPECT().CreateRecord(ctx, p, domain, ipNetwork, ip1, api.TTLAuto, false, "hello").Return(record3, true),
					p.EXPECT().Noticef(pp.EmojiCreation, "Added a new %s record of %q (ID: %s)", "AAAA", "sub.test.org", record3),
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
					p.EXPECT().Noticef(pp.EmojiDeletion, "Deleted a stale %s record of %q (ID: %s)", "AAAA", "sub.test.org", record2), //nolint:lll
					h.EXPECT().CreateRecord(ctx, p, domain, ipNetwork, ip1, api.TTLAuto, false, "hello").Return(record3, true),
					p.EXPECT().Noticef(pp.EmojiCreation, "Added a new %s record of %q (ID: %s)", "AAAA", "sub.test.org", record3),                           //nolint:lll
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
					p.EXPECT().Noticef(pp.EmojiDeletion, "Deleted a stale %s record of %q (ID: %s)", "AAAA", "sub.test.org", record1), //nolint:lll
					h.EXPECT().UpdateRecord(ctx, p, domain, ipNetwork, record2, ip1).Return(false),
					h.EXPECT().DeleteRecord(ctx, p, domain, ipNetwork, record2).Return(true),
					p.EXPECT().Noticef(pp.EmojiDeletion, "Deleted a stale %s record of %q (ID: %s)", "AAAA", "sub.test.org", record2), //nolint:lll
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
					p.EXPECT().Noticef(pp.EmojiDeletion, "Deleted a stale %s record of %q (ID: %s)", "AAAA", "sub.test.org", record1), //nolint:lll
					h.EXPECT().UpdateRecord(ctx, p, domain, ipNetwork, record2, ip1).Return(false),
					h.EXPECT().DeleteRecord(ctx, p, domain, ipNetwork, record2).Return(true),
					p.EXPECT().Noticef(pp.EmojiDeletion, "Deleted a stale %s record of %q (ID: %s)", "AAAA", "sub.test.org", record2), //nolint:lll
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
					p.EXPECT().Noticef(pp.EmojiDeletion, "Deleted a stale %s record of %q (ID: %s)", "AAAA", "sub.test.org", record1), //nolint:lll
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
					p.EXPECT().Noticef(pp.EmojiDeletion, "Deleted a stale %s record of %q (ID: %s)", "AAAA", "sub.test.org", record1), //nolint:lll
					h.EXPECT().DeleteRecord(ctx, p, domain, ipNetwork, record2).Return(true),
					p.EXPECT().Noticef(pp.EmojiDeletion, "Deleted a stale %s record of %q (ID: %s)", "AAAA", "sub.test.org", record2), //nolint:lll
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

func TestSetWAFList(t *testing.T) {
	t.Parallel()

	const listName = "list"
	const listDescription = "My List"

	var (
		ip4           = netip.MustParseAddr("10.0.0.1")
		ip6           = netip.MustParseAddr("2001:db8::1111")
		prefix4       = api.WAFListItem{Prefix: netip.MustParsePrefix("10.0.0.1/32"), ID: "pre4"}
		prefix6       = api.WAFListItem{Prefix: netip.MustParsePrefix("2001:0db8::/64"), ID: "pre6"}
		prefix4range1 = api.WAFListItem{Prefix: netip.MustParsePrefix("10.0.0.0/16"), ID: "ip4-16"}
		prefix4range2 = api.WAFListItem{Prefix: netip.MustParsePrefix("10.0.0.0/20"), ID: "ip4-20"}
		prefix4range3 = api.WAFListItem{Prefix: netip.MustParsePrefix("10.0.0.0/24"), ID: "ip4-24"}
		prefix4wrong1 = api.WAFListItem{Prefix: netip.MustParsePrefix("20.0.0.0/16"), ID: "ip4-16"}
		prefix4wrong2 = api.WAFListItem{Prefix: netip.MustParsePrefix("20.0.0.0/20"), ID: "ip4-20"}
		prefix4wrong3 = api.WAFListItem{Prefix: netip.MustParsePrefix("20.0.0.0/24"), ID: "ip4-24"}
		prefix6range1 = api.WAFListItem{Prefix: netip.MustParsePrefix("2001:db8::/32"), ID: "ip6-32"}
		prefix6range2 = api.WAFListItem{Prefix: netip.MustParsePrefix("2001:db8::/40"), ID: "ip6-40"}
		prefix6range3 = api.WAFListItem{Prefix: netip.MustParsePrefix("2001:db8::/48"), ID: "ip6-48"}
		prefix6wrong1 = api.WAFListItem{Prefix: netip.MustParsePrefix("4001:db8::/32"), ID: "ip6-32"}
		prefix6wrong2 = api.WAFListItem{Prefix: netip.MustParsePrefix("4001:db8::/40"), ID: "ip6-40"}
		prefix6wrong3 = api.WAFListItem{Prefix: netip.MustParsePrefix("4001:db8::/48"), ID: "ip6-48"}
	)

	type items = []api.WAFListItem
	type ipmap = map[ipnet.Type]netip.Addr

	for name, tc := range map[string]struct {
		detected     ipmap
		resp         setter.ResponseCode
		prepareMocks func(ctx context.Context, cancel func(), p *mocks.MockPP, m *mocks.MockHandle)
	}{
		"ensurefail": {
			ipmap{ipnet.IP4: ip4, ipnet.IP6: ip6},
			setter.ResponseFailed,
			func(ctx context.Context, _ func(), p *mocks.MockPP, m *mocks.MockHandle) {
				m.EXPECT().EnsureWAFList(ctx, p, listName, listDescription).Return(false, false)
			},
		},
		"created": {
			ipmap{ipnet.IP4: ip4, ipnet.IP6: ip6},
			setter.ResponseUpdated,
			func(ctx context.Context, _ func(), p *mocks.MockPP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().EnsureWAFList(ctx, p, listName, listDescription).Return(false, true),
					p.EXPECT().Noticef(pp.EmojiCreation, "Created a new list named %q", listName),
					m.EXPECT().ListWAFListItems(ctx, p, listName).Return(items{}, false, true),
					m.EXPECT().CreateWAFListItems(ctx, p,
						listName, []netip.Prefix{prefix4.Prefix, prefix6.Prefix}, "").Return(true),
					p.EXPECT().Noticef(pp.EmojiCreation,
						"Added %s to the list %q", "10.0.0.1", listName),
					p.EXPECT().Noticef(pp.EmojiCreation,
						"Added %s to the list %q", "2001:db8::/64", listName),
					m.EXPECT().DeleteWAFListItems(ctx, p, listName, []string{}).Return(true),
				)
			},
		},
		"listfail": {
			ipmap{ipnet.IP4: ip4, ipnet.IP6: ip6},
			setter.ResponseFailed,
			func(ctx context.Context, _ func(), p *mocks.MockPP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().EnsureWAFList(ctx, p, listName, listDescription).Return(true, true),
					m.EXPECT().ListWAFListItems(ctx, p, listName).Return(nil, false, false),
				)
			},
		},
		"noop": {
			ipmap{ipnet.IP4: ip4, ipnet.IP6: ip6},
			setter.ResponseNoop,
			func(ctx context.Context, _ func(), p *mocks.MockPP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().EnsureWAFList(ctx, p, listName, listDescription).Return(true, true),
					m.EXPECT().ListWAFListItems(ctx, p, listName).Return(items{prefix4, prefix6}, false, true),
					p.EXPECT().Infof(pp.EmojiAlreadyDone, "The list %q is already up to date", listName),
				)
			},
		},
		"noop/cached": {
			ipmap{ipnet.IP4: ip4, ipnet.IP6: ip6},
			setter.ResponseNoop,
			func(ctx context.Context, _ func(), p *mocks.MockPP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().EnsureWAFList(ctx, p, listName, listDescription).Return(true, true),
					m.EXPECT().ListWAFListItems(ctx, p, listName).Return(items{prefix4, prefix6}, true, true),
					p.EXPECT().Infof(pp.EmojiAlreadyDone, "The list %q is already up to date (cached)", listName),
				)
			},
		},
		"test1": {
			ipmap{ipnet.IP4: ip4, ipnet.IP6: ip6},
			setter.ResponseUpdated,
			func(ctx context.Context, _ func(), p *mocks.MockPP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().EnsureWAFList(ctx, p, listName, listDescription).Return(true, true),
					m.EXPECT().ListWAFListItems(ctx, p, listName).
						Return(items{
							prefix6range1,
							prefix4wrong2,
							prefix6range2,
							prefix6range3,
							prefix4range2,
							prefix4range3,
							prefix6wrong2,
							prefix6wrong3,
							prefix4wrong3,
							prefix4range1,
							prefix4wrong1,
							prefix6wrong1,
						}, false, true),
					m.EXPECT().CreateWAFListItems(ctx, p, listName,
						nil, "").Return(true),
					m.EXPECT().DeleteWAFListItems(ctx, p, listName,
						gomock.InAnyOrder([]string{
							prefix4wrong2.ID,
							prefix6wrong2.ID,
							prefix6wrong3.ID,
							prefix4wrong3.ID,
							prefix4wrong1.ID,
							prefix6wrong1.ID,
						})).Return(true),
				)
				p.EXPECT().Noticef(pp.EmojiDeletion,
					"Deleted %s from the list %q", "20.0.0.0/20", listName)
				p.EXPECT().Noticef(pp.EmojiDeletion,
					"Deleted %s from the list %q", "4001:db8::/40", listName)
				p.EXPECT().Noticef(pp.EmojiDeletion,
					"Deleted %s from the list %q", "4001:db8::/48", listName)
				p.EXPECT().Noticef(pp.EmojiDeletion,
					"Deleted %s from the list %q", "20.0.0.0/24", listName)
				p.EXPECT().Noticef(pp.EmojiDeletion,
					"Deleted %s from the list %q", "20.0.0.0/16", listName)
				p.EXPECT().Noticef(pp.EmojiDeletion,
					"Deleted %s from the list %q", "4001:db8::/32", listName)
			},
		},
		"test2": {
			ipmap{ipnet.IP4: ip4, ipnet.IP6: ip6},
			setter.ResponseUpdated,
			func(ctx context.Context, _ func(), p *mocks.MockPP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().EnsureWAFList(ctx, p, listName, listDescription).Return(true, true),
					m.EXPECT().ListWAFListItems(ctx, p, listName).
						Return(items{
							prefix4wrong2,
							prefix6wrong2,
							prefix6wrong3,
							prefix4wrong3,
							prefix4wrong1,
							prefix6wrong1,
						}, false, true),
					m.EXPECT().CreateWAFListItems(ctx, p, listName,
						[]netip.Prefix{prefix4.Prefix, prefix6.Prefix}, "").Return(true),
					p.EXPECT().Noticef(pp.EmojiCreation,
						"Added %s to the list %q", "10.0.0.1", listName),
					p.EXPECT().Noticef(pp.EmojiCreation,
						"Added %s to the list %q", "2001:db8::/64", listName),
					m.EXPECT().DeleteWAFListItems(ctx, p, listName,
						gomock.InAnyOrder([]string{
							prefix4wrong2.ID,
							prefix6wrong2.ID,
							prefix6wrong3.ID,
							prefix4wrong3.ID,
							prefix4wrong1.ID,
							prefix6wrong1.ID,
						})).Return(true),
				)
				p.EXPECT().Noticef(pp.EmojiDeletion,
					"Deleted %s from the list %q", "20.0.0.0/20", listName)
				p.EXPECT().Noticef(pp.EmojiDeletion,
					"Deleted %s from the list %q", "4001:db8::/40", listName)
				p.EXPECT().Noticef(pp.EmojiDeletion,
					"Deleted %s from the list %q", "4001:db8::/48", listName)
				p.EXPECT().Noticef(pp.EmojiDeletion,
					"Deleted %s from the list %q", "20.0.0.0/24", listName)
				p.EXPECT().Noticef(pp.EmojiDeletion,
					"Deleted %s from the list %q", "20.0.0.0/16", listName)
				p.EXPECT().Noticef(pp.EmojiDeletion,
					"Deleted %s from the list %q", "4001:db8::/32", listName)
			},
		},
		"create-fail": {
			ipmap{ipnet.IP4: ip4, ipnet.IP6: ip6},
			setter.ResponseFailed,
			func(ctx context.Context, _ func(), p *mocks.MockPP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().EnsureWAFList(ctx, p, listName, listDescription).Return(true, true),
					m.EXPECT().ListWAFListItems(ctx, p, listName).Return(items{}, false, true),
					m.EXPECT().CreateWAFListItems(ctx, p, listName,
						[]netip.Prefix{prefix4.Prefix, prefix6.Prefix}, "").Return(false),
				)
			},
		},
		"delete-fail": {
			ipmap{ipnet.IP4: ip4, ipnet.IP6: ip6},
			setter.ResponseFailed,
			func(ctx context.Context, _ func(), p *mocks.MockPP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().EnsureWAFList(ctx, p, listName, listDescription).Return(true, true),
					m.EXPECT().ListWAFListItems(ctx, p, listName).
						Return(items{
							prefix6range1,
							prefix4wrong2,
							prefix6range2,
							prefix6range3,
							prefix4range2,
							prefix4range3,
							prefix6wrong2,
							prefix6wrong3,
							prefix4wrong3,
							prefix4range1,
							prefix4wrong1,
							prefix6wrong1,
						}, false, true),
					m.EXPECT().CreateWAFListItems(ctx, p, listName,
						nil, "").Return(true),
					m.EXPECT().DeleteWAFListItems(ctx, p, listName,
						gomock.InAnyOrder([]string{
							prefix4wrong2.ID,
							prefix6wrong2.ID,
							prefix6wrong3.ID,
							prefix4wrong3.ID,
							prefix4wrong1.ID,
							prefix6wrong1.ID,
						})).Return(false),
				)
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

			resp := s.SetWAFList(ctx, mockPP, listName, listDescription, tc.detected, "")
			require.Equal(t, tc.resp, resp)
		})
	}
}

func TestDeleteWAFList(t *testing.T) {
	t.Parallel()

	const listName = "list"

	for name, tc := range map[string]struct {
		resp         setter.ResponseCode
		prepareMocks func(ctx context.Context, cancel func(), p *mocks.MockPP, m *mocks.MockHandle)
	}{
		"fail": {
			setter.ResponseFailed,
			func(ctx context.Context, _ func(), p *mocks.MockPP, m *mocks.MockHandle) {
				m.EXPECT().DeleteWAFList(ctx, p, listName).Return(false)
			},
		},
		"deleted": {
			setter.ResponseUpdated,
			func(ctx context.Context, _ func(), p *mocks.MockPP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().DeleteWAFList(ctx, p, listName).Return(true),
					p.EXPECT().Noticef(pp.EmojiDeletion, "The list %q was deleted", listName),
				)
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

			resp := s.DeleteWAFList(ctx, mockPP, listName)
			require.Equal(t, tc.resp, resp)
		})
	}
}
