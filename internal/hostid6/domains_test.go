package hostid6_test

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/hostid6"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
)

func TestDeriveDomainsRejectsEmptyPolicy(t *testing.T) {
	t.Parallel()

	d := domain.FQDN("a.example.com")
	require.PanicsWithValue(t,
		"hostid6.DeriveDomains received an empty host-ID policy; this should not happen; please report it",
		func() {
			hostid6.DeriveDomains(
				[]domain.Domain{d},
				map[domain.Domain]hostid6.Set{}, // no policy ⇒ zero Set
				[]ipnet.RawEntry{ipnet.RawEntryFrom(netip.MustParseAddr("2001:db8::"), 64)},
			)
		},
	)
}

func TestDeriveDomainsTargets(t *testing.T) {
	t.Parallel()

	d1 := domain.FQDN("a.example.com")
	d2 := domain.FQDN("b.example.com")

	// Raw entries are intentionally out of address order to exercise sorting.
	rawEntries := []ipnet.RawEntry{
		ipnet.RawEntryFrom(netip.MustParseAddr("2001:db8:1::5"), 64),
		ipnet.RawEntryFrom(netip.MustParseAddr("2001:db8:0::5"), 64),
	}

	targets, problems := hostid6.DeriveDomains(
		[]domain.Domain{d1, d2},
		map[domain.Domain]hostid6.Set{
			// One literal per entry, distinct from d2 to show domains are independent.
			d1: hostid6.NewSet(mustLiteral(t, "::1")),
			// Preserve and "::5" collide on every entry to exercise de-duplication.
			d2: hostid6.NewSet(hostid6.Preserve(), mustLiteral(t, "::5")),
		},
		rawEntries,
	)

	require.Nil(t, problems)
	require.Equal(t, hostid6.TargetsByDomain{
		d1: {
			netip.MustParseAddr("2001:db8:0::1"),
			netip.MustParseAddr("2001:db8:1::1"),
		},
		d2: {
			netip.MustParseAddr("2001:db8:0::5"),
			netip.MustParseAddr("2001:db8:1::5"),
		},
	}, targets)
}

func TestDeriveDomainsProblems(t *testing.T) {
	t.Parallel()

	d1 := domain.FQDN("a.example.com")
	d2 := domain.FQDN("b.example.com")

	mac := hostid6.MAC([6]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55})
	literal := mustLiteral(t, "::1")     // max prefix length 127
	literal0 := mustLiteral(t, "8000::") // max prefix length 0

	raw48 := ipnet.RawEntryFrom(netip.MustParseAddr("2001:db8::"), 48)       // MAC: too short
	raw65 := ipnet.RawEntryFrom(netip.MustParseAddr("2001:db8::"), 65)       // MAC: too long
	raw128 := ipnet.RawEntryFrom(netip.MustParseAddr("2001:db8::abcd"), 128) // MAC: too long; literal: incompatible

	targets, problems := hostid6.DeriveDomains(
		[]domain.Domain{d1, d2},
		map[domain.Domain]hostid6.Set{
			d1: hostid6.NewSet(mac),
			// literal0 fails on every entry (max 0); literal fails only on the /128.
			// The two literal groups share a Kind but differ in PrefixLenBound, so the
			// problem sort must fall through to the PrefixLenBound tiebreaker.
			d2: hostid6.NewSet(mac, literal, literal0),
		},
		[]ipnet.RawEntry{raw48, raw65, raw128},
	)

	// Any incompatibility discards every target for the whole update.
	require.Nil(t, targets)

	require.Equal(t, []hostid6.ProblemGroup{
		{
			Kind:           hostid6.LiteralPrefixTooLong,
			PrefixLenBound: 0,
			Domains:        []domain.Domain{d2},
			Derivations:    hostid6.NewSet(literal0),
			Observed:       []ipnet.RawEntry{raw48, raw65, raw128},
		},
		{
			Kind:           hostid6.LiteralPrefixTooLong,
			PrefixLenBound: 127,
			Domains:        []domain.Domain{d2},
			Derivations:    hostid6.NewSet(literal),
			Observed:       []ipnet.RawEntry{raw128},
		},
		{
			Kind:           hostid6.MACPrefixTooLong,
			PrefixLenBound: 64,
			Domains:        []domain.Domain{d1, d2},
			Derivations:    hostid6.NewSet(mac),
			Observed:       []ipnet.RawEntry{raw65, raw128},
		},
		{
			Kind:           hostid6.MACPrefixTooShort,
			PrefixLenBound: 64,
			Domains:        []domain.Domain{d1, d2},
			Derivations:    hostid6.NewSet(mac),
			Observed:       []ipnet.RawEntry{raw48},
		},
	}, problems)
}
