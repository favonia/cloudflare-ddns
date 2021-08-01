package ipnet_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/favonia/cloudflare-ddns-go/internal/ipnet"
)

func TestString(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		input  ipnet.Type
		output string
	}{
		"4":   {ipnet.IP4, "IPv4"},
		"6":   {ipnet.IP6, "IPv6"},
		"100": {ipnet.Type(100), "(unrecognized IP network)"},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.output, tc.input.String())
		})
	}
}

func TestRecordType(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		input  ipnet.Type
		output string
	}{
		"4":   {ipnet.IP4, "A"},
		"6":   {ipnet.IP6, "AAAA"},
		"100": {ipnet.Type(100), ""},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.output, tc.input.RecordType())
		})
	}
}

func TestInt(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		input  ipnet.Type
		output int
	}{
		"4":   {ipnet.IP4, 4},
		"6":   {ipnet.IP6, 6},
		"100": {ipnet.Type(100), 0},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.output, tc.input.Int())
		})
	}
}
