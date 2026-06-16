package config_test

import (
	"bytes"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/config"
	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/domainentry"
	"github.com/favonia/cloudflare-ddns/internal/hostid6"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider"
	"github.com/favonia/cloudflare-ddns/internal/provider/protocol"
	"github.com/favonia/cloudflare-ddns/internal/syntax"
)

func mustEntries(t *testing.T, input string) []domainentry.Entry {
	t.Helper()

	entries, diagnostics, err := domainentry.Parse(input)
	require.Nil(t, err)
	require.Empty(t, diagnostics)
	return entries
}

func buildDomainConfig(t *testing.T, domains, ip4Domains, ip6Domains string, ppfmt pp.PP) (*config.BuiltConfig, bool) {
	t.Helper()

	raw := config.DefaultRaw()
	raw.Domains = mustEntries(t, domains)
	raw.IP4Domains = mustEntries(t, ip4Domains)
	raw.IP6Domains = mustEntries(t, ip6Domains)
	return raw.BuildConfig(ppfmt)
}

func hostID6SetStrings(set hostid6.Set) []string {
	values := set.Values()
	syntaxes := make([]string, 0, len(values))
	for _, value := range values {
		syntaxes = append(syntaxes, value.String())
	}
	return syntaxes
}

func TestBuildConfigMergesHostID6OpinionsAndDefaults(t *testing.T) {
	t.Parallel()

	built, ok := buildDomainConfig(t,
		"b.example{hostid6=[::2,::1]},a.example{},b.example{hostid6=[::1,::2]},v4.example",
		"v4-only.example",
		"a.example{hostid6=::3},v6-only.example",
		pp.NewSilent(),
	)
	require.True(t, ok)

	require.Equal(t,
		[]string{"a.example", "b.example", "v4-only.example", "v4.example"},
		summarizeDomains(built.Update.Domains[ipnet.IP4]),
	)
	require.Equal(t,
		[]string{"a.example", "b.example", "v4.example", "v6-only.example"},
		summarizeDomains(built.Update.Domains[ipnet.IP6]),
	)
	require.Equal(t, []string{"::3"}, hostID6SetStrings(built.Update.HostID6[domain.FQDN("a.example")]))
	require.Equal(t, []string{"::1", "::2"}, hostID6SetStrings(built.Update.HostID6[domain.FQDN("b.example")]))
	require.Equal(t, []string{"preserve"}, hostID6SetStrings(built.Update.HostID6[domain.FQDN("v4.example")]))
	require.Equal(t, []string{"preserve"}, hostID6SetStrings(built.Update.HostID6[domain.FQDN("v6-only.example")]))
	require.NotContains(t, built.Update.HostID6, domain.FQDN("v4-only.example"))
}

func TestBuildConfigAcceptsCanonicalEquivalentRepeatedHostID6Opinions(t *testing.T) {
	t.Parallel()

	built, ok := buildDomainConfig(t,
		"example.org{hostid6=[0:0::1,mac(00:11:22:33:44:AA)],hostid6=[::1,mac(00-11-22-33-44-aa)]},"+
			"example.org{hostid6=[mac(00-11-22-33-44-AA),::1]}",
		"",
		"",
		pp.NewSilent(),
	)
	require.True(t, ok)
	require.Equal(t,
		[]string{"::1", "mac(00-11-22-33-44-aa)"},
		hostID6SetStrings(built.Update.HostID6[domain.FQDN("example.org")]),
	)
}

func TestBuildConfigRejectsConflictingHostID6Opinions(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name       string
		domains    string
		ip6Domains string
		message    string
	}{
		{
			name:       "within declaration",
			domains:    "example.org{hostid6=[::1,::2],hostid6=[::2,::3]}",
			ip6Domains: "",
			message:    "Conflicting hostid6 settings for example.org: DOMAINS declaration 1 hostid6 assignment 1 configures [::1,::2], while DOMAINS declaration 1 hostid6 assignment 2 configures [::2,::3]; configure exactly the same hostid6 set in every declaration or omit it from partial declarations\n",
		},
		{
			name:       "within setting",
			domains:    "example.org{hostid6=preserve},example.org{hostid6=::1}",
			ip6Domains: "",
			message:    "Conflicting hostid6 settings for example.org: DOMAINS declaration 1 hostid6 assignment 1 configures [preserve], while DOMAINS declaration 2 hostid6 assignment 1 configures [::1]; configure exactly the same hostid6 set in every declaration or omit it from partial declarations\n",
		},
		{
			name:       "across settings",
			domains:    "example.org{hostid6=[::1,::2]}",
			ip6Domains: "example.org{hostid6=[::2,::3]}",
			message:    "Conflicting hostid6 settings for example.org: DOMAINS declaration 1 hostid6 assignment 1 configures [::1,::2], while IP6_DOMAINS declaration 1 hostid6 assignment 1 configures [::2,::3]; configure exactly the same hostid6 set in every declaration or omit it from partial declarations\n",
		},
		{
			name:       "intentional literal and MAC identities differ",
			domains:    "example.org{hostid6=::211:22ff:fe33:4455},example.org{hostid6=mac(00-11-22-33-44-55)}",
			ip6Domains: "",
			message:    "Conflicting hostid6 settings for example.org: DOMAINS declaration 1 hostid6 assignment 1 configures [::211:22ff:fe33:4455], while DOMAINS declaration 2 hostid6 assignment 1 configures [mac(00-11-22-33-44-55)]; configure exactly the same hostid6 set in every declaration or omit it from partial declarations\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var output bytes.Buffer
			built, ok := buildDomainConfig(t, tc.domains, "", tc.ip6Domains, pp.New(&output, false, pp.Quiet))
			require.False(t, ok)
			require.Nil(t, built)
			require.Equal(t, tc.message, output.String())
		})
	}
}

func TestBuildConfigRejectsZeroExplicitHostID6OpinionAsImpossible(t *testing.T) {
	t.Parallel()

	raw := config.DefaultRaw()
	raw.Domains = []domainentry.Entry{{
		Domain:          domain.FQDN("example.org"),
		HostID6Opinions: []hostid6.Set{{}},
		Span:            syntax.Span{Start: 0, End: 0},
	}}

	var output bytes.Buffer
	built, ok := raw.BuildConfig(pp.New(&output, false, pp.Quiet))
	require.False(t, ok)
	require.Nil(t, built)
	require.Equal(t,
		"DOMAINS declaration 1 hostid6 assignment 1 for example.org contains an empty host-ID set; "+
			"this should not happen. Please report it at "+pp.IssueReportingURL+"\n",
		output.String(),
	)
}

func TestBuildConfigWarnsOncePerSuspiciousMAC(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	built, ok := buildDomainConfig(t,
		"z.example{hostid6=mac(01:11:22:33:44:55)},a.example{hostid6=[mac(01-11-22-33-44-55),mac(00-00-00-00-00-00),mac(ff-ff-ff-ff-ff-ff),mac(02-11-22-33-44-55)]}",
		"",
		"z.example{hostid6=mac(01-11-22-33-44-55)}",
		pp.New(&output, false, pp.Quiet),
	)
	require.True(t, ok)
	require.NotNil(t, built)
	require.Equal(t,
		"hostid6=mac(00-00-00-00-00-00) for a.example uses the all-zero MAC address, which commonly represents an unset, placeholder, deliberately configured, or broken identity\n"+
			"hostid6=mac(01-11-22-33-44-55) for a.example and z.example uses a group MAC address; the derived IPv6 address is still unicast, but this MAC may not uniquely identify the intended host\n"+
			"hostid6=mac(ff-ff-ff-ff-ff-ff) for a.example uses the Ethernet broadcast destination and cannot identify one host\n",
		output.String(),
	)
}

func TestBuildConfigRejectsKnownIncompatibleIPv6RawData(t *testing.T) {
	t.Parallel()

	raw := config.DefaultRaw()
	raw.Provider[ipnet.IP4] = nil
	raw.Provider[ipnet.IP6] = provider.MustNewStatic(ipnet.IP6, 64, "2001:db8::1/65,2001:db8:1::1/65")
	raw.Domains = mustEntries(t,
		"alpha.example{hostid6=[2001::1,mac(00-11-22-33-44-55)]},"+
			"beta.example{hostid6=[2001::1,mac(aa-bb-cc-dd-ee-ff)]}",
	)

	var output bytes.Buffer
	built, ok := raw.BuildConfig(pp.New(&output, false, pp.Quiet))

	require.False(t, ok)
	require.Nil(t, built)
	require.Equal(t,
		"IP6_PROVIDER=static:2001:db8::1/65,2001:db8:1::1/65 is incompatible with hostid6=2001::1 "+
			"for alpha.example and beta.example: requires prefixes no longer than /2, but includes "+
			"2001:db8::1/65 and 2001:db8:1::1/65; change the listed hostid6 setting or IP6_PROVIDER\n"+
			"IP6_PROVIDER=static:2001:db8::1/65,2001:db8:1::1/65 is incompatible with "+
			"hostid6=[mac(00-11-22-33-44-55),mac(aa-bb-cc-dd-ee-ff)] for alpha.example and beta.example: "+
			"requires prefixes no longer than /64, but includes 2001:db8::1/65 and 2001:db8:1::1/65; "+
			"change the listed hostid6 setting or IP6_PROVIDER\n",
		output.String(),
	)
}

func TestBuildConfigRejectsKnownMACHostIDUnderShortPrefix(t *testing.T) {
	t.Parallel()

	raw := config.DefaultRaw()
	raw.Provider[ipnet.IP4] = nil
	raw.Provider[ipnet.IP6] = provider.MustNewStatic(ipnet.IP6, 64, "2001:db8::1/56")
	raw.Domains = mustEntries(t, "example.org{hostid6=mac(00-11-22-33-44-55)}")

	var output bytes.Buffer
	built, ok := raw.BuildConfig(pp.New(&output, false, pp.Quiet))

	require.False(t, ok)
	require.Nil(t, built)
	require.Equal(t,
		"IP6_PROVIDER=static:2001:db8::1/56 is incompatible with hostid6=mac(00-11-22-33-44-55) for example.org: "+
			"requires a /64 prefix, but includes 2001:db8::1/56; change the listed hostid6 setting or IP6_PROVIDER\n"+
			"Modified EUI-64 host IDs are only defined within a /64 prefix. "+
			"Assuming the subnet bits are all zero, mac(00-11-22-33-44-55) gives ::211:22ff:fe33:4455; "+
			"look up the subnet bits between your prefix and /64 (often zero on a single-subnet network), "+
			"prepend them, and use the result as a literal hostid6 until shorter prefixes are supported. "+
			"Please open an issue at "+pp.IssueReportingURL+" if you need this\n",
		output.String(),
	)
}

func TestBuildConfigAcceptsIPv6RawDataWithoutProvableIncompatibility(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		provider provider.Provider
	}{
		{
			name:     "compatible-static",
			provider: provider.MustNewStatic(ipnet.IP6, 64, "2001:db8::1/64"),
		},
		{
			name:     "static-empty",
			provider: provider.NewStaticEmpty(),
		},
		{
			name:     "dynamic",
			provider: provider.NewCloudflareTrace(),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			raw := config.DefaultRaw()
			raw.Provider[ipnet.IP4] = nil
			raw.Provider[ipnet.IP6] = tc.provider
			raw.Domains = mustEntries(t, "example.org{hostid6=mac(00-11-22-33-44-55)}")

			built, ok := raw.BuildConfig(pp.NewSilent())

			require.True(t, ok)
			require.NotNil(t, built)
		})
	}
}

func TestBuildConfigOrdersKnownIPv6IncompatibilitiesDeterministically(t *testing.T) {
	t.Parallel()

	raw := config.DefaultRaw()
	raw.Provider[ipnet.IP4] = nil
	raw.Provider[ipnet.IP6] = provider.MustNewStatic(ipnet.IP6, 64, "2001:db8::1/128")
	raw.Domains = mustEntries(t, "example.org{hostid6=[::1,::2]}")

	var output bytes.Buffer
	built, ok := raw.BuildConfig(pp.New(&output, false, pp.Quiet))

	require.False(t, ok)
	require.Nil(t, built)
	require.Equal(t,
		"IP6_PROVIDER=static:2001:db8::1/128 is incompatible with hostid6=::2 for example.org: "+
			"requires prefixes no longer than /126, but includes 2001:db8::1/128; "+
			"change the listed hostid6 setting or IP6_PROVIDER\n"+
			"IP6_PROVIDER=static:2001:db8::1/128 is incompatible with hostid6=::1 for example.org: "+
			"requires prefixes no longer than /127, but includes 2001:db8::1/128; "+
			"change the listed hostid6 setting or IP6_PROVIDER\n",
		output.String(),
	)
}

func TestBuildConfigRejectsBrokenKnownIPv6RawDataWithoutPanicking(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		entries  []ipnet.RawEntry
		expected string
	}{
		{
			name:     "invalid",
			entries:  []ipnet.RawEntry{{}},
			expected: "IP6_PROVIDER=broken exposed invalid configuration-time known raw data; this should not happen. Please report it at " + pp.IssueReportingURL + "\n",
		},
		{
			name:     "wrong-family",
			entries:  []ipnet.RawEntry{ipnet.RawEntryFrom(netip.MustParseAddr("192.0.2.1"), 32)},
			expected: "IP6_PROVIDER=broken exposed configuration-time known raw data 192.0.2.1/32 that is not valid IPv6; this should not happen. Please report it at " + pp.IssueReportingURL + "\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			raw := config.DefaultRaw()
			raw.Provider[ipnet.IP4] = nil
			raw.Provider[ipnet.IP6] = protocol.Static{ProviderName: "broken", RawEntries: tc.entries}
			raw.Domains = mustEntries(t, "example.org{hostid6=::1}")

			var output bytes.Buffer
			require.NotPanics(t, func() {
				built, ok := raw.BuildConfig(pp.New(&output, false, pp.Quiet))
				require.False(t, ok)
				require.Nil(t, built)
			})
			require.Equal(t, tc.expected, output.String())
		})
	}
}
