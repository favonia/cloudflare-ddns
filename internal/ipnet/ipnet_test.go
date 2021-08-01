package ipnet_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/favonia/cloudflare-ddns-go/internal/ipnet"
)

func TestString(t *testing.T) {
	t.Parallel()

	for _, c := range [...]struct {
		input  ipnet.Type
		output string
	}{
		{ipnet.IP4, "IPv4"},
		{ipnet.IP6, "IPv6"},
		{ipnet.Type(100), "(unrecognized IP network)"},
	} {
		assert.Equal(t, c.output, c.input.String())
	}
}

func TestRecordType(t *testing.T) {
	t.Parallel()

	for _, c := range [...]struct {
		input  ipnet.Type
		output string
	}{
		{ipnet.IP4, "A"},
		{ipnet.IP6, "AAAA"},
		{ipnet.Type(100), ""},
	} {
		assert.Equal(t, c.output, c.input.RecordType())
	}
}

func TestInt(t *testing.T) {
	t.Parallel()

	for _, c := range [...]struct {
		input  ipnet.Type
		output int
	}{
		{ipnet.IP4, 4},
		{ipnet.IP6, 6},
		{ipnet.Type(100), 0},
	} {
		assert.Equal(t, c.output, c.input.Int())
	}
}
