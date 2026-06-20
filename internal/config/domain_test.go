package config_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/config"
	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/domainentry"
	"github.com/favonia/cloudflare-ddns/internal/hostid6"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider"
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

func TestBuildConfigEmitsExperimentalNoticeForExplicitHostID6(t *testing.T) {
	t.Parallel()

	raw := config.DefaultRaw()
	raw.Domains = mustEntries(t, "example.org{hostid6=::1}")

	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)
	mockPP.EXPECT().IsShowing(pp.Info).Return(false)
	mockPP.EXPECT().InfoOncef(
		pp.MessageExperimentalHostID6,
		pp.EmojiExperimental,
		`You are using the experimental "hostid6" domain field for IPv6 DNS (unreleased)`,
	)

	built, ok := raw.BuildConfig(mockPP)

	require.True(t, ok)
	require.NotNil(t, built)
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
			message:    `Conflicting hostid6 settings for example.org: the same DOMAINS entry has "hostid6=[::1,::2]" and "hostid6=[::2,::3]"; use only one hostid6 assignment, or make the assignments identical` + "\n",
		},
		{
			name:       "within setting",
			domains:    "example.org{hostid6=preserve},example.org{hostid6=::1}",
			ip6Domains: "",
			message:    `Conflicting hostid6 settings for example.org: DOMAINS has "hostid6=preserve" and also "hostid6=::1"; use the same hostid6 set everywhere example.org configures hostid6, or remove the extra hostid6 assignment` + "\n",
		},
		{
			name:       "across settings",
			domains:    "example.org{hostid6=[::1,::2]}",
			ip6Domains: "example.org{hostid6=[::2,::3]}",
			message:    `Conflicting hostid6 settings for example.org: DOMAINS has "hostid6=[::1,::2]", but IP6_DOMAINS has "hostid6=[::2,::3]"; use the same hostid6 set everywhere example.org configures hostid6, or remove the extra hostid6 assignment` + "\n",
		},
		{
			name:       "intentional literal and MAC identities differ",
			domains:    "example.org{hostid6=::211:22ff:fe33:4455},example.org{hostid6=mac(00-11-22-33-44-55)}",
			ip6Domains: "",
			message:    `Conflicting hostid6 settings for example.org: DOMAINS has "hostid6=::211:22ff:fe33:4455" and also "hostid6=mac(00-11-22-33-44-55)"; use the same hostid6 set everywhere example.org configures hostid6, or remove the extra hostid6 assignment` + "\n",
		},
		{
			name:       "preserves source snippets",
			domains:    "example.org{hostid6=[0:0::1,mac(00:11:22:33:44:55)]},example.org{hostid6=[::2,mac(00-11-22-33-44-55)]}",
			ip6Domains: "",
			message:    `Conflicting hostid6 settings for example.org: DOMAINS has "hostid6=[0:0::1,mac(00:11:22:33:44:55)]" and also "hostid6=[::2,mac(00-11-22-33-44-55)]"; use the same hostid6 set everywhere example.org configures hostid6, or remove the extra hostid6 assignment` + "\n",
		},
		{
			name:       "quotes source snippets with newlines",
			domains:    "example.org{hostid6 = [ ::1 ,\n ::2 ]},example.org{hostid6 = ::3}",
			ip6Domains: "",
			message:    "Conflicting hostid6 settings for example.org: DOMAINS has \"hostid6 = [ ::1 ,\\n ::2 ]\" and also \"hostid6 = ::3\"; use the same hostid6 set everywhere example.org configures hostid6, or remove the extra hostid6 assignment\n",
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
		Domain: domain.FQDN("example.org"),
		HostID6Opinions: []domainentry.HostID6Opinion{{
			Set:           hostid6.Set{},
			SourceSnippet: "",
		}},
		Span: syntax.Span{Start: 0, End: 0},
	}}

	var output bytes.Buffer
	built, ok := raw.BuildConfig(pp.New(&output, false, pp.Quiet))
	require.False(t, ok)
	require.Nil(t, built)
	require.Equal(t,
		"An internal error produced an empty hostid6 set for example.org in DOMAINS; "+
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
		"hostid6=mac(00-00-00-00-00-00) for a.example uses the all-zero MAC address; check whether this is the MAC address you intended to use\n"+
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
		"IP6_PROVIDER=static:2001:db8::1/65,2001:db8:1::1/65 cannot be used for alpha.example and beta.example "+
			"with hostid6=2001::1: it requires prefixes no longer than /2, but the provider includes "+
			"2001:db8::1/65 and 2001:db8:1::1/65; change IP6_PROVIDER or that hostid6 setting\n"+
			"IP6_PROVIDER=static:2001:db8::1/65,2001:db8:1::1/65 cannot be used for alpha.example and beta.example "+
			"with hostid6=[mac(00-11-22-33-44-55),mac(aa-bb-cc-dd-ee-ff)]: it requires prefixes no longer than /64, "+
			"but the provider includes 2001:db8::1/65 and 2001:db8:1::1/65; change IP6_PROVIDER or that hostid6 setting\n",
		output.String(),
	)
}

func TestBuildConfigRejectsKnownMACHostIDUnderShortPrefix(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		hostID6  string
		expected string
	}{
		{
			name:    "single-mac",
			hostID6: "mac(00-11-22-33-44-55)",
			expected: "IP6_PROVIDER=static:2001:db8::1/56 cannot be used for example.org " +
				"with hostid6=mac(00-11-22-33-44-55): " +
				"it requires a /64 prefix, but the provider includes 2001:db8::1/56; change IP6_PROVIDER or that hostid6 setting\n" +
				"MAC-based host IDs require a /64 prefix. For 2001:db8::1/56, look up the subnet bits between /56 and /64; " +
				"the MAC-derived interface identifier is ::211:22ff:fe33:4455. If those subnet bits are zero, use " +
				"hostid6=::211:22ff:fe33:4455. If they are not zero, insert them into the hostid6 literal before the " +
				"interface identifier. Please open an issue at " + pp.IssueReportingURL + " if you need direct MAC support for shorter prefixes\n",
		},
		{
			name:    "multiple-macs",
			hostID6: "[mac(00-11-22-33-44-55),mac(aa-bb-cc-dd-ee-ff)]",
			expected: "IP6_PROVIDER=static:2001:db8::1/56 cannot be used for example.org " +
				"with hostid6=[mac(00-11-22-33-44-55),mac(aa-bb-cc-dd-ee-ff)]: " +
				"it requires a /64 prefix, but the provider includes 2001:db8::1/56; change IP6_PROVIDER or that hostid6 setting\n" +
				"MAC-based host IDs require a /64 prefix. For 2001:db8::1/56, look up the subnet bits between /56 and /64; " +
				"the MAC-derived interface identifiers are ::211:22ff:fe33:4455 and ::a8bb:ccff:fedd:eeff. " +
				"If those subnet bits are zero, use hostid6=[::211:22ff:fe33:4455,::a8bb:ccff:fedd:eeff]. " +
				"If they are not zero, insert them into the hostid6 literal before the interface identifier. " +
				"Please open an issue at " + pp.IssueReportingURL + " if you need direct MAC support for shorter prefixes\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			raw := config.DefaultRaw()
			raw.Provider[ipnet.IP4] = nil
			raw.Provider[ipnet.IP6] = provider.MustNewStatic(ipnet.IP6, 64, "2001:db8::1/56")
			raw.Domains = mustEntries(t, "example.org{hostid6="+tc.hostID6+"}")

			var output bytes.Buffer
			built, ok := raw.BuildConfig(pp.New(&output, false, pp.Quiet))

			require.False(t, ok)
			require.Nil(t, built)
			require.Equal(t, tc.expected, output.String())
		})
	}
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
		"IP6_PROVIDER=static:2001:db8::1/128 cannot be used for example.org with hostid6=::2: "+
			"it requires prefixes no longer than /126, but the provider includes 2001:db8::1/128; "+
			"change IP6_PROVIDER or that hostid6 setting\n"+
			"IP6_PROVIDER=static:2001:db8::1/128 cannot be used for example.org with hostid6=::1: "+
			"it requires prefixes no longer than /127, but the provider includes 2001:db8::1/128; "+
			"change IP6_PROVIDER or that hostid6 setting\n",
		output.String(),
	)
}

func TestBuildConfigSuppressesSuspiciousMACWarningForIgnoredHostID6(t *testing.T) {
	t.Parallel()

	raw := config.DefaultRaw()
	raw.Provider = map[ipnet.Family]provider.Provider{ipnet.IP4: provider.NewCloudflareTrace()}
	raw.IP4Domains = mustEntries(t, "keep.example")
	raw.Domains = mustEntries(t, "keep.example{hostid6=mac(00-00-00-00-00-00)}")

	var output bytes.Buffer
	built, ok := raw.BuildConfig(pp.New(&output, false, pp.Quiet))
	require.True(t, ok)
	require.NotNil(t, built)
	require.Equal(t,
		"hostid6=mac(00-00-00-00-00-00) for keep.example is ignored because IPv6 is disabled\n",
		output.String(),
	)
}

func TestBuildConfigWarnsIP6DomainsMembershipShadowedByDisabledIP6(t *testing.T) {
	t.Parallel()

	raw := config.DefaultRaw()
	raw.Provider = map[ipnet.Family]provider.Provider{ipnet.IP4: provider.NewCloudflareTrace()}
	raw.IP4Domains = mustEntries(t, "x.example")
	raw.IP6Domains = mustEntries(t, "x.example")

	var output bytes.Buffer
	built, ok := raw.BuildConfig(pp.New(&output, false, pp.Quiet))
	require.True(t, ok)
	require.NotNil(t, built)
	require.Equal(t,
		"The IP6_DOMAINS listing of x.example is ignored because IPv6 is disabled\n",
		output.String(),
	)
}

func TestBuildConfigWarnsIP4DomainsMembershipShadowedByDisabledIP4(t *testing.T) {
	t.Parallel()

	raw := config.DefaultRaw()
	raw.Provider = map[ipnet.Family]provider.Provider{ipnet.IP6: provider.NewCloudflareTrace()}
	raw.IP4Domains = mustEntries(t, "x.example")
	raw.IP6Domains = mustEntries(t, "x.example")

	var output bytes.Buffer
	built, ok := raw.BuildConfig(pp.New(&output, false, pp.Quiet))
	require.True(t, ok)
	require.NotNil(t, built)
	require.Equal(t,
		"The IP4_DOMAINS listing of x.example is ignored because IPv4 is disabled\n",
		output.String(),
	)
}

func TestBuildConfigGroupsMultipleIP6DomainsMembershipsShadowedByDisabledIP6(t *testing.T) {
	t.Parallel()

	raw := config.DefaultRaw()
	raw.Provider = map[ipnet.Family]provider.Provider{ipnet.IP4: provider.NewCloudflareTrace()}
	raw.IP4Domains = mustEntries(t, "a.example,b.example,c.example")
	raw.IP6Domains = mustEntries(t, "a.example,b.example,c.example")

	var output bytes.Buffer
	built, ok := raw.BuildConfig(pp.New(&output, false, pp.Quiet))
	require.True(t, ok)
	require.NotNil(t, built)
	require.Equal(t,
		"The IP6_DOMAINS listing of a.example, b.example, and c.example is ignored because IPv6 is disabled\n",
		output.String(),
	)
}

func TestBuildConfigWarnsExplicitHostID6ShadowedByDisabledIP6(t *testing.T) {
	t.Parallel()

	raw := config.DefaultRaw()
	raw.Provider = map[ipnet.Family]provider.Provider{ipnet.IP4: provider.NewCloudflareTrace()}
	raw.Domains = mustEntries(t, "x.example{hostid6=mac(00-11-22-33-44-55)}")

	var output bytes.Buffer
	built, ok := raw.BuildConfig(pp.New(&output, false, pp.Quiet))
	require.True(t, ok)
	require.NotNil(t, built)
	require.Equal(t,
		"hostid6=mac(00-11-22-33-44-55) for x.example is ignored because IPv6 is disabled\n",
		output.String(),
	)
}

func TestBuildConfigGroupsHostID6BySetValueShadowedByDisabledIP6(t *testing.T) {
	t.Parallel()

	raw := config.DefaultRaw()
	raw.Provider = map[ipnet.Family]provider.Provider{ipnet.IP4: provider.NewCloudflareTrace()}
	raw.Domains = mustEntries(t,
		"a.example{hostid6=mac(00-11-22-33-44-55)},b.example{hostid6=mac(00-11-22-33-44-55)}",
	)

	var output bytes.Buffer
	built, ok := raw.BuildConfig(pp.New(&output, false, pp.Quiet))
	require.True(t, ok)
	require.NotNil(t, built)
	require.Equal(t,
		"hostid6=mac(00-11-22-33-44-55) for a.example and b.example is ignored because IPv6 is disabled\n",
		output.String(),
	)
}

func TestBuildConfigSuppressesHostID6WarningWhenDomainAlsoListedInIP6Domains(t *testing.T) {
	t.Parallel()

	raw := config.DefaultRaw()
	raw.Provider = map[ipnet.Family]provider.Provider{ipnet.IP4: provider.NewCloudflareTrace()}
	raw.Domains = mustEntries(t, "x.example{hostid6=mac(00-11-22-33-44-55)}")
	raw.IP6Domains = mustEntries(t, "x.example")

	var output bytes.Buffer
	built, ok := raw.BuildConfig(pp.New(&output, false, pp.Quiet))
	require.True(t, ok)
	require.NotNil(t, built)
	require.Equal(t,
		"The IP6_DOMAINS listing of x.example is ignored because IPv6 is disabled\n",
		output.String(),
	)
}

func TestBuildConfigWarnsBothMembershipAndHostID6ForDistinctDomains(t *testing.T) {
	t.Parallel()

	raw := config.DefaultRaw()
	raw.Provider = map[ipnet.Family]provider.Provider{ipnet.IP4: provider.NewCloudflareTrace()}
	raw.IP4Domains = mustEntries(t, "keep.example")
	raw.Domains = mustEntries(t, "keep.example{hostid6=mac(00-11-22-33-44-55)}")
	raw.IP6Domains = mustEntries(t, "dead.example")

	var output bytes.Buffer
	built, ok := raw.BuildConfig(pp.New(&output, false, pp.Quiet))
	require.True(t, ok)
	require.NotNil(t, built)
	require.Equal(t,
		"The IP6_DOMAINS listing of dead.example is ignored because IPv6 is disabled\n"+
			"hostid6=mac(00-11-22-33-44-55) for keep.example is ignored because IPv6 is disabled\n",
		output.String(),
	)
}

func TestBuildConfigWarnsExplicitDefaultHostID6ShadowedByDisabledIP6(t *testing.T) {
	t.Parallel()

	raw := config.DefaultRaw()
	raw.Provider = map[ipnet.Family]provider.Provider{ipnet.IP4: provider.NewCloudflareTrace()}
	raw.Domains = mustEntries(t, "x.example{hostid6=preserve}")

	var output bytes.Buffer
	built, ok := raw.BuildConfig(pp.New(&output, false, pp.Quiet))
	require.True(t, ok)
	require.NotNil(t, built)
	require.Equal(t,
		"hostid6=preserve for x.example is ignored because IPv6 is disabled\n",
		output.String(),
	)
}

func TestBuildConfigDoesNotWarnDefaultHostID6OrFamilyAgnosticDomainUnderSingleStack(t *testing.T) {
	t.Parallel()

	raw := config.DefaultRaw()
	raw.Provider = map[ipnet.Family]provider.Provider{ipnet.IP4: provider.NewCloudflareTrace()}
	raw.Domains = mustEntries(t, "x.example")

	var output bytes.Buffer
	built, ok := raw.BuildConfig(pp.New(&output, false, pp.Quiet))
	require.True(t, ok)
	require.NotNil(t, built)
	require.Empty(t, output.String())
}

func TestBuildConfigWarnsOnceForEntirelyShadowedDomain(t *testing.T) {
	t.Parallel()

	raw := config.DefaultRaw()
	raw.Provider = map[ipnet.Family]provider.Provider{ipnet.IP4: provider.NewCloudflareTrace()}
	raw.IP4Domains = mustEntries(t, "keep.example")
	raw.IP6Domains = mustEntries(t, "dead.example")

	var output bytes.Buffer
	built, ok := raw.BuildConfig(pp.New(&output, false, pp.Quiet))
	require.True(t, ok)
	require.NotNil(t, built)
	require.Equal(t,
		"The IP6_DOMAINS listing of dead.example is ignored because IPv6 is disabled\n",
		output.String(),
	)
}
