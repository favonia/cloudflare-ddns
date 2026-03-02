package main

import (
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/cron"
	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider"
)

// resetInitConfigEnv clears every environment variable read by initConfig so
// the test starts from the command's defaults instead of the caller's shell.
func resetInitConfigEnv(t *testing.T) {
	t.Helper()

	for _, key := range []string{
		"CLOUDFLARE_API_TOKEN", "CLOUDFLARE_API_TOKEN_FILE",
		"CF_API_TOKEN", "CF_API_TOKEN_FILE", "CF_ACCOUNT_ID",
		"IP4_PROVIDER", "IP6_PROVIDER",
		"DOMAINS", "IP4_DOMAINS", "IP6_DOMAINS", "WAF_LISTS",
		"UPDATE_CRON",
		"UPDATE_ON_START",
		"DELETE_ON_STOP",
		"CACHE_EXPIRATION",
		"TTL",
		"PROXIED",
		"RECORD_COMMENT",
		"MANAGED_RECORDS_COMMENT_REGEX",
		"WAF_LIST_DESCRIPTION",
		"WAF_LIST_ITEM_COMMENT",
		"MANAGED_WAF_LIST_ITEM_COMMENT_REGEX",
		"DETECTION_TIMEOUT",
		"UPDATE_TIMEOUT",
		"HEALTHCHECKS",
		"UPTIMEKUMA",
		"SHOUTRRR",
	} {
		t.Setenv(key, "")
	}
}

// TestInitConfigManagedCommentOwnershipRegexes exercises initConfig's successful
// entry-point path with a minimal valid environment.
//
// The assertions check the observable contract of that function: it reads env
// vars, restores normalized config invariants, preserves defaults for untouched
// settings, and returns the runtime objects needed by the command to proceed.
//
// It stays at the boundary of initConfig by checking the returned config and
// constructor success, leaving setter behavior and record-update logic to their
// own package tests.
func TestInitConfigManagedCommentOwnershipRegexes(t *testing.T) {
	resetInitConfigEnv(t)
	t.Setenv("CLOUDFLARE_API_TOKEN", "deadbeaf")
	t.Setenv("DOMAINS", "example.org")
	t.Setenv("WAF_LISTS", "account/list")
	t.Setenv("RECORD_COMMENT", "managed")
	t.Setenv("MANAGED_RECORDS_COMMENT_REGEX", "^managed$")
	t.Setenv("WAF_LIST_DESCRIPTION", "shared list")
	t.Setenv("WAF_LIST_ITEM_COMMENT", "managed-waf")
	t.Setenv("MANAGED_WAF_LIST_ITEM_COMMENT_REGEX", "^managed-waf$")

	// Run the production initialization path quietly; the assertions below define
	// the successful return contract for initConfig.
	cfg, s, ok := initConfig(pp.New(io.Discard, false, pp.Quiet))
	require.True(t, ok)
	require.NotNil(t, cfg)
	require.NotNil(t, s)
	auth, ok := cfg.Auth.(*api.CloudflareAuth)
	require.True(t, ok)
	require.Equal(t, "deadbeaf", auth.Token)
	require.Empty(t, auth.BaseURL)
	require.Equal(t, "cloudflare.trace", provider.Name(cfg.Provider[ipnet.IP4]))
	require.Equal(t, "cloudflare.trace", provider.Name(cfg.Provider[ipnet.IP6]))
	require.Equal(t, map[ipnet.Type][]domain.Domain{
		ipnet.IP4: {domain.FQDN("example.org")},
		ipnet.IP6: {domain.FQDN("example.org")},
	}, cfg.Domains)
	require.Equal(t, []api.WAFList{{AccountID: "account", Name: "list"}}, cfg.WAFLists)
	require.Equal(t, "@every 5m", cron.DescribeSchedule(cfg.UpdateCron))
	require.True(t, cfg.UpdateOnStart)
	require.False(t, cfg.DeleteOnStop)
	require.Equal(t, 6*time.Hour, cfg.CacheExpiration)
	require.Equal(t, api.TTLAuto, cfg.TTL)
	require.Equal(t, "false", cfg.ProxiedTemplate)
	require.Equal(t, map[domain.Domain]bool{
		domain.FQDN("example.org"): false,
	}, cfg.Proxied)
	require.Equal(t, "managed", cfg.RecordComment)
	require.Equal(t, "^managed$", cfg.ManagedRecordsCommentRegexTemplate)
	// initConfig exposes the normalized config, so this test checks the compiled
	// ownership regexes there without reaching into setter internals.
	require.NotNil(t, cfg.ManagedRecordsCommentRegex)
	require.Equal(t, "^managed$", cfg.ManagedRecordsCommentRegex.String())
	require.Equal(t, "shared list", cfg.WAFListDescription)
	require.Equal(t, "managed-waf", cfg.WAFListItemComment)
	require.Equal(t, "^managed-waf$", cfg.ManagedWAFListItemCommentRegexTemplate)
	require.NotNil(t, cfg.ManagedWAFListItemCommentRegex)
	require.Equal(t, "^managed-waf$", cfg.ManagedWAFListItemCommentRegex.String())
	require.Equal(t, 5*time.Second, cfg.DetectionTimeout)
	require.Equal(t, 30*time.Second, cfg.UpdateTimeout)
	require.NotNil(t, cfg.Monitor)
	require.NotNil(t, cfg.Notifier)
}
