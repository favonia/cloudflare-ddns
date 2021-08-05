package api_test

import (
	"sort"
	"testing"
	"testing/quick"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns-go/internal/api"
)

func TestFQDNString(t *testing.T) {
	t.Parallel()

	require.NoError(t, quick.Check(
		func(s string) bool {
			return api.FQDN(s).String() == s
		},
		nil,
	))
}

func TestNewFQDN(t *testing.T) {
	t.Parallel()
	for _, tc := range [...]struct {
		input     string
		expected  api.FQDN
		ok        bool
		errString string
	}{
		// The following examples were reproduced from https://unicode.org/cldr/utility/idna.jsp
		{"fass.de", "fass.de", true, ""},
		{"faß.de", "faß.de", true, ""},
		{"fäß.de", "fäß.de", true, ""},
		{"xn--fa-hia.de", "faß.de", true, ""},
		{"₹.com", "₹.com", true, ""},
		{"𑀓.com", "𑀓.com", true, ""},
		{"\u0080.com", "xn--a.com", false, "idna: disallowed rune U+0080"},
		{"xn--a.com", "xn--a.com", false, "idna: invalid label \"\\u0080\""},
		{"a\u200Cb", "xn--ab-j1t", false, "idna: invalid label \"a\\u200cb\""},
		{"xn--ab-j1t", "xn--ab-j1t", false, "idna: invalid label \"a\\u200cb\""},
		{"öbb.at", "öbb.at", true, ""},
		{"ÖBB.at", "öbb.at", true, ""},
		{"ÖBB.at", "öbb.at", true, ""},
		{"ȡog.de", "ȡog.de", true, ""},
		{"☕.de", "☕.de", true, ""},
		{"I♥NY.de", "i♥ny.de", true, ""},
		{"ＡＢＣ・日本.co.jp", "abc・日本.co.jp", true, ""},
		{"日本｡co｡jp", "日本.co.jp", true, ""},
		{"日本｡co．jp", "日本.co.jp", true, ""},
		{"日本⒈co．jp", "xn--co-wuw5954azlb.jp", false, "idna: disallowed rune U+2488"},
		{"x\u0327\u0301.de", "x̧́.de", true, ""},
		{"x\u0301\u0327.de", "x̧́.de", true, ""},
		{"σόλος.gr", "σόλος.gr", true, ""},
		{"Σόλος.gr", "σόλος.gr", true, ""},
		{"ΣΌΛΟΣ.grﻋﺮﺑﻲ.de", "xn--wxaikc6b.xn--gr-gtd9a1b0g.de", false, "idna: invalid label \"σόλοσ.grعربي.de\""},
		{"عربي.de", "عربي.de", true, ""},
		{"نامهای.de", "نامهای.de", true, ""},
		{"نامه\u200Cای.de", "نامه\u200cای.de", true, ""},
		// some other test cases
		{"xn--a.xn--a.xn--a.com", "xn--a.xn--a.xn--a.com", false, "idna: invalid label \"\\u0080\""},
		{"a.com...｡", "a.com...", true, ""},
		{"..｡..a.com", "a.com", true, ""},
		{"O\u0308", "\u00F6", true, ""},
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
		// The following examples were adapted from https://unicode.org/cldr/utility/idna.jsp
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
