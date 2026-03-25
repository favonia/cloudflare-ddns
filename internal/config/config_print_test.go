package config_test

import (
	"regexp"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/config"
	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/heartbeat"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/notifier"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider"
)

func printItem(t *testing.T, ppfmt *mocks.MockPP, key string, value any) *mocks.MockPPInfofCall {
	t.Helper()
	return ppfmt.EXPECT().Infof(pp.EmojiBullet, "%-*s %s", 28, key, value)
}

func defaultPrintedConfig(raw *config.RawConfig) *config.BuiltConfig {
	handleConfig := &config.HandleConfig{} //nolint:exhaustruct // This helper intentionally starts from the zero value and fills only the fields print tests use.
	handleConfig.Auth = raw.Auth
	handleConfig.Options.CacheExpiration = raw.CacheExpiration
	handleConfig.Options.ManagedRecordsCommentRegex = regexp.MustCompile(raw.ManagedRecordsCommentRegex)
	handleConfig.Options.ManagedWAFListItemsCommentRegex = regexp.MustCompile(raw.ManagedWAFListItemsCommentRegex)

	lifecycleConfig := &config.LifecycleConfig{} //nolint:exhaustruct // This helper intentionally starts from the zero value and fills only the fields print tests use.
	lifecycleConfig.UpdateCron = raw.UpdateCron
	lifecycleConfig.UpdateOnStart = raw.UpdateOnStart
	lifecycleConfig.DeleteOnStop = raw.DeleteOnStop

	updateConfig := &config.UpdateConfig{} //nolint:exhaustruct // This helper intentionally starts from the zero value and fills only the fields print tests use.
	updateConfig.Provider = map[ipnet.Family]provider.Provider{
		ipnet.IP4: raw.Provider[ipnet.IP4],
		ipnet.IP6: raw.Provider[ipnet.IP6],
	}
	updateConfig.Domains = map[ipnet.Family][]domain.Domain{
		ipnet.IP4: nil,
		ipnet.IP6: nil,
	}
	updateConfig.WAFLists = raw.WAFLists
	updateConfig.TTL = raw.TTL
	updateConfig.Proxied = map[domain.Domain]bool{}
	updateConfig.RecordComment = raw.RecordComment
	updateConfig.WAFListDescription = raw.WAFListDescription
	updateConfig.WAFListItemComment = raw.WAFListItemComment
	updateConfig.DefaultPrefixLen = map[ipnet.Family]int{
		ipnet.IP4: raw.IP4DefaultPrefixLen,
		ipnet.IP6: raw.IP6DefaultPrefixLen,
	}
	updateConfig.DetectionTimeout = raw.DetectionTimeout
	updateConfig.UpdateTimeout = raw.UpdateTimeout

	return &config.BuiltConfig{
		Handle:    handleConfig,
		Lifecycle: lifecycleConfig,
		Update:    updateConfig,
	}
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
		printItem(t, innerMockPP, "IPv4 default prefix length:", "/32"),
		printItem(t, innerMockPP, "IPv6-enabled domains:", "(none)"),
		printItem(t, innerMockPP, "IPv6 provider:", "cloudflare.trace"),
		printItem(t, innerMockPP, "IPv6 default prefix length:", "/64"),
		printItem(t, innerMockPP, "WAF lists:", "(none)"),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "%s", "Scheduling:"),
		printItem(t, innerMockPP, "Timezone:", gomock.AnyOf("UTC (currently UTC+00)", "Local (currently UTC+00)")),
		printItem(t, innerMockPP, "Update schedule:", "@every 5m"),
		printItem(t, innerMockPP, "Update on start?", "true"),
		printItem(t, innerMockPP, "Delete on stop?", "false"),
		printItem(t, innerMockPP, "Cache expiration:", "6h0m0s"),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "%s", "DNS and WAF fallback values:"),
		printItem(t, innerMockPP, "TTL:", "1 (auto)"),
		printItem(t, innerMockPP, "Proxied domains:", "(none)"),
		printItem(t, innerMockPP, "Unproxied domains:", "(none)"),
		printItem(t, innerMockPP, "DNS record comment:", "(empty)"),
		printItem(t, innerMockPP, "WAF list description:", "(empty)"),
		printItem(t, innerMockPP, "WAF list item comment:", "(empty)"),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "%s", "Timeouts:"),
		printItem(t, innerMockPP, "IP detection:", "5s"),
		printItem(t, innerMockPP, "Record/list updating:", "30s"),
	)
	raw := config.DefaultRaw()
	builtConfig := defaultPrintedConfig(raw)
	config.Print(mockPP, builtConfig, heartbeat.NewComposed(), notifier.NewComposed())
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
		printItem(t, innerMockPP, "IPv4 default prefix length:", "/32"),
		printItem(t, innerMockPP, "IPv6-enabled domains:", "test6.org, *.test6.org"),
		printItem(t, innerMockPP, "IPv6 provider:", "cloudflare.trace"),
		printItem(t, innerMockPP, "IPv6 default prefix length:", "/64"),
		printItem(t, innerMockPP, "WAF lists:", "(none)"),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "%s", "Ownership filters:"),
		printItem(t, innerMockPP, "DNS record comment regex:", "^Created by Cloudflare DDNS$"),
		printItem(t, innerMockPP, "WAF list item comment regex:", "^managed-waf-item$"),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "%s", "Scheduling:"),
		printItem(t, innerMockPP, "Timezone:", gomock.AnyOf("UTC (currently UTC+00)", "Local (currently UTC+00)")),
		printItem(t, innerMockPP, "Update schedule:", "@every 5m"),
		printItem(t, innerMockPP, "Update on start?", "true"),
		printItem(t, innerMockPP, "Delete on stop?", "false"),
		printItem(t, innerMockPP, "Cache expiration:", "6h0m0s"),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "%s", "DNS and WAF fallback values:"),
		printItem(t, innerMockPP, "TTL:", "30000"),
		printItem(t, innerMockPP, "Proxied domains:", "a, b"),
		printItem(t, innerMockPP, "Unproxied domains:", "c, d"),
		printItem(t, innerMockPP, "DNS record comment:", "\"Created by Cloudflare DDNS\""),
		printItem(t, innerMockPP, "WAF list description:", "(empty)"),
		printItem(t, innerMockPP, "WAF list item comment:", "\"managed-waf-item\""),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "%s", "Timeouts:"),
		printItem(t, innerMockPP, "IP detection:", "5s"),
		printItem(t, innerMockPP, "Record/list updating:", "30s"),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "%s", "Heartbeats:"),
		printItem(t, innerMockPP, "Meow:", "purrrr"),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "%s", "Notification services (via shoutrrr):"),
		printItem(t, innerMockPP, "Snake:", "hissss"),
	)

	raw := config.DefaultRaw()
	raw.RecordComment = "Created by Cloudflare DDNS"
	raw.ManagedRecordsCommentRegex = "^Created by Cloudflare DDNS$"
	raw.WAFListItemComment = "managed-waf-item"
	raw.ManagedWAFListItemsCommentRegex = "^managed-waf-item$"

	builtConfig := defaultPrintedConfig(raw)
	builtConfig.Update.Domains[ipnet.IP4] = []domain.Domain{domain.FQDN("test4.org"), domain.Wildcard("test4.org")}
	builtConfig.Update.Domains[ipnet.IP6] = []domain.Domain{domain.FQDN("test6.org"), domain.Wildcard("test6.org")}
	builtConfig.Update.TTL = 30000
	builtConfig.Update.Proxied[domain.FQDN("a")] = true
	builtConfig.Update.Proxied[domain.FQDN("b")] = true
	builtConfig.Update.Proxied[domain.FQDN("c")] = false
	builtConfig.Update.Proxied[domain.FQDN("d")] = false

	hb := mocks.NewMockHeartbeat(mockCtrl)
	hb.EXPECT().Describe(gomock.Any()).
		DoAndReturn(func(f func(string, string) bool) {
			f("Meow", "purrrr")
		}).AnyTimes()

	n := mocks.NewMockNotifier(mockCtrl)
	n.EXPECT().Describe(gomock.Any()).
		DoAndReturn(func(f func(string, string) bool) {
			f("Snake", "hissss")
		}).AnyTimes()

	config.Print(mockPP, builtConfig, hb, n)
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
		printItem(t, innerMockPP, "IPv4 default prefix length:", "/32"),
		printItem(t, innerMockPP, "IPv6-enabled domains:", "(none)"),
		printItem(t, innerMockPP, "IPv6 provider:", "cloudflare.trace"),
		printItem(t, innerMockPP, "IPv6 default prefix length:", "/64"),
		printItem(t, innerMockPP, "WAF lists:", "(none)"),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "%s", "Ownership filters:"),
		printItem(t, innerMockPP, "DNS record comment regex:", "\"^Created by\\tCloudflare DDNS$\""),
		printItem(t, innerMockPP, "WAF list item comment regex:", "\"^managed\\twaf$\""),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "%s", "Scheduling:"),
		printItem(t, innerMockPP, "Timezone:", gomock.AnyOf("UTC (currently UTC+00)", "Local (currently UTC+00)")),
		printItem(t, innerMockPP, "Update schedule:", "@every 5m"),
		printItem(t, innerMockPP, "Update on start?", "true"),
		printItem(t, innerMockPP, "Delete on stop?", "false"),
		printItem(t, innerMockPP, "Cache expiration:", "6h0m0s"),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "%s", "DNS and WAF fallback values:"),
		printItem(t, innerMockPP, "TTL:", "1 (auto)"),
		printItem(t, innerMockPP, "Proxied domains:", "(none)"),
		printItem(t, innerMockPP, "Unproxied domains:", "(none)"),
		printItem(t, innerMockPP, "DNS record comment:", "(empty)"),
		printItem(t, innerMockPP, "WAF list description:", "(empty)"),
		printItem(t, innerMockPP, "WAF list item comment:", "(empty)"),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "%s", "Timeouts:"),
		printItem(t, innerMockPP, "IP detection:", "5s"),
		printItem(t, innerMockPP, "Record/list updating:", "30s"),
	)

	raw := config.DefaultRaw()
	raw.ManagedRecordsCommentRegex = "^Created by\tCloudflare DDNS$"
	raw.ManagedWAFListItemsCommentRegex = "^managed\twaf$"

	builtConfig := defaultPrintedConfig(raw)
	config.Print(mockPP, builtConfig, heartbeat.NewComposed(), notifier.NewComposed())
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
		mockPP.EXPECT().Infof(pp.EmojiConfig, "%s", "DNS and WAF fallback values:"),
		printItem(t, innerMockPP, "TTL:", "0"),
		printItem(t, innerMockPP, "Proxied domains:", "(none)"),
		printItem(t, innerMockPP, "Unproxied domains:", "(none)"),
		printItem(t, innerMockPP, "DNS record comment:", "(empty)"),
		printItem(t, innerMockPP, "WAF list description:", "(empty)"),
		printItem(t, innerMockPP, "WAF list item comment:", "(empty)"),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "%s", "Timeouts:"),
		printItem(t, innerMockPP, "IP detection:", "0s"),
		printItem(t, innerMockPP, "Record/list updating:", "0s"),
	)
	builtConfig := &config.BuiltConfig{
		Handle:    &config.HandleConfig{},    //nolint:exhaustruct
		Lifecycle: &config.LifecycleConfig{}, //nolint:exhaustruct
		Update:    &config.UpdateConfig{},    //nolint:exhaustruct
	}
	config.Print(mockPP, builtConfig, heartbeat.NewComposed(), notifier.NewComposed())
}

//nolint:paralleltest // environment vars are global
func TestPrintHidden(t *testing.T) {
	mockCtrl := gomock.NewController(t)

	store(t, "TZ", "UTC")

	mockPP := mocks.NewMockPP(mockCtrl)
	mockPP.EXPECT().IsShowing(pp.Info).Return(false)

	builtConfig := &config.BuiltConfig{
		Handle:    &config.HandleConfig{},    //nolint:exhaustruct
		Lifecycle: &config.LifecycleConfig{}, //nolint:exhaustruct
		Update:    &config.UpdateConfig{},    //nolint:exhaustruct
	}
	config.Print(mockPP, builtConfig, nil, nil)
}
