package domainexp_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/domainexp"
	"github.com/favonia/cloudflare-ddns/internal/hostid6"
	"github.com/favonia/cloudflare-ddns/internal/syntax"
)

func describeSet(set hostid6.Set) []string {
	values := set.Values()
	descriptions := make([]string, len(values))
	for i, value := range values {
		descriptions[i] = value.Describe()
	}
	return descriptions
}

func TestParseEntriesPlainEntryExactSpan(t *testing.T) {
	t.Parallel()

	entries, diagnostics, err := domainexp.ParseEntries("example.org")

	require.Nil(t, err)
	require.Empty(t, diagnostics)
	require.Equal(t, []domainexp.Entry{{
		Domain:          domain.FQDN("example.org"),
		HostID6Opinions: nil,
		Span:            syntax.Span{Start: 0, End: 11},
	}}, entries)
}

func TestParseEntriesDomainsAndSpans(t *testing.T) {
	t.Parallel()

	entries, diagnostics, err := domainexp.ParseEntries("faß.de,*.☕.de")

	require.Nil(t, err)
	require.Empty(t, diagnostics)
	require.Equal(t, []domainexp.Entry{
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
	entries, diagnostics, err := domainexp.ParseEntries(input)

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

func TestParseEntriesSemanticDiagnosticsAndRecovery(t *testing.T) {
	t.Parallel()

	input := "localhost,good.example,example.org{unknown=::1},example.net{hostid6=192.0.2.1},example.com{hostid6=mac(bad)}"
	entries, diagnostics, err := domainexp.ParseEntries(input)

	require.Nil(t, err)
	require.Equal(t, []domainexp.Entry{{
		Domain:          domain.FQDN("good.example"),
		HostID6Opinions: nil,
		Span:            syntax.Span{Start: 10, End: 22},
	}}, entries)
	require.Len(t, diagnostics, 4)
	require.Equal(t, syntax.Span{Start: 0, End: 9}, diagnostics[0].Span)
	require.ErrorIs(t, diagnostics[0].Cause, domainexp.ErrInvalidDomain)
	require.Equal(t, syntax.Span{Start: 35, End: 42}, diagnostics[1].Span)
	require.ErrorIs(t, diagnostics[1].Cause, domainexp.ErrUnknownDomainField)
	require.Equal(t, syntax.Span{Start: 68, End: 77}, diagnostics[2].Span)
	require.ErrorIs(t, diagnostics[2].Cause, domainexp.ErrInvalidHostID6)
	require.Equal(t, syntax.Span{Start: 103, End: 106}, diagnostics[3].Span)
	require.ErrorIs(t, diagnostics[3].Cause, domainexp.ErrInvalidMAC)
}

func TestParseEntriesRejectsQuotedCommaList(t *testing.T) {
	t.Parallel()

	entries, diagnostics, err := domainexp.ParseEntries(`example.org{hostid6="::1,::2"}`)

	require.Nil(t, entries)
	require.Empty(t, diagnostics)
	require.NotNil(t, err)
	require.Equal(t, syntax.Span{Start: 25, End: 29}, err.Span)
}

func TestParseEntriesStopsOnMalformedNesting(t *testing.T) {
	t.Parallel()

	entries, diagnostics, err := domainexp.ParseEntries("localhost,example.org{hostid6=[::1,,::2]},good.example")

	require.Nil(t, entries)
	require.Empty(t, diagnostics)
	require.NotNil(t, err)
	require.Equal(t, syntax.Span{Start: 35, End: 36}, err.Span)
}

func TestParseEntriesReturnsCompatibilityDiagnostics(t *testing.T) {
	t.Parallel()

	entries, diagnostics, err := domainexp.ParseEntries(",example.org example.net,,")

	require.Nil(t, err)
	require.Equal(t, []domainexp.Entry{
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
	require.ErrorIs(t, diagnostics[0].Cause, domainexp.ErrExtraComma)
	require.Equal(t, syntax.Span{Start: 12, End: 13}, diagnostics[1].Span)
	require.ErrorIs(t, diagnostics[1].Cause, domainexp.ErrMissingComma)
}

func TestParseEntriesManyLeadingCommasReturnOneDiagnostic(t *testing.T) {
	t.Parallel()

	const commaCount = 4096
	entries, diagnostics, err := domainexp.ParseEntries(strings.Repeat(",", commaCount) + "example.org")

	require.Nil(t, err)
	require.Equal(t, []domainexp.Entry{{
		Domain:          domain.FQDN("example.org"),
		HostID6Opinions: nil,
		Span:            syntax.Span{Start: commaCount, End: commaCount + len("example.org")},
	}}, entries)
	require.Equal(t, []domainexp.EntryDiagnostic{{
		Span:  syntax.Span{Start: 0, End: 1},
		Cause: domainexp.ErrExtraComma,
	}}, diagnostics)
}

func TestParseEntriesAcceptsTrailingCommaWithoutDiagnostic(t *testing.T) {
	t.Parallel()

	entries, diagnostics, err := domainexp.ParseEntries("example.org,")

	require.Nil(t, err)
	require.Empty(t, diagnostics)
	require.Equal(t, []domainexp.Entry{{
		Domain:          domain.FQDN("example.org"),
		HostID6Opinions: nil,
		Span:            syntax.Span{Start: 0, End: 11},
	}}, entries)
}

func TestParseEntriesTypedCauses(t *testing.T) {
	t.Parallel()

	for _, cause := range []error{
		domainexp.ErrInvalidDomain,
		domainexp.ErrUnknownDomainField,
		domainexp.ErrInvalidHostID6,
		domainexp.ErrInvalidMAC,
		domainexp.ErrExtraComma,
		domainexp.ErrMissingComma,
	} {
		require.NotErrorIs(t, cause, syntax.ErrUnexpectedToken)
	}
}

func TestEntryDiagnosticDescriptions(t *testing.T) {
	t.Parallel()

	input := "localhost,example.org{unknown=::1},example.net{hostid6=192.0.2.1},example.com{hostid6=mac(bad)}"
	_, diagnostics, err := domainexp.ParseEntries(input)

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
