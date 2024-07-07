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

func printItem(ppfmt *mocks.MockPP, key string, value any) *mocks.PPInfofCall {
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
		mockPP.EXPECT().IncIndent().Return(mockPP),
		mockPP.EXPECT().IncIndent().Return(innerMockPP),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "Domains and IP providers:"),
		printItem(innerMockPP, "IPv4 domains:", "(none)"),
		printItem(innerMockPP, "IPv4 provider:", "cloudflare.trace"),
		printItem(innerMockPP, "IPv6 domains:", "(none)"),
		printItem(innerMockPP, "IPv6 provider:", "cloudflare.trace"),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "Scheduling:"),
		printItem(innerMockPP, "Timezone:", gomock.AnyOf("UTC (currently UTC+00)", "Local (currently UTC+00)")),
		printItem(innerMockPP, "Update schedule:", "@every 5m"),
		printItem(innerMockPP, "Update on start?", "true"),
		printItem(innerMockPP, "Delete on stop?", "false"),
		printItem(innerMockPP, "Cache expiration:", "6h0m0s"),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "Parameters of new DNS records:"),
		printItem(innerMockPP, "TTL:", "1 (auto)"),
		printItem(innerMockPP, "Proxied domains:", "(none)"),
		printItem(innerMockPP, "Unproxied domains:", "(none)"),
		printItem(innerMockPP, "Record comment:", "(empty)"),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "Timeouts:"),
		printItem(innerMockPP, "IP detection:", "5s"),
		printItem(innerMockPP, "Record updating:", "30s"),
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
		mockPP.EXPECT().IncIndent().Return(mockPP),
		mockPP.EXPECT().IncIndent().Return(innerMockPP),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "Domains and IP providers:"),
		printItem(innerMockPP, "IPv4 domains:", "test4.org, *.test4.org"),
		printItem(innerMockPP, "IPv4 provider:", "cloudflare.trace"),
		printItem(innerMockPP, "IPv6 domains:", "test6.org, *.test6.org"),
		printItem(innerMockPP, "IPv6 provider:", "cloudflare.trace"),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "Scheduling:"),
		printItem(innerMockPP, "Timezone:", gomock.AnyOf("UTC (currently UTC+00)", "Local (currently UTC+00)")),
		printItem(innerMockPP, "Update schedule:", "@every 5m"),
		printItem(innerMockPP, "Update on start?", "true"),
		printItem(innerMockPP, "Delete on stop?", "false"),
		printItem(innerMockPP, "Cache expiration:", "6h0m0s"),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "Parameters of new DNS records:"),
		printItem(innerMockPP, "TTL:", "30000"),
		printItem(innerMockPP, "Proxied domains:", "a, b"),
		printItem(innerMockPP, "Unproxied domains:", "c, d"),
		printItem(innerMockPP, "Record comment:", "\"Created by Cloudflare DDNS\""),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "Timeouts:"),
		printItem(innerMockPP, "IP detection:", "5s"),
		printItem(innerMockPP, "Record updating:", "30s"),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "Monitors:"),
		printItem(innerMockPP, "Meow:", "purrrr"),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "Notification services (via shoutrrr):"),
		printItem(innerMockPP, "Snake:", "hissss"),
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
		mockPP.EXPECT().IncIndent().Return(mockPP),
		mockPP.EXPECT().IncIndent().Return(innerMockPP),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "Domains and IP providers:"),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "Scheduling:"),
		printItem(innerMockPP, "Timezone:", gomock.AnyOf("UTC (currently UTC+00)", "Local (currently UTC+00)")),
		printItem(innerMockPP, "Update schedule:", "@once"),
		printItem(innerMockPP, "Update on start?", "false"),
		printItem(innerMockPP, "Delete on stop?", "false"),
		printItem(innerMockPP, "Cache expiration:", "0s"),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "Parameters of new DNS records:"),
		printItem(innerMockPP, "TTL:", "0"),
		printItem(innerMockPP, "Proxied domains:", "(none)"),
		printItem(innerMockPP, "Unproxied domains:", "(none)"),
		printItem(innerMockPP, "Record comment:", "(empty)"),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "Timeouts:"),
		printItem(innerMockPP, "IP detection:", "0s"),
		printItem(innerMockPP, "Record updating:", "0s"),
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
