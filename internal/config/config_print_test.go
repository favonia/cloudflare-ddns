package config_test

import (
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/config"
	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider"
)

func printItem(t *testing.T, ppfmt *mocks.MockPP, key string, value any) *mocks.MockPPInfofCall {
	t.Helper()
	return ppfmt.EXPECT().Infof(pp.EmojiBullet, "%-*s %s", 28, key, value)
}

func defaultPrintedConfigs(raw *config.RawConfig) (*config.HandleConfig, *config.LifecycleConfig, *config.UpdateConfig) {
	handleConfig := &config.HandleConfig{} //nolint:exhaustruct // This helper intentionally starts from the zero value and fills only the fields print tests use.
	handleConfig.Auth = raw.Auth
	handleConfig.CacheExpiration = raw.CacheExpiration

	lifecycleConfig := &config.LifecycleConfig{} //nolint:exhaustruct // This helper intentionally starts from the zero value and fills only the fields print tests use.
	lifecycleConfig.UpdateCron = raw.UpdateCron
	lifecycleConfig.UpdateOnStart = raw.UpdateOnStart
	lifecycleConfig.DeleteOnStop = raw.DeleteOnStop
	lifecycleConfig.Monitor = raw.Monitor
	lifecycleConfig.Notifier = raw.Notifier

	updateConfig := &config.UpdateConfig{} //nolint:exhaustruct // This helper intentionally starts from the zero value and fills only the fields print tests use.
	updateConfig.Provider = map[ipnet.Type]provider.Provider{
		ipnet.IP4: raw.Provider[ipnet.IP4],
		ipnet.IP6: raw.Provider[ipnet.IP6],
	}
	updateConfig.Domains = map[ipnet.Type][]domain.Domain{
		ipnet.IP4: nil,
		ipnet.IP6: nil,
	}
	updateConfig.WAFLists = raw.WAFLists
	updateConfig.TTL = raw.TTL
	updateConfig.Proxied = map[domain.Domain]bool{}
	updateConfig.RecordComment = raw.RecordComment
	updateConfig.WAFListDescription = raw.WAFListDescription
	updateConfig.DetectionTimeout = raw.DetectionTimeout
	updateConfig.UpdateTimeout = raw.UpdateTimeout

	return handleConfig, lifecycleConfig, updateConfig
}

//nolint:paralleltest // changing the environment variable TZ
func TestPrintDefault(t *testing.T) {
	mockCtrl := gomock.NewController(t)

	store(t, "TZ", "UTC")

	mockPP := mocks.NewMockPP(mockCtrl)
	innerMockPP := mocks.NewMockPP(mockCtrl)
	gomock.InOrder(
		mockPP.EXPECT().IsShowing(pp.Info).Return(true),
		mockPP.EXPECT().Infof(pp.EmojiEnvVars, "Current settings:"),
		mockPP.EXPECT().Indent().Return(mockPP),
		mockPP.EXPECT().Indent().Return(innerMockPP),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "%s", "Domains, IP providers, and WAF lists:"),
		printItem(t, innerMockPP, "IPv4-enabled domains:", "(none)"),
		printItem(t, innerMockPP, "IPv4 provider:", "cloudflare.trace"),
		printItem(t, innerMockPP, "IPv6-enabled domains:", "(none)"),
		printItem(t, innerMockPP, "IPv6 provider:", "cloudflare.trace"),
		printItem(t, innerMockPP, "WAF lists:", "(none)"),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "%s", "Scheduling:"),
		printItem(t, innerMockPP, "Timezone:", gomock.AnyOf("UTC (currently UTC+00)", "Local (currently UTC+00)")),
		printItem(t, innerMockPP, "Update schedule:", "@every 5m"),
		printItem(t, innerMockPP, "Update on start?", "true"),
		printItem(t, innerMockPP, "Delete on stop?", "false"),
		printItem(t, innerMockPP, "Cache expiration:", "6h0m0s"),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "%s", "Parameters of new DNS records and WAF lists:"),
		printItem(t, innerMockPP, "TTL:", "1 (auto)"),
		printItem(t, innerMockPP, "Proxied domains:", "(none)"),
		printItem(t, innerMockPP, "Unproxied domains:", "(none)"),
		printItem(t, innerMockPP, "DNS record comment:", "(empty)"),
		printItem(t, innerMockPP, "WAF list description:", "(empty)"),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "%s", "Timeouts:"),
		printItem(t, innerMockPP, "IP detection:", "5s"),
		printItem(t, innerMockPP, "Record/list updating:", "30s"),
	)
	raw := config.DefaultRaw()
	handleConfig, lifecycleConfig, updateConfig := defaultPrintedConfigs(raw)
	raw.Print(mockPP, handleConfig, lifecycleConfig, updateConfig)
}

//nolint:paralleltest // changing the environment variable TZ
func TestPrintValues(t *testing.T) {
	mockCtrl := gomock.NewController(t)

	store(t, "TZ", "UTC")

	mockPP := mocks.NewMockPP(mockCtrl)
	innerMockPP := mocks.NewMockPP(mockCtrl)
	gomock.InOrder(
		mockPP.EXPECT().IsShowing(pp.Info).Return(true),
		mockPP.EXPECT().Infof(pp.EmojiEnvVars, "Current settings:"),
		mockPP.EXPECT().Indent().Return(mockPP),
		mockPP.EXPECT().Indent().Return(innerMockPP),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "%s", "Domains, IP providers, and WAF lists:"),
		printItem(t, innerMockPP, "IPv4-enabled domains:", "test4.org, *.test4.org"),
		printItem(t, innerMockPP, "IPv4 provider:", "cloudflare.trace"),
		printItem(t, innerMockPP, "IPv6-enabled domains:", "test6.org, *.test6.org"),
		printItem(t, innerMockPP, "IPv6 provider:", "cloudflare.trace"),
		printItem(t, innerMockPP, "WAF lists:", "(none)"),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "%s", "Ownership filters:"),
		printItem(t, innerMockPP, "DNS record comment regex:", "^Created by Cloudflare DDNS$"),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "%s", "Scheduling:"),
		printItem(t, innerMockPP, "Timezone:", gomock.AnyOf("UTC (currently UTC+00)", "Local (currently UTC+00)")),
		printItem(t, innerMockPP, "Update schedule:", "@every 5m"),
		printItem(t, innerMockPP, "Update on start?", "true"),
		printItem(t, innerMockPP, "Delete on stop?", "false"),
		printItem(t, innerMockPP, "Cache expiration:", "6h0m0s"),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "%s", "Parameters of new DNS records and WAF lists:"),
		printItem(t, innerMockPP, "TTL:", "30000"),
		printItem(t, innerMockPP, "Proxied domains:", "a, b"),
		printItem(t, innerMockPP, "Unproxied domains:", "c, d"),
		printItem(t, innerMockPP, "DNS record comment:", "\"Created by Cloudflare DDNS\""),
		printItem(t, innerMockPP, "WAF list description:", "(empty)"),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "%s", "Timeouts:"),
		printItem(t, innerMockPP, "IP detection:", "5s"),
		printItem(t, innerMockPP, "Record/list updating:", "30s"),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "%s", "Monitors:"),
		printItem(t, innerMockPP, "Meow:", "purrrr"),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "%s", "Notification services (via shoutrrr):"),
		printItem(t, innerMockPP, "Snake:", "hissss"),
	)

	raw := config.DefaultRaw()
	raw.RecordComment = "Created by Cloudflare DDNS"
	raw.ManagedRecordsCommentRegexTemplate = "^Created by Cloudflare DDNS$"

	handleConfig, lifecycleConfig, updateConfig := defaultPrintedConfigs(raw)
	updateConfig.Domains[ipnet.IP4] = []domain.Domain{domain.FQDN("test4.org"), domain.Wildcard("test4.org")}
	updateConfig.Domains[ipnet.IP6] = []domain.Domain{domain.FQDN("test6.org"), domain.Wildcard("test6.org")}
	updateConfig.TTL = 30000
	updateConfig.Proxied[domain.FQDN("a")] = true
	updateConfig.Proxied[domain.FQDN("b")] = true
	updateConfig.Proxied[domain.FQDN("c")] = false
	updateConfig.Proxied[domain.FQDN("d")] = false

	m := mocks.NewMockMonitor(mockCtrl)
	m.EXPECT().Describe(gomock.Any()).
		DoAndReturn(func(f func(string, string) bool) {
			f("Meow", "purrrr")
		}).AnyTimes()
	raw.Monitor = m
	lifecycleConfig.Monitor = m

	n := mocks.NewMockNotifier(mockCtrl)
	n.EXPECT().Describe(gomock.Any()).
		DoAndReturn(func(f func(string, string) bool) {
			f("Snake", "hissss")
		}).AnyTimes()
	raw.Notifier = n
	lifecycleConfig.Notifier = n

	raw.Print(mockPP, handleConfig, lifecycleConfig, updateConfig)
}

//nolint:paralleltest // changing the environment variable TZ
func TestPrintCommentRegexQuotedWhenNeeded(t *testing.T) {
	mockCtrl := gomock.NewController(t)

	store(t, "TZ", "UTC")

	mockPP := mocks.NewMockPP(mockCtrl)
	innerMockPP := mocks.NewMockPP(mockCtrl)
	gomock.InOrder(
		mockPP.EXPECT().IsShowing(pp.Info).Return(true),
		mockPP.EXPECT().Infof(pp.EmojiEnvVars, "Current settings:"),
		mockPP.EXPECT().Indent().Return(mockPP),
		mockPP.EXPECT().Indent().Return(innerMockPP),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "%s", "Domains, IP providers, and WAF lists:"),
		printItem(t, innerMockPP, "IPv4-enabled domains:", "(none)"),
		printItem(t, innerMockPP, "IPv4 provider:", "cloudflare.trace"),
		printItem(t, innerMockPP, "IPv6-enabled domains:", "(none)"),
		printItem(t, innerMockPP, "IPv6 provider:", "cloudflare.trace"),
		printItem(t, innerMockPP, "WAF lists:", "(none)"),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "%s", "Ownership filters:"),
		printItem(t, innerMockPP, "DNS record comment regex:", "\"^Created by\\tCloudflare DDNS$\""),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "%s", "Scheduling:"),
		printItem(t, innerMockPP, "Timezone:", gomock.AnyOf("UTC (currently UTC+00)", "Local (currently UTC+00)")),
		printItem(t, innerMockPP, "Update schedule:", "@every 5m"),
		printItem(t, innerMockPP, "Update on start?", "true"),
		printItem(t, innerMockPP, "Delete on stop?", "false"),
		printItem(t, innerMockPP, "Cache expiration:", "6h0m0s"),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "%s", "Parameters of new DNS records and WAF lists:"),
		printItem(t, innerMockPP, "TTL:", "1 (auto)"),
		printItem(t, innerMockPP, "Proxied domains:", "(none)"),
		printItem(t, innerMockPP, "Unproxied domains:", "(none)"),
		printItem(t, innerMockPP, "DNS record comment:", "(empty)"),
		printItem(t, innerMockPP, "WAF list description:", "(empty)"),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "%s", "Timeouts:"),
		printItem(t, innerMockPP, "IP detection:", "5s"),
		printItem(t, innerMockPP, "Record/list updating:", "30s"),
	)

	raw := config.DefaultRaw()
	raw.ManagedRecordsCommentRegexTemplate = "^Created by\tCloudflare DDNS$"

	handleConfig, lifecycleConfig, updateConfig := defaultPrintedConfigs(raw)
	raw.Print(mockPP, handleConfig, lifecycleConfig, updateConfig)
}

//nolint:paralleltest // changing the environment variable TZ
func TestPrintEmpty(t *testing.T) {
	mockCtrl := gomock.NewController(t)

	store(t, "TZ", "UTC")

	mockPP := mocks.NewMockPP(mockCtrl)
	innerMockPP := mocks.NewMockPP(mockCtrl)
	gomock.InOrder(
		mockPP.EXPECT().IsShowing(pp.Info).Return(true),
		mockPP.EXPECT().Infof(pp.EmojiEnvVars, "Current settings:"),
		mockPP.EXPECT().Indent().Return(mockPP),
		mockPP.EXPECT().Indent().Return(innerMockPP),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "%s", "Domains, IP providers, and WAF lists:"),
		printItem(t, innerMockPP, "WAF lists:", "(none)"),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "%s", "Scheduling:"),
		printItem(t, innerMockPP, "Timezone:", gomock.AnyOf("UTC (currently UTC+00)", "Local (currently UTC+00)")),
		printItem(t, innerMockPP, "Update schedule:", "@once"),
		printItem(t, innerMockPP, "Update on start?", "false"),
		printItem(t, innerMockPP, "Delete on stop?", "false"),
		printItem(t, innerMockPP, "Cache expiration:", "0s"),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "%s", "Parameters of new DNS records and WAF lists:"),
		printItem(t, innerMockPP, "TTL:", "0"),
		printItem(t, innerMockPP, "Proxied domains:", "(none)"),
		printItem(t, innerMockPP, "Unproxied domains:", "(none)"),
		printItem(t, innerMockPP, "DNS record comment:", "(empty)"),
		printItem(t, innerMockPP, "WAF list description:", "(empty)"),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "%s", "Timeouts:"),
		printItem(t, innerMockPP, "IP detection:", "0s"),
		printItem(t, innerMockPP, "Record/list updating:", "0s"),
	)
	var raw config.RawConfig
	var handleConfig config.HandleConfig
	var lifecycleConfig config.LifecycleConfig
	var updateConfig config.UpdateConfig
	raw.Print(mockPP, &handleConfig, &lifecycleConfig, &updateConfig)
}

//nolint:paralleltest // environment vars are global
func TestPrintHidden(t *testing.T) {
	mockCtrl := gomock.NewController(t)

	store(t, "TZ", "UTC")

	mockPP := mocks.NewMockPP(mockCtrl)
	mockPP.EXPECT().IsShowing(pp.Info).Return(false)

	var raw config.RawConfig
	var handleConfig config.HandleConfig
	var lifecycleConfig config.LifecycleConfig
	var updateConfig config.UpdateConfig
	raw.Print(mockPP, &handleConfig, &lifecycleConfig, &updateConfig)
}
