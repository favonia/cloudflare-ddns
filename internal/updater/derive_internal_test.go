package updater

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/hostid6"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/provider"
)

func mustHostID6Literal(t *testing.T, text string) hostid6.Derivation {
	t.Helper()

	derivation, err := hostid6.Literal(netip.MustParseAddr(text))
	require.NoError(t, err)
	return derivation
}

func rawIP6(text string, prefixLen int) ipnet.RawEntry {
	return ipnet.RawEntryFrom(netip.MustParseAddr(text), prefixLen)
}

func TestDeriveIP6DNSTargetsEmptyRawData(t *testing.T) {
	t.Parallel()

	alpha := domain.FQDN("alpha.example")
	beta := domain.FQDN("beta.example")
	targets, problems := deriveIP6DNSTargets(
		[]domain.Domain{alpha, beta},
		map[domain.Domain]hostid6.Set{
			alpha: hostid6.DefaultSet(),
			beta:  hostid6.NewSet(mustHostID6Literal(t, "::1")),
		},
		provider.NewKnownDetectionResult(nil),
	)

	require.Equal(t, dnsTargetsByDomain{
		alpha: {},
		beta:  {},
	}, targets)
	require.Nil(t, problems)
}

func TestDeriveIP6DNSTargetsCrossProduct(t *testing.T) {
	t.Parallel()

	alpha := domain.FQDN("alpha.example")
	beta := domain.FQDN("beta.example")
	literal := mustHostID6Literal(t, "::1")
	mac := hostid6.MAC([6]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55})

	targets, problems := deriveIP6DNSTargets(
		[]domain.Domain{beta, alpha},
		map[domain.Domain]hostid6.Set{
			alpha: hostid6.NewSet(hostid6.Preserve(), literal, mac),
			beta:  hostid6.NewSet(literal),
		},
		provider.NewKnownDetectionResult([]ipnet.RawEntry{
			rawIP6("2001:db8:1::abcd", 64),
			rawIP6("2001:db8:2::1", 64),
		}),
	)

	require.Nil(t, problems)
	require.Equal(t, dnsTargetsByDomain{
		alpha: {
			netip.MustParseAddr("2001:db8:1::1"),
			netip.MustParseAddr("2001:db8:1::abcd"),
			netip.MustParseAddr("2001:db8:1:0:211:22ff:fe33:4455"),
			netip.MustParseAddr("2001:db8:2::1"),
			netip.MustParseAddr("2001:db8:2:0:211:22ff:fe33:4455"),
		},
		beta: {
			netip.MustParseAddr("2001:db8:1::1"),
			netip.MustParseAddr("2001:db8:2::1"),
		},
	}, targets)
}

func TestDeriveIP6DNSTargetsGroupsAllProblems(t *testing.T) {
	t.Parallel()

	alpha := domain.FQDN("alpha.example")
	beta := domain.FQDN("beta.example")
	literal126a := mustHostID6Literal(t, "::2")
	literal126b := mustHostID6Literal(t, "::3")
	literal127 := mustHostID6Literal(t, "::1")
	macA := hostid6.MAC([6]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55})
	macB := hostid6.MAC([6]byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff})
	rawA := rawIP6("2001:db8::1", 128)
	rawB := rawIP6("2001:db8::2", 128)

	targets, problems := deriveIP6DNSTargets(
		[]domain.Domain{beta, alpha},
		map[domain.Domain]hostid6.Set{
			alpha: hostid6.NewSet(hostid6.Preserve(), literal126b, literal127, macA),
			beta:  hostid6.NewSet(literal126a, literal126b, macB),
		},
		provider.NewKnownDetectionResult([]ipnet.RawEntry{rawB, rawA}),
	)

	require.Nil(t, targets)
	require.Equal(t, []hostID6ProblemGroup{
		{
			Kind:           hostid6.LiteralIncompatibility,
			PrefixLenBound: 126,
			Domains:        []domain.Domain{alpha, beta},
			Derivations:    hostid6.NewSet(literal126a, literal126b),
			Observed:       []ipnet.RawEntry{rawA, rawB},
		},
		{
			Kind:           hostid6.LiteralIncompatibility,
			PrefixLenBound: 127,
			Domains:        []domain.Domain{alpha},
			Derivations:    hostid6.NewSet(literal127),
			Observed:       []ipnet.RawEntry{rawA, rawB},
		},
		{
			Kind:           hostid6.MACPrefixTooLong,
			PrefixLenBound: 64,
			Domains:        []domain.Domain{alpha, beta},
			Derivations:    hostid6.NewSet(macA, macB),
			Observed:       []ipnet.RawEntry{rawA, rawB},
		},
	}, problems)
}

func TestDeriveIP6DNSTargetsRejectsBrokenInternalInputs(t *testing.T) {
	t.Parallel()

	alpha := domain.FQDN("alpha.example")

	require.Panics(t, func() {
		deriveIP6DNSTargets(
			[]domain.Domain{alpha},
			map[domain.Domain]hostid6.Set{alpha: hostid6.DefaultSet()},
			provider.NewUnavailableDetectionResult(),
		)
	})
	require.Panics(t, func() {
		deriveIP6DNSTargets(
			[]domain.Domain{alpha},
			map[domain.Domain]hostid6.Set{},
			provider.NewKnownDetectionResult(nil),
		)
	})
}
