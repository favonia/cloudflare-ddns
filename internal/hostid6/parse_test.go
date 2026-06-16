package hostid6_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/hostid6"
)

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
			require.Equal(t, "mac(00-11-22-33-44-aa)", hostid6.MAC(actual).String())
		} else {
			require.Equal(t, expected, actual)
			require.Equal(t, "mac(00-11-22-33-44-55)", hostid6.MAC(actual).String())
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
		"00.11.22.33.44.55",
	} {
		_, err := hostid6.ParseMAC(text)
		require.Error(t, err, text)
	}
}
