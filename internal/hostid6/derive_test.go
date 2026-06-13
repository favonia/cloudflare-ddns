package hostid6_test

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/hostid6"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
)

func TestDerivePreserve(t *testing.T) {
	t.Parallel()

	raw := ipnet.RawEntryFrom(netip.MustParseAddr("2001:db8::abcd"), 64)
	target, problem := hostid6.Derive(raw, hostid6.Preserve())

	require.Nil(t, problem)
	require.Equal(t, netip.MustParseAddr("2001:db8::abcd"), target)
}

func TestDeriveLiteral(t *testing.T) {
	t.Parallel()

	raw := ipnet.RawEntryFrom(netip.MustParseAddr("2001:db8:1234:5678::abcd"), 48)
	literal := mustLiteral(t, "::1")
	target, problem := hostid6.Derive(raw, literal)

	require.Nil(t, problem)
	require.Equal(t, netip.MustParseAddr("2001:db8:1234::1"), target)
}

func TestDeriveLiteralIncompatibility(t *testing.T) {
	t.Parallel()

	for _, tc := range [...]struct {
		literal      string
		prefixLen    int
		maxPrefixLen int
	}{
		{"8000::", 1, 0},
		{"0:0:0:1::", 64, 63},
		{"::1", 128, 127},
	} {
		raw := ipnet.RawEntryFrom(netip.MustParseAddr("2001:db8::"), tc.prefixLen)
		derivation := mustLiteral(t, tc.literal)
		target, problem := hostid6.Derive(raw, derivation)

		require.Equal(t, netip.Addr{}, target)
		require.Equal(t, &hostid6.Incompatibility{
			Kind:           hostid6.LiteralIncompatibility,
			Derivation:     derivation,
			ObservedPrefix: raw,
			MaxPrefixLen:   tc.maxPrefixLen,
		}, problem)
	}
}

func TestDeriveModifiedEUI64(t *testing.T) {
	t.Parallel()

	derivation := hostid6.MAC([6]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55})
	for _, tc := range [...]struct {
		raw      ipnet.RawEntry
		expected netip.Addr
	}{
		{
			ipnet.RawEntryFrom(netip.MustParseAddr("ffff::"), 0),
			netip.MustParseAddr("::211:22ff:fe33:4455"),
		},
		{
			ipnet.RawEntryFrom(netip.MustParseAddr("2001:db8:1234:5678::abcd"), 48),
			netip.MustParseAddr("2001:db8:1234:0:211:22ff:fe33:4455"),
		},
		{
			ipnet.RawEntryFrom(netip.MustParseAddr("2001:db8:1234:5678::abcd"), 64),
			netip.MustParseAddr("2001:db8:1234:5678:211:22ff:fe33:4455"),
		},
	} {
		target, problem := hostid6.Derive(tc.raw, derivation)
		require.Nil(t, problem)
		require.Equal(t, tc.expected, target)
	}
}

func TestDeriveMACIncompatibility(t *testing.T) {
	t.Parallel()

	derivation := hostid6.MAC([6]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
	for _, prefixLen := range [...]int{65, 128} {
		raw := ipnet.RawEntryFrom(netip.MustParseAddr("::"), prefixLen)
		target, problem := hostid6.Derive(raw, derivation)

		require.Equal(t, netip.Addr{}, target)
		require.Equal(t, &hostid6.Incompatibility{
			Kind:           hostid6.MACIncompatibility,
			Derivation:     derivation,
			ObservedPrefix: raw,
			MaxPrefixLen:   64,
		}, problem)
	}
}
