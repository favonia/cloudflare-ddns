package config_test

// vim: nowrap

import (
	"regexp"
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
	)
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
	keyManagedRecordsCommentRegex := "MANAGED_RECORDS_COMMENT_REGEX"

	for name, tc := range map[string]struct {
		input         *config.Config
		ok            bool
		expected      *config.Config
		prepareMockPP func(m *mocks.MockPP)
	}{
		"nothing-to-do": {
			input: &config.Config{ //nolint:exhaustruct
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
		"once/update-on-start": {
			input: &config.Config{ //nolint:exhaustruct
				Domains: map[ipnet.Type][]domain.Domain{
					ipnet.IP4: {domain.FQDN("a.b.c")},
				},
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
		"once/delete-on-stop": {
			input: &config.Config{ //nolint:exhaustruct
				Domains: map[ipnet.Type][]domain.Domain{
					ipnet.IP4: {domain.FQDN("a.b.c")},
				},
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
		"nilprovider": {
			input: &config.Config{ //nolint:exhaustruct
				UpdateOnStart: true,
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP4: nil,
					ipnet.IP6: nil,
				},
				Domains: map[ipnet.Type][]domain.Domain{
					ipnet.IP4: {domain.FQDN("a.b.c")},
				},
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
		"dns6empty": {
			input: &config.Config{ //nolint:exhaustruct
				UpdateOnStart: true,
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP4: provider.NewCloudflareTrace(),
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				Domains: map[ipnet.Type][]domain.Domain{
					ipnet.IP4: {domain.FQDN("a.b.c")},
				},
				ProxiedTemplate:  "false",
				DetectionTimeout: 5 * time.Second,
			},
			ok: true,
			expected: &config.Config{ //nolint:exhaustruct
				UpdateOnStart: true,
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP4: provider.NewCloudflareTrace(),
				},
				Domains: map[ipnet.Type][]domain.Domain{
					ipnet.IP4: {domain.FQDN("a.b.c")},
				},
				ProxiedTemplate: "false",
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
		"dns6empty-ip4none": {
			input: &config.Config{ //nolint:exhaustruct
				UpdateOnStart: true,
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				Domains: map[ipnet.Type][]domain.Domain{
					ipnet.IP4: {domain.FQDN("a.b.c")},
				},
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
		"ip4none": {
			input: &config.Config{ //nolint:exhaustruct
				UpdateOnStart: true,
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				Domains: map[ipnet.Type][]domain.Domain{
					ipnet.IP4: {domain.FQDN("a.b.c"), domain.FQDN("d.e.f")},
					ipnet.IP6: {domain.FQDN("a.b.c"), domain.FQDN("g.h.i")},
				},
				ProxiedTemplate:  "false",
				DetectionTimeout: 5 * time.Second,
			},
			ok: true,
			expected: &config.Config{ //nolint:exhaustruct
				UpdateOnStart: true,
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				Domains: map[ipnet.Type][]domain.Domain{
					ipnet.IP4: {domain.FQDN("a.b.c"), domain.FQDN("d.e.f")},
					ipnet.IP6: {domain.FQDN("a.b.c"), domain.FQDN("g.h.i")},
				},
				ProxiedTemplate: "false",
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
					m.EXPECT().Noticef(pp.EmojiUserWarning, "Domain %q is ignored because it is only for %s but %s is disabled", "d.e.f", "IPv4", "IPv4"),
				)
			},
		},
		"ignored/dns": {
			input: &config.Config{ //nolint:exhaustruct
				UpdateOnStart: true,
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				Domains:                            map[ipnet.Type][]domain.Domain{},
				WAFLists:                           []api.WAFList{{AccountID: "account", Name: "list"}},
				TTL:                                10000,
				ProxiedTemplate:                    "true",
				RecordComment:                      "hello",
				ManagedRecordsCommentRegexTemplate: "he",
				DetectionTimeout:                   5 * time.Second,
			},
			ok: true,
			expected: &config.Config{ //nolint:exhaustruct
				UpdateOnStart: true,
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				Domains:                            map[ipnet.Type][]domain.Domain{},
				WAFLists:                           []api.WAFList{{AccountID: "account", Name: "list"}},
				TTL:                                10000,
				ProxiedTemplate:                    "true",
				Proxied:                            map[domain.Domain]bool{},
				RecordComment:                      "hello",
				ManagedRecordsCommentRegexTemplate: "he",
				ManagedRecordsCommentRegex:         regexp.MustCompile("he"),
				DetectionTimeout:                   5 * time.Second,
			},
			prepareMockPP: func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().IsShowing(pp.Info).Return(true),
					m.EXPECT().Infof(pp.EmojiEnvVars, "Checking settings . . ."),
					m.EXPECT().Indent().Return(m),
					m.EXPECT().Noticef(pp.EmojiUserWarning, "TTL=%v is ignored because no domains will be updated", api.TTL(10000)),
					m.EXPECT().Noticef(pp.EmojiUserWarning, "PROXIED=%s is ignored because no domains will be updated", "true"),
					m.EXPECT().Noticef(pp.EmojiUserWarning, "RECORD_COMMENT=%s is ignored because no domains will be updated", "hello"),
					m.EXPECT().Noticef(pp.EmojiUserWarning, "MANAGED_RECORDS_COMMENT_REGEX=%s is ignored because no domains will be updated", "he"),
				)
			},
		},
		"managed-record-regex/valid": {
			input: &config.Config{ //nolint:exhaustruct
				UpdateOnStart: true,
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				Domains: map[ipnet.Type][]domain.Domain{
					ipnet.IP6: {domain.FQDN("a.b.c")},
				},
				ProxiedTemplate:                    "false",
				RecordComment:                      "hello-123",
				ManagedRecordsCommentRegexTemplate: `^hello-[0-9]+$`,
				DetectionTimeout:                   5 * time.Second,
			},
			ok: true,
			expected: &config.Config{ //nolint:exhaustruct
				UpdateOnStart: true,
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				Domains: map[ipnet.Type][]domain.Domain{
					ipnet.IP6: {domain.FQDN("a.b.c")},
				},
				ProxiedTemplate: "false",
				Proxied: map[domain.Domain]bool{
					domain.FQDN("a.b.c"): false,
				},
				RecordComment:                      "hello-123",
				ManagedRecordsCommentRegexTemplate: `^hello-[0-9]+$`,
				ManagedRecordsCommentRegex:         regexp.MustCompile(`^hello-[0-9]+$`),
				DetectionTimeout:                   5 * time.Second,
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
			input: &config.Config{ //nolint:exhaustruct
				UpdateOnStart: true,
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				Domains: map[ipnet.Type][]domain.Domain{
					ipnet.IP6: {domain.FQDN("a.b.c")},
				},
				ProxiedTemplate:                    "false",
				ManagedRecordsCommentRegexTemplate: "(",
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
			input: &config.Config{ //nolint:exhaustruct
				UpdateOnStart: true,
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				Domains: map[ipnet.Type][]domain.Domain{
					ipnet.IP6: {domain.FQDN("a.b.c")},
				},
				ProxiedTemplate:                    "false",
				RecordComment:                      "hello",
				ManagedRecordsCommentRegexTemplate: "^world$",
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
		"ignored/waf": {
			input: &config.Config{ //nolint:exhaustruct
				UpdateOnStart: true,
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				Domains: map[ipnet.Type][]domain.Domain{
					ipnet.IP6: {domain.FQDN("a.b.c")},
				},
				ProxiedTemplate: "true",
				Proxied: map[domain.Domain]bool{
					domain.FQDN("a.b.c"): true,
				},
				WAFListDescription: "My list",
				DetectionTimeout:   5 * time.Second,
			},
			ok: true,
			expected: &config.Config{ //nolint:exhaustruct
				UpdateOnStart: true,
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				Domains: map[ipnet.Type][]domain.Domain{
					ipnet.IP6: {domain.FQDN("a.b.c")},
				},
				ProxiedTemplate: "true",
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
				UpdateOnStart: true,
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				Domains: map[ipnet.Type][]domain.Domain{
					ipnet.IP6: {domain.FQDN("a.b.c"), domain.FQDN("a.bb.c"), domain.FQDN("a.d.e.f")},
				},
				ProxiedTemplate:  ` true && !is(a.bb.c) `,
				DetectionTimeout: 5 * time.Second,
			},
			ok: true,
			expected: &config.Config{ //nolint:exhaustruct
				UpdateOnStart: true,
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				Domains: map[ipnet.Type][]domain.Domain{
					ipnet.IP6: {domain.FQDN("a.b.c"), domain.FQDN("a.bb.c"), domain.FQDN("a.d.e.f")},
				},
				ProxiedTemplate: ` true && !is(a.bb.c) `,
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
				UpdateOnStart: true,
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				Domains: map[ipnet.Type][]domain.Domain{
					ipnet.IP6: {domain.FQDN("a.b.c"), domain.FQDN("a.bb.c"), domain.FQDN("a.d.e.f")},
				},
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
				UpdateOnStart: true,
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				Domains: map[ipnet.Type][]domain.Domain{
					ipnet.IP6: {domain.FQDN("a.b.c")},
				},
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
				UpdateOnStart: true,
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				Domains: map[ipnet.Type][]domain.Domain{
					ipnet.IP6: {domain.FQDN("a.b.c")},
				},
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
				require.NotNil(t, cfg.ManagedRecordsCommentRegex)
				require.Equal(t, cfg.ManagedRecordsCommentRegexTemplate, cfg.ManagedRecordsCommentRegex.String())

				expected := *tc.expected
				if expected.ManagedRecordsCommentRegex == nil && expected.ManagedRecordsCommentRegexTemplate == "" {
					expected.ManagedRecordsCommentRegex = regexp.MustCompile("")
				}
				require.Equal(t, &expected, cfg)
			} else {
				require.Nil(t, tc.expected) // check the test case itself is okay
				require.Equal(t, tc.input, cfg)
			}
		})
	}
}
