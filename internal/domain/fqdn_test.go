package domain_test

import (
	"testing"
	"testing/quick"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/domain"
)

func TestFQDNString(t *testing.T) {
	t.Parallel()

	require.NoError(t, quick.Check(
		func(s string) bool {
			return domain.FQDN(s).DNSNameASCII() == s
		},
		nil,
	))
}

func TestFQDNDescribe(t *testing.T) {
	t.Parallel()
	for _, tc := range [...]struct {
		input    string
		expected string
	}{
		// The following examples were adapted from https://unicode.org/cldr/utility/idna.jsp
		{"fass.de", "fass.de"},
		{"xn--fa-hia.de", "faß.de"},
		{"xn--f-qfao.de", "fäß.de"},
		{"xn--fa-hia.de", "faß.de"},
		{"xn--yzg.com", "₹.com"},
		{"xn--n00d.com", "𑀓.com"},
		{"xn--a.com", "xn--a.com"},
		{"xn--a.com", "xn--a.com"},
		{"xn--ab-j1t", "xn--ab-j1t"},
		{"xn--ab-j1t", "xn--ab-j1t"},
		{"xn--bb-eka.at", "öbb.at"},
		{"xn--og-09a.de", "ȡog.de"},
		{"xn--53h.de", "☕.de"},
		{"xn--iny-zx5a.de", "i♥ny.de"},
		{"xn--abc-rs4b422ycvb.co.jp", "abc・日本.co.jp"},
		{"xn--wgv71a.co.jp", "日本.co.jp"},
		{"xn--co-wuw5954azlb.jp", "xn--co-wuw5954azlb.jp"},
		{"xn--x-xbb7i.de", "x̧́.de"},
		{"xn--wxaijb9b.gr", "σόλος.gr"},
		{
			"xn--wxaikc6b.xn--gr-gtd9a1b0g.de",
			"xn--wxaikc6b.xn--gr-gtd9a1b0g.de",
		},
		{"xn--ngbrx4e.de", "عربي.de"},
		{"xn--mgba3gch31f.de", "نامهای.de"},
		{"xn--mgba3gch31f060k.de", "نامه\u200cای.de"},
		// some other test cases
		{"xn--a.xn--a.xn--a.com", "xn--a.xn--a.xn--a.com"},
		{"a.com....", "a.com...."},
		{"a.com", "a.com"},
	} {
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.expected, domain.FQDN(tc.input).Describe())
		})
	}
}

//nolint:dupl
func TestFQDNSplitter(t *testing.T) {
	t.Parallel()
	type r = string
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
			var rs []r
			for s := domain.FQDN(tc.input).Split(); s.IsValid(); s = s.Next() {
				rs = append(rs, s.ZoneNameASCII())
			}
			require.Equal(t, tc.expected, rs)
		})
	}
}
