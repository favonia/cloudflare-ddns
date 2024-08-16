package config_test

import (
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/config"
	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/monitor"
	"github.com/favonia/cloudflare-ddns/internal/notifier"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func printItem(t *testing.T, ppfmt *mocks.MockPP, key string, value any) *mocks.PPInfofCall {
	t.Helper()
	return ppfmt.EXPECT().Infof(pp.EmojiBullet, "%-*s %s", 24, key, value)
}

//nolint:paralleltest // changing the environment variable TZ
func TestPrintDefault(t *testing.T) {
	mockCtrl := gomock.NewController(t)

	store(t, "TZ", "UTC")

	mockPP := mocks.NewMockPP(mockCtrl)
	innerMockPP := mocks.NewMockPP(mockCtrl)
	gomock.InOrder(
		mockPP.EXPECT().IsEnabledFor(pp.Info).Return(true),
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
	config.Default().Print(mockPP)
}

//nolint:paralleltest // changing the environment variable TZ
func TestPrintValues(t *testing.T) {
	mockCtrl := gomock.NewController(t)

	store(t, "TZ", "UTC")

	mockPP := mocks.NewMockPP(mockCtrl)
	innerMockPP := mocks.NewMockPP(mockCtrl)
	gomock.InOrder(
		mockPP.EXPECT().IsEnabledFor(pp.Info).Return(true),
		mockPP.EXPECT().Infof(pp.EmojiEnvVars, "Current settings:"),
		mockPP.EXPECT().Indent().Return(mockPP),
		mockPP.EXPECT().Indent().Return(innerMockPP),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "%s", "Domains, IP providers, and WAF lists:"),
		printItem(t, innerMockPP, "IPv4-enabled domains:", "test4.org, *.test4.org"),
		printItem(t, innerMockPP, "IPv4 provider:", "cloudflare.trace"),
		printItem(t, innerMockPP, "IPv6-enabled domains:", "test6.org, *.test6.org"),
		printItem(t, innerMockPP, "IPv6 provider:", "cloudflare.trace"),
		printItem(t, innerMockPP, "WAF lists:", "(none)"),
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

	c := config.Default()

	c.Domains[ipnet.IP4] = []domain.Domain{domain.FQDN("test4.org"), domain.Wildcard("test4.org")}
	c.Domains[ipnet.IP6] = []domain.Domain{domain.FQDN("test6.org"), domain.Wildcard("test6.org")}

	c.TTL = 30000

	c.Proxied = map[domain.Domain]bool{}
	c.Proxied[domain.FQDN("a")] = true
	c.Proxied[domain.FQDN("b")] = true
	c.Proxied[domain.FQDN("c")] = false
	c.Proxied[domain.FQDN("d")] = false

	c.RecordComment = "Created by Cloudflare DDNS"

	m := mocks.NewMockMonitor(mockCtrl)
	m.EXPECT().Describe(gomock.Any()).
		DoAndReturn(func(f func(string, string)) {
			f("Meow", "purrrr")
		}).AnyTimes()
	c.Monitors = []monitor.Monitor{m}

	n := mocks.NewMockNotifier(mockCtrl)
	n.EXPECT().Describe(gomock.Any()).
		DoAndReturn(func(f func(string, string)) {
			f("Snake", "hissss")
		}).AnyTimes()
	c.Notifiers = []notifier.Notifier{n}

	c.Print(mockPP)
}

//nolint:paralleltest // changing the environment variable TZ
func TestPrintEmpty(t *testing.T) {
	mockCtrl := gomock.NewController(t)

	store(t, "TZ", "UTC")

	mockPP := mocks.NewMockPP(mockCtrl)
	innerMockPP := mocks.NewMockPP(mockCtrl)
	gomock.InOrder(
		mockPP.EXPECT().IsEnabledFor(pp.Info).Return(true),
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
	var cfg config.Config
	cfg.Print(mockPP)
}

//nolint:paralleltest // environment vars are global
func TestPrintHidden(t *testing.T) {
	mockCtrl := gomock.NewController(t)

	store(t, "TZ", "UTC")

	mockPP := mocks.NewMockPP(mockCtrl)
	mockPP.EXPECT().IsEnabledFor(pp.Info).Return(false)

	var cfg config.Config
	cfg.Print(mockPP)
}
