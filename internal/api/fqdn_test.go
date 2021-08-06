package api_test

import (
	"sort"
	"testing"
	"testing/quick"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/api"
)

func TestFQDNToASCII(t *testing.T) {
	t.Parallel()

	require.NoError(t, quick.Check(
		func(s string) bool {
			return api.FQDN(s).ToASCII() == s
		},
		nil,
	))
}

func TestFQDNDescribe(t *testing.T) {
	t.Parallel()
	for _, tc := range [...]struct {
		input     string
		expected  string
		ok        bool
		errString string
	}{
		// The following examples were adapted from https://unicode.org/cldr/utility/idna.jsp
		{"fass.de", "fass.de", true, ""},
		{"xn--fa-hia.de", "faß.de", true, ""},
		{"xn--f-qfao.de", "fäß.de", true, ""},
		{"xn--fa-hia.de", "faß.de", true, ""},
		{"xn--yzg.com", "₹.com", true, ""},
		{"xn--n00d.com", "𑀓.com", true, ""},
		{"xn--a.com", "xn--a.com", false, "idna: disallowed rune U+0080"},
		{"xn--a.com", "xn--a.com", false, "idna: invalid label \"\\u0080\""},
		{"xn--ab-j1t", "xn--ab-j1t", false, "idna: invalid label \"a\\u200cb\""},
		{"xn--ab-j1t", "xn--ab-j1t", false, "idna: invalid label \"a\\u200cb\""},
		{"xn--bb-eka.at", "öbb.at", true, ""},
		{"xn--og-09a.de", "ȡog.de", true, ""},
		{"xn--53h.de", "☕.de", true, ""},
		{"xn--iny-zx5a.de", "i♥ny.de", true, ""},
		{"xn--abc-rs4b422ycvb.co.jp", "abc・日本.co.jp", true, ""},
		{"xn--wgv71a.co.jp", "日本.co.jp", true, ""},
		{"xn--co-wuw5954azlb.jp", "xn--co-wuw5954azlb.jp", false, "idna: disallowed rune U+2488"},
		{"xn--x-xbb7i.de", "x̧́.de", true, ""},
		{"xn--wxaijb9b.gr", "σόλος.gr", true, ""},
		{
			"xn--wxaikc6b.xn--gr-gtd9a1b0g.de",
			"xn--wxaikc6b.xn--gr-gtd9a1b0g.de",
			false, "idna: invalid label \"σόλοσ.grعربي.de\"",
		},
		{"xn--ngbrx4e.de", "عربي.de", true, ""},
		{"xn--mgba3gch31f.de", "نامهای.de", true, ""},
		{"xn--mgba3gch31f060k.de", "نامه\u200cای.de", true, ""},
		// some other test cases
		{"xn--a.xn--a.xn--a.com", "xn--a.xn--a.xn--a.com", false, "idna: invalid label \"\\u0080\""},
		{"a.com....", "a.com....", true, ""},
		{"a.com", "a.com", true, ""},
	} {
		tc := tc
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.expected, api.FQDN(tc.input).Describe())
		})
	}
}

func TestNewFQDN(t *testing.T) {
	t.Parallel()
	for _, tc := range [...]struct {
		input     string
		expected  api.FQDN
		ok        bool
		errString string
	}{
		// The following examples were adapted from https://unicode.org/cldr/utility/idna.jsp
		{"fass.de", "fass.de", true, ""},
		{"faß.de", "xn--fa-hia.de", true, ""},
		{"fäß.de", "xn--f-qfao.de", true, ""},
		{"xn--fa-hia.de", "xn--fa-hia.de", true, ""},
		{"₹.com", "xn--yzg.com", true, ""},
		{"𑀓.com", "xn--n00d.com", true, ""},
		{"\u0080.com", "xn--a.com", false, "idna: disallowed rune U+0080"},
		{"xn--a.com", "xn--a.com", false, "idna: invalid label \"\\u0080\""},
		{"a\u200Cb", "xn--ab-j1t", false, "idna: invalid label \"a\\u200cb\""},
		{"xn--ab-j1t", "xn--ab-j1t", false, "idna: invalid label \"a\\u200cb\""},
		{"\u00F6bb.at", "xn--bb-eka.at", true, ""},
		{"o\u0308bb.at", "xn--bb-eka.at", true, ""},
		{"\u00D6BB.at", "xn--bb-eka.at", true, ""},
		{"O\u0308BB.at", "xn--bb-eka.at", true, ""},
		{"ȡog.de", "xn--og-09a.de", true, ""},
		{"☕.de", "xn--53h.de", true, ""},
		{"I♥NY.de", "xn--iny-zx5a.de", true, ""},
		{"ＡＢＣ・日本.co.jp", "xn--abc-rs4b422ycvb.co.jp", true, ""},
		{"日本｡co｡jp", "xn--wgv71a.co.jp", true, ""},
		{"日本｡co．jp", "xn--wgv71a.co.jp", true, ""},
		{"日本⒈co．jp", "xn--co-wuw5954azlb.jp", false, "idna: disallowed rune U+2488"},
		{"x\u0327\u0301.de", "xn--x-xbb7i.de", true, ""},
		{"x\u0301\u0327.de", "xn--x-xbb7i.de", true, ""},
		{"σόλος.gr", "xn--wxaijb9b.gr", true, ""},
		{"Σόλος.gr", "xn--wxaijb9b.gr", true, ""},
		{"ΣΌΛΟΣ.grﻋﺮﺑﻲ.de", "xn--wxaikc6b.xn--gr-gtd9a1b0g.de", false, "idna: invalid label \"σόλοσ.grعربي.de\""},
		{"عربي.de", "xn--ngbrx4e.de", true, ""},
		{"نامهای.de", "xn--mgba3gch31f.de", true, ""},
		{"نامه\u200Cای.de", "xn--mgba3gch31f060k.de", true, ""},
		// some other test cases
		{"xn--a.xn--a.xn--a.com", "xn--a.xn--a.xn--a.com", false, "idna: invalid label \"\\u0080\""},
		{"a.com...｡", "a.com...", true, ""},
		{"..｡..a.com", "a.com", true, ""},
	} {
		tc := tc
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			normalized, err := api.NewFQDN(tc.input)
			require.Equal(t, tc.expected, normalized)
			if tc.ok {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tc.errString)
			}
		})
	}
}

func TestSortFQDNs(t *testing.T) {
	t.Parallel()

	require.NoError(t, quick.Check(
		func(s []api.FQDN) bool {
			copied := make([]api.FQDN, len(s))
			copy(copied, s)
			api.SortFQDNs(s)
			switch {
			case !assert.ElementsMatch(t, copied, s):
				return false
			case !sort.SliceIsSorted(s, func(i, j int) bool { return s[i] < s[j] }):
				return false
			default:
				return true
			}
		},
		nil,
	))
}

func TestSortFQDNSplitter(t *testing.T) {
	t.Parallel()
	type ss = []string
	for _, tc := range [...]struct {
		input    string
		expected []string
	}{
		{"...", ss{"...", "..", ".", ""}},
		{"aaa...", ss{"aaa...", "..", ".", ""}},
		{".aaa..", ss{".aaa..", "aaa..", ".", ""}},
		{"..aaa.", ss{"..aaa.", ".aaa.", "aaa.", ""}},
		{"...aaa", ss{"...aaa", "..aaa", ".aaa", "aaa", ""}},
	} {
		tc := tc
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			var ss []string
			for s := api.NewFQDNSplitter(api.FQDN(tc.input)); s.IsValid(); s.Next() {
				ss = append(ss, s.Suffix())
			}
			require.Equal(t, tc.expected, ss)
		})
	}
}
