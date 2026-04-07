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
	"github.com/favonia/cloudflare-ddns/internal/testenv"
)

func quotedIgnoredValuePreview(value string) string {
	runes := []rune(value)
	if len(runes) > 48 {
		value = string(runes[:48]) + "..."
	}
	return strconv.Quote(value)
}

func defaultPrefixLen() map[ipnet.Family]int {
	return map[ipnet.Family]int{
		ipnet.IP4: 32,
		ipnet.IP6: 64,
	}
}

//nolint:paralleltest // environment variables are global
func TestReadEnvWithOnlyToken(t *testing.T) {
	mockCtrl := gomock.NewController(t)

	testenv.ClearAll(t)
	store(t, "CLOUDFLARE_API_TOKEN", "deadbeaf")

	// Start from a zero-value RawConfig (not DefaultRaw) to verify that
	// ReadEnv prints whatever the caller's initial field values are, even
	// when those values are zero.  This exercises the "Using default" log
	// path without depending on the production defaults.
	var cfg config.RawConfig
	mockPP := mocks.NewMockPP(mockCtrl)
	innerMockPP := mocks.NewMockPP(mockCtrl)
	gomock.InOrder(
		mockPP.EXPECT().IsShowing(pp.Info).Return(true),
		mockPP.EXPECT().Infof(pp.EmojiEnvVars, "Reading settings . . ."),
		mockPP.EXPECT().Indent().Return(innerMockPP),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Using default %s=%d", "IP4_DEFAULT_PREFIX_LEN", 0),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Using default %s=%d", "IP6_DEFAULT_PREFIX_LEN", 0),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Using default %s=%s", "IP4_PROVIDER", "none"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Using default %s=%s", "IP6_PROVIDER", "none"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Using default %s=%s", "UPDATE_CRON", "@once"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Using default %s=%t", "UPDATE_ON_START", false),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Using default %s=%t", "DELETE_ON_STOP", false),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Using default %s=%v", "CACHE_EXPIRATION", time.Duration(0)),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Using default %s=%d", "TTL", api.TTL(0)),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Using default %s=%v", "DETECTION_TIMEOUT", time.Duration(0)),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Using default %s=%v", "UPDATE_TIMEOUT", time.Duration(0)),
	)
	ok := cfg.ReadEnv(mockPP)
	require.True(t, ok)
}

//nolint:paralleltest // environment variables are global
func TestReadEnvEmpty(t *testing.T) {
	mockCtrl := gomock.NewController(t)

	testenv.ClearAll(t)

	var cfg config.RawConfig
	mockPP := mocks.NewMockPP(mockCtrl)
	innerMockPP := mocks.NewMockPP(mockCtrl)
	gomock.InOrder(
		mockPP.EXPECT().IsShowing(pp.Info).Return(true),
		mockPP.EXPECT().Infof(pp.EmojiEnvVars, "Reading settings . . ."),
		mockPP.EXPECT().Indent().Return(innerMockPP),
		innerMockPP.EXPECT().Noticef(pp.EmojiUserError,
			"Either %s or %s must be set", "CLOUDFLARE_API_TOKEN", "CLOUDFLARE_API_TOKEN_FILE"),
	)
	ok := cfg.ReadEnv(mockPP)
	require.False(t, ok)
}

func TestBuildConfig(t *testing.T) {
	t.Parallel()

	// Keep PROXIED coverage here minimal and integration-focused:
	// parser behavior is tested comprehensively in internal/domainexp/parser_test.go.
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
				Provider: map[ipnet.Family]provider.Provider{
					ipnet.IP4: provider.NewCloudflareTrace(),
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
					m.EXPECT().Noticef(pp.EmojiUserError, "DELETE_ON_STOP=true with UPDATE_CRON=@once requires IP4_PROVIDER to be static.empty or none; got IP4_PROVIDER=%q", "cloudflare.trace"),
				)
			},
		},
		"once/delete-on-stop/both-families-invalid": {
			input: &config.RawConfig{ //nolint:exhaustruct
				DeleteOnStop:  true,
				UpdateOnStart: true,
				Provider: map[ipnet.Family]provider.Provider{
					ipnet.IP4: provider.NewCloudflareTrace(),
					ipnet.IP6: provider.NewCloudflareDOH(),
				},
				IP4Domains:        []domain.Domain{domain.FQDN("a.b.c")},
				IP6Domains:        []domain.Domain{domain.FQDN("d.e.f")},
				ProxiedExpression: "false",
			},
			ok:       false,
			expected: nil,
			prepareMockPP: func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().IsShowing(pp.Info).Return(true),
					m.EXPECT().Infof(pp.EmojiEnvVars, "Checking settings . . ."),
					m.EXPECT().Indent().Return(m),
					m.EXPECT().Noticef(pp.EmojiUserError, "DELETE_ON_STOP=true with UPDATE_CRON=@once requires IP4_PROVIDER and IP6_PROVIDER to be static.empty or none; got IP4_PROVIDER=%q and IP6_PROVIDER=%q", "cloudflare.trace", "cloudflare.doh"),
				)
			},
		},
		"once/delete-on-stop/ip6-only-invalid": {
			input: &config.RawConfig{ //nolint:exhaustruct
				DeleteOnStop:  true,
				UpdateOnStart: true,
				Provider: map[ipnet.Family]provider.Provider{
					ipnet.IP4: provider.NewStaticEmpty(),
					ipnet.IP6: provider.NewCloudflareDOH(),
				},
				IP4Domains:        []domain.Domain{domain.FQDN("a.b.c")},
				IP6Domains:        []domain.Domain{domain.FQDN("d.e.f")},
				ProxiedExpression: "false",
			},
			ok:       false,
			expected: nil,
			prepareMockPP: func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().IsShowing(pp.Info).Return(true),
					m.EXPECT().Infof(pp.EmojiEnvVars, "Checking settings . . ."),
					m.EXPECT().Indent().Return(m),
					m.EXPECT().Noticef(pp.EmojiUserError, "DELETE_ON_STOP=true with UPDATE_CRON=@once requires IP6_PROVIDER to be static.empty or none; got IP6_PROVIDER=%q", "cloudflare.doh"),
				)
			},
		},
		"once/delete-on-stop/explicit-empty-single-family": {
			input: &config.RawConfig{ //nolint:exhaustruct
				DeleteOnStop:        true,
				UpdateOnStart:       true,
				IP4DefaultPrefixLen: 32,
				IP6DefaultPrefixLen: 64,
				Provider: map[ipnet.Family]provider.Provider{
					ipnet.IP4: provider.NewStaticEmpty(),
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
					DeleteOnStop:  true,
				},
				update: &config.UpdateConfig{ //nolint:exhaustruct
					Provider: map[ipnet.Family]provider.Provider{
						ipnet.IP4: provider.NewStaticEmpty(),
					},
					Domains: map[ipnet.Family][]domain.Domain{
						ipnet.IP4: {domain.FQDN("a.b.c")},
						ipnet.IP6: nil,
					},
					DefaultPrefixLen: defaultPrefixLen(),
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
						"IP4_PROVIDER is configured to clear %s while IP6_PROVIDER is %q",
						"managed DNS records for the configured domains", "none"),
				)
			},
		},
		"once/delete-on-stop/explicit-empty-both-families": {
			input: &config.RawConfig{ //nolint:exhaustruct
				DeleteOnStop:        true,
				UpdateOnStart:       true,
				IP4DefaultPrefixLen: 32,
				IP6DefaultPrefixLen: 64,
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
					DeleteOnStop:  true,
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
					DefaultPrefixLen: defaultPrefixLen(),
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
						"Both IP4_PROVIDER and IP6_PROVIDER are configured to clear %s",
						"managed DNS records for the configured domains"),
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
				UpdateOnStart:       true,
				DetectionTimeout:    5 * time.Second,
				IP4DefaultPrefixLen: 32,
				IP6DefaultPrefixLen: 64,
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
					DefaultPrefixLen: defaultPrefixLen(),
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
				UpdateOnStart:       true,
				IP4DefaultPrefixLen: 32,
				IP6DefaultPrefixLen: 64,
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
					DefaultPrefixLen: defaultPrefixLen(),
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
						"Both IP4_PROVIDER and IP6_PROVIDER are configured to clear %s",
						"managed DNS records for the configured domains"),
				)
			},
		},
		"both-static-empty-warning/domains-and-waf": {
			input: &config.RawConfig{ //nolint:exhaustruct
				IP4DefaultPrefixLen: 32,
				IP6DefaultPrefixLen: 64,
				UpdateOnStart:       true,
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
					WAFLists:         []api.WAFList{{AccountID: "account", Name: "list"}},
					DefaultPrefixLen: defaultPrefixLen(),
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
						"Both IP4_PROVIDER and IP6_PROVIDER are configured to clear %s",
						"managed DNS records and WAF IP items for the configured scope"),
				)
			},
		},
		"both-static-empty-warning/waf-only": {
			input: &config.RawConfig{ //nolint:exhaustruct
				IP4DefaultPrefixLen: 32,
				IP6DefaultPrefixLen: 64,
				UpdateOnStart:       true,
				TTL:                 api.TTLAuto,
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
					WAFLists:         []api.WAFList{{AccountID: "account", Name: "list"}},
					DefaultPrefixLen: defaultPrefixLen(),
					TTL:              api.TTLAuto,
					Proxied:          map[domain.Domain]bool{},
				},
			},
			prepareMockPP: func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().IsShowing(pp.Info).Return(true),
					m.EXPECT().Infof(pp.EmojiEnvVars, "Checking settings . . ."),
					m.EXPECT().Indent().Return(m),
					m.EXPECT().Noticef(pp.EmojiUserWarning,
						"Both IP4_PROVIDER and IP6_PROVIDER are configured to clear %s",
						"managed WAF IP items for the configured lists"),
				)
			},
		},
		"ip4-none-ip6-static-empty/domains": {
			input: &config.RawConfig{ //nolint:exhaustruct
				IP4DefaultPrefixLen: 32,
				IP6DefaultPrefixLen: 64,
				UpdateOnStart:       true,
				Provider: map[ipnet.Family]provider.Provider{
					ipnet.IP6: provider.NewStaticEmpty(),
				},
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
						ipnet.IP6: provider.NewStaticEmpty(),
					},
					Domains: map[ipnet.Family][]domain.Domain{
						ipnet.IP4: nil,
						ipnet.IP6: {domain.FQDN("d.e.f")},
					},
					DefaultPrefixLen: defaultPrefixLen(),
					Proxied: map[domain.Domain]bool{
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
						"IP6_PROVIDER is configured to clear %s while IP4_PROVIDER is %q",
						"managed DNS records for the configured domains", "none"),
				)
			},
		},
		"ip4-static-empty-ip6-none/domains": {
			input: &config.RawConfig{ //nolint:exhaustruct
				IP4DefaultPrefixLen: 32,
				IP6DefaultPrefixLen: 64,
				UpdateOnStart:       true,
				Provider: map[ipnet.Family]provider.Provider{
					ipnet.IP4: provider.NewStaticEmpty(),
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
					Provider: map[ipnet.Family]provider.Provider{
						ipnet.IP4: provider.NewStaticEmpty(),
					},
					Domains: map[ipnet.Family][]domain.Domain{
						ipnet.IP4: {domain.FQDN("a.b.c")},
						ipnet.IP6: nil,
					},
					DefaultPrefixLen: defaultPrefixLen(),
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
						"IP4_PROVIDER is configured to clear %s while IP6_PROVIDER is %q",
						"managed DNS records for the configured domains", "none"),
				)
			},
		},
		"ip4none": {
			input: &config.RawConfig{ //nolint:exhaustruct
				IP4DefaultPrefixLen: 32,
				IP6DefaultPrefixLen: 64,
				UpdateOnStart:       true,
				DetectionTimeout:    5 * time.Second,
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
					DefaultPrefixLen: defaultPrefixLen(),
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
				IP4DefaultPrefixLen: 32,
				IP6DefaultPrefixLen: 64,
				UpdateOnStart:       true,
				WAFLists:            []api.WAFList{{AccountID: "account", Name: "list"}},
				TTL:                 10000,
				RecordComment:       "hello",
				DetectionTimeout:    5 * time.Second,
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
							ManagedRecordsCommentRegex:        regexp.MustCompile(""),
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
					DefaultPrefixLen: defaultPrefixLen(),
					Proxied:          map[domain.Domain]bool{},
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
				IP4DefaultPrefixLen: 32,
				IP6DefaultPrefixLen: 64,
				UpdateOnStart:       true,
				RecordComment:       "hello-123",
				DetectionTimeout:    5 * time.Second,
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
					DefaultPrefixLen: defaultPrefixLen(),
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
				IP4DefaultPrefixLen:             32,
				IP6DefaultPrefixLen:             64,
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
					DefaultPrefixLen: defaultPrefixLen(),
					Proxied:          map[domain.Domain]bool{},
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
		"ownership-warning/dns-isolated-waf-not": {
			input: &config.RawConfig{ //nolint:exhaustruct
				IP4DefaultPrefixLen:        32,
				IP6DefaultPrefixLen:        64,
				UpdateOnStart:              true,
				RecordComment:              "managed-dns",
				ManagedRecordsCommentRegex: "^managed-dns$",
				WAFLists:                   []api.WAFList{{AccountID: "account", Name: "list"}},
				WAFListItemComment:         "managed-waf",
				Provider: map[ipnet.Family]provider.Provider{
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				IP6Domains:        []domain.Domain{domain.FQDN("a.b.c")},
				ProxiedExpression: "false",
			},
			ok: true,
			expected: &builtConfig{
				handle: &config.HandleConfig{ //nolint:exhaustruct
					Options: api.HandleOptions{
						CacheExpiration: 0,
						HandleOwnershipPolicy: api.HandleOwnershipPolicy{
							ManagedRecordsCommentRegex:        regexp.MustCompile("^managed-dns$"),
							ManagedWAFListItemsCommentRegex:   nil,
							AllowWholeWAFListDeleteOnShutdown: false,
						},
					},
				},
				lifecycle: &config.LifecycleConfig{ //nolint:exhaustruct
					UpdateOnStart: true,
				},
				update: &config.UpdateConfig{ //nolint:exhaustruct
					Provider: map[ipnet.Family]provider.Provider{
						ipnet.IP6: provider.NewCloudflareTrace(),
					},
					Domains: map[ipnet.Family][]domain.Domain{
						ipnet.IP4: nil,
						ipnet.IP6: {domain.FQDN("a.b.c")},
					},
					WAFLists:           []api.WAFList{{AccountID: "account", Name: "list"}},
					DefaultPrefixLen:   defaultPrefixLen(),
					Proxied:            map[domain.Domain]bool{domain.FQDN("a.b.c"): false},
					RecordComment:      "managed-dns",
					WAFListItemComment: "managed-waf",
				},
			},
			prepareMockPP: func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().IsShowing(pp.Info).Return(true),
					m.EXPECT().Infof(pp.EmojiEnvVars, "Checking settings . . ."),
					m.EXPECT().Indent().Return(m),
					m.EXPECT().Noticef(pp.EmojiUserWarning,
						"DNS ownership isolation is enabled via MANAGED_RECORDS_COMMENT_REGEX (%s), but "+
							"WAF_LIST_ITEM_COMMENT (%s) is set while MANAGED_WAF_LIST_ITEMS_COMMENT_REGEX is empty; "+
							"the comment only affects newly written WAF list items, so WAF mutation scope is still not ownership-isolated",
						`"^managed-dns$"`,
						`"managed-waf"`,
					),
				)
			},
		},
		"ownership-warning/waf-isolated-dns-not": {
			input: &config.RawConfig{ //nolint:exhaustruct
				IP4DefaultPrefixLen:             32,
				IP6DefaultPrefixLen:             64,
				UpdateOnStart:                   true,
				RecordComment:                   "managed-dns",
				WAFLists:                        []api.WAFList{{AccountID: "account", Name: "list"}},
				WAFListItemComment:              "managed-waf",
				ManagedWAFListItemsCommentRegex: "^managed-waf$",
				Provider: map[ipnet.Family]provider.Provider{
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				IP6Domains:        []domain.Domain{domain.FQDN("a.b.c")},
				ProxiedExpression: "false",
			},
			ok: true,
			expected: &builtConfig{
				handle: &config.HandleConfig{ //nolint:exhaustruct
					Options: api.HandleOptions{
						CacheExpiration: 0,
						HandleOwnershipPolicy: api.HandleOwnershipPolicy{
							ManagedRecordsCommentRegex:        nil,
							ManagedWAFListItemsCommentRegex:   regexp.MustCompile("^managed-waf$"),
							AllowWholeWAFListDeleteOnShutdown: false,
						},
					},
				},
				lifecycle: &config.LifecycleConfig{ //nolint:exhaustruct
					UpdateOnStart: true,
				},
				update: &config.UpdateConfig{ //nolint:exhaustruct
					Provider: map[ipnet.Family]provider.Provider{
						ipnet.IP6: provider.NewCloudflareTrace(),
					},
					Domains: map[ipnet.Family][]domain.Domain{
						ipnet.IP4: nil,
						ipnet.IP6: {domain.FQDN("a.b.c")},
					},
					WAFLists:           []api.WAFList{{AccountID: "account", Name: "list"}},
					DefaultPrefixLen:   defaultPrefixLen(),
					Proxied:            map[domain.Domain]bool{domain.FQDN("a.b.c"): false},
					RecordComment:      "managed-dns",
					WAFListItemComment: "managed-waf",
				},
			},
			prepareMockPP: func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().IsShowing(pp.Info).Return(true),
					m.EXPECT().Infof(pp.EmojiEnvVars, "Checking settings . . ."),
					m.EXPECT().Indent().Return(m),
					m.EXPECT().Noticef(pp.EmojiUserWarning,
						"WAF ownership isolation is enabled via MANAGED_WAF_LIST_ITEMS_COMMENT_REGEX (%s), but "+
							"RECORD_COMMENT (%s) is set while MANAGED_RECORDS_COMMENT_REGEX is empty; "+
							"the comment only affects newly written DNS records, so DNS mutation scope is still not ownership-isolated",
						`"^managed-waf$"`,
						`"managed-dns"`,
					),
				)
			},
		},
		"ownership-warning/dns-isolated-waf-not-without-comment-signal": {
			input: &config.RawConfig{ //nolint:exhaustruct
				IP4DefaultPrefixLen:        32,
				IP6DefaultPrefixLen:        64,
				UpdateOnStart:              true,
				RecordComment:              "managed-dns",
				ManagedRecordsCommentRegex: "^managed-dns$",
				WAFLists:                   []api.WAFList{{AccountID: "account", Name: "list"}},
				Provider: map[ipnet.Family]provider.Provider{
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				IP6Domains:        []domain.Domain{domain.FQDN("a.b.c")},
				ProxiedExpression: "false",
			},
			ok: true,
			expected: &builtConfig{
				handle: &config.HandleConfig{ //nolint:exhaustruct
					Options: api.HandleOptions{
						CacheExpiration: 0,
						HandleOwnershipPolicy: api.HandleOwnershipPolicy{
							ManagedRecordsCommentRegex:        regexp.MustCompile("^managed-dns$"),
							ManagedWAFListItemsCommentRegex:   nil,
							AllowWholeWAFListDeleteOnShutdown: false,
						},
					},
				},
				lifecycle: &config.LifecycleConfig{ //nolint:exhaustruct
					UpdateOnStart: true,
				},
				update: &config.UpdateConfig{ //nolint:exhaustruct
					Provider: map[ipnet.Family]provider.Provider{
						ipnet.IP6: provider.NewCloudflareTrace(),
					},
					Domains: map[ipnet.Family][]domain.Domain{
						ipnet.IP4: nil,
						ipnet.IP6: {domain.FQDN("a.b.c")},
					},
					WAFLists:         []api.WAFList{{AccountID: "account", Name: "list"}},
					DefaultPrefixLen: defaultPrefixLen(),
					Proxied:          map[domain.Domain]bool{domain.FQDN("a.b.c"): false},
					RecordComment:    "managed-dns",
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
		"ownership-warning/waf-isolated-dns-not-without-comment-signal": {
			input: &config.RawConfig{ //nolint:exhaustruct
				IP4DefaultPrefixLen:             32,
				IP6DefaultPrefixLen:             64,
				UpdateOnStart:                   true,
				WAFLists:                        []api.WAFList{{AccountID: "account", Name: "list"}},
				WAFListItemComment:              "managed-waf",
				ManagedWAFListItemsCommentRegex: "^managed-waf$",
				Provider: map[ipnet.Family]provider.Provider{
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				IP6Domains:        []domain.Domain{domain.FQDN("a.b.c")},
				ProxiedExpression: "false",
			},
			ok: true,
			expected: &builtConfig{
				handle: &config.HandleConfig{ //nolint:exhaustruct
					Options: api.HandleOptions{
						CacheExpiration: 0,
						HandleOwnershipPolicy: api.HandleOwnershipPolicy{
							ManagedRecordsCommentRegex:        nil,
							ManagedWAFListItemsCommentRegex:   regexp.MustCompile("^managed-waf$"),
							AllowWholeWAFListDeleteOnShutdown: false,
						},
					},
				},
				lifecycle: &config.LifecycleConfig{ //nolint:exhaustruct
					UpdateOnStart: true,
				},
				update: &config.UpdateConfig{ //nolint:exhaustruct
					Provider: map[ipnet.Family]provider.Provider{
						ipnet.IP6: provider.NewCloudflareTrace(),
					},
					Domains: map[ipnet.Family][]domain.Domain{
						ipnet.IP4: nil,
						ipnet.IP6: {domain.FQDN("a.b.c")},
					},
					WAFLists:           []api.WAFList{{AccountID: "account", Name: "list"}},
					DefaultPrefixLen:   defaultPrefixLen(),
					Proxied:            map[domain.Domain]bool{domain.FQDN("a.b.c"): false},
					WAFListItemComment: "managed-waf",
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
				IP4DefaultPrefixLen: 32,
				IP6DefaultPrefixLen: 64,
				UpdateOnStart:       true,
				WAFListDescription:  "My list",
				DetectionTimeout:    5 * time.Second,
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
					DefaultPrefixLen: defaultPrefixLen(),
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
				IP4DefaultPrefixLen:             32,
				IP6DefaultPrefixLen:             64,
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
					Options: api.HandleOptions{}, //nolint:exhaustruct
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
					DefaultPrefixLen: defaultPrefixLen(),
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
		"ignored/ip4-prefix-len": {
			input: &config.RawConfig{ //nolint:exhaustruct
				IP4DefaultPrefixLen: 24,
				IP6DefaultPrefixLen: 64,
				UpdateOnStart:       true,
				DetectionTimeout:    5 * time.Second,
				Provider: map[ipnet.Family]provider.Provider{
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				IP6Domains:        []domain.Domain{domain.FQDN("a.b.c")},
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
						ipnet.IP4: nil,
						ipnet.IP6: {domain.FQDN("a.b.c")},
					},
					DefaultPrefixLen: map[ipnet.Family]int{
						ipnet.IP4: 24,
						ipnet.IP6: 64,
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
					m.EXPECT().Noticef(pp.EmojiUserWarning,
						"IP4_DEFAULT_PREFIX_LEN=%d is ignored because no domains or WAF lists use IPv4", 24),
				)
			},
		},
		"ignored/ip6-prefix-len": {
			input: &config.RawConfig{ //nolint:exhaustruct
				IP4DefaultPrefixLen: 32,
				IP6DefaultPrefixLen: 48,
				UpdateOnStart:       true,
				DetectionTimeout:    5 * time.Second,
				Provider: map[ipnet.Family]provider.Provider{
					ipnet.IP4: provider.NewCloudflareTrace(),
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
					DefaultPrefixLen: map[ipnet.Family]int{
						ipnet.IP4: 32,
						ipnet.IP6: 48,
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
					m.EXPECT().Noticef(pp.EmojiUserWarning,
						"IP6_DEFAULT_PREFIX_LEN=%d is ignored because no domains or WAF lists use IPv6", 48),
				)
			},
		},
		"ignored/ip4-prefix-len-at-default": {
			input: &config.RawConfig{ //nolint:exhaustruct
				IP4DefaultPrefixLen: 32,
				IP6DefaultPrefixLen: 64,
				UpdateOnStart:       true,
				DetectionTimeout:    5 * time.Second,
				Provider: map[ipnet.Family]provider.Provider{
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				IP6Domains:        []domain.Domain{domain.FQDN("a.b.c")},
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
						ipnet.IP4: nil,
						ipnet.IP6: {domain.FQDN("a.b.c")},
					},
					DefaultPrefixLen: defaultPrefixLen(),
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
		"proxied": {
			input: &config.RawConfig{ //nolint:exhaustruct
				IP4DefaultPrefixLen: 32,
				IP6DefaultPrefixLen: 64,
				UpdateOnStart:       true,
				DetectionTimeout:    5 * time.Second,
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
					DefaultPrefixLen: defaultPrefixLen(),
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
		"proxied/empty-list": {
			input: &config.RawConfig{ //nolint:exhaustruct
				IP4DefaultPrefixLen: 32,
				IP6DefaultPrefixLen: 64,
				UpdateOnStart:       true,
				Provider: map[ipnet.Family]provider.Provider{
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				IP6Domains:        []domain.Domain{domain.FQDN("a.b.c")},
				ProxiedExpression: "is()",
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
						ipnet.IP6: provider.NewCloudflareTrace(),
					},
					Domains: map[ipnet.Family][]domain.Domain{
						ipnet.IP4: nil,
						ipnet.IP6: {domain.FQDN("a.b.c")},
					},
					DefaultPrefixLen: defaultPrefixLen(),
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
						`%s (%q) uses %s() with an empty domain list, which always evaluates to false`,
						keyProxied, "is()", "is"),
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
