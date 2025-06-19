package config_test

import (
	"io"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/favonia/cloudflare-ddns/internal/config"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

//nolint:paralleltest // environment vars are global
func TestReadStaticIPs(t *testing.T) {
	for name, tc := range map[string]struct {
		input    string
		set      bool
		expected []string
	}{
		"empty":      {"", false, nil},
		"empty-set":  {"", true, nil},
		"single":     {"1.2.3.4", true, []string{"1.2.3.4"}},
		"multiple":   {"1.2.3.4,5.6.7.8", true, []string{"1.2.3.4", "5.6.7.8"}},
		"whitespace": {" 1.2.3.4 , 5.6.7.8 ", true, []string{"1.2.3.4", "5.6.7.8"}},
		"trailing":   {"1.2.3.4,", true, []string{"1.2.3.4"}},
	} {
		t.Run(name, func(t *testing.T) {
			set(t, "TEST_STATIC", tc.set, tc.input)
			var field []string
			ppfmt := pp.NewDefault(io.Discard)
			ok := config.ReadStaticIPs(ppfmt, "TEST_STATIC", &field)
			assert.True(t, ok)
			assert.Equal(t, tc.expected, field)
		})
	}
}

//nolint:paralleltest // environment vars are global  
func TestReadStaticIPMap(t *testing.T) {
	for name, tc := range map[string]struct {
		IP4_STATIC string
		IP4_SET    bool
		IP6_STATIC string
		IP6_SET    bool
		expected   map[ipnet.Type][]string
	}{
		"empty": {
			IP4_STATIC: "",
			IP4_SET:    false,
			IP6_STATIC: "",
			IP6_SET:    false,
			expected:   map[ipnet.Type][]string{ipnet.IP4: nil, ipnet.IP6: nil},
		},
		"ipv4-only": {
			IP4_STATIC: "1.2.3.4,5.6.7.8",
			IP4_SET:    true,
			IP6_STATIC: "",
			IP6_SET:    false,
			expected:   map[ipnet.Type][]string{ipnet.IP4: {"1.2.3.4", "5.6.7.8"}, ipnet.IP6: nil},
		},
		"ipv6-only": {
			IP4_STATIC: "",
			IP4_SET:    false,
			IP6_STATIC: "2001:db8::1,2001:db8::2",
			IP6_SET:    true,
			expected:   map[ipnet.Type][]string{ipnet.IP4: nil, ipnet.IP6: {"2001:db8::1", "2001:db8::2"}},
		},
		"both": {
			IP4_STATIC: "1.2.3.4",
			IP4_SET:    true,
			IP6_STATIC: "2001:db8::1",
			IP6_SET:    true,
			expected:   map[ipnet.Type][]string{ipnet.IP4: {"1.2.3.4"}, ipnet.IP6: {"2001:db8::1"}},
		},
	} {
		t.Run(name, func(t *testing.T) {
			set(t, "IP4_STATIC", tc.IP4_SET, tc.IP4_STATIC)
			set(t, "IP6_STATIC", tc.IP6_SET, tc.IP6_STATIC)
			field := map[ipnet.Type][]string{}
			ppfmt := pp.NewDefault(io.Discard)
			ok := config.ReadStaticIPMap(ppfmt, &field)
			assert.True(t, ok)
			assert.Equal(t, tc.expected, field)
		})
	}
}
