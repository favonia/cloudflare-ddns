package main

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/config"
	"github.com/favonia/cloudflare-ddns/internal/cron"
	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/monitor"
	"github.com/favonia/cloudflare-ddns/internal/notifier"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider"
	"github.com/favonia/cloudflare-ddns/internal/setter"
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
// The assertions check the observable contract of that function: it reads env
// vars into RawConfig, builds the handle/lifecycle/update configs, preserves defaults
// for untouched settings, and returns the runtime objects needed by the command
// to proceed.
//
// It stays at the boundary of initConfig by checking the returned raw/runtime
// configs and constructor success, leaving setter behavior and record-update
// logic to their own package tests.
func TestInitConfigManagedRecordsCommentRegex(t *testing.T) {
	resetInitConfigEnv(t)
	t.Setenv("CLOUDFLARE_API_TOKEN", "deadbeaf")
	t.Setenv("DOMAINS", "example.org")
	t.Setenv("RECORD_COMMENT", "managed")
	t.Setenv("MANAGED_RECORDS_COMMENT_REGEX", "^managed$")

	// Run the production initialization path quietly; the assertions below define
	// the successful return contract for initConfig.
	raw, handleConfig, lifecycleConfig, updateConfig, s, ok := initConfig(pp.New(io.Discard, false, pp.Quiet))
	require.True(t, ok)
	require.NotNil(t, raw)
	require.NotNil(t, handleConfig)
	require.NotNil(t, lifecycleConfig)
	require.NotNil(t, updateConfig)
	require.NotNil(t, s)
	auth, ok := handleConfig.Auth.(*api.CloudflareAuth)
	require.True(t, ok)
	require.Equal(t, "deadbeaf", auth.Token)
	require.Empty(t, auth.BaseURL)
	require.Equal(t, "cloudflare.trace", provider.Name(updateConfig.Provider[ipnet.IP4]))
	require.Equal(t, "cloudflare.trace", provider.Name(updateConfig.Provider[ipnet.IP6]))
	require.Equal(t, map[ipnet.Type][]domain.Domain{
		ipnet.IP4: {domain.FQDN("example.org")},
		ipnet.IP6: {domain.FQDN("example.org")},
	}, updateConfig.Domains)
	require.Empty(t, updateConfig.WAFLists)
	require.Equal(t, "@every 5m", cron.DescribeSchedule(lifecycleConfig.UpdateCron))
	require.True(t, lifecycleConfig.UpdateOnStart)
	require.False(t, lifecycleConfig.DeleteOnStop)
	require.Equal(t, 6*time.Hour, handleConfig.Options.CacheExpiration)
	require.Equal(t, api.TTLAuto, updateConfig.TTL)
	require.Equal(t, "false", raw.ProxiedExpression)
	require.Equal(t, []domain.Domain{domain.FQDN("example.org")}, raw.Domains)
	require.Empty(t, raw.IP4Domains)
	require.Empty(t, raw.IP6Domains)
	require.Equal(t, map[domain.Domain]bool{
		domain.FQDN("example.org"): false,
	}, updateConfig.Proxied)
	require.Equal(t, "managed", updateConfig.RecordComment)
	require.Equal(t, "^managed$", raw.ManagedRecordsCommentRegex)
	// initConfig exposes both configs, so this test checks the raw regex and the
	// compiled handle-bound form without reaching into setter internals.
	require.NotNil(t, handleConfig.Options.ManagedRecordsCommentRegex)
	require.Equal(t, "^managed$", handleConfig.Options.ManagedRecordsCommentRegex.String())
	require.Empty(t, updateConfig.WAFListDescription)
	require.Equal(t, 5*time.Second, updateConfig.DetectionTimeout)
	require.Equal(t, 30*time.Second, updateConfig.UpdateTimeout)
	require.NotNil(t, lifecycleConfig.Monitor)
	require.NotNil(t, lifecycleConfig.Notifier)
}

//nolint:paralleltest // environment variables are global
func TestInitConfigReadFailure(t *testing.T) {
	resetInitConfigEnv(t)

	raw, handleConfig, lifecycleConfig, updateConfig, s, ok := initConfig(pp.New(io.Discard, false, pp.Quiet))
	require.False(t, ok)
	require.NotNil(t, raw)
	require.Nil(t, handleConfig)
	require.Nil(t, lifecycleConfig)
	require.Nil(t, updateConfig)
	require.Nil(t, s)
	require.Nil(t, raw.Auth)
	require.Empty(t, raw.Domains)
}

func TestInitConfigBuildFailure(t *testing.T) {
	resetInitConfigEnv(t)
	t.Setenv("CLOUDFLARE_API_TOKEN", "deadbeaf")
	t.Setenv("DOMAINS", "example.org")
	t.Setenv("MANAGED_RECORDS_COMMENT_REGEX", "(")

	raw, handleConfig, lifecycleConfig, updateConfig, s, ok := initConfig(pp.New(io.Discard, false, pp.Quiet))
	require.False(t, ok)
	require.NotNil(t, raw)
	require.Nil(t, handleConfig)
	require.Nil(t, lifecycleConfig)
	require.Nil(t, updateConfig)
	require.Nil(t, s)
	require.Equal(t, "(", raw.ManagedRecordsCommentRegex)
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
	mockMonitor := mocks.NewMockMonitor(mockCtrl)
	mockNotifier := mocks.NewMockNotifier(mockCtrl)
	mockSetter := mocks.NewMockSetter(mockCtrl)
	ppfmt := pp.New(io.Discard, false, pp.Quiet)

	domain4 := domain.FQDN("example.org")
	wafList := api.WAFList{AccountID: "acc", Name: "office"}
	params := api.RecordParams{
		TTL:     api.TTLAuto,
		Proxied: false,
		Comment: "managed",
	}

	lifecycleConfig := &config.LifecycleConfig{
		UpdateCron:    nil,
		UpdateOnStart: false,
		DeleteOnStop:  true,
		Monitor:       mockMonitor,
		Notifier:      mockNotifier,
	}
	updateConfig := &config.UpdateConfig{
		Provider: map[ipnet.Type]provider.Provider{
			ipnet.IP4: provider.MustNewLiteral("192.0.2.1"),
			ipnet.IP6: nil,
		},
		Domains: map[ipnet.Type][]domain.Domain{
			ipnet.IP4: {domain4},
			ipnet.IP6: nil,
		},
		WAFLists:           []api.WAFList{wafList},
		TTL:                api.TTLAuto,
		Proxied:            map[domain.Domain]bool{domain4: false},
		RecordComment:      "managed",
		WAFListDescription: "managed list",
		DetectionTimeout:   time.Second,
		UpdateTimeout:      time.Second,
	}

	mockSetter.EXPECT().FinalDelete(gomock.Any(), ppfmt, ipnet.IP4, domain4, params).Return(setter.ResponseUpdated)
	mockSetter.EXPECT().FinalClearWAFList(gomock.Any(), ppfmt, wafList, "managed list").Return(setter.ResponseUpdated)
	mockMonitor.EXPECT().Log(gomock.Any(), ppfmt, gomock.Any()).DoAndReturn(
		func(_ context.Context, _ pp.PP, msg monitor.Message) bool {
			require.True(t, msg.OK)
			require.Contains(t, msg.Format(), "Deleted A of example.org")
			require.Contains(t, msg.Format(), "Cleared list(s) acc/office")
			return true
		},
	)
	mockNotifier.EXPECT().Send(gomock.Any(), ppfmt, gomock.Any()).DoAndReturn(
		func(_ context.Context, _ pp.PP, msg notifier.Message) bool {
			require.Contains(t, msg.Format(), "Deleted A records of example.org.")
			require.Contains(t, msg.Format(), "Cleared WAF list(s) acc/office.")
			return true
		},
	)

	stopUpdating(context.Background(), ppfmt, lifecycleConfig, updateConfig, mockSetter)
}

func TestStopUpdatingSkipsDeleteOnStop(t *testing.T) {
	t.Parallel()

	mockCtrl := gomock.NewController(t)
	mockMonitor := mocks.NewMockMonitor(mockCtrl)
	mockNotifier := mocks.NewMockNotifier(mockCtrl)
	mockSetter := mocks.NewMockSetter(mockCtrl)

	stopUpdating(
		context.Background(),
		pp.New(io.Discard, false, pp.Quiet),
		&config.LifecycleConfig{
			UpdateCron:    nil,
			UpdateOnStart: false,
			DeleteOnStop:  false,
			Monitor:       mockMonitor,
			Notifier:      mockNotifier,
		},
		&config.UpdateConfig{
			Provider:           nil,
			Domains:            nil,
			WAFLists:           nil,
			TTL:                0,
			Proxied:            nil,
			RecordComment:      "",
			WAFListDescription: "",
			DetectionTimeout:   0,
			UpdateTimeout:      0,
		},
		mockSetter,
	)
}
