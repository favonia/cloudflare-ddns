package config_test

import (
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/config"
	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/monitor"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

type someMatcher struct {
	matchers []gomock.Matcher
}

func (sm someMatcher) Matches(x any) bool {
	for _, m := range sm.matchers {
		if m.Matches(x) {
			return true
		}
	}
	return false
}

func (sm someMatcher) String() string {
	ss := make([]string, 0, len(sm.matchers))
	for _, matcher := range sm.matchers {
		ss = append(ss, matcher.String())
	}
	return strings.Join(ss, " | ")
}

func Some(xs ...any) gomock.Matcher {
	ms := make([]gomock.Matcher, 0, len(xs))
	for _, x := range xs {
		if m, ok := x.(gomock.Matcher); ok {
			ms = append(ms, m)
		} else {
			ms = append(ms, gomock.Eq(x))
		}
	}
	return someMatcher{ms}
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
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "%-*s %s", 24, "IPv4 domains:", "(none)"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "%-*s %s", 24, "IPv4 provider:", "cloudflare.trace"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "%-*s %s", 24, "IPv6 domains:", "(none)"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "%-*s %s", 24, "IPv6 provider:", "cloudflare.trace"),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "Scheduling:"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "%-*s %s", 24, "Timezone:", Some("UTC (UTC+00 now)", "Local (UTC+00 now)")), //nolint:lll
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "%-*s %s", 24, "Update frequency:", "@every 5m"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "%-*s %s", 24, "Update on start?", "true"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "%-*s %s", 24, "Delete on stop?", "false"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "%-*s %s", 24, "Cache expiration:", "6h0m0s"),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "New DNS records:"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "%-*s %s", 24, "TTL:", "1 (auto)"),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "Timeouts:"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "%-*s %s", 24, "IP detection:", "5s"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "%-*s %s", 24, "Record updating:", "30s"),
	)
	config.Default().Print(mockPP)
}

//nolint:paralleltest // changing the environment variable TZ
func TestPrintMaps(t *testing.T) {
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
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "%-*s %s", 24, "IPv4 domains:", "test4.org, *.test4.org"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "%-*s %s", 24, "IPv4 provider:", "cloudflare.trace"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "%-*s %s", 24, "IPv6 domains:", "test6.org, *.test6.org"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "%-*s %s", 24, "IPv6 provider:", "cloudflare.trace"),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "Scheduling:"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "%-*s %s", 24, "Timezone:", Some("UTC (UTC+00 now)", "Local (UTC+00 now)")), //nolint:lll
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "%-*s %s", 24, "Update frequency:", "@every 5m"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "%-*s %s", 24, "Update on start?", "true"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "%-*s %s", 24, "Delete on stop?", "false"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "%-*s %s", 24, "Cache expiration:", "6h0m0s"),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "New DNS records:"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "%-*s %s", 24, "TTL:", "30000"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "%-*s %s", 24, "Proxied domains:", "a, b"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "%-*s %s", 24, "Unproxied domains:", "c, d"),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "Timeouts:"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "%-*s %s", 24, "IP detection:", "5s"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "%-*s %s", 24, "Record updating:", "30s"),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "Monitors:"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "%-*s %s", 24, "Healthchecks:", "(URL redacted)"),
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

	m, ok := monitor.NewHealthchecks(mockPP, "https://user:pass@host/path")
	require.True(t, ok)
	c.Monitors = []monitor.Monitor{m}

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
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "%-*s %s", 24, "Timezone:", Some("UTC (UTC+00 now)", "Local (UTC+00 now)")), //nolint:lll
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "%-*s %s", 24, "Update frequency:", "@disabled"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "%-*s %s", 24, "Update on start?", "false"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "%-*s %s", 24, "Delete on stop?", "false"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "%-*s %s", 24, "Cache expiration:", "0s"),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "New DNS records:"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "%-*s %s", 24, "TTL:", "0"),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "Timeouts:"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "%-*s %s", 24, "IP detection:", "0s"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "%-*s %s", 24, "Record updating:", "0s"),
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
