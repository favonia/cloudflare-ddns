package domain_test

import (
	"slices"
	"testing"
	"testing/quick"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/domain"
)

func TestNew(t *testing.T) {
	t.Parallel()
	type f = domain.FQDN
	type w = domain.Wildcard
	for _, tc := range [...]struct {
		input     string
		expected  domain.Domain
		ok        bool
		errString string
	}{
		{"tHe.CaPiTaL.cAsE", f("the.capital.case"), true, ""},
		// The following examples were adapted from https://unicode.org/cldr/utility/idna.jsp
		{"fass.de", f("fass.de"), true, ""},
		{"faß.de", f("xn--fa-hia.de"), true, ""},
		{"fäß.de", f("xn--f-qfao.de"), true, ""},
		{"xn--fa-hia.de", f("xn--fa-hia.de"), true, ""},
		{"₹.com", f("xn--yzg.com"), true, ""},
		{"𑀓.com", f("xn--n00d.com"), true, ""},
		{"\u0080.com", f("xn--a.com"), false, "idna: disallowed rune U+0080"},
		{"xn--a.com", f("xn--a.com"), false, `idna: invalid label "\u0080"`},
		{"a\u200Cb.org", f("xn--ab-j1t.org"), false, `idna: invalid label "a\u200cb"`},
		{"xn--ab-j1t.org", f("xn--ab-j1t.org"), false, `idna: invalid label "a\u200cb"`},
		{"\u00F6bb.at", f("xn--bb-eka.at"), true, ""},
		{"o\u0308bb.at", f("xn--bb-eka.at"), true, ""},
		{"\u00D6BB.at", f("xn--bb-eka.at"), true, ""},
		{"O\u0308BB.at", f("xn--bb-eka.at"), true, ""},
		{"ȡog.de", f("xn--og-09a.de"), true, ""},
		{"☕.de", f("xn--53h.de"), true, ""},
		{"I♥NY.de", f("xn--iny-zx5a.de"), true, ""},
		{"ＡＢＣ・日本.co.jp", f("xn--abc-rs4b422ycvb.co.jp"), true, ""},
		{"日本｡co｡jp", f("xn--wgv71a.co.jp"), true, ""},
		{"日本｡co．jp", f("xn--wgv71a.co.jp"), true, ""},
		{"日本⒈co．jp", f("xn--co-wuw5954azlb.jp"), false, "idna: disallowed rune U+2488"},
		{"x\u0327\u0301.de", f("xn--x-xbb7i.de"), true, ""},
		{"x\u0301\u0327.de", f("xn--x-xbb7i.de"), true, ""},
		{"σόλος.gr", f("xn--wxaijb9b.gr"), true, ""},
		{"Σόλος.gr", f("xn--wxaijb9b.gr"), true, ""},
		{"ΣΌΛΟΣ.grﻋﺮﺑﻲ.de", f("xn--wxaikc6b.xn--gr-gtd9a1b0g.de"), false, `idna: invalid label "σόλοσ.grعربي.de"`},
		{"عربي.de", f("xn--ngbrx4e.de"), true, ""},
		{"نامهای.de", f("xn--mgba3gch31f.de"), true, ""},
		{"نامه\u200Cای.de", f("xn--mgba3gch31f060k.de"), true, ""},
		// wildcards
		{"*.fass.de", w("fass.de"), true, ""},
		{"*.faß.de", w("xn--fa-hia.de"), true, ""},
		{"*.fäß.de", w("xn--f-qfao.de"), true, ""},
		{"*.xn--fa-hia.de", w("xn--fa-hia.de"), true, ""},
		{"*.₹.com", w("xn--yzg.com"), true, ""},
		{"*.𑀓.com", w("xn--n00d.com"), true, ""},
		{"*.\u0080.com", w("xn--a.com"), false, `idna: invalid label "\u0080"`},
		{"*.xn--a.com", w("xn--a.com"), false, `idna: invalid label "\u0080"`},
		{"*.a\u200Cb", w("xn--ab-j1t"), false, `idna: invalid label "a\u200cb"`},
		{"*.xn--ab-j1t", w("xn--ab-j1t"), false, `idna: invalid label "a\u200cb"`},
		{"*.\u00F6bb.at", w("xn--bb-eka.at"), true, ""},
		{"*.o\u0308bb.at", w("xn--bb-eka.at"), true, ""},
		{"*.\u00D6BB.at", w("xn--bb-eka.at"), true, ""},
		{"*.O\u0308BB.at", w("xn--bb-eka.at"), true, ""},
		{"*.ȡog.de", w("xn--og-09a.de"), true, ""},
		{"*.☕.de", w("xn--53h.de"), true, ""},
		{"*.I♥NY.de", w("xn--iny-zx5a.de"), true, ""},
		{"*.ＡＢＣ・日本.co.jp", w("xn--abc-rs4b422ycvb.co.jp"), true, ""},
		{"*｡日本｡co｡jp", w("xn--wgv71a.co.jp"), true, ""},
		{"*｡日本｡co．jp", w("xn--wgv71a.co.jp"), true, ""},
		{"*．日本｡co．jp", w("xn--wgv71a.co.jp"), true, ""},
		{"*．日本⒈co．jp", w("xn--co-wuw5954azlb.jp"), false, `idna: invalid label "日本⒈co"`},
		{"*.x\u0327\u0301.de", w("xn--x-xbb7i.de"), true, ""},
		{"*.x\u0301\u0327.de", w("xn--x-xbb7i.de"), true, ""},
		{"*.σόλος.gr", w("xn--wxaijb9b.gr"), true, ""},
		{"*.Σόλος.gr", w("xn--wxaijb9b.gr"), true, ""},
		{
			"*.ΣΌΛΟΣ.grﻋﺮﺑﻲ.de", w("xn--wxaikc6b.xn--gr-gtd9a1b0g.de"),
			false,
			`idna: invalid label "xn--wxaikc6b.xn--gr-gtd9a1b0g.de"`,
		},
		{"*.عربي.de", w("xn--ngbrx4e.de"), true, ""},
		{"*.نامهای.de", w("xn--mgba3gch31f.de"), true, ""},
		{"*.نامه\u200Cای.de", w("xn--mgba3gch31f060k.de"), true, ""},
		// some other test cases
		{"xn--a.xn--a.xn--a.com", f("xn--a.xn--a.xn--a.com"), false, `idna: invalid label "\u0080"`},
		{"a.com...｡", f("a.com"), true, ""},
		{"..｡..a.com", f("a.com"), true, ""},
		{"*.xn--a.xn--a.xn--a.com", w("xn--a.xn--a.xn--a.com"), false, `idna: invalid label "\u0080"`},
		{"*.a.com...｡", w("a.com"), true, ""},
		{"*...｡..a.com", w(".....a.com"), true, ""},
		{"*.A......", w("a"), true, ""},
		{"*｡A｡｡｡｡｡", w("a"), true, ""},
		{"*.*.*", w("*.*"), false, `idna: disallowed rune U+002A`},
		{"*......", w(""), false, "too few labels"},
		{"*｡｡｡｡｡｡", w(""), false, "too few labels"},
		{"......", f(""), false, "too few labels"},
		{"｡｡｡｡｡｡", f(""), false, "too few labels"},
	} {
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			normalized, err := domain.New(tc.input)
			require.Equal(t, tc.expected, normalized)
			if tc.ok {
				require.NoError(t, err)
				require.Empty(t, tc.errString)
			} else {
				require.EqualError(t, err, tc.errString)
			}
		})
	}
}

func TestNewTooFewLabels(t *testing.T) {
	t.Parallel()
	for _, input := range [...]string{"com", "localhost", "org", "hello.", ".", "*"} {
		t.Run(input, func(t *testing.T) {
			t.Parallel()
			_, err := domain.New(input)
			require.ErrorIs(t, err, domain.ErrTooFewLabels)
		})
	}
}

func TestSortDomains(t *testing.T) {
	t.Parallel()

	require.NoError(t, quick.Check(
		func(fs []domain.FQDN, ws []domain.Wildcard) bool {
			merged := make([]domain.Domain, 0, len(fs)+len(ws))

			for _, f := range fs {
				merged = append(merged, f)
			}
			for _, w := range ws {
				merged = append(merged, w)
			}

			copied := make([]domain.Domain, len(merged))
			copy(copied, merged)
			domain.SortDomains(merged)

			require.ElementsMatch(t, copied, merged)
			require.True(t, slices.IsSortedFunc(merged, domain.CompareDomain))

			return true
		},
		nil,
	))
}
