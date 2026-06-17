package domainentry_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/domainentry"
	"github.com/favonia/cloudflare-ddns/internal/hostid6"
	"github.com/favonia/cloudflare-ddns/internal/syntax"
)

func describeSet(set hostid6.Set) []string {
	values := set.Values()
	descriptions := make([]string, len(values))
	for i, value := range values {
		descriptions[i] = value.String()
	}
	return descriptions
}

func TestParseEntriesPlainEntryExactSpan(t *testing.T) {
	t.Parallel()

	entries, diagnostics, err := domainentry.Parse("example.org")

	require.Nil(t, err)
	require.Empty(t, diagnostics)
	require.Equal(t, []domainentry.Entry{{
		Domain:          domain.FQDN("example.org"),
		HostID6Opinions: nil,
		Span:            syntax.Span{Start: 0, End: 11},
	}}, entries)
}

func TestParseEntriesEmptyInput(t *testing.T) {
	t.Parallel()

	entries, diagnostics, err := domainentry.Parse("")

	require.Nil(t, err)
	require.Empty(t, entries)
	require.Empty(t, diagnostics)
}

func TestParseEntriesReportsInvalidValueInBracketList(t *testing.T) {
	t.Parallel()

	entries, diagnostics, err := domainentry.Parse("example.org{hostid6=[bad,::1]}")

	require.Nil(t, err)
	require.Empty(t, entries)
	require.Len(t, diagnostics, 1)
	require.Equal(t, domainentry.KindInvalidHostID6, diagnostics[0].Kind)
}

func TestParseEntriesDomainsAndSpans(t *testing.T) {
	t.Parallel()

	entries, diagnostics, err := domainentry.Parse("faß.de,*.☕.de")

	require.Nil(t, err)
	require.Empty(t, diagnostics)
	require.Equal(t, []domainentry.Entry{
		{
			Domain:          domain.FQDN("xn--fa-hia.de"),
			HostID6Opinions: nil,
			Span:            syntax.Span{Start: 0, End: 7},
		},
		{
			Domain:          domain.Wildcard("xn--53h.de"),
			HostID6Opinions: nil,
			Span:            syntax.Span{Start: 8, End: 16},
		},
	}, entries)
}

func TestParseEntriesHostID6Sets(t *testing.T) {
	t.Parallel()

	input := "example.org{hostid6=[::2,preserve,::1,::2,],hostid6=mac(00-11-22-33-44-55),hostid6=::3}"
	entries, diagnostics, err := domainentry.Parse(input)

	require.Nil(t, err)
	require.Empty(t, diagnostics)
	require.Len(t, entries, 1)
	require.Equal(t, domain.FQDN("example.org"), entries[0].Domain)
	require.Equal(t, syntax.Span{Start: 0, End: len(input)}, entries[0].Span)
	require.Len(t, entries[0].HostID6Opinions, 3)
	require.Equal(t, []string{"preserve", "::1", "::2"}, describeSet(entries[0].HostID6Opinions[0]))
	require.Equal(t, []string{"mac(00-11-22-33-44-55)"}, describeSet(entries[0].HostID6Opinions[1]))
	require.Equal(t, []string{"::3"}, describeSet(entries[0].HostID6Opinions[2]))
}

func TestParseEntriesAcceptsUniversalTrailingCommas(t *testing.T) {
	t.Parallel()

	input := "example.org{hostid6=[preserve,],},"
	entries, diagnostics, err := domainentry.Parse(input)

	require.Nil(t, err)
	require.Empty(t, diagnostics)
	require.Equal(t, []domainentry.Entry{{
		Domain:          domain.FQDN("example.org"),
		HostID6Opinions: []hostid6.Set{hostid6.DefaultSet()},
		Span:            syntax.Span{Start: 0, End: len(input) - 1},
	}}, entries)
}

func TestParseEntriesSemanticDiagnosticsAndRecovery(t *testing.T) {
	t.Parallel()

	input := "localhost,good.example,example.org{unknown=::1},example.net{hostid6=192.0.2.1},example.com{hostid6=mac(bad)}"
	entries, diagnostics, err := domainentry.Parse(input)

	require.Nil(t, err)
	require.Equal(t, []domainentry.Entry{{
		Domain:          domain.FQDN("good.example"),
		HostID6Opinions: nil,
		Span:            syntax.Span{Start: 10, End: 22},
	}}, entries)
	require.Len(t, diagnostics, 4)
	require.Equal(t, syntax.Span{Start: 0, End: 9}, diagnostics[0].Span)
	require.Equal(t, domainentry.KindInvalidDomain, diagnostics[0].Kind)
	require.Equal(t, syntax.Span{Start: 35, End: 42}, diagnostics[1].Span)
	require.Equal(t, domainentry.KindUnknownDomainField, diagnostics[1].Kind)
	require.Equal(t, syntax.Span{Start: 68, End: 77}, diagnostics[2].Span)
	require.Equal(t, domainentry.KindInvalidHostID6, diagnostics[2].Kind)
	require.Equal(t, syntax.Span{Start: 103, End: 106}, diagnostics[3].Span)
	require.Equal(t, domainentry.KindInvalidMAC, diagnostics[3].Kind)
}

func TestParseEntriesRejectsQuotedCommaList(t *testing.T) {
	t.Parallel()

	entries, diagnostics, err := domainentry.Parse(`example.org{hostid6="::1,::2"}`)

	require.Nil(t, entries)
	require.Empty(t, diagnostics)
	require.NotNil(t, err)
	require.Equal(t, syntax.Span{Start: 25, End: 29}, err.Span)
}

func TestParseEntriesStopsOnAmbiguousMalformedNesting(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name  string
		input string
		span  syntax.Span
	}{
		{
			name:  "value list interior empty",
			input: "localhost,example.org{hostid6=[::1,,::2]},good.example",
			span:  syntax.Span{Start: 35, End: 36},
		},
		{
			name:  "field block interior empty",
			input: "localhost,example.org{hostid6=::1,,hostid6=::2},good.example",
			span:  syntax.Span{Start: 34, End: 35},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			entries, diagnostics, err := domainentry.Parse(tc.input)

			require.Nil(t, entries)
			require.Empty(t, diagnostics)
			require.NotNil(t, err)
			require.Equal(t, tc.span, err.Span)
		})
	}
}

func TestParseEntriesRejectsStructuredDomainExpressions(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		input string
		span  syntax.Span
	}{
		{input: "foo=bar", span: syntax.Span{Start: 0, End: 3}},
		{input: "mac(foo){}", span: syntax.Span{Start: 0, End: 3}},
		{input: "mac(foo){hostid6=::1}", span: syntax.Span{Start: 0, End: 3}},
		{input: "example.org{mac(foo),hostid6=::1}", span: syntax.Span{Start: 12, End: 15}},
		{input: "example.org{hostid6=[foo=bar,::1]}", span: syntax.Span{Start: 21, End: 24}},
		{input: "example.org{mac(foo)=::1}", span: syntax.Span{Start: 12, End: 15}},
	} {
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()

			entries, diagnostics, err := domainentry.Parse(tc.input)

			require.Nil(t, entries)
			require.Empty(t, diagnostics)
			require.ErrorIs(t, err, syntax.ErrUnexpectedToken)
			require.Equal(t, tc.span, err.Span)
		})
	}
}

func TestParseEntriesReturnsCompatibilityDiagnostics(t *testing.T) {
	t.Parallel()

	entries, diagnostics, err := domainentry.Parse(",example.org example.net,,")

	require.Nil(t, err)
	require.Equal(t, []domainentry.Entry{
		{
			Domain:          domain.FQDN("example.org"),
			HostID6Opinions: nil,
			Span:            syntax.Span{Start: 1, End: 12},
		},
		{
			Domain:          domain.FQDN("example.net"),
			HostID6Opinions: nil,
			Span:            syntax.Span{Start: 13, End: 24},
		},
	}, entries)
	require.Len(t, diagnostics, 2)
	require.Equal(t, syntax.Span{Start: 0, End: 1}, diagnostics[0].Span)
	require.Equal(t, domainentry.KindExtraComma, diagnostics[0].Kind)
	require.Equal(t, syntax.Span{Start: 12, End: 13}, diagnostics[1].Span)
	require.Equal(t, domainentry.KindMissingComma, diagnostics[1].Kind)
}

func TestParseEntriesManyLeadingCommasReturnOneDiagnostic(t *testing.T) {
	t.Parallel()

	const commaCount = 4096
	entries, diagnostics, err := domainentry.Parse(strings.Repeat(",", commaCount) + "example.org")

	require.Nil(t, err)
	require.Equal(t, []domainentry.Entry{{
		Domain:          domain.FQDN("example.org"),
		HostID6Opinions: nil,
		Span:            syntax.Span{Start: commaCount, End: commaCount + len("example.org")},
	}}, entries)
	require.Equal(t, []domainentry.Diagnostic{{
		Span:   syntax.Span{Start: 0, End: 1},
		Kind:   domainentry.KindExtraComma,
		Detail: nil,
	}}, diagnostics)
}

func TestParseEntriesManyMissingCommasReturnOneDiagnostic(t *testing.T) {
	t.Parallel()

	entries, diagnostics, err := domainentry.Parse("example.org example.net example.com")

	require.Nil(t, err)
	require.Equal(t, []domainentry.Entry{
		{Domain: domain.FQDN("example.org"), HostID6Opinions: nil, Span: syntax.Span{Start: 0, End: 11}},
		{Domain: domain.FQDN("example.net"), HostID6Opinions: nil, Span: syntax.Span{Start: 12, End: 23}},
		{Domain: domain.FQDN("example.com"), HostID6Opinions: nil, Span: syntax.Span{Start: 24, End: 35}},
	}, entries)
	require.Equal(t, []domainentry.Diagnostic{{
		Span:   syntax.Span{Start: 11, End: 12},
		Kind:   domainentry.KindMissingComma,
		Detail: nil,
	}}, diagnostics)
}

func TestParseEntriesStopsMissingCommaRecoveryAfterSemanticError(t *testing.T) {
	t.Parallel()

	entries, diagnostics, err := domainentry.Parse("localhost good.example")

	require.Nil(t, err)
	require.Empty(t, entries)
	require.Len(t, diagnostics, 2)
	require.Equal(t, domainentry.KindInvalidDomain, diagnostics[0].Kind)
	require.Equal(t, domainentry.KindMissingComma, diagnostics[1].Kind)
}

func TestParseEntriesRecoversAtTopLevelCommaAfterFirstFieldError(t *testing.T) {
	t.Parallel()

	input := "example.org{unknown=::1,hostid6=bad},good.example"
	entries, diagnostics, err := domainentry.Parse(input)

	require.Nil(t, err)
	require.Equal(t, []domainentry.Entry{{
		Domain:          domain.FQDN("good.example"),
		HostID6Opinions: nil,
		Span:            syntax.Span{Start: 37, End: 49},
	}}, entries)
	require.Len(t, diagnostics, 1)
	require.Equal(t, domainentry.KindUnknownDomainField, diagnostics[0].Kind)
}

func TestParseEntriesAcceptsTrailingCommaWithoutDiagnostic(t *testing.T) {
	t.Parallel()

	entries, diagnostics, err := domainentry.Parse("example.org,")

	require.Nil(t, err)
	require.Empty(t, diagnostics)
	require.Equal(t, []domainentry.Entry{{
		Domain:          domain.FQDN("example.org"),
		HostID6Opinions: nil,
		Span:            syntax.Span{Start: 0, End: 11},
	}}, entries)
}

func TestParseEntriesReportsExtraTrailingCommas(t *testing.T) {
	t.Parallel()

	input := "example.org,,,,,,"
	entries, diagnostics, err := domainentry.Parse(input)

	require.Nil(t, err)
	require.Equal(t, []domainentry.Entry{{
		Domain:          domain.FQDN("example.org"),
		HostID6Opinions: nil,
		Span:            syntax.Span{Start: 0, End: 11},
	}}, entries)
	require.Len(t, diagnostics, 1)
	require.Equal(t, domainentry.KindExtraComma, diagnostics[0].Kind)
}

func TestParseEntriesRejectsExtraTrailingCommasInStrictLists(t *testing.T) {
	t.Parallel()

	for _, input := range []string{
		"example.org{hostid6=::1,,,,,,}",
		"example.org{hostid6=[::1,,,,,,]}",
	} {
		t.Run(input, func(t *testing.T) {
			t.Parallel()

			entries, diagnostics, err := domainentry.Parse(input)

			require.Nil(t, entries)
			require.Empty(t, diagnostics)
			require.ErrorIs(t, err, syntax.ErrUnexpectedToken)
		})
	}
}

func TestEntryDiagnosticDescriptions(t *testing.T) {
	t.Parallel()

	input := "localhost,example.org{unknown=::1},example.net{hostid6=192.0.2.1},example.com{hostid6=mac(bad)}"
	_, diagnostics, err := domainentry.Parse(input)

	require.Nil(t, err)
	require.Equal(t, []string{
		`invalid domain "localhost": not fully qualified`,
		`unknown domain field "unknown"`,
		`invalid hostid6 value "192.0.2.1": host-ID literal must be an unzoned IPv6 address`,
		`invalid hostid6 MAC address "bad": invalid 48-bit MAC address`,
	}, []string{
		diagnostics[0].Description(input),
		diagnostics[1].Description(input),
		diagnostics[2].Description(input),
		diagnostics[3].Description(input),
	})
}

func TestEntryDiagnosticDescriptionsForCommaKinds(t *testing.T) {
	t.Parallel()

	input := ",example.org example.net"
	_, diagnostics, err := domainentry.Parse(input)

	require.Nil(t, err)
	require.Len(t, diagnostics, 2)
	require.Equal(t, domainentry.KindExtraComma, diagnostics[0].Kind)
	require.Equal(t, "extra comma", diagnostics[0].Description(input))
	require.Equal(t, domainentry.KindMissingComma, diagnostics[1].Kind)
	require.Equal(t, "missing comma", diagnostics[1].Description(input))
}

func TestEntryDiagnosticDescriptionPanicsOnUnknownKind(t *testing.T) {
	t.Parallel()

	diagnostic := domainentry.Diagnostic{
		Span:   syntax.Span{Start: 0, End: 0},
		Kind:   domainentry.DiagnosticKind(-1),
		Detail: nil,
	}
	require.Panics(t, func() { _ = diagnostic.Description("") })
}
