package config_test

// vim: nowrap

import (
	"regexp"
	"strconv"
	"strings"
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

func quotedIgnoredValuePreview(value string) string {
	runes := []rune(value)
	if len(runes) > 48 {
		value = string(runes[:48]) + "..."
	}
	return strconv.Quote(value)
}

func unsetAll(t *testing.T) {
	t.Helper()
	unset(t,
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
		"MANAGED_WAF_LIST_ITEMS_COMMENT_REGEX",
		"DETECTION_TIMEOUT",
		"UPDATE_TIMEOUT",
		"HEALTHCHECKS",
		"UPTIMEKUMA",
		"SHOUTRRR",
	)
}

//nolint:paralleltest // environment variables are global
func TestReadEnvWithOnlyToken(t *testing.T) {
	mockCtrl := gomock.NewController(t)

	unsetAll(t)
	store(t, "CLOUDFLARE_API_TOKEN", "deadbeaf")

	var cfg config.RawConfig
	mockPP := mocks.NewMockPP(mockCtrl)
	innerMockPP := mocks.NewMockPP(mockCtrl)
	gomock.InOrder(
		mockPP.EXPECT().IsShowing(pp.Info).Return(true),
		mockPP.EXPECT().Infof(pp.EmojiEnvVars, "Reading settings . . ."),
		mockPP.EXPECT().Indent().Return(innerMockPP),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Use default %s=%s", "IP4_PROVIDER", "none"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Use default %s=%s", "IP6_PROVIDER", "none"),
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

	var cfg config.RawConfig
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

func TestBuildConfig(t *testing.T) {
	t.Parallel()

	keyProxied := "PROXIED"
	keyManagedRecordsCommentRegex := "MANAGED_RECORDS_COMMENT_REGEX"
	keyManagedWAFListItemsCommentRegex := "MANAGED_WAF_LIST_ITEMS_COMMENT_REGEX"

	type builtConfig struct {
		handle    *config.HandleConfig
		lifecycle *config.LifecycleConfig
		update    *config.UpdateConfig
	}

	for name, tc := range map[string]struct {
		input         *config.RawConfig
		ok            bool
		expected      *builtConfig
		prepareMockPP func(m *mocks.MockPP)
	}{
		"nothing-to-do": {
			input:    &config.RawConfig{}, //nolint:exhaustruct
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
		"once/update-on-start": {
			input: &config.RawConfig{ //nolint:exhaustruct
				UpdateOnStart: false,
				IP4Domains:    []domain.Domain{domain.FQDN("a.b.c")},
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
		"once/delete-on-stop": {
			input: &config.RawConfig{ //nolint:exhaustruct
				DeleteOnStop:  true,
				UpdateOnStart: true,
				IP4Domains:    []domain.Domain{domain.FQDN("a.b.c")},
			},
			ok:       false,
			expected: nil,
			prepareMockPP: func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().IsShowing(pp.Info).Return(true),
					m.EXPECT().Infof(pp.EmojiEnvVars, "Checking settings . . ."),
					m.EXPECT().Indent().Return(m),
					m.EXPECT().Noticef(pp.EmojiUserError, "DELETE_ON_STOP=true with UPDATE_CRON=@once would immediately delete managed domains and WAF content"),
				)
			},
		},
		"nilprovider": {
			input: &config.RawConfig{ //nolint:exhaustruct
				UpdateOnStart: true,
				Provider: map[ipnet.Family]provider.Provider{
					ipnet.IP4: nil,
					ipnet.IP6: nil,
				},
				IP4Domains:        []domain.Domain{domain.FQDN("a.b.c")},
				ProxiedExpression: "false",
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
		"dns6empty": {
			input: &config.RawConfig{ //nolint:exhaustruct
				UpdateOnStart:    true,
				DetectionTimeout: 5 * time.Second,
				Provider: map[ipnet.Family]provider.Provider{
					ipnet.IP4: provider.NewCloudflareTrace(),
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				IP4Domains:        []domain.Domain{domain.FQDN("a.b.c")},
				ProxiedExpression: "false",
			},
			ok: true,
			expected: &builtConfig{
				handle: &config.HandleConfig{ //nolint:exhaustruct
					Options: api.HandleOptions{}, //nolint:exhaustruct
				},
				lifecycle: &config.LifecycleConfig{ //nolint:exhaustruct
					UpdateOnStart: true,
				},
				update: &config.UpdateConfig{ //nolint:exhaustruct
					DetectionTimeout: 5 * time.Second,
					Provider: map[ipnet.Family]provider.Provider{
						ipnet.IP4: provider.NewCloudflareTrace(),
					},
					Domains: map[ipnet.Family][]domain.Domain{
						ipnet.IP4: {domain.FQDN("a.b.c")},
						ipnet.IP6: nil,
					},
					Proxied: map[domain.Domain]bool{
						domain.FQDN("a.b.c"): false,
					},
				},
			},
			prepareMockPP: func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().IsShowing(pp.Info).Return(true),
					m.EXPECT().Infof(pp.EmojiEnvVars, "Checking settings . . ."),
					m.EXPECT().Indent().Return(m),
					m.EXPECT().Noticef(pp.EmojiUserWarning, "IP%d_PROVIDER (%s) is ignored because no domains or WAF lists use %s", 6, `"cloudflare.trace"`, "IPv6"),
				)
			},
		},
		"dns6empty-ip4none": {
			input: &config.RawConfig{ //nolint:exhaustruct
				UpdateOnStart: true,
				Provider: map[ipnet.Family]provider.Provider{
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				IP4Domains: []domain.Domain{domain.FQDN("a.b.c")},
			},
			ok:       false,
			expected: nil,
			prepareMockPP: func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().IsShowing(pp.Info).Return(true),
					m.EXPECT().Infof(pp.EmojiEnvVars, "Checking settings . . ."),
					m.EXPECT().Indent().Return(m),
					m.EXPECT().Noticef(pp.EmojiUserWarning, "IP%d_PROVIDER (%s) is ignored because no domains or WAF lists use %s", 6, `"cloudflare.trace"`, "IPv6"),
					m.EXPECT().Noticef(pp.EmojiUserError, "Nothing to update because both IP4_PROVIDER and IP6_PROVIDER are %q", "none"),
				)
			},
		},
		"both-static-empty-warning": {
			input: &config.RawConfig{ //nolint:exhaustruct
				UpdateOnStart: true,
				Provider: map[ipnet.Family]provider.Provider{
					ipnet.IP4: provider.NewStaticEmpty(),
					ipnet.IP6: provider.NewStaticEmpty(),
				},
				IP4Domains:        []domain.Domain{domain.FQDN("a.b.c")},
				IP6Domains:        []domain.Domain{domain.FQDN("d.e.f")},
				ProxiedExpression: "false",
			},
			ok: true,
			expected: &builtConfig{
				handle: &config.HandleConfig{ //nolint:exhaustruct
					Options: api.HandleOptions{}, //nolint:exhaustruct
				},
				lifecycle: &config.LifecycleConfig{ //nolint:exhaustruct
					UpdateOnStart: true,
				},
				update: &config.UpdateConfig{ //nolint:exhaustruct
					Provider: map[ipnet.Family]provider.Provider{
						ipnet.IP4: provider.NewStaticEmpty(),
						ipnet.IP6: provider.NewStaticEmpty(),
					},
					Domains: map[ipnet.Family][]domain.Domain{
						ipnet.IP4: {domain.FQDN("a.b.c")},
						ipnet.IP6: {domain.FQDN("d.e.f")},
					},
					Proxied: map[domain.Domain]bool{
						domain.FQDN("a.b.c"): false,
						domain.FQDN("d.e.f"): false,
					},
				},
			},
			prepareMockPP: func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().IsShowing(pp.Info).Return(true),
					m.EXPECT().Infof(pp.EmojiEnvVars, "Checking settings . . ."),
					m.EXPECT().Indent().Return(m),
					m.EXPECT().Noticef(pp.EmojiUserWarning,
						`Both IP4_PROVIDER and IP6_PROVIDER are "static.empty"; this updater will clear managed DNS records for the configured domains`),
				)
			},
		},
		"both-static-empty-warning/domains-and-waf": {
			input: &config.RawConfig{ //nolint:exhaustruct
				UpdateOnStart: true,
				Provider: map[ipnet.Family]provider.Provider{
					ipnet.IP4: provider.NewStaticEmpty(),
					ipnet.IP6: provider.NewStaticEmpty(),
				},
				IP4Domains:        []domain.Domain{domain.FQDN("a.b.c")},
				WAFLists:          []api.WAFList{{AccountID: "account", Name: "list"}},
				ProxiedExpression: "false",
			},
			ok: true,
			expected: &builtConfig{
				handle: &config.HandleConfig{ //nolint:exhaustruct
					Options: api.HandleOptions{}, //nolint:exhaustruct
				},
				lifecycle: &config.LifecycleConfig{ //nolint:exhaustruct
					UpdateOnStart: true,
				},
				update: &config.UpdateConfig{ //nolint:exhaustruct
					Provider: map[ipnet.Family]provider.Provider{
						ipnet.IP4: provider.NewStaticEmpty(),
						ipnet.IP6: provider.NewStaticEmpty(),
					},
					Domains: map[ipnet.Family][]domain.Domain{
						ipnet.IP4: {domain.FQDN("a.b.c")},
						ipnet.IP6: nil,
					},
					WAFLists: []api.WAFList{{AccountID: "account", Name: "list"}},
					Proxied: map[domain.Domain]bool{
						domain.FQDN("a.b.c"): false,
					},
				},
			},
			prepareMockPP: func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().IsShowing(pp.Info).Return(true),
					m.EXPECT().Infof(pp.EmojiEnvVars, "Checking settings . . ."),
					m.EXPECT().Indent().Return(m),
					m.EXPECT().Noticef(pp.EmojiUserWarning,
						`Both IP4_PROVIDER and IP6_PROVIDER are "static.empty"; this updater will clear managed DNS records and WAF IP items for the configured scope`),
				)
			},
		},
		"both-static-empty-warning/waf-only": {
			input: &config.RawConfig{ //nolint:exhaustruct
				UpdateOnStart: true,
				TTL:           api.TTLAuto,
				Provider: map[ipnet.Family]provider.Provider{
					ipnet.IP4: provider.NewStaticEmpty(),
					ipnet.IP6: provider.NewStaticEmpty(),
				},
				WAFLists:          []api.WAFList{{AccountID: "account", Name: "list"}},
				ProxiedExpression: "false",
			},
			ok: true,
			expected: &builtConfig{
				handle: &config.HandleConfig{ //nolint:exhaustruct
					Options: api.HandleOptions{}, //nolint:exhaustruct
				},
				lifecycle: &config.LifecycleConfig{ //nolint:exhaustruct
					UpdateOnStart: true,
				},
				update: &config.UpdateConfig{ //nolint:exhaustruct
					Provider: map[ipnet.Family]provider.Provider{
						ipnet.IP4: provider.NewStaticEmpty(),
						ipnet.IP6: provider.NewStaticEmpty(),
					},
					Domains: map[ipnet.Family][]domain.Domain{
						ipnet.IP4: nil,
						ipnet.IP6: nil,
					},
					WAFLists: []api.WAFList{{AccountID: "account", Name: "list"}},
					TTL:      api.TTLAuto,
					Proxied: map[domain.Domain]bool{},
				},
			},
			prepareMockPP: func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().IsShowing(pp.Info).Return(true),
					m.EXPECT().Infof(pp.EmojiEnvVars, "Checking settings . . ."),
					m.EXPECT().Indent().Return(m),
					m.EXPECT().Noticef(pp.EmojiUserWarning,
						`Both IP4_PROVIDER and IP6_PROVIDER are "static.empty"; this updater will clear managed WAF IP items for the configured lists`),
				)
			},
		},
		"ip4none": {
			input: &config.RawConfig{ //nolint:exhaustruct
				UpdateOnStart:    true,
				DetectionTimeout: 5 * time.Second,
				Provider: map[ipnet.Family]provider.Provider{
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				IP4Domains:        []domain.Domain{domain.FQDN("a.b.c"), domain.FQDN("d.e.f")},
				IP6Domains:        []domain.Domain{domain.FQDN("a.b.c"), domain.FQDN("g.h.i")},
				ProxiedExpression: "false",
			},
			ok: true,
			expected: &builtConfig{
				handle: &config.HandleConfig{ //nolint:exhaustruct
					Options: api.HandleOptions{}, //nolint:exhaustruct
				},
				lifecycle: &config.LifecycleConfig{ //nolint:exhaustruct
					UpdateOnStart: true,
				},
				update: &config.UpdateConfig{ //nolint:exhaustruct
					DetectionTimeout: 5 * time.Second,
					Provider: map[ipnet.Family]provider.Provider{
						ipnet.IP6: provider.NewCloudflareTrace(),
					},
					Domains: map[ipnet.Family][]domain.Domain{
						ipnet.IP4: {domain.FQDN("a.b.c"), domain.FQDN("d.e.f")},
						ipnet.IP6: {domain.FQDN("a.b.c"), domain.FQDN("g.h.i")},
					},
					Proxied: map[domain.Domain]bool{
						domain.FQDN("a.b.c"): false,
						domain.FQDN("g.h.i"): false,
					},
				},
			},
			prepareMockPP: func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().IsShowing(pp.Info).Return(true),
					m.EXPECT().Infof(pp.EmojiEnvVars, "Checking settings . . ."),
					m.EXPECT().Indent().Return(m),
					m.EXPECT().Noticef(pp.EmojiUserWarning, "Domain %q is ignored because it is only for %s but %s is disabled", "d.e.f", "IPv4", "IPv4"),
				)
			},
		},
		"ignored/dns": {
			input: &config.RawConfig{ //nolint:exhaustruct
				UpdateOnStart:    true,
				WAFLists:         []api.WAFList{{AccountID: "account", Name: "list"}},
				TTL:              10000,
				RecordComment:    "hello",
				DetectionTimeout: 5 * time.Second,
				Provider: map[ipnet.Family]provider.Provider{
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				ProxiedExpression:          "true",
				ManagedRecordsCommentRegex: "he",
			},
			ok: true,
			expected: &builtConfig{
				handle: &config.HandleConfig{ //nolint:exhaustruct
					Options: api.HandleOptions{
						CacheExpiration: 0,
						HandleOwnershipPolicy: api.HandleOwnershipPolicy{
							ManagedRecordsCommentRegex:        regexp.MustCompile("he"),
							ManagedWAFListItemsCommentRegex:   nil,
							AllowWholeWAFListDeleteOnShutdown: false,
						},
					},
				},
				lifecycle: &config.LifecycleConfig{ //nolint:exhaustruct
					UpdateOnStart: true,
				},
				update: &config.UpdateConfig{ //nolint:exhaustruct
					WAFLists:         []api.WAFList{{AccountID: "account", Name: "list"}},
					TTL:              10000,
					RecordComment:    "hello",
					DetectionTimeout: 5 * time.Second,
					Provider: map[ipnet.Family]provider.Provider{
						ipnet.IP6: provider.NewCloudflareTrace(),
					},
					Domains: map[ipnet.Family][]domain.Domain{
						ipnet.IP4: nil,
						ipnet.IP6: nil,
					},
					Proxied: map[domain.Domain]bool{},
				},
			},
			prepareMockPP: func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().IsShowing(pp.Info).Return(true),
					m.EXPECT().Infof(pp.EmojiEnvVars, "Checking settings . . ."),
					m.EXPECT().Indent().Return(m),
					m.EXPECT().Noticef(pp.EmojiUserWarning, "TTL=%v is ignored because no domains will be updated", api.TTL(10000)),
					m.EXPECT().Noticef(pp.EmojiUserWarning, "PROXIED (%s) is ignored because no domains will be updated", quotedIgnoredValuePreview("true")),
					m.EXPECT().Noticef(pp.EmojiUserWarning, "RECORD_COMMENT (%s) is ignored because no domains will be updated", quotedIgnoredValuePreview("hello")),
					m.EXPECT().Noticef(pp.EmojiUserWarning, "MANAGED_RECORDS_COMMENT_REGEX (%s) is ignored because no domains will be updated", quotedIgnoredValuePreview("he")),
				)
			},
		},
		"managed-record-regex/valid": {
			input: &config.RawConfig{ //nolint:exhaustruct
				UpdateOnStart:    true,
				RecordComment:    "hello-123",
				DetectionTimeout: 5 * time.Second,
				Provider: map[ipnet.Family]provider.Provider{
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				IP6Domains:                 []domain.Domain{domain.FQDN("a.b.c")},
				ProxiedExpression:          "false",
				ManagedRecordsCommentRegex: `^hello-[0-9]+$`,
			},
			ok: true,
			expected: &builtConfig{
				handle: &config.HandleConfig{ //nolint:exhaustruct
					Options: api.HandleOptions{
						CacheExpiration: 0,
						HandleOwnershipPolicy: api.HandleOwnershipPolicy{
							ManagedRecordsCommentRegex:        regexp.MustCompile(`^hello-[0-9]+$`),
							ManagedWAFListItemsCommentRegex:   nil,
							AllowWholeWAFListDeleteOnShutdown: false,
						},
					},
				},
				lifecycle: &config.LifecycleConfig{ //nolint:exhaustruct
					UpdateOnStart: true,
				},
				update: &config.UpdateConfig{ //nolint:exhaustruct
					RecordComment:    "hello-123",
					DetectionTimeout: 5 * time.Second,
					Provider: map[ipnet.Family]provider.Provider{
						ipnet.IP6: provider.NewCloudflareTrace(),
					},
					Domains: map[ipnet.Family][]domain.Domain{
						ipnet.IP4: nil,
						ipnet.IP6: {domain.FQDN("a.b.c")},
					},
					Proxied: map[domain.Domain]bool{
						domain.FQDN("a.b.c"): false,
					},
				},
			},
			prepareMockPP: func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().IsShowing(pp.Info).Return(true),
					m.EXPECT().Infof(pp.EmojiEnvVars, "Checking settings . . ."),
					m.EXPECT().Indent().Return(m),
				)
			},
		},
		"managed-record-regex/invalid": {
			input: &config.RawConfig{ //nolint:exhaustruct
				UpdateOnStart: true,
				Provider: map[ipnet.Family]provider.Provider{
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				IP6Domains:                 []domain.Domain{domain.FQDN("a.b.c")},
				ProxiedExpression:          "false",
				ManagedRecordsCommentRegex: "(",
			},
			ok:       false,
			expected: nil,
			prepareMockPP: func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().IsShowing(pp.Info).Return(true),
					m.EXPECT().Infof(pp.EmojiEnvVars, "Checking settings . . ."),
					m.EXPECT().Indent().Return(m),
					m.EXPECT().Noticef(pp.EmojiUserError, keyManagedRecordsCommentRegex+"=%q is invalid: %v", "(", gomock.Any()),
				)
			},
		},
		"managed-record-regex/mismatch": {
			input: &config.RawConfig{ //nolint:exhaustruct
				UpdateOnStart: true,
				RecordComment: "hello",
				Provider: map[ipnet.Family]provider.Provider{
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				IP6Domains:                 []domain.Domain{domain.FQDN("a.b.c")},
				ProxiedExpression:          "false",
				ManagedRecordsCommentRegex: "^world$",
			},
			ok:       false,
			expected: nil,
			prepareMockPP: func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().IsShowing(pp.Info).Return(true),
					m.EXPECT().Infof(pp.EmojiEnvVars, "Checking settings . . ."),
					m.EXPECT().Indent().Return(m),
					m.EXPECT().Noticef(pp.EmojiUserError, "RECORD_COMMENT=%q does not match MANAGED_RECORDS_COMMENT_REGEX=%q", "hello", "^world$"),
				)
			},
		},
		"managed-waf-item-regex/valid": {
			input: &config.RawConfig{ //nolint:exhaustruct
				UpdateOnStart:                   true,
				WAFLists:                        []api.WAFList{{AccountID: "account", Name: "list"}},
				TTL:                             api.TTLAuto,
				ProxiedExpression:               "false",
				WAFListItemComment:              "managed-123",
				ManagedWAFListItemsCommentRegex: `^managed-[0-9]+$`,
				DetectionTimeout:                5 * time.Second,
				Provider: map[ipnet.Family]provider.Provider{
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
			},
			ok: true,
			expected: &builtConfig{
				handle: &config.HandleConfig{ //nolint:exhaustruct
					Options: api.HandleOptions{ //nolint:exhaustruct
						HandleOwnershipPolicy: api.HandleOwnershipPolicy{
							ManagedWAFListItemsCommentRegex: regexp.MustCompile(`^managed-[0-9]+$`),
						},
					},
				},
				lifecycle: &config.LifecycleConfig{ //nolint:exhaustruct
					UpdateOnStart: true,
				},
				update: &config.UpdateConfig{ //nolint:exhaustruct
					WAFLists:           []api.WAFList{{AccountID: "account", Name: "list"}},
					TTL:                api.TTLAuto,
					WAFListItemComment: "managed-123",
					DetectionTimeout:   5 * time.Second,
					Provider: map[ipnet.Family]provider.Provider{
						ipnet.IP6: provider.NewCloudflareTrace(),
					},
					Domains: map[ipnet.Family][]domain.Domain{
						ipnet.IP4: nil,
						ipnet.IP6: nil,
					},
					Proxied: map[domain.Domain]bool{},
				},
			},
			prepareMockPP: func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().IsShowing(pp.Info).Return(true),
					m.EXPECT().Infof(pp.EmojiEnvVars, "Checking settings . . ."),
					m.EXPECT().Indent().Return(m),
				)
			},
		},
		"managed-waf-item-regex/invalid": {
			input: &config.RawConfig{ //nolint:exhaustruct
				UpdateOnStart:                   true,
				WAFLists:                        []api.WAFList{{AccountID: "account", Name: "list"}},
				TTL:                             api.TTLAuto,
				ProxiedExpression:               "false",
				ManagedWAFListItemsCommentRegex: "(",
				Provider: map[ipnet.Family]provider.Provider{
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
			},
			ok:       false,
			expected: nil,
			prepareMockPP: func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().IsShowing(pp.Info).Return(true),
					m.EXPECT().Infof(pp.EmojiEnvVars, "Checking settings . . ."),
					m.EXPECT().Indent().Return(m),
					m.EXPECT().Noticef(pp.EmojiUserError, keyManagedWAFListItemsCommentRegex+"=%q is invalid: %v", "(", gomock.Any()),
				)
			},
		},
		"managed-waf-item-regex/mismatch": {
			input: &config.RawConfig{ //nolint:exhaustruct
				UpdateOnStart:                   true,
				WAFLists:                        []api.WAFList{{AccountID: "account", Name: "list"}},
				TTL:                             api.TTLAuto,
				ProxiedExpression:               "false",
				WAFListItemComment:              "hello",
				ManagedWAFListItemsCommentRegex: "^world$",
				Provider: map[ipnet.Family]provider.Provider{
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
			},
			ok:       false,
			expected: nil,
			prepareMockPP: func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().IsShowing(pp.Info).Return(true),
					m.EXPECT().Infof(pp.EmojiEnvVars, "Checking settings . . ."),
					m.EXPECT().Indent().Return(m),
					m.EXPECT().Noticef(pp.EmojiUserError, "WAF_LIST_ITEM_COMMENT=%q does not match MANAGED_WAF_LIST_ITEMS_COMMENT_REGEX=%q", "hello", "^world$"),
				)
			},
		},
		"ignored/waf": {
			input: &config.RawConfig{ //nolint:exhaustruct
				UpdateOnStart:      true,
				WAFListDescription: "My list",
				DetectionTimeout:   5 * time.Second,
				Provider: map[ipnet.Family]provider.Provider{
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				IP6Domains:        []domain.Domain{domain.FQDN("a.b.c")},
				ProxiedExpression: "true",
			},
			ok: true,
			expected: &builtConfig{
				handle: &config.HandleConfig{ //nolint:exhaustruct
					Options: api.HandleOptions{}, //nolint:exhaustruct
				},
				lifecycle: &config.LifecycleConfig{ //nolint:exhaustruct
					UpdateOnStart: true,
				},
				update: &config.UpdateConfig{ //nolint:exhaustruct
					WAFListDescription: "My list",
					DetectionTimeout:   5 * time.Second,
					Provider: map[ipnet.Family]provider.Provider{
						ipnet.IP6: provider.NewCloudflareTrace(),
					},
					Domains: map[ipnet.Family][]domain.Domain{
						ipnet.IP4: nil,
						ipnet.IP6: {domain.FQDN("a.b.c")},
					},
					Proxied: map[domain.Domain]bool{
						domain.FQDN("a.b.c"): true,
					},
				},
			},
			prepareMockPP: func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().IsShowing(pp.Info).Return(true),
					m.EXPECT().Infof(pp.EmojiEnvVars, "Checking settings . . ."),
					m.EXPECT().Indent().Return(m),
					m.EXPECT().Noticef(pp.EmojiUserWarning,
						"WAF_LIST_DESCRIPTION (%s) is ignored because WAF_LISTS is empty", `"My list"`),
				)
			},
		},
		"ignored/waf/quoted-preview": {
			input: &config.RawConfig{ //nolint:exhaustruct
				UpdateOnStart:                   true,
				WAFListDescription:              strings.Repeat("a", 48),
				WAFListItemComment:              strings.Repeat("b", 49),
				ManagedWAFListItemsCommentRegex: strings.Repeat(".", 49),
				DetectionTimeout:                5 * time.Second,
				Provider: map[ipnet.Family]provider.Provider{
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				IP6Domains:        []domain.Domain{domain.FQDN("a.b.c")},
				ProxiedExpression: "false",
			},
			ok: true,
			expected: &builtConfig{
				handle: &config.HandleConfig{ //nolint:exhaustruct
					Options: api.HandleOptions{ //nolint:exhaustruct
						HandleOwnershipPolicy: api.HandleOwnershipPolicy{
							ManagedWAFListItemsCommentRegex: regexp.MustCompile(strings.Repeat(".", 49)),
						},
					},
				},
				lifecycle: &config.LifecycleConfig{ //nolint:exhaustruct
					UpdateOnStart: true,
				},
				update: &config.UpdateConfig{ //nolint:exhaustruct
					WAFListDescription: strings.Repeat("a", 48),
					WAFListItemComment: strings.Repeat("b", 49),
					DetectionTimeout:   5 * time.Second,
					Provider: map[ipnet.Family]provider.Provider{
						ipnet.IP6: provider.NewCloudflareTrace(),
					},
					Domains: map[ipnet.Family][]domain.Domain{
						ipnet.IP4: nil,
						ipnet.IP6: {domain.FQDN("a.b.c")},
					},
					Proxied: map[domain.Domain]bool{
						domain.FQDN("a.b.c"): false,
					},
				},
			},
			prepareMockPP: func(m *mocks.MockPP) {
				wafListDescription := strings.Repeat("a", 48)
				wafListItemComment := strings.Repeat("b", 49)
				managedWAFListItemsCommentRegex := strings.Repeat(".", 49)
				gomock.InOrder(
					m.EXPECT().IsShowing(pp.Info).Return(true),
					m.EXPECT().Infof(pp.EmojiEnvVars, "Checking settings . . ."),
					m.EXPECT().Indent().Return(m),
					m.EXPECT().Noticef(pp.EmojiUserWarning,
						"WAF_LIST_DESCRIPTION (%s) is ignored because WAF_LISTS is empty",
						quotedIgnoredValuePreview(wafListDescription)),
					m.EXPECT().Noticef(pp.EmojiUserWarning,
						"WAF_LIST_ITEM_COMMENT (%s) is ignored because WAF_LISTS is empty",
						quotedIgnoredValuePreview(wafListItemComment)),
					m.EXPECT().Noticef(pp.EmojiUserWarning,
						"MANAGED_WAF_LIST_ITEMS_COMMENT_REGEX (%s) is ignored because WAF_LISTS is empty",
						quotedIgnoredValuePreview(managedWAFListItemsCommentRegex)),
				)
			},
		},
		"proxied": {
			input: &config.RawConfig{ //nolint:exhaustruct
				UpdateOnStart:    true,
				DetectionTimeout: 5 * time.Second,
				Provider: map[ipnet.Family]provider.Provider{
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				IP6Domains:        []domain.Domain{domain.FQDN("a.b.c"), domain.FQDN("a.bb.c"), domain.FQDN("a.d.e.f")},
				ProxiedExpression: ` true && !is(a.bb.c) `,
			},
			ok: true,
			expected: &builtConfig{
				handle: &config.HandleConfig{ //nolint:exhaustruct
					Options: api.HandleOptions{}, //nolint:exhaustruct
				},
				lifecycle: &config.LifecycleConfig{ //nolint:exhaustruct
					UpdateOnStart: true,
				},
				update: &config.UpdateConfig{ //nolint:exhaustruct
					DetectionTimeout: 5 * time.Second,
					Provider: map[ipnet.Family]provider.Provider{
						ipnet.IP6: provider.NewCloudflareTrace(),
					},
					Domains: map[ipnet.Family][]domain.Domain{
						ipnet.IP4: nil,
						ipnet.IP6: {domain.FQDN("a.b.c"), domain.FQDN("a.bb.c"), domain.FQDN("a.d.e.f")},
					},
					Proxied: map[domain.Domain]bool{
						domain.FQDN("a.b.c"):   true,
						domain.FQDN("a.bb.c"):  false,
						domain.FQDN("a.d.e.f"): true,
					},
				},
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
			input: &config.RawConfig{ //nolint:exhaustruct
				UpdateOnStart: true,
				Provider: map[ipnet.Family]provider.Provider{
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				IP6Domains:        []domain.Domain{domain.FQDN("a.b.c"), domain.FQDN("a.bb.c"), domain.FQDN("a.d.e.f")},
				ProxiedExpression: `range`,
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
			input: &config.RawConfig{ //nolint:exhaustruct
				UpdateOnStart: true,
				Provider: map[ipnet.Family]provider.Provider{
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				IP6Domains:        []domain.Domain{domain.FQDN("a.b.c")},
				ProxiedExpression: `999`,
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
			input: &config.RawConfig{ //nolint:exhaustruct
				UpdateOnStart: true,
				Provider: map[ipnet.Family]provider.Provider{
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				IP6Domains:        []domain.Domain{domain.FQDN("a.b.c")},
				ProxiedExpression: `is(12345`,
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

			raw := tc.input
			original := *raw
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}

			builtConfig, ok := raw.BuildConfig(mockPP)
			require.Equal(t, tc.ok, ok)
			if tc.ok {
				require.NotNil(t, builtConfig)
				require.NotNil(t, builtConfig.Handle)
				require.NotNil(t, builtConfig.Lifecycle)
				require.NotNil(t, builtConfig.Update)
				require.NotNil(t, builtConfig.Handle.Options.ManagedRecordsCommentRegex)
				require.NotNil(t, builtConfig.Handle.Options.ManagedWAFListItemsCommentRegex)
				require.Equal(t, raw.ManagedRecordsCommentRegex, builtConfig.Handle.Options.ManagedRecordsCommentRegex.String())
				require.Equal(t, raw.ManagedWAFListItemsCommentRegex, builtConfig.Handle.Options.ManagedWAFListItemsCommentRegex.String())

				expectedHandle := *tc.expected.handle
				if expectedHandle.Options.ManagedRecordsCommentRegex == nil {
					expectedHandle.Options.ManagedRecordsCommentRegex = regexp.MustCompile("")
				}
				if expectedHandle.Options.ManagedWAFListItemsCommentRegex == nil {
					expectedHandle.Options.ManagedWAFListItemsCommentRegex = regexp.MustCompile("")
				}
				expectedHandle.Options.AllowWholeWAFListDeleteOnShutdown = expectedHandle.Options.ManagedWAFListItemsCommentRegex.String() == ""
				require.Equal(t, &expectedHandle, builtConfig.Handle)
				require.Equal(t, tc.expected.lifecycle, builtConfig.Lifecycle)
				require.Equal(t, tc.expected.update, builtConfig.Update)
			} else {
				require.Nil(t, builtConfig)
			}
			require.Equal(t, original, *raw)
		})
	}
}
