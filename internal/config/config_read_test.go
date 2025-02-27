// vim: nowrap
package config_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/config"
	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider"
)

func unsetAll(t *testing.T) {
	t.Helper()
	unset(t,
		"CLOUDFLARE_API_TOKEN", "CLOUDFLARE_API_TOKEN_FILE",
		"CF_API_TOKEN", "CF_API_TOKEN_FILE", "CF_ACCOUNT_ID",
		"IP4_PROVIDER", "IP6_PROVIDER",
		"DOMAINS", "IP4_DOMAINS", "IP6_DOMAINS", "WAF_LISTS",
		"IP6_PREFIX_LEN",
		"UPDATE_CRON",
		"UPDATE_ON_START",
		"DELETE_ON_STOP",
		"CACHE_EXPIRATION",
		"TTL",
		"PROXIED",
		"RECORD_COMMENT",
		"WAF_LIST_DESCRIPTION",
		"DETECTION_TIMEOUT",
		"UPDATE_TIMEOUT",
		"HEALTHCHECKS",
		"UPTIMEKUMA",
		"SHOUTRRR",
	)
}

func TestConstant(t *testing.T) {
	t.Parallel()
	// The normalization code assumes these constants. It may require updates.
	require.Equal(t, 12, api.WAFListMinPrefixLen[ipnet.IP6])
	require.Equal(t, 64, api.WAFListMaxPrefixLen[ipnet.IP6])
}

//nolint:paralleltest // environment variables are global
func TestReadEnvWithOnlyToken(t *testing.T) {
	mockCtrl := gomock.NewController(t)

	unsetAll(t)
	store(t, "CLOUDFLARE_API_TOKEN", "deadbeaf")

	var cfg config.Config
	mockPP := mocks.NewMockPP(mockCtrl)
	innerMockPP := mocks.NewMockPP(mockCtrl)
	gomock.InOrder(
		mockPP.EXPECT().IsShowing(pp.Info).Return(true),
		mockPP.EXPECT().Infof(pp.EmojiEnvVars, "Reading settings . . ."),
		mockPP.EXPECT().Indent().Return(innerMockPP),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Use default %s=%s", "IP4_PROVIDER", "none"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Use default %s=%s", "IP6_PROVIDER", "none"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Use default %s=%d", "IP6_PREFIX_LEN", 0),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Use default %s=%s", "UPDATE_CRON", "@once"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Use default %s=%t", "UPDATE_ON_START", false),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Use default %s=%t", "DELETE_ON_STOP", false),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Use default %s=%v", "CACHE_EXPIRATION", time.Duration(0)),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Use default %s=%d", "TTL", api.TTL(0)),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Use default %s=%v", "DETECTION_TIMEOUT", time.Duration(0)),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Use default %s=%v", "UPDATE_TIMEOUT", time.Duration(0)),
	)
	ok := cfg.ReadEnv(mockPP)
	require.True(t, ok)
}

//nolint:paralleltest // environment variables are global
func TestReadEnvEmpty(t *testing.T) {
	mockCtrl := gomock.NewController(t)

	unsetAll(t)

	var cfg config.Config
	mockPP := mocks.NewMockPP(mockCtrl)
	innerMockPP := mocks.NewMockPP(mockCtrl)
	gomock.InOrder(
		mockPP.EXPECT().IsShowing(pp.Info).Return(true),
		mockPP.EXPECT().Infof(pp.EmojiEnvVars, "Reading settings . . ."),
		mockPP.EXPECT().Indent().Return(innerMockPP),
		innerMockPP.EXPECT().Noticef(pp.EmojiUserError,
			"Needs either %s or %s", "CLOUDFLARE_API_TOKEN", "CLOUDFLARE_API_TOKEN_FILE"),
	)
	ok := cfg.ReadEnv(mockPP)
	require.False(t, ok)
}

func TestNormalize(t *testing.T) {
	t.Parallel()

	keyProxied := "PROXIED"

	for name, tc := range map[string]struct {
		input         *config.Config
		ok            bool
		expected      *config.Config
		prepareMockPP func(m *mocks.MockPP)
	}{
		"nothing-to-do": {
			input: &config.Config{ //nolint:exhaustruct
				IP6PrefixLen: 64,
			},
			ok:       false,
			expected: nil,
			prepareMockPP: func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().IsShowing(pp.Info).Return(true),
					m.EXPECT().Infof(pp.EmojiEnvVars, "Checking settings . . ."),
					m.EXPECT().Indent().Return(m),
					m.EXPECT().Noticef(pp.EmojiUserError, "Nothing was specified in DOMAINS, IP4_DOMAINS, IP6_DOMAINS, or WAF_LISTS"),
				)
			},
		},
		"update-cron-once/update-on-start": {
			input: &config.Config{ //nolint:exhaustruct
				Domains: map[ipnet.Type][]domain.Domain{
					ipnet.IP4: {domain.FQDN("a.b.c")},
				},
				IP6PrefixLen:  64,
				UpdateOnStart: false,
			},
			ok:       false,
			expected: nil,
			prepareMockPP: func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().IsShowing(pp.Info).Return(true),
					m.EXPECT().Infof(pp.EmojiEnvVars, "Checking settings . . ."),
					m.EXPECT().Indent().Return(m),
					m.EXPECT().Noticef(pp.EmojiUserError, "UPDATE_ON_START=false is incompatible with UPDATE_CRON=@once"),
				)
			},
		},
		"update-cron-once/delete-on-stop": {
			input: &config.Config{ //nolint:exhaustruct
				Domains: map[ipnet.Type][]domain.Domain{
					ipnet.IP4: {domain.FQDN("a.b.c")},
				},
				IP6PrefixLen:  64,
				DeleteOnStop:  true,
				UpdateOnStart: true,
			},
			ok:       false,
			expected: nil,
			prepareMockPP: func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().IsShowing(pp.Info).Return(true),
					m.EXPECT().Infof(pp.EmojiEnvVars, "Checking settings . . ."),
					m.EXPECT().Indent().Return(m),
					m.EXPECT().Noticef(pp.EmojiUserError, "DELETE_ON_STOP=true will immediately delete all domains and WAF lists when UPDATE_CRON=@once"),
				)
			},
		},
		"nil-provider": {
			input: &config.Config{ //nolint:exhaustruct
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP4: nil,
					ipnet.IP6: nil,
				},
				Domains: map[ipnet.Type][]domain.Domain{
					ipnet.IP4: {domain.FQDN("a.b.c")},
				},
				IP6PrefixLen:    64,
				UpdateOnStart:   true,
				ProxiedTemplate: "false",
			},
			ok:       false,
			expected: nil,
			prepareMockPP: func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().IsShowing(pp.Info).Return(true),
					m.EXPECT().Infof(pp.EmojiEnvVars, "Checking settings . . ."),
					m.EXPECT().Indent().Return(m),
					m.EXPECT().Noticef(pp.EmojiUserError, "Nothing to update because both IP4_PROVIDER and IP6_PROVIDER are %q", "none"),
				)
			},
		},
		"dns6-empty": {
			input: &config.Config{ //nolint:exhaustruct
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP4: provider.NewCloudflareTrace(),
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				Domains: map[ipnet.Type][]domain.Domain{
					ipnet.IP4: {domain.FQDN("a.b.c")},
				},
				IP6PrefixLen:     64,
				UpdateOnStart:    true,
				ProxiedTemplate:  "false",
				DetectionTimeout: 5 * time.Second,
			},
			ok: true,
			expected: &config.Config{ //nolint:exhaustruct
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP4: provider.NewCloudflareTrace(),
				},
				Domains: map[ipnet.Type][]domain.Domain{
					ipnet.IP4: {domain.FQDN("a.b.c")},
				},
				IP6PrefixLen:     64,
				IP6HostID:        map[domain.Domain]ipnet.HostID{},
				WAFListPrefixLen: map[ipnet.Type]int{},
				UpdateOnStart:    true,
				ProxiedTemplate:  "false",
				Proxied: map[domain.Domain]bool{
					domain.FQDN("a.b.c"): false,
				},
				DetectionTimeout: 5 * time.Second,
			},
			prepareMockPP: func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().IsShowing(pp.Info).Return(true),
					m.EXPECT().Infof(pp.EmojiEnvVars, "Checking settings . . ."),
					m.EXPECT().Indent().Return(m),
					m.EXPECT().Noticef(pp.EmojiUserWarning, "IP%d_PROVIDER was changed to %q because no domains or WAF lists use %s", 6, "none", "IPv6"),
				)
			},
		},
		"dns6-empty/ip4-none": {
			input: &config.Config{ //nolint:exhaustruct
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				Domains: map[ipnet.Type][]domain.Domain{
					ipnet.IP4: {domain.FQDN("a.b.c")},
				},
				IP6PrefixLen:  64,
				UpdateOnStart: true,
			},
			ok:       false,
			expected: nil,
			prepareMockPP: func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().IsShowing(pp.Info).Return(true),
					m.EXPECT().Infof(pp.EmojiEnvVars, "Checking settings . . ."),
					m.EXPECT().Indent().Return(m),
					m.EXPECT().Noticef(pp.EmojiUserWarning, "IP%d_PROVIDER was changed to %q because no domains or WAF lists use %s", 6, "none", "IPv6"),
					m.EXPECT().Noticef(pp.EmojiUserError, "Nothing to update because both IP4_PROVIDER and IP6_PROVIDER are %q", "none"),
				)
			},
		},
		"ip4-none": {
			input: &config.Config{ //nolint:exhaustruct
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				Domains: map[ipnet.Type][]domain.Domain{
					ipnet.IP4: {domain.FQDN("a.b.c"), domain.FQDN("d.e.f")},
					ipnet.IP6: {domain.FQDN("a.b.c"), domain.FQDN("g.h.i")},
				},
				IP6PrefixLen:     64,
				UpdateOnStart:    true,
				ProxiedTemplate:  "false",
				DetectionTimeout: 5 * time.Second,
			},
			ok: true,
			expected: &config.Config{ //nolint:exhaustruct
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				Domains: map[ipnet.Type][]domain.Domain{
					ipnet.IP4: {domain.FQDN("a.b.c"), domain.FQDN("d.e.f")},
					ipnet.IP6: {domain.FQDN("a.b.c"), domain.FQDN("g.h.i")},
				},
				IP6PrefixLen:     64,
				IP6HostID:        map[domain.Domain]ipnet.HostID{},
				WAFListPrefixLen: map[ipnet.Type]int{},
				UpdateOnStart:    true,
				ProxiedTemplate:  "false",
				Proxied: map[domain.Domain]bool{
					domain.FQDN("a.b.c"): false,
					domain.FQDN("g.h.i"): false,
				},
				DetectionTimeout: 5 * time.Second,
			},
			prepareMockPP: func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().IsShowing(pp.Info).Return(true),
					m.EXPECT().Infof(pp.EmojiEnvVars, "Checking settings . . ."),
					m.EXPECT().Indent().Return(m),
					m.EXPECT().Noticef(pp.EmojiUserWarning, "The domain %q is ignored because it is only for %s but %s is disabled", "d.e.f", "IPv4", "IPv4"),
				)
			},
		},
		"host": {
			input: &config.Config{ //nolint:exhaustruct
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP4: provider.NewCloudflareTrace(),
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				Domains: map[ipnet.Type][]domain.Domain{
					ipnet.IP4: {domain.FQDN("a.b.c"), domain.FQDN("d.e.f")},
					ipnet.IP6: {domain.FQDN("a.b.c"), domain.FQDN("g.h.i"), domain.FQDN("j.k.l"), domain.FQDN("m.n.o")},
				},
				IP6PrefixLen: 56,
				IP6HostID: map[domain.Domain]ipnet.HostID{
					domain.FQDN("a.b.c"): ipnet.IP6Suffix{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
					domain.FQDN("g.h.i"): ipnet.EUI48{0, 1, 2, 3, 4, 5},
					domain.FQDN("j.k.l"): nil,
				},
				UpdateOnStart:    true,
				ProxiedTemplate:  "false",
				TTL:              api.TTLAuto,
				DetectionTimeout: 5 * time.Second,
			},
			ok: true,
			expected: &config.Config{ //nolint:exhaustruct
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP4: provider.NewCloudflareTrace(),
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				Domains: map[ipnet.Type][]domain.Domain{
					ipnet.IP4: {domain.FQDN("a.b.c"), domain.FQDN("d.e.f")},
					ipnet.IP6: {domain.FQDN("a.b.c"), domain.FQDN("g.h.i"), domain.FQDN("j.k.l"), domain.FQDN("m.n.o")},
				},
				IP6PrefixLen:     56,
				WAFListPrefixLen: map[ipnet.Type]int{},
				IP6HostID: map[domain.Domain]ipnet.HostID{
					domain.FQDN("a.b.c"): ipnet.IP6Suffix{0, 0, 0, 0, 0, 0, 0, 7, 8, 9, 10, 11, 12, 13, 14, 15},
					domain.FQDN("g.h.i"): ipnet.EUI48{0, 1, 2, 3, 4, 5},
				},
				UpdateOnStart:   true,
				ProxiedTemplate: "false",
				Proxied: map[domain.Domain]bool{
					domain.FQDN("a.b.c"): false,
					domain.FQDN("d.e.f"): false,
					domain.FQDN("g.h.i"): false,
					domain.FQDN("j.k.l"): false,
					domain.FQDN("m.n.o"): false,
				},
				TTL:              api.TTLAuto,
				DetectionTimeout: 5 * time.Second,
			},
			prepareMockPP: func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().IsShowing(pp.Info).Return(true),
					m.EXPECT().Infof(pp.EmojiEnvVars, "Checking settings . . ."),
					m.EXPECT().Indent().Return(m),
					m.EXPECT().Infof(pp.EmojiTruncate, "The host ID %q of %q was truncated to %q (with %d higher bits removed)", "1:203:405:607:809:a0b:c0d:e0f", "a.b.c", "::7:809:a0b:c0d:e0f", 56),
				)
			},
		},
		"host/large-prefix-len": {
			input: &config.Config{ //nolint:exhaustruct
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP4: provider.NewCloudflareTrace(),
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				Domains: map[ipnet.Type][]domain.Domain{
					ipnet.IP4: {domain.FQDN("a.b.c"), domain.FQDN("d.e.f")},
					ipnet.IP6: {domain.FQDN("a.b.c"), domain.FQDN("g.h.i")},
				},
				IP6PrefixLen: 96,
				IP6HostID: map[domain.Domain]ipnet.HostID{
					domain.FQDN("a.b.c"): ipnet.IP6Suffix{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
					domain.FQDN("g.h.i"): ipnet.EUI48{0, 1, 2, 3, 4, 5},
				},
				UpdateOnStart:    true,
				ProxiedTemplate:  "false",
				TTL:              api.TTLAuto,
				DetectionTimeout: 5 * time.Second,
			},
			ok:       false,
			expected: nil,
			prepareMockPP: func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().IsShowing(pp.Info).Return(true),
					m.EXPECT().Infof(pp.EmojiEnvVars, "Checking settings . . ."),
					m.EXPECT().Indent().Return(m),
					m.EXPECT().Infof(pp.EmojiTruncate, "The host ID %q of %q was truncated to %q (with %d higher bits removed)", "1:203:405:607:809:a0b:c0d:e0f", "a.b.c", "::c0d:e0f", 96),
					m.EXPECT().Noticef(pp.EmojiUserError, "IP6_PREFIX_LEN (%d) is too large (> 64) to use the MAC (EUI-48) address %q as the IPv6 host ID of %q. Converting a MAC address to a host ID requires IPv6 Stateless Address Auto-configuration (SLAAC), which necessitates an IPv6 range of size at least /64 (represented by a prefix length at most 64).", 96, "00:01:02:03:04:05", "g.h.i"),
				)
			},
		},
		"host/no-ip6-provider": {
			input: &config.Config{ //nolint:exhaustruct
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP4: provider.NewCloudflareTrace(),
				},
				Domains: map[ipnet.Type][]domain.Domain{
					ipnet.IP4: {domain.FQDN("a.b.c")},
					ipnet.IP6: {domain.FQDN("a.b.c"), domain.FQDN("d.e.f")},
				},
				IP6PrefixLen: 60,
				IP6HostID: map[domain.Domain]ipnet.HostID{
					domain.FQDN("a.b.c"): ipnet.EUI48{0, 1, 2, 3, 4, 5},
					domain.FQDN("d.e.f"): ipnet.EUI48{6, 7, 8, 9, 10, 11},
				},
				UpdateOnStart:    true,
				ProxiedTemplate:  "false",
				TTL:              api.TTLAuto,
				DetectionTimeout: 5 * time.Second,
			},
			ok: true,
			expected: &config.Config{ //nolint:exhaustruct
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP4: provider.NewCloudflareTrace(),
				},
				Domains: map[ipnet.Type][]domain.Domain{
					ipnet.IP4: {domain.FQDN("a.b.c")},
					ipnet.IP6: {domain.FQDN("a.b.c"), domain.FQDN("d.e.f")},
				},
				IP6PrefixLen:     60,
				IP6HostID:        map[domain.Domain]ipnet.HostID{},
				WAFListPrefixLen: map[ipnet.Type]int{},
				UpdateOnStart:    true,
				ProxiedTemplate:  "false",
				Proxied: map[domain.Domain]bool{
					domain.FQDN("a.b.c"): false,
				},
				TTL:              api.TTLAuto,
				DetectionTimeout: 5 * time.Second,
			},
			prepareMockPP: func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().IsShowing(pp.Info).Return(true),
					m.EXPECT().Infof(pp.EmojiEnvVars, "Checking settings . . ."),
					m.EXPECT().Indent().Return(m),
					m.EXPECT().Noticef(pp.EmojiUserWarning, "The domain %q is ignored because it is only for %s but %s is disabled", "d.e.f", "IPv6", "IPv6"),
					m.EXPECT().Noticef(pp.EmojiUserWarning, "The host ID %q of %q is ignored because IPv6 is disabled", "00:01:02:03:04:05", "a.b.c"),
					m.EXPECT().Noticef(pp.EmojiUserWarning, "IP6_PREFIX_LEN=%d is ignored because no domains use IPv6 host IDs", 60),
				)
			},
		},
		"waf": {
			input: &config.Config{ //nolint:exhaustruct
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP4: provider.NewCloudflareTrace(),
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				Domains:            map[ipnet.Type][]domain.Domain{},
				WAFLists:           []api.WAFList{{AccountID: "account", Name: "list"}},
				IP6PrefixLen:       64,
				UpdateOnStart:      true,
				ProxiedTemplate:    "false",
				TTL:                api.TTLAuto,
				WAFListDescription: "My list",
				DetectionTimeout:   5 * time.Second,
			},
			ok: true,
			expected: &config.Config{ //nolint:exhaustruct
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP4: provider.NewCloudflareTrace(),
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				Domains:            map[ipnet.Type][]domain.Domain{},
				WAFLists:           []api.WAFList{{AccountID: "account", Name: "list"}},
				IP6PrefixLen:       64,
				IP6HostID:          map[domain.Domain]ipnet.HostID{},
				WAFListPrefixLen:   map[ipnet.Type]int{ipnet.IP4: 32, ipnet.IP6: 64},
				UpdateOnStart:      true,
				ProxiedTemplate:    "false",
				Proxied:            map[domain.Domain]bool{},
				TTL:                api.TTLAuto,
				WAFListDescription: "My list",
				DetectionTimeout:   5 * time.Second,
			},
			prepareMockPP: func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().IsShowing(pp.Info).Return(true),
					m.EXPECT().Infof(pp.EmojiEnvVars, "Checking settings . . ."),
					m.EXPECT().Indent().Return(m),
				)
			},
		},
		"waf/large-prefix-len": {
			input: &config.Config{ //nolint:exhaustruct
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				Domains:            map[ipnet.Type][]domain.Domain{},
				WAFLists:           []api.WAFList{{AccountID: "account", Name: "list"}},
				IP6PrefixLen:       96,
				UpdateOnStart:      true,
				ProxiedTemplate:    "false",
				TTL:                api.TTLAuto,
				WAFListDescription: "My list",
				DetectionTimeout:   5 * time.Second,
			},
			ok: true,
			expected: &config.Config{ //nolint:exhaustruct
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				Domains:            map[ipnet.Type][]domain.Domain{},
				WAFLists:           []api.WAFList{{AccountID: "account", Name: "list"}},
				IP6PrefixLen:       96,
				IP6HostID:          map[domain.Domain]ipnet.HostID{},
				WAFListPrefixLen:   map[ipnet.Type]int{ipnet.IP6: 64},
				UpdateOnStart:      true,
				ProxiedTemplate:    "false",
				Proxied:            map[domain.Domain]bool{},
				TTL:                api.TTLAuto,
				WAFListDescription: "My list",
				DetectionTimeout:   5 * time.Second,
			},
			prepareMockPP: func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().IsShowing(pp.Info).Return(true),
					m.EXPECT().Infof(pp.EmojiEnvVars, "Checking settings . . ."),
					m.EXPECT().Indent().Return(m),
					m.EXPECT().Noticef(pp.EmojiUserWarning, "The detected IPv6 range in WAF lists will be forced to have a prefix length of 64 (instead of IP6_PREFIX_LEN (%d)) due to Cloudflare's limitations", 96),
				)
			},
		},
		"waf/small-prefix-len": {
			input: &config.Config{ //nolint:exhaustruct
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				Domains:            map[ipnet.Type][]domain.Domain{},
				WAFLists:           []api.WAFList{{AccountID: "account", Name: "list"}},
				IP6PrefixLen:       10,
				UpdateOnStart:      true,
				ProxiedTemplate:    "false",
				TTL:                api.TTLAuto,
				WAFListDescription: "My list",
				DetectionTimeout:   5 * time.Second,
			},
			ok:       false,
			expected: nil,
			prepareMockPP: func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().IsShowing(pp.Info).Return(true),
					m.EXPECT().Infof(pp.EmojiEnvVars, "Checking settings . . ."),
					m.EXPECT().Indent().Return(m),
					m.EXPECT().Noticef(pp.EmojiUserError, "WAF lists do not support IPv6 ranges with a prefix length as small as IP6_PREFIX_LEN (%d); it must be at least 12", 10),
				)
			},
		},
		"dns-ignored-params": {
			input: &config.Config{ //nolint:exhaustruct
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				Domains:          map[ipnet.Type][]domain.Domain{},
				IP6PrefixLen:     64,
				IP6HostID:        map[domain.Domain]ipnet.HostID{},
				WAFLists:         []api.WAFList{{AccountID: "account", Name: "list"}},
				WAFListPrefixLen: map[ipnet.Type]int{},
				UpdateOnStart:    true,
				TTL:              10000,
				ProxiedTemplate:  "true",
				RecordComment:    "hello",
				DetectionTimeout: 5 * time.Second,
			},
			ok: true,
			expected: &config.Config{ //nolint:exhaustruct
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				Domains:          map[ipnet.Type][]domain.Domain{},
				IP6PrefixLen:     64,
				IP6HostID:        map[domain.Domain]ipnet.HostID{},
				WAFLists:         []api.WAFList{{AccountID: "account", Name: "list"}},
				WAFListPrefixLen: map[ipnet.Type]int{ipnet.IP6: 64},
				UpdateOnStart:    true,
				TTL:              10000,
				ProxiedTemplate:  "true",
				Proxied:          map[domain.Domain]bool{},
				RecordComment:    "hello",
				DetectionTimeout: 5 * time.Second,
			},
			prepareMockPP: func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().IsShowing(pp.Info).Return(true),
					m.EXPECT().Infof(pp.EmojiEnvVars, "Checking settings . . ."),
					m.EXPECT().Indent().Return(m),
					m.EXPECT().Noticef(pp.EmojiUserWarning, "TTL=%d is ignored because no domains will be updated", 10000),
					m.EXPECT().Noticef(pp.EmojiUserWarning, "PROXIED=%s is ignored because no domains will be updated", "true"),
					m.EXPECT().Noticef(pp.EmojiUserWarning, "RECORD_COMMENT=%s is ignored because no domains will be updated", "hello"),
				)
			},
		},
		"waf-ignored-comment": {
			input: &config.Config{ //nolint:exhaustruct
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				Domains: map[ipnet.Type][]domain.Domain{
					ipnet.IP6: {domain.FQDN("a.b.c")},
				},
				IP6PrefixLen:    64,
				ProxiedTemplate: "true",
				Proxied: map[domain.Domain]bool{
					domain.FQDN("a.b.c"): true,
				},
				WAFListDescription: "My list",
				UpdateOnStart:      true,
				DetectionTimeout:   5 * time.Second,
			},
			ok: true,
			expected: &config.Config{ //nolint:exhaustruct
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				Domains: map[ipnet.Type][]domain.Domain{
					ipnet.IP6: {domain.FQDN("a.b.c")},
				},
				IP6PrefixLen:     64,
				IP6HostID:        map[domain.Domain]ipnet.HostID{},
				WAFListPrefixLen: map[ipnet.Type]int{},
				UpdateOnStart:    true,
				ProxiedTemplate:  "true",
				Proxied: map[domain.Domain]bool{
					domain.FQDN("a.b.c"): true,
				},
				WAFListDescription: "My list",
				DetectionTimeout:   5 * time.Second,
			},
			prepareMockPP: func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().IsShowing(pp.Info).Return(true),
					m.EXPECT().Infof(pp.EmojiEnvVars, "Checking settings . . ."),
					m.EXPECT().Indent().Return(m),
					m.EXPECT().Noticef(pp.EmojiUserWarning, "WAF_LIST_DESCRIPTION=%s is ignored because no WAF lists will be updated", "My list"),
				)
			},
		},
		"proxied": {
			input: &config.Config{ //nolint:exhaustruct
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				Domains: map[ipnet.Type][]domain.Domain{
					ipnet.IP6: {domain.FQDN("a.b.c"), domain.FQDN("a.bb.c"), domain.FQDN("a.d.e.f")},
				},
				IP6PrefixLen:     64,
				WAFListPrefixLen: map[ipnet.Type]int{},
				UpdateOnStart:    true,
				ProxiedTemplate:  ` true && !is(a.bb.c) `,
				DetectionTimeout: 5 * time.Second,
			},
			ok: true,
			expected: &config.Config{ //nolint:exhaustruct
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				Domains: map[ipnet.Type][]domain.Domain{
					ipnet.IP6: {domain.FQDN("a.b.c"), domain.FQDN("a.bb.c"), domain.FQDN("a.d.e.f")},
				},
				IP6PrefixLen:     64,
				IP6HostID:        map[domain.Domain]ipnet.HostID{},
				WAFListPrefixLen: map[ipnet.Type]int{},
				UpdateOnStart:    true,
				ProxiedTemplate:  ` true && !is(a.bb.c) `,
				Proxied: map[domain.Domain]bool{
					domain.FQDN("a.b.c"):   true,
					domain.FQDN("a.bb.c"):  false,
					domain.FQDN("a.d.e.f"): true,
				},
				DetectionTimeout: 5 * time.Second,
			},
			prepareMockPP: func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().IsShowing(pp.Info).Return(true),
					m.EXPECT().Infof(pp.EmojiEnvVars, "Checking settings . . ."),
					m.EXPECT().Indent().Return(m),
				)
			},
		},
		"proxied/invalid/1": {
			input: &config.Config{ //nolint:exhaustruct
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				Domains: map[ipnet.Type][]domain.Domain{
					ipnet.IP6: {domain.FQDN("a.b.c"), domain.FQDN("a.bb.c"), domain.FQDN("a.d.e.f")},
				},
				IP6PrefixLen:    64,
				UpdateOnStart:   true,
				ProxiedTemplate: `range`,
			},
			ok:       false,
			expected: nil,
			prepareMockPP: func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().IsShowing(pp.Info).Return(true),
					m.EXPECT().Infof(pp.EmojiEnvVars, "Checking settings . . ."),
					m.EXPECT().Indent().Return(m),
					m.EXPECT().Noticef(pp.EmojiUserError, "%s (%q) is not a boolean expression: got unexpected token %q", keyProxied, `range`, `range`),
				)
			},
		},
		"proxied/invalid/2": {
			input: &config.Config{ //nolint:exhaustruct
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				Domains: map[ipnet.Type][]domain.Domain{
					ipnet.IP6: {domain.FQDN("a.b.c")},
				},
				IP6PrefixLen:    64,
				UpdateOnStart:   true,
				ProxiedTemplate: `999`,
			},
			ok:       false,
			expected: nil,
			prepareMockPP: func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().IsShowing(pp.Info).Return(true),
					m.EXPECT().Infof(pp.EmojiEnvVars, "Checking settings . . ."),
					m.EXPECT().Indent().Return(m),
					m.EXPECT().Noticef(pp.EmojiUserError, "%s (%q) is not a boolean expression: got unexpected token %q", keyProxied, `999`, `999`),
				)
			},
		},
		"proxied/invalid/3": {
			input: &config.Config{ //nolint:exhaustruct
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				Domains: map[ipnet.Type][]domain.Domain{
					ipnet.IP6: {domain.FQDN("a.b.c")},
				},
				IP6PrefixLen:    64,
				UpdateOnStart:   true,
				ProxiedTemplate: `is(12345`,
			},
			ok:       false,
			expected: nil,
			prepareMockPP: func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().IsShowing(pp.Info).Return(true),
					m.EXPECT().Infof(pp.EmojiEnvVars, "Checking settings . . ."),
					m.EXPECT().Indent().Return(m),
					m.EXPECT().Noticef(pp.EmojiUserError, `%s (%q) is missing %q at the end`, keyProxied, `is(12345`, ")"),
				)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			mockCtrl := gomock.NewController(t)

			cfg := tc.input
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}
			ok := cfg.Normalize(mockPP)
			require.Equal(t, tc.ok, ok)
			if tc.ok {
				require.Equal(t, tc.expected, cfg)
			} else {
				require.Nil(t, tc.expected) // check the test case itself is okay
				require.Equal(t, tc.input, cfg)
			}
		})
	}
}
