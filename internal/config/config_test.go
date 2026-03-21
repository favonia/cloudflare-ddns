package config_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/config"
	"github.com/favonia/cloudflare-ddns/internal/cron"
	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider"
)

func TestDefaultConfigNotNil(t *testing.T) {
	t.Parallel()

	require.NotNil(t, config.DefaultRaw())
}

type rawConfigSummary struct {
	ip4Provider                     string
	ip6Provider                     string
	domains                         []string
	ip4Domains                      []string
	ip6Domains                      []string
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
		domains:                         summarizeDomains(raw.Domains),
		ip4Domains:                      summarizeDomains(raw.IP4Domains),
		ip6Domains:                      summarizeDomains(raw.IP6Domains),
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

	unsetAll(t)
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
