package main

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/config"
	"github.com/favonia/cloudflare-ddns/internal/cron"
	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/heartbeat"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/notifier"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider"
	"github.com/favonia/cloudflare-ddns/internal/setter"
)

// resetInitConfigEnv clears every environment variable read during startup so
// the test starts from the command's defaults instead of the caller's shell.
func resetInitConfigEnv(t *testing.T) {
	t.Helper()

	for _, key := range []string{
		"CLOUDFLARE_API_TOKEN", "CLOUDFLARE_API_TOKEN_FILE",
		"CF_API_TOKEN", "CF_API_TOKEN_FILE", "CF_ACCOUNT_ID",
		"IP4_PROVIDER", "IP6_PROVIDER", "IP4_POLICY", "IP6_POLICY",
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
		"MANAGED_WAF_LIST_ITEMS_COMMENT_REGEX",
		"DETECTION_TIMEOUT",
		"UPDATE_TIMEOUT",
		"HEALTHCHECKS",
		"UPTIMEKUMA",
		"SHOUTRRR",
	} {
		t.Setenv(key, "")
	}
}

// TestInitConfigManagedRecordsCommentRegex exercises initConfig's successful
// entry-point path with a minimal valid environment.
//
// The assertions check the observable contract of that function: it builds the
// handle/lifecycle/update configs, preserves defaults for untouched settings,
// and returns the built config plus setter needed by the command to proceed.
//
// It stays at the boundary of initConfig by checking the returned raw/runtime
// settings and constructor success, leaving setter behavior and record-update
// logic to their own package tests.
func TestInitConfigManagedRecordsCommentRegex(t *testing.T) {
	resetInitConfigEnv(t)
	t.Setenv("CLOUDFLARE_API_TOKEN", "deadbeaf")
	t.Setenv("DOMAINS", "example.org")
	t.Setenv("RECORD_COMMENT", "managed")
	t.Setenv("MANAGED_RECORDS_COMMENT_REGEX", "^managed$")
	t.Setenv("WAF_LIST_ITEM_COMMENT", "managed-waf")
	t.Setenv("MANAGED_WAF_LIST_ITEMS_COMMENT_REGEX", "^managed-waf$")

	// Run the production initialization path silently; the assertions below define
	// the successful return contract for initConfig.
	builtConfig, s, ok := initConfig(
		pp.NewSilent(),
		heartbeat.NewComposed(),
		notifier.NewComposed(),
	)
	require.True(t, ok)
	require.NotNil(t, builtConfig)
	require.NotNil(t, builtConfig.Handle)
	require.NotNil(t, builtConfig.Lifecycle)
	require.NotNil(t, builtConfig.Update)
	require.NotNil(t, s)
	handleConfig := builtConfig.Handle
	lifecycleConfig := builtConfig.Lifecycle
	updateConfig := builtConfig.Update
	auth, ok := handleConfig.Auth.(*api.CloudflareAuth)
	require.True(t, ok)
	require.Equal(t, "deadbeaf", auth.Token)
	require.Empty(t, auth.BaseURL)
	require.Equal(t, "cloudflare.trace", provider.Name(updateConfig.Provider[ipnet.IP4]))
	require.Equal(t, "cloudflare.trace", provider.Name(updateConfig.Provider[ipnet.IP6]))
	require.Equal(t, map[ipnet.Family][]domain.Domain{
		ipnet.IP4: {domain.FQDN("example.org")},
		ipnet.IP6: {domain.FQDN("example.org")},
	}, updateConfig.Domains)
	require.Empty(t, updateConfig.WAFLists)
	require.Equal(t, "@every 5m", cron.DescribeSchedule(lifecycleConfig.UpdateCron))
	require.True(t, lifecycleConfig.UpdateOnStart)
	require.False(t, lifecycleConfig.DeleteOnStop)
	require.Equal(t, 6*time.Hour, handleConfig.Options.CacheExpiration)
	require.Equal(t, api.TTLAuto, updateConfig.TTL)
	require.Equal(t, map[domain.Domain]bool{
		domain.FQDN("example.org"): false,
	}, updateConfig.Proxied)
	require.Equal(t, "managed", updateConfig.RecordComment)
	// initConfig exposes the compiled handle-bound form without reaching into
	// setter internals.
	require.NotNil(t, handleConfig.Options.ManagedRecordsCommentRegex)
	require.Equal(t, "^managed$", handleConfig.Options.ManagedRecordsCommentRegex.String())
	require.NotNil(t, handleConfig.Options.ManagedWAFListItemsCommentRegex)
	require.Equal(t, "^managed-waf$", handleConfig.Options.ManagedWAFListItemsCommentRegex.String())
	require.Empty(t, updateConfig.WAFListDescription)
	require.Equal(t, "managed-waf", updateConfig.WAFListItemComment)
	require.Equal(t, 5*time.Second, updateConfig.DetectionTimeout)
	require.Equal(t, 30*time.Second, updateConfig.UpdateTimeout)
}

//nolint:paralleltest // environment variables are global
func TestInitConfigReadFailure(t *testing.T) {
	resetInitConfigEnv(t)

	builtConfig, s, ok := initConfig(
		pp.NewSilent(),
		heartbeat.NewComposed(),
		notifier.NewComposed(),
	)
	require.False(t, ok)
	require.Nil(t, builtConfig)
	require.Nil(t, s)
}

func TestInitConfigBuildFailure(t *testing.T) {
	resetInitConfigEnv(t)
	t.Setenv("CLOUDFLARE_API_TOKEN", "deadbeaf")
	t.Setenv("DOMAINS", "example.org")
	t.Setenv("MANAGED_RECORDS_COMMENT_REGEX", "(")

	builtConfig, s, ok := initConfig(
		pp.NewSilent(),
		heartbeat.NewComposed(),
		notifier.NewComposed(),
	)
	require.False(t, ok)
	require.Nil(t, builtConfig)
	require.Nil(t, s)
}

func TestRealMainReporterFailure(t *testing.T) {
	resetInitConfigEnv(t)
	t.Setenv("CLOUDFLARE_API_TOKEN", "deadbeaf")
	t.Setenv("HEALTHCHECKS", "\001")
	t.Setenv("QUIET", "true")

	require.Equal(t, 1, realMain())
}

func TestRealMainSetupPPFailure(t *testing.T) {
	resetInitConfigEnv(t)
	t.Setenv("EMOJI", "invalid")

	require.Equal(t, 1, realMain())
}

//nolint:paralleltest // Version is a global linker-injected variable
func TestFormatName(t *testing.T) {
	oldVersion := Version
	t.Cleanup(func() {
		Version = oldVersion
	})

	Version = ""
	require.Equal(t, "Cloudflare DDNS", formatName())

	Version = "v1.2.3"
	require.Equal(t, "Cloudflare DDNS (v1.2.3)", formatName())
}

func TestRealMainConfigFailure(t *testing.T) {
	resetInitConfigEnv(t)
	t.Setenv("QUIET", "true")

	require.Equal(t, 1, realMain())
}

func TestStopUpdatingDeleteOnStop(t *testing.T) {
	t.Parallel()

	mockCtrl := gomock.NewController(t)
	mockHeartbeat := mocks.NewMockHeartbeat(mockCtrl)
	mockNotifier := mocks.NewMockNotifier(mockCtrl)
	mockSetter := mocks.NewMockSetter(mockCtrl)
	ppfmt := pp.NewSilent()

	domain4 := domain.FQDN("example.org")
	wafList := api.WAFList{AccountID: "acc", Name: "office"}
	params := api.RecordParams{
		TTL:     api.TTLAuto,
		Proxied: false,
		Comment: "managed",
		Tags:    nil,
	}

	lifecycleConfig := &config.LifecycleConfig{
		UpdateCron:    nil,
		UpdateOnStart: false,
		DeleteOnStop:  true,
	}
	updateConfig := &config.UpdateConfig{
		Provider: map[ipnet.Family]provider.Provider{
			ipnet.IP4: provider.MustNewStatic(ipnet.IP4, 32, "192.0.2.1"),
			ipnet.IP6: nil,
		},
		Domains: map[ipnet.Family][]domain.Domain{
			ipnet.IP4: {domain4},
			ipnet.IP6: nil,
		},
		WAFLists: []api.WAFList{wafList},
		DefaultPrefixLen: map[ipnet.Family]int{
			ipnet.IP4: 32,
			ipnet.IP6: 64,
		},
		TTL:                api.TTLAuto,
		Proxied:            map[domain.Domain]bool{domain4: false},
		RecordComment:      "managed",
		WAFListDescription: "managed list",
		WAFListItemComment: "",
		DetectionTimeout:   time.Second,
		UpdateTimeout:      time.Second,
	}

	mockSetter.EXPECT().FinalDelete(gomock.Any(), ppfmt, ipnet.IP4, domain4, params).Return(setter.ResponseUpdated)
	mockSetter.EXPECT().FinalClearWAFList(gomock.Any(), ppfmt, wafList, "managed list", gomock.Any()).Return(setter.ResponseUpdated)
	mockHeartbeat.EXPECT().Log(gomock.Any(), ppfmt, gomock.Any()).DoAndReturn(
		func(_ context.Context, _ pp.PP, msg heartbeat.Message) bool {
			require.True(t, msg.OK)
			require.Contains(t, msg.Format(), "Deleted A records for example.org")
			require.Contains(t, msg.Format(), "Cleaned WAF list(s) acc/office")
			return true
		},
	)
	mockNotifier.EXPECT().Send(gomock.Any(), ppfmt, gomock.Any()).DoAndReturn(
		func(_ context.Context, _ pp.PP, msg notifier.Message) bool {
			require.Contains(t, msg.Format(), "Deleted A records for example.org.")
			require.Contains(t, msg.Format(), "Cleaned WAF list(s) acc/office.")
			return true
		},
	)

	stopUpdating(context.Background(), ppfmt, lifecycleConfig, updateConfig, mockHeartbeat, mockNotifier, mockSetter)
}

func TestStopUpdatingSkipsDeleteOnStop(t *testing.T) {
	t.Parallel()

	mockCtrl := gomock.NewController(t)
	mockHeartbeat := mocks.NewMockHeartbeat(mockCtrl)
	mockNotifier := mocks.NewMockNotifier(mockCtrl)
	mockSetter := mocks.NewMockSetter(mockCtrl)

	stopUpdating(
		context.Background(),
		pp.NewSilent(),
		&config.LifecycleConfig{
			UpdateCron:    nil,
			UpdateOnStart: false,
			DeleteOnStop:  false,
		},
		&config.UpdateConfig{
			Provider: nil,
			Domains:  nil,
			WAFLists: nil,
			DefaultPrefixLen: map[ipnet.Family]int{
				ipnet.IP4: 32,
				ipnet.IP6: 64,
			},
			TTL:                0,
			Proxied:            nil,
			RecordComment:      "",
			WAFListDescription: "",
			WAFListItemComment: "",
			DetectionTimeout:   0,
			UpdateTimeout:      0,
		},
		mockHeartbeat,
		mockNotifier,
		mockSetter,
	)
}
