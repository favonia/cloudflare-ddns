package hostid6_test

import (
	"net/netip"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/hostid6"
)

func mustLiteral(t *testing.T, text string) hostid6.Derivation {
	t.Helper()

	derivation, err := hostid6.Literal(netip.MustParseAddr(text))
	require.NoError(t, err)
	return derivation
}

func TestIntentionalIdentity(t *testing.T) {
	t.Parallel()

	literal := mustLiteral(t, "::211:22ff:fe33:4455")
	mac := hostid6.MAC([6]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55})

	require.NotEqual(t, literal, mac)
	require.False(t, hostid6.EqualSet(hostid6.NewSet(literal), hostid6.NewSet(mac)))
}

func TestLiteralValidation(t *testing.T) {
	t.Parallel()

	for _, addr := range [...]netip.Addr{
		{},
		netip.MustParseAddr("192.0.2.1"),
		netip.MustParseAddr("fe80::1%eth0"),
	} {
		_, err := hostid6.Literal(addr)
		require.Error(t, err)
	}
}

func TestCanonicalSet(t *testing.T) {
	t.Parallel()

	one := mustLiteral(t, "::1")
	two := mustLiteral(t, "::2")
	mac := hostid6.MAC([6]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55})

	actual := hostid6.NewSet(two, mac, one, hostid6.Preserve(), one)
	expected := hostid6.NewSet(hostid6.Preserve(), one, two, mac)

	require.True(t, hostid6.EqualSet(expected, actual))
	require.Equal(t, []hostid6.Derivation{hostid6.Preserve(), one, two, mac}, slices.Collect(actual.All()))
	require.True(t, hostid6.EqualSet(
		hostid6.NewSet(hostid6.Preserve(), mustLiteral(t, "::1"), mustLiteral(t, "0:0::1")),
		hostid6.NewSet(mustLiteral(t, "::1"), hostid6.Preserve()),
	))
	require.Equal(t, hostid6.NewSet(hostid6.Preserve()), hostid6.DefaultSet())
}

func TestNewSetRejectsEmptySet(t *testing.T) {
	t.Parallel()

	require.Panics(t, func() { hostid6.NewSet() })
}

func TestZeroSetRepresentsAbsence(t *testing.T) {
	t.Parallel()

	var zero hostid6.Set

	require.True(t, zero.IsZero())
	require.Zero(t, zero.Len())
	require.Empty(t, zero.Values())
	require.Empty(t, slices.Collect(zero.All()))
	require.True(t, hostid6.EqualSet(zero, hostid6.Set{}))
	require.False(t, hostid6.EqualSet(zero, hostid6.DefaultSet()))
	require.False(t, hostid6.DefaultSet().IsZero())
}

func TestSetDoesNotShareInputStorage(t *testing.T) {
	t.Parallel()

	one := mustLiteral(t, "::1")
	input := []hostid6.Derivation{one, hostid6.Preserve()}
	set := hostid6.NewSet(input...)

	input[0] = hostid6.MAC([6]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff})
	input[1] = one

	require.Equal(t, []hostid6.Derivation{hostid6.Preserve(), one}, set.Values())
}

func TestSetDoesNotExposeMutableStorage(t *testing.T) {
	t.Parallel()

	one := mustLiteral(t, "::1")
	set := hostid6.NewSet(hostid6.Preserve(), one)
	values := set.Values()

	values[0] = one
	values[1] = hostid6.Preserve()

	require.Equal(t, []hostid6.Derivation{hostid6.Preserve(), one}, set.Values())
	require.Equal(t, []string{"preserve", "::1"}, slices.Collect(func(yield func(string) bool) {
		for derivation := range set.All() {
			if !yield(derivation.Describe()) {
				return
			}
		}
	}))
}

func TestDescribe(t *testing.T) {
	t.Parallel()

	require.Equal(t, "preserve", hostid6.Preserve().Describe())
	require.Equal(t, "::1", mustLiteral(t, "0:0::1").Describe())
	require.Equal(t, "mac(00-11-22-33-44-55)", hostid6.MAC([6]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}).Describe())
}

func TestParseMACAcceptedForms(t *testing.T) {
	t.Parallel()

	expected := [6]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}
	for _, text := range [...]string{
		"00-11-22-33-44-55",
		"00:11:22:33:44:55",
		"00-11-22-33-44-AA",
		"00:11:22:33:44:AA",
	} {
		actual, err := hostid6.ParseMAC(text)
		require.NoError(t, err)
		if text[len(text)-2:] == "AA" {
			require.Equal(t, [6]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0xaa}, actual)
			require.Equal(t, "mac(00-11-22-33-44-aa)", hostid6.MAC(actual).Describe())
		} else {
			require.Equal(t, expected, actual)
			require.Equal(t, "mac(00-11-22-33-44-55)", hostid6.MAC(actual).Describe())
		}
	}

	ordered, err := hostid6.ParseMAC("01-23-45-67-89-ab")
	require.NoError(t, err)
	require.Equal(t, [6]byte{0x01, 0x23, 0x45, 0x67, 0x89, 0xab}, ordered)
}

func TestParseMACRejectedForms(t *testing.T) {
	t.Parallel()

	for _, text := range [...]string{
		"",
		"0011.2233.4455",
		"00-11-22-33-44",
		"00-11-22-33-44-555",
		"00-11-22-33-44-gg",
		"00-11-22-33-44-55-66-77",
		"00-11:22-33-44-55",
		"0-11-22-33-44-55",
	} {
		_, err := hostid6.ParseMAC(text)
		require.Error(t, err, text)
	}
}
