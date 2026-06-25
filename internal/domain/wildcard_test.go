package domain_test

import (
	"testing"
	"testing/quick"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/domain"
)

func TestWildcardString(t *testing.T) {
	t.Parallel()

	require.NoError(t, quick.Check(
		func(s string) bool {
			if s == "" {
				return domain.Wildcard(s).DNSNameASCII() == "*"
			}
			return domain.Wildcard(s).DNSNameASCII() == "*."+s
		},
		nil,
	))
}

func TestWildcardDescribe(t *testing.T) {
	t.Parallel()
	for _, tc := range [...]struct {
		input    string
		expected string
	}{
		// The following examples were adapted from https://unicode.org/cldr/utility/idna.jsp
		{"fass.de", "*.fass.de"},
		{"xn--fa-hia.de", "*.faß.de"},
		{"xn--f-qfao.de", "*.fäß.de"},
		{"xn--fa-hia.de", "*.faß.de"},
		{"xn--yzg.com", "*.₹.com"},
		{"xn--n00d.com", "*.𑀓.com"},
		{"xn--a.com", "*.xn--a.com"},
		{"xn--a.com", "*.xn--a.com"},
		{"xn--ab-j1t", "*.xn--ab-j1t"},
		{"xn--ab-j1t", "*.xn--ab-j1t"},
		{"xn--bb-eka.at", "*.öbb.at"},
		{"xn--og-09a.de", "*.ȡog.de"},
		{"xn--53h.de", "*.☕.de"},
		{"xn--iny-zx5a.de", "*.i♥ny.de"},
		{"xn--abc-rs4b422ycvb.co.jp", "*.abc・日本.co.jp"},
		{"xn--wgv71a.co.jp", "*.日本.co.jp"},
		{"xn--co-wuw5954azlb.jp", "*.xn--co-wuw5954azlb.jp"},
		{"xn--x-xbb7i.de", "*.x̧́.de"},
		{"xn--wxaijb9b.gr", "*.σόλος.gr"},
		{
			"xn--wxaikc6b.xn--gr-gtd9a1b0g.de",
			"*.xn--wxaikc6b.xn--gr-gtd9a1b0g.de",
		},
		{"xn--ngbrx4e.de", "*.عربي.de"},
		{"xn--mgba3gch31f.de", "*.نامهای.de"},
		{"xn--mgba3gch31f060k.de", "*.نامه\u200cای.de"},
		// some other test cases
		{"", "*"},
		{"xn--a.xn--a.xn--a.com", "*.xn--a.xn--a.xn--a.com"},
		{"a.com....", "*.a.com...."},
		{"a.com", "*.a.com"},
	} {
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.expected, domain.Wildcard(tc.input).Describe())
		})
	}
}

func TestWildcardStringDescribe(t *testing.T) {
	t.Parallel()
	require.Equal(t, "*", domain.Wildcard("").String())
	require.Equal(t, "*", domain.Wildcard("").Describe())
	require.Equal(t, "*.example.org", domain.Wildcard("example.org").String())
	require.Equal(t, "*.example.org", domain.Wildcard("example.org").Describe())
}

func TestWildcardHasStrictSuffix(t *testing.T) {
	t.Parallel()
	for _, tc := range [...]struct {
		input    domain.Wildcard
		suffix   domain.Suffix
		expected bool
	}{
		{"example.org", "example.org", true}, // *.example.org is strictly under example.org
		{"example.org", "org", true},         // and under org
		{"example.org", "", true},            // and under the root
		{"", "org", false},                   // bare * is not under org
		{"", "", true},                       // bare * is under the root
	} {
		t.Run(string(tc.input)+"/"+string(tc.suffix), func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.expected, tc.input.HasStrictSuffix(tc.suffix))
		})
	}
}

func TestWildcardZones(t *testing.T) {
	t.Parallel()
	type r = domain.Suffix
	for _, tc := range [...]struct {
		input    string
		expected []r
	}{
		{"a.b.c", []r{"a.b.c", "b.c", "c", ""}},
		{"...", []r{"...", "..", ".", ""}},
		{"aaa...", []r{"aaa...", "..", ".", ""}},
		{".aaa..", []r{".aaa..", "aaa..", ".", ""}},
		{"..aaa.", []r{"..aaa.", ".aaa.", "aaa.", ""}},
		{"...aaa", []r{"...aaa", "..aaa", ".aaa", "aaa", ""}},
	} {
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			i := 0
			for zone := range domain.Wildcard(tc.input).Zones {
				require.Equal(t, tc.expected[i], zone)
				i++
			}
		})
	}
}
