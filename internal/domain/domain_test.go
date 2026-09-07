package domain_test

import (
	"slices"
	"strings"
	"testing"
	"testing/quick"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/domain"
)

func normalization(leading, extraTrailing bool) domain.Normalization {
	return domain.Normalization{
		RemovedLeadingDots:       leading,
		RemovedExtraTrailingDots: extraTrailing,
	}
}

func TestNew(t *testing.T) {
	t.Parallel()
	type f = domain.FQDN
	type w = domain.Wildcard
	for _, tc := range [...]struct {
		input         string
		expected      domain.Domain
		normalization domain.Normalization
		err           error
	}{
		{"example.org", f("example.org"), normalization(false, false), nil},
		{"example.org.", f("example.org"), normalization(false, false), nil},
		{".example.org", f("example.org"), normalization(true, false), nil},
		{"..example.org", f("example.org"), normalization(true, false), nil},
		{"example.org..", f("example.org"), normalization(false, true), nil},
		{"example.org...", f("example.org"), normalization(false, true), nil},
		{"..example.org...", f("example.org"), normalization(true, true), nil},
		{"*.example.org", w("example.org"), normalization(false, false), nil},
		{"*.example.org.", w("example.org"), normalization(false, false), nil},
		{"*.example.org..", w("example.org"), normalization(false, true), nil},
		{"..*.example.org...", w("example.org"), normalization(true, true), nil},
		{"......", f(""), normalization(false, true), domain.ErrTooFewLabels},
		{"*......", w(""), normalization(false, true), domain.ErrTooFewLabels},
		{"a..example.org", nil, normalization(false, false), domain.ErrEmptyInteriorLabel},
		{"*..example.org", nil, normalization(false, false), domain.ErrEmptyInteriorLabel},
		{"*...example.org", nil, normalization(false, false), domain.ErrEmptyInteriorLabel},
		{"*.a..example.org", nil, normalization(false, false), domain.ErrEmptyInteriorLabel},
		{"\u3002example.org", f("example.org"), normalization(true, false), nil},
		{"\uff0eexample.org", f("example.org"), normalization(true, false), nil},
		{"\uff61example.org", f("example.org"), normalization(true, false), nil},
		{"example.org\u3002\u3002", f("example.org"), normalization(false, true), nil},
		{"example.org\uff0e\uff0e", f("example.org"), normalization(false, true), nil},
		{"example.org\uff61\uff61", f("example.org"), normalization(false, true), nil},
		{"\u3002example.org\uff0e\uff61", f("example.org"), normalization(true, true), nil},
		{"a\u3002\u3002example.org", nil, normalization(false, false), domain.ErrEmptyInteriorLabel},
		{"a\uff0e\uff0eexample.org", nil, normalization(false, false), domain.ErrEmptyInteriorLabel},
		{"a\uff61\uff61example.org", nil, normalization(false, false), domain.ErrEmptyInteriorLabel},
		{"a\u3002\uff0eexample.org", nil, normalization(false, false), domain.ErrEmptyInteriorLabel},
	} {
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			got, normalization, err := domain.New(tc.input)
			require.Equal(t, tc.expected, got)
			require.Equal(t, tc.normalization, normalization)
			if tc.err == nil {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, tc.err)
			}
		})
	}
}

func TestEmptyInteriorLabelRejection(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name  string
		input string
	}{
		{"plain-long-runs", "a......b.....c.....d"},
		{"wildcard-immediate-only", "*..a"},
		{"wildcard-immediate-and-later", "*......a.....b"},
		{"wildcard-later-only", "*.a.....b"},
		{"unicode-immediate-and-later", "*｡｡a。｡b"},
		{"unicode-later-only", "*.a｡｡b"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, _, newErr := domain.New(tc.input)
			require.EqualError(t, newErr, domain.ErrEmptyInteriorLabel.Error())
			require.ErrorIs(t, newErr, domain.ErrEmptyInteriorLabel)

			_, _, suffixErr := domain.NewSuffix(tc.input)
			require.ErrorIs(t, suffixErr, domain.ErrEmptyInteriorLabel)
		})
	}
}

func TestNewIDNA(t *testing.T) {
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
		{"*.xn--a.xn--a.xn--a.com", w("xn--a.xn--a.xn--a.com"), false, `idna: invalid label "\u0080"`},
		{"*.*.*", w("*.*"), false, `idna: disallowed rune U+002A`},
	} {
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			normalized, normalization, err := domain.New(tc.input)
			require.Equal(t, tc.expected, normalized)
			require.Empty(t, normalization)
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
			_, _, err := domain.New(input)
			require.ErrorIs(t, err, domain.ErrTooFewLabels)
		})
	}
}

func TestNewTooFewLabelsTakesPrecedenceOverIDNAErrors(t *testing.T) {
	t.Parallel()

	got, normalization, err := domain.New("\u0080")
	require.Equal(t, domain.FQDN("xn--a"), got)
	require.Empty(t, normalization)
	require.ErrorIs(t, err, domain.ErrTooFewLabels)
}

func TestConstructorErrorResults(t *testing.T) {
	t.Parallel()

	t.Run("short target retains normalization", func(t *testing.T) {
		t.Parallel()
		got, n, err := domain.New(".org..")
		require.ErrorIs(t, err, domain.ErrTooFewLabels)
		require.Equal(t, domain.FQDN("org"), got)
		require.Equal(t, normalization(true, true), n)
	})

	t.Run("invalid wildcard has no usable result", func(t *testing.T) {
		t.Parallel()
		got, n, err := domain.New(".*..example.org..")
		require.ErrorIs(t, err, domain.ErrEmptyInteriorLabel)
		require.Nil(t, got)
		require.Zero(t, n)
		suffix, sn, serr := domain.NewSuffix(".*..example.org..")
		require.ErrorIs(t, serr, domain.ErrEmptyInteriorLabel)
		require.Empty(t, suffix)
		require.Zero(t, sn)
	})

	t.Run("IDNA failure returns diagnostic value only", func(t *testing.T) {
		t.Parallel()
		got, n, err := domain.New(".bad*.example..")
		require.Error(t, err)
		require.NotErrorIs(t, err, domain.ErrTooFewLabels)
		require.NotErrorIs(t, err, domain.ErrEmptyInteriorLabel)
		require.Equal(t, domain.FQDN("bad*.example"), got)
		require.Zero(t, n)
		suffix, sn, serr := domain.NewSuffix(".bad*.example..")
		require.Error(t, serr)
		require.NotErrorIs(t, serr, domain.ErrWildcardSuffix)
		require.Equal(t, domain.Suffix("bad*.example"), suffix)
		require.Zero(t, sn)
	})
}

func TestConstructedDomainInvariant(t *testing.T) {
	t.Parallel()

	assertDomain := func(t *testing.T, input string) {
		t.Helper()
		got, _, err := domain.New(input)
		require.NoError(t, err, "input: %q", input)
		require.NotNil(t, got)
		require.NotEmpty(t, got.DNSNameASCII())
		ascii := got.DNSNameASCII()
		require.False(t, strings.HasPrefix(ascii, "."))
		require.False(t, strings.HasSuffix(ascii, "."))
		require.NotContains(t, ascii, "..")
		reparsed, reparsedNormalization, reparsedErr := domain.New(ascii)
		require.NoError(t, reparsedErr)
		require.Equal(t, normalization(false, false), reparsedNormalization)
		require.Equal(t, got, reparsed)
		require.NotEmpty(t, got.String())
		require.NotEmpty(t, got.Describe())
		require.Zero(t, domain.CompareDomain(got, reparsed))
		require.True(t, got.HasStrictSuffix(domain.Suffix("")))
		var zones []domain.Suffix
		got.Zones(func(zone domain.Suffix) bool {
			zones = append(zones, zone)
			return true
		})
		require.NotEmpty(t, zones)
		require.Equal(t, strings.TrimPrefix(ascii, "*."), zones[0].DNSNameASCII())
	}

	assertSuffix := func(t *testing.T, input string) {
		t.Helper()
		got, _, err := domain.NewSuffix(input)
		require.NoError(t, err, "input: %q", input)
		ascii := got.DNSNameASCII()
		require.False(t, strings.HasPrefix(ascii, "."))
		require.False(t, strings.HasSuffix(ascii, "."))
		require.NotContains(t, ascii, "..")
		reparsed, reparsedNormalization, reparsedErr := domain.NewSuffix(ascii)
		require.NoError(t, reparsedErr)
		require.Equal(t, normalization(false, false), reparsedNormalization)
		require.Equal(t, got, reparsed)
		require.NotEmpty(t, got.String())
	}

	for _, input := range [...]string{
		"example.org", "example.org.", ".example.org", "..example.org",
		"example.org..", "example.org...", "..example.org...", "*.example.org",
		"*.example.org.", "*.example.org..", "\u3002example.org", "\uff0eexample.org",
		"\uff61example.org", "example.org\u3002\u3002", "example.org\uff0e\uff0e",
		"example.org\uff61\uff61", "\u3002example.org\uff0e\uff61",
	} {
		assertDomain(t, input)
	}
	for _, input := range [...]string{
		"", ".", "..", "...", "example.org", "org", "example.org.",
		".example.org", "example.org..", "..example.org...", "\u3002",
		"\uff0e\uff0e", "\uff61\uff61\uff61", "\u3002example.org", "\uff0eexample.org",
		"\uff61example.org", "example.org\u3002\u3002", "example.org\uff0e\uff0e",
		"example.org\uff61\uff61", "\u3002example.org\uff0e\uff61",
	} {
		assertSuffix(t, input)
	}

	require.NoError(t, quick.Check(func(input string) bool {
		// Arbitrary inputs need only satisfy the invariant when accepted.
		if _, _, err := domain.New(input); err == nil {
			assertDomain(t, input)
		}
		if _, _, err := domain.NewSuffix(input); err == nil {
			assertSuffix(t, input)
		}
		return true
	}, nil))

	canonicalLabel := func(value string) string {
		value = strings.Map(func(r rune) rune {
			if r >= 'a' && r <= 'z' {
				return r
			}
			return -1
		}, value)
		if value == "" {
			return "example"
		}
		return value[:min(len(value), 12)]
	}
	for _, tc := range []struct {
		name        string
		buildInput  func(string, string) string
		checkSuffix bool
	}{
		{
			name:        "fqdn with multiple interior labels",
			buildInput:  func(left, right string) string { return left + "." + right + ".org" },
			checkSuffix: true,
		},
		{
			name:        "wildcard with multiple interior labels",
			buildInput:  func(left, right string) string { return "*." + left + "." + right + ".org" },
			checkSuffix: false,
		},
		{
			name:        "IDNA with multiple interior labels",
			buildInput:  func(left, right string) string { return "faß." + left + "." + right + ".org" },
			checkSuffix: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.NoError(t, quick.Check(func(left, right string, leading, trailing uint8) bool {
				input := strings.Repeat(".", int(leading%3)) +
					tc.buildInput(canonicalLabel(left), canonicalLabel(right)) +
					strings.Repeat(".", int(trailing%4))
				assertDomain(t, input)
				if tc.checkSuffix {
					assertSuffix(t, input)
				} else {
					_, n, err := domain.NewSuffix(input)
					require.ErrorIs(t, err, domain.ErrWildcardSuffix)
					require.Equal(t, normalization(leading%3 != 0, trailing%4 >= 2), n)
				}
				return true
			}, nil))
		})
	}
	for _, tc := range []struct {
		name         string
		interiorDots string
		err          error
	}{
		{name: "one interior dot", interiorDots: ".", err: nil},
		{name: "empty interior label", interiorDots: "..", err: domain.ErrEmptyInteriorLabel},
		{name: "multiple empty interior labels", interiorDots: "...", err: domain.ErrEmptyInteriorLabel},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.NoError(t, quick.Check(func(left, right string, leading, trailing uint8) bool {
				input := strings.Repeat(".", int(leading%3)) +
					canonicalLabel(left) + tc.interiorDots + canonicalLabel(right) + ".org" +
					strings.Repeat(".", int(trailing%4))
				if tc.err == nil {
					assertDomain(t, input)
					assertSuffix(t, input)
				} else {
					d, dn, derr := domain.New(input)
					require.ErrorIs(t, derr, tc.err)
					require.Nil(t, d)
					require.Zero(t, dn)
					s, sn, serr := domain.NewSuffix(input)
					require.ErrorIs(t, serr, tc.err)
					require.Empty(t, s)
					require.Zero(t, sn)
				}
				return true
			}, nil))
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
