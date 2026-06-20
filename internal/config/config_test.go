package config_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/config"
	"github.com/favonia/cloudflare-ddns/internal/cron"
	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/domainentry"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider"
	"github.com/favonia/cloudflare-ddns/internal/syntax"
	"github.com/favonia/cloudflare-ddns/internal/testenv"
)

func TestDefaultConfigNotNil(t *testing.T) {
	t.Parallel()

	require.NotNil(t, config.DefaultRaw())
}

type rawConfigSummary struct {
	ip4Provider                     string
	ip6Provider                     string
	domains                         []rawEntrySummary
	ip4Domains                      []rawEntrySummary
	ip6Domains                      []rawEntrySummary
	wafLists                        []string
	updateCron                      string
	updateOnStart                   bool
	deleteOnStop                    bool
	ttl                             api.TTL
	proxiedExpression               string
	recordComment                   string
	managedRecordsCommentRegex      string
	wafListDescription              string
	wafListItemComment              string
	managedWAFListItemsCommentRegex string
	cacheExpiration                 time.Duration
	detectionTimeout                time.Duration
	updateTimeout                   time.Duration
}

type rawEntrySummary struct {
	domain          string
	hostID6Opinions [][]string
}

func summarizeEntries(entries []domainentry.Entry) []rawEntrySummary {
	summary := make([]rawEntrySummary, 0, len(entries))
	for _, entry := range entries {
		opinions := make([][]string, 0, len(entry.HostID6Opinions))
		for _, opinion := range entry.HostID6Opinions {
			values := opinion.Set.Values()
			descriptions := make([]string, 0, len(values))
			for _, value := range values {
				descriptions = append(descriptions, value.Describe())
			}
			opinions = append(opinions, descriptions)
		}
		summary = append(summary, rawEntrySummary{
			domain:          entry.Domain.Describe(),
			hostID6Opinions: opinions,
		})
	}
	return summary
}

func summarizeDomains(domains []domain.Domain) []string {
	summary := make([]string, 0, len(domains))
	for _, d := range domains {
		summary = append(summary, d.Describe())
	}
	return summary
}

func summarizeWAFLists(lists []api.WAFList) []string {
	summary := make([]string, 0, len(lists))
	for _, l := range lists {
		summary = append(summary, l.Describe())
	}
	return summary
}

func summarizeRawConfig(raw *config.RawConfig) rawConfigSummary {
	return rawConfigSummary{
		ip4Provider:                     provider.Name(raw.Provider[ipnet.IP4]),
		ip6Provider:                     provider.Name(raw.Provider[ipnet.IP6]),
		domains:                         summarizeEntries(raw.Domains),
		ip4Domains:                      summarizeEntries(raw.IP4Domains),
		ip6Domains:                      summarizeEntries(raw.IP6Domains),
		wafLists:                        summarizeWAFLists(raw.WAFLists),
		updateCron:                      cron.DescribeSchedule(raw.UpdateCron),
		updateOnStart:                   raw.UpdateOnStart,
		deleteOnStop:                    raw.DeleteOnStop,
		ttl:                             raw.TTL,
		proxiedExpression:               raw.ProxiedExpression,
		recordComment:                   raw.RecordComment,
		managedRecordsCommentRegex:      raw.ManagedRecordsCommentRegex,
		wafListDescription:              raw.WAFListDescription,
		wafListItemComment:              raw.WAFListItemComment,
		managedWAFListItemsCommentRegex: raw.ManagedWAFListItemsCommentRegex,
		cacheExpiration:                 raw.CacheExpiration,
		detectionTimeout:                raw.DetectionTimeout,
		updateTimeout:                   raw.UpdateTimeout,
	}
}

func readUpdaterSettingsFromDefaultRaw(t *testing.T, env map[string]string) *config.RawConfig {
	t.Helper()

	testenv.ClearAll(t)
	store(t, "CLOUDFLARE_API_TOKEN", "deadbeaf")
	for key, value := range env {
		store(t, key, value)
	}

	raw := config.DefaultRaw()
	require.True(t, raw.ReadEnv(pp.NewSilent()))
	return raw
}

func canonicalExplicitUpdaterDefaults() map[string]string {
	return map[string]string{
		"DOMAINS":                              "",
		"IP4_DOMAINS":                          "",
		"IP6_DOMAINS":                          "",
		"WAF_LISTS":                            "",
		"IP4_PROVIDER":                         "cloudflare.trace",
		"IP6_PROVIDER":                         "cloudflare.trace",
		"UPDATE_CRON":                          "@every 5m",
		"UPDATE_ON_START":                      "true",
		"DELETE_ON_STOP":                       "false",
		"CACHE_EXPIRATION":                     "6h0m0s",
		"TTL":                                  "1",
		"PROXIED":                              "false",
		"RECORD_COMMENT":                       "",
		"MANAGED_RECORDS_COMMENT_REGEX":        "",
		"WAF_LIST_DESCRIPTION":                 "",
		"WAF_LIST_ITEM_COMMENT":                "",
		"MANAGED_WAF_LIST_ITEMS_COMMENT_REGEX": "",
		"DETECTION_TIMEOUT":                    "5s",
		"UPDATE_TIMEOUT":                       "30s",
	}
}

//nolint:paralleltest // environment variables are global
func TestOptionalUpdaterSettingsOmissionMatchesCanonicalExplicitValues(t *testing.T) {
	// Canonical explicit defaults mirrored from README.markdown's "All Settings"
	// tables for the optional updater settings owned by RawConfig.
	explicitDefaults := canonicalExplicitUpdaterDefaults()

	implicitDefaults := readUpdaterSettingsFromDefaultRaw(t, nil)
	explicitDefaultsRaw := readUpdaterSettingsFromDefaultRaw(t, explicitDefaults)

	require.Equal(t, summarizeRawConfig(implicitDefaults), summarizeRawConfig(explicitDefaultsRaw))
}

//nolint:paralleltest // environment variables are global
func TestReadEnvDomainDiagnostics(t *testing.T) {
	for name, tc := range map[string]struct {
		key      string
		value    string
		expected string
	}{
		"malformed-entry": {
			key:      "DOMAINS",
			value:    "example.org{hostid6=[::1,,::2]}",
			expected: "DOMAINS (\"example.org{hostid6=[::1,,::2]}\") has unexpected token \",\"\n",
		},
		"compatibility-warnings-before-recovered-error": {
			key:   "DOMAINS",
			value: ",good.example bad.example,localhost",
			expected: "DOMAINS (\",good.example bad.example,localhost\") contains extra commas; this is accepted for now but will be rejected in version 2.0.0\n" +
				"DOMAINS (\",good.example bad.example,localhost\") is missing commas; this is accepted for now but will be rejected in version 2.0.0\n" +
				"DOMAINS (\",good.example bad.example,localhost\") has invalid domain \"localhost\": not fully qualified\n",
		},
		"ordered-recovered-semantic-errors": {
			key:   "DOMAINS",
			value: "localhost,good.example,example.org{unknown=::1},example.net{hostid6=192.0.2.1},example.com{hostid6=mac(bad)}",
			expected: "DOMAINS (\"localhost,good.example,example.org{unknown=::1},example.net{hostid6=192.0.2.1},example.com{hostid6=mac(bad)}\") has invalid domain \"localhost\": not fully qualified\n" +
				"DOMAINS (\"localhost,good.example,example.org{unknown=::1},example.net{hostid6=192.0.2.1},example.com{hostid6=mac(bad)}\") has unknown domain field \"unknown\"\n" +
				"DOMAINS (\"localhost,good.example,example.org{unknown=::1},example.net{hostid6=192.0.2.1},example.com{hostid6=mac(bad)}\") has invalid hostid6 value \"192.0.2.1\": host-ID literal must be an unzoned IPv6 address\n" +
				"DOMAINS (\"localhost,good.example,example.org{unknown=::1},example.net{hostid6=192.0.2.1},example.com{hostid6=mac(bad)}\") has invalid hostid6 MAC address \"bad\": invalid 48-bit MAC address\n",
		},
		"ipv4-hostid6": {
			key:      "IP4_DOMAINS",
			value:    "example.org{hostid6=::1}",
			expected: "IP4_DOMAINS (\"example.org{hostid6=::1}\") configures hostid6 for example.org, but hostid6 only affects IPv6; remove hostid6 from this IP4_DOMAINS entry, or configure the IPv6 entry in DOMAINS or IP6_DOMAINS\n",
		},
	} {
		t.Run(name, func(t *testing.T) {
			testenv.ClearAll(t)
			store(t, "CLOUDFLARE_API_TOKEN", "deadbeaf")
			store(t, tc.key, tc.value)
			cfg := config.DefaultRaw()
			oldEntries := []domainentry.Entry{{
				Domain:          domain.FQDN("old.example"),
				HostID6Opinions: nil,
				Span:            syntax.Span{Start: 0, End: 0},
			}}
			switch tc.key {
			case "DOMAINS":
				cfg.Domains = oldEntries
			case "IP4_DOMAINS":
				cfg.IP4Domains = oldEntries
			}
			var output bytes.Buffer

			ok := cfg.ReadEnv(pp.New(&output, false, pp.Quiet))

			require.False(t, ok)
			require.Equal(t, tc.expected, output.String())
			switch tc.key {
			case "DOMAINS":
				require.Equal(t, oldEntries, cfg.Domains)
			case "IP4_DOMAINS":
				require.Equal(t, oldEntries, cfg.IP4Domains)
			}
		})
	}
}

//nolint:paralleltest // environment variables are global
func TestReadEnvDomainDiagnosticSeverityAndOrder(t *testing.T) {
	const value = ",good.example bad.example,localhost"
	testenv.ClearAll(t)
	store(t, "CLOUDFLARE_API_TOKEN", "deadbeaf")
	store(t, "DOMAINS", value)
	cfg := config.DefaultRaw()
	var output bytes.Buffer

	ok := cfg.ReadEnv(pp.New(&output, true, pp.Quiet))

	require.False(t, ok)
	require.Equal(t,
		"😦 DOMAINS (\",good.example bad.example,localhost\") contains extra commas; this is accepted for now but will be rejected in version 2.0.0\n"+
			"😦 DOMAINS (\",good.example bad.example,localhost\") is missing commas; this is accepted for now but will be rejected in version 2.0.0\n"+
			"😡 DOMAINS (\",good.example bad.example,localhost\") has invalid domain \"localhost\": not fully qualified\n",
		output.String(),
	)
}

//nolint:paralleltest // environment variables are global
func TestReadEnvDomainCompatibilityWarningUsesBoundedPreview(t *testing.T) {
	value := "," + strings.Repeat("a", 60) + ".example"
	testenv.ClearAll(t)
	store(t, "CLOUDFLARE_API_TOKEN", "deadbeaf")
	store(t, "DOMAINS", value)
	cfg := config.DefaultRaw()
	var output bytes.Buffer

	ok := cfg.ReadEnv(pp.New(&output, true, pp.Quiet))

	require.True(t, ok)
	require.Equal(t,
		"😦 DOMAINS (\",aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa...\") contains extra commas; this is accepted for now but will be rejected in version 2.0.0\n",
		output.String(),
	)
}

type authSummary struct {
	token   string
	baseURL string
}

type handleConfigSummary struct {
	auth                              authSummary
	cacheExpiration                   time.Duration
	managedRecordsCommentRegex        string
	managedWAFListItemsCommentRegex   string
	allowWholeWAFListDeleteOnShutdown bool
}

type lifecycleConfigSummary struct {
	updateCron    string
	updateOnStart bool
	deleteOnStop  bool
}

type updateConfigSummary struct {
	ip4Provider        string
	ip6Provider        string
	ip4Domains         []string
	ip6Domains         []string
	wafLists           []string
	ttl                api.TTL
	proxied            map[string]bool
	recordComment      string
	wafListDesc        string
	wafListItemComment string
	detectionTimeout   time.Duration
	updateTimeout      time.Duration
}

type builtConfigSummary struct {
	handle    handleConfigSummary
	lifecycle lifecycleConfigSummary
	update    updateConfigSummary
}

func summarizeAuth(t *testing.T, auth api.Auth) authSummary {
	t.Helper()

	switch a := auth.(type) {
	case *api.CloudflareAuth:
		if a == nil {
			return authSummary{
				token:   "",
				baseURL: "",
			}
		}
		return authSummary{
			token:   a.Token,
			baseURL: a.BaseURL,
		}
	default:
		t.Fatalf("unexpected auth type %T", auth)
		return authSummary{
			token:   "",
			baseURL: "",
		}
	}
}

func summarizeProxiedMap(proxied map[domain.Domain]bool) map[string]bool {
	summary := make(map[string]bool, len(proxied))
	for d, value := range proxied {
		summary[d.Describe()] = value
	}
	return summary
}

func summarizeBuiltConfig(t *testing.T, built *config.BuiltConfig) builtConfigSummary {
	t.Helper()

	require.NotNil(t, built)
	require.NotNil(t, built.Handle)
	require.NotNil(t, built.Lifecycle)
	require.NotNil(t, built.Update)

	return builtConfigSummary{
		handle: handleConfigSummary{
			auth:                              summarizeAuth(t, built.Handle.Auth),
			cacheExpiration:                   built.Handle.Options.CacheExpiration,
			managedRecordsCommentRegex:        built.Handle.Options.ManagedRecordsCommentRegex.String(),
			managedWAFListItemsCommentRegex:   built.Handle.Options.ManagedWAFListItemsCommentRegex.String(),
			allowWholeWAFListDeleteOnShutdown: built.Handle.Options.AllowWholeWAFListDeleteOnShutdown,
		},
		lifecycle: lifecycleConfigSummary{
			updateCron:    cron.DescribeSchedule(built.Lifecycle.UpdateCron),
			updateOnStart: built.Lifecycle.UpdateOnStart,
			deleteOnStop:  built.Lifecycle.DeleteOnStop,
		},
		update: updateConfigSummary{
			ip4Provider:        provider.Name(built.Update.Provider[ipnet.IP4]),
			ip6Provider:        provider.Name(built.Update.Provider[ipnet.IP6]),
			ip4Domains:         summarizeDomains(built.Update.Domains[ipnet.IP4]),
			ip6Domains:         summarizeDomains(built.Update.Domains[ipnet.IP6]),
			wafLists:           summarizeWAFLists(built.Update.WAFLists),
			ttl:                built.Update.TTL,
			proxied:            summarizeProxiedMap(built.Update.Proxied),
			recordComment:      built.Update.RecordComment,
			wafListDesc:        built.Update.WAFListDescription,
			wafListItemComment: built.Update.WAFListItemComment,
			detectionTimeout:   built.Update.DetectionTimeout,
			updateTimeout:      built.Update.UpdateTimeout,
		},
	}
}

func readAndBuildUpdaterConfig(t *testing.T, env map[string]string) *config.BuiltConfig {
	t.Helper()

	raw := readUpdaterSettingsFromDefaultRaw(t, env)
	built, ok := raw.BuildConfig(pp.NewSilent())
	require.True(t, ok)
	return built
}

//nolint:paralleltest // environment variables are global
func TestOptionalUpdaterSettingsBuiltConfigOmissionMatchesCanonicalExplicitValues(t *testing.T) {
	implicitEnv := map[string]string{
		"DOMAINS": "example.org",
	}

	explicitEnv := canonicalExplicitUpdaterDefaults()
	explicitEnv["DOMAINS"] = "example.org"

	implicitBuilt := readAndBuildUpdaterConfig(t, implicitEnv)
	explicitBuilt := readAndBuildUpdaterConfig(t, explicitEnv)

	require.Equal(t, summarizeBuiltConfig(t, implicitBuilt), summarizeBuiltConfig(t, explicitBuilt))
}

//nolint:paralleltest // environment variables are global
func TestBuildConfigProjectsStructuredDomainEntries(t *testing.T) {
	built := readAndBuildUpdaterConfig(t, map[string]string{
		"DOMAINS": "example.org{hostid6=::1}",
	})

	require.Equal(t, []string{"example.org"}, summarizeDomains(built.Update.Domains[ipnet.IP4]))
	require.Equal(t, []string{"example.org"}, summarizeDomains(built.Update.Domains[ipnet.IP6]))
}
