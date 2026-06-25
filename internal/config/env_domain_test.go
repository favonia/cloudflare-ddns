//nolint:testpackage // These tests cover the unexported domain reader directly because they validate helper behavior.
package config

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/domainentry"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/syntax"
)

func oldEntry() domainentry.Entry {
	return domainentry.Entry{
		Domain:          domain.FQDN("old.example"),
		HostID6Opinions: nil,
		Span:            syntax.Span{Start: 0, End: 0},
	}
}

func family(value ipnet.Family) *ipnet.Family {
	return new(value)
}

//nolint:paralleltest // environment vars are global
func TestReadDomainsPlainLists(t *testing.T) {
	key := keyPrefix + "DOMAINS"
	for name, tc := range map[string]struct {
		set      bool
		value    string
		oldField []domainentry.Entry
		expected []domainentry.Entry
	}{
		"nil": {
			set:      false,
			value:    "",
			oldField: []domainentry.Entry{oldEntry()},
			expected: nil,
		},
		"empty": {
			set:      true,
			value:    "",
			oldField: []domainentry.Entry{oldEntry()},
			expected: nil,
		},
		"plain": {
			set:      true,
			value:    " 書.org ,  Bücher.org  ",
			oldField: []domainentry.Entry{oldEntry()},
			expected: []domainentry.Entry{
				{Domain: domain.FQDN("xn--rov.org"), HostID6Opinions: nil, Span: syntax.Span{Start: 0, End: 7}},
				{Domain: domain.FQDN("xn--bcher-kva.org"), HostID6Opinions: nil, Span: syntax.Span{Start: 11, End: 22}},
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			set(t, key, tc.set, tc.value)
			field := tc.oldField
			mockPP := mocks.NewMockPP(gomock.NewController(t))

			ok := readDomains(mockPP, key, nil, &field)

			require.True(t, ok)
			require.Equal(t, tc.expected, field)
		})
	}
}

//nolint:paralleltest // environment vars are global
func TestReadDomainsAcceptsHostID6ForMixedAndIPv6Settings(t *testing.T) {
	for _, tc := range []struct {
		key    string
		family *ipnet.Family
	}{
		{key: "DOMAINS", family: nil},
		{key: "IP6_DOMAINS", family: family(ipnet.IP6)},
	} {
		t.Run(tc.key, func(t *testing.T) {
			store(t, tc.key, "example.org{hostid6=::1}")
			var field []domainentry.Entry
			mockPP := mocks.NewMockPP(gomock.NewController(t))

			ok := readDomains(mockPP, tc.key, tc.family, &field)

			require.True(t, ok)
			require.Len(t, field, 1)
			require.Equal(t, domain.FQDN("example.org"), field[0].Domain)
			require.Len(t, field[0].HostID6Opinions, 1)
		})
	}
}

//nolint:paralleltest // environment vars are global
func TestReadDomainsAcceptsPlainIPv4Entry(t *testing.T) {
	store(t, "IP4_DOMAINS", "example.org")
	var field []domainentry.Entry
	mockPP := mocks.NewMockPP(gomock.NewController(t))

	ok := readDomains(mockPP, "IP4_DOMAINS", family(ipnet.IP4), &field)

	require.True(t, ok)
	require.Equal(t, []domainentry.Entry{{
		Domain:          domain.FQDN("example.org"),
		HostID6Opinions: nil,
		Span:            syntax.Span{Start: 0, End: 11},
	}}, field)
}

//nolint:paralleltest // environment vars are global
func TestReadDomainsRejectsHostID6ForIPv4Setting(t *testing.T) {
	const value = "example.org{hostid6=::1}"
	store(t, "IP4_DOMAINS", value)
	oldField := []domainentry.Entry{oldEntry()}
	field := oldField
	mockPP := mocks.NewMockPP(gomock.NewController(t))
	mockPP.EXPECT().Noticef(
		pp.EmojiUserError,
		`%s (%q) configures hostid6 for %s, but hostid6 only affects IPv6; remove hostid6 from this %s entry, or configure the IPv6 entry in DOMAINS or IP6_DOMAINS`,
		"IP4_DOMAINS", value, "example.org",
		"IP4_DOMAINS",
	)

	ok := readDomains(mockPP, "IP4_DOMAINS", family(ipnet.IP4), &field)

	require.False(t, ok)
	require.Equal(t, oldField, field)
}

//nolint:paralleltest // environment vars are global
func TestReadDomainsReportsSemanticDiagnosticsInSourceOrder(t *testing.T) {
	const value = "localhost,good.example,example.org{unknown=::1},example.net{hostid6=192.0.2.1},example.com{hostid6=mac(bad)}"
	store(t, "DOMAINS", value)
	oldField := []domainentry.Entry{oldEntry()}
	field := oldField
	mockPP := mocks.NewMockPP(gomock.NewController(t))
	gomock.InOrder(
		mockPP.EXPECT().Noticef(pp.EmojiUserError, `%s (%q) has %s`, "DOMAINS", value, `invalid domain "localhost": too few labels`),
		mockPP.EXPECT().Noticef(pp.EmojiUserError, `%s (%q) has %s`, "DOMAINS", value, `unknown domain field "unknown"`),
		mockPP.EXPECT().Noticef(pp.EmojiUserError, `%s (%q) has %s`, "DOMAINS", value, `invalid hostid6 value "192.0.2.1": host-ID literal must be an unzoned IPv6 address`),
		mockPP.EXPECT().Noticef(pp.EmojiUserError, `%s (%q) has %s`, "DOMAINS", value, `invalid hostid6 MAC address "bad": invalid 48-bit MAC address`),
	)

	ok := readDomains(mockPP, "DOMAINS", nil, &field)

	require.False(t, ok)
	require.Equal(t, oldField, field)
}

//nolint:paralleltest // environment vars are global
func TestReadDomainsReportsCompatibilityWarningsBeforeLaterRecoveredSemanticError(t *testing.T) {
	const value = ",good.example bad.example,localhost"
	store(t, "DOMAINS", value)
	var field []domainentry.Entry
	mockPP := mocks.NewMockPP(gomock.NewController(t))
	gomock.InOrder(
		mockPP.EXPECT().Noticef(pp.EmojiUserWarning, `%s (%s) contains extra commas; this is accepted for now but will be rejected in version 2.0.0`, "DOMAINS", `",good.example bad.example,localhost"`),
		mockPP.EXPECT().Noticef(pp.EmojiUserWarning, `%s (%s) is missing commas; this is accepted for now but will be rejected in version 2.0.0`, "DOMAINS", `",good.example bad.example,localhost"`),
		mockPP.EXPECT().Noticef(pp.EmojiUserError, `%s (%q) has %s`, "DOMAINS", value, `invalid domain "localhost": too few labels`),
	)

	ok := readDomains(mockPP, "DOMAINS", nil, &field)

	require.False(t, ok)
	require.Nil(t, field)
}

//nolint:paralleltest // environment vars are global
func TestReadDomainsReportsExtraTrailingCommasForVersion2(t *testing.T) {
	const value = "example.org,,,,,,"
	store(t, "DOMAINS", value)
	var field []domainentry.Entry
	mockPP := mocks.NewMockPP(gomock.NewController(t))
	mockPP.EXPECT().Noticef(
		pp.EmojiUserWarning,
		`%s (%s) contains extra commas; this is accepted for now but will be rejected in version 2.0.0`,
		"DOMAINS", `"`+value+`"`,
	)

	ok := readDomains(mockPP, "DOMAINS", nil, &field)

	require.True(t, ok)
	require.Len(t, field, 1)
	require.Equal(t, domain.FQDN("example.org"), field[0].Domain)
}

//nolint:paralleltest // environment vars are global
func TestReadDomainsReportsMalformedEntryWithoutParserFormIDs(t *testing.T) {
	for _, value := range []string{
		"example.org{hostid6=[::1,,::2]}",
		"example.org{hostid6=::1,,hostid6=::2}",
		"example.org{hostid6=[::1,,,,,,]}",
		"example.org{hostid6=::1,,,,,,}",
	} {
		t.Run(value, func(t *testing.T) {
			store(t, "DOMAINS", value)
			oldField := []domainentry.Entry{oldEntry()}
			field := oldField
			mockPP := mocks.NewMockPP(gomock.NewController(t))
			mockPP.EXPECT().Noticef(
				pp.EmojiUserError,
				`%s (%q) has unexpected token %q`,
				"DOMAINS", value, ",",
			)

			ok := readDomains(mockPP, "DOMAINS", nil, &field)

			require.False(t, ok)
			require.Equal(t, oldField, field)
		})
	}
}

//nolint:paralleltest // environment vars are global
func TestReadDomainsReportsStructuredEntryParseErrors(t *testing.T) {
	for _, tc := range []struct {
		name       string
		value      string
		prepareLog func(*mocks.MockPP)
	}{
		{
			name:  "unexpected token",
			value: "example.org{hostid6=mac)}",
			prepareLog: func(mockPP *mocks.MockPP) {
				mockPP.EXPECT().Noticef(
					pp.EmojiUserError,
					`%s (%q) has unexpected token %q when %q is expected`,
					"DOMAINS", "example.org{hostid6=mac)}", ")", "(",
				)
			},
		},
		{
			name:  "missing token",
			value: "example.org{hostid6=::1",
			prepareLog: func(mockPP *mocks.MockPP) {
				mockPP.EXPECT().Noticef(
					pp.EmojiUserError,
					`%s (%q) is missing %q at the end`,
					"DOMAINS", "example.org{hostid6=::1", "}",
				)
			},
		},
		{
			name:  "malformed",
			value: string([]byte{0x80}),
			prepareLog: func(mockPP *mocks.MockPP) {
				mockPP.EXPECT().Noticef(
					pp.EmojiUserError,
					"%s (%q) is malformed: %v",
					"DOMAINS", string([]byte{0x80}), syntax.ErrInvalidUTF8,
				)
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store(t, "DOMAINS", tc.value)
			oldField := []domainentry.Entry{oldEntry()}
			field := oldField
			mockPP := mocks.NewMockPP(gomock.NewController(t))
			tc.prepareLog(mockPP)

			ok := readDomains(mockPP, "DOMAINS", nil, &field)

			require.False(t, ok)
			require.Equal(t, oldField, field)
		})
	}
}
