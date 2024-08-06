package config_test

import (
	"context"
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
	store(t, "CF_API_TOKEN", "deadbeaf")

	var cfg config.Config
	mockPP := mocks.NewMockPP(mockCtrl)
	innerMockPP := mocks.NewMockPP(mockCtrl)
	gomock.InOrder(
		mockPP.EXPECT().IsEnabledFor(pp.Info).Return(true),
		mockPP.EXPECT().Infof(pp.EmojiEnvVars, "Reading settings . . ."),
		mockPP.EXPECT().IncIndent().Return(innerMockPP),
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
		mockPP.EXPECT().IsEnabledFor(pp.Info).Return(true),
		mockPP.EXPECT().Infof(pp.EmojiEnvVars, "Reading settings . . ."),
		mockPP.EXPECT().IncIndent().Return(innerMockPP),
		innerMockPP.EXPECT().Errorf(pp.EmojiUserError, "Needs either CF_API_TOKEN or CF_API_TOKEN_FILE"),
	)
	ok := cfg.ReadEnv(mockPP)
	require.False(t, ok)
}

type fakeAuth struct {
	supportsRecords  bool
	supportsWAFLists bool
}

func (a fakeAuth) New(context.Context, pp.PP, time.Duration) (api.Handle, bool) {
	return nil, false
}
func (a fakeAuth) SupportsRecords() bool  { return a.supportsRecords }
func (a fakeAuth) SupportsWAFLists() bool { return a.supportsWAFLists }

//nolint:funlen
func TestNormalize(t *testing.T) {
	t.Parallel()

	keyProxied := "PROXIED"

	for name, tc := range map[string]struct {
		input         *config.Config
		ok            bool
		expected      *config.Config
		prepareMockPP func(m *mocks.MockPP)
	}{
		"authnil": {
			input:    &config.Config{}, //nolint:exhaustruct
			ok:       false,
			expected: nil,
			prepareMockPP: func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().IsEnabledFor(pp.Info).Return(true),
					m.EXPECT().Infof(pp.EmojiEnvVars, "Checking settings . . ."),
					m.EXPECT().IncIndent().Return(m),
					m.EXPECT().Errorf(pp.EmojiImpossible, "Authorization info is missing, but this should be impossible"),
					m.EXPECT().Errorf(pp.EmojiImpossible, "Please report the bug at https://github.com/favonia/cloudflare-ddns/issues/new"), //nolint:lll
				)
			},
		},
		"nothing-to-do": {
			input: &config.Config{ //nolint:exhaustruct
				Auth: fakeAuth{true, true},
			},
			ok:       false,
			expected: nil,
			prepareMockPP: func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().IsEnabledFor(pp.Info).Return(true),
					m.EXPECT().Infof(pp.EmojiEnvVars, "Checking settings . . ."),
					m.EXPECT().IncIndent().Return(m),
					m.EXPECT().Errorf(pp.EmojiUserError, "Nothing was specified in DOMAINS, IP4_DOMAINS, IP6_DOMAINS, or WAF_LISTS"),
				)
			},
		},
		"auth-dns-unsupported": {
			input: &config.Config{ //nolint:exhaustruct
				Auth: fakeAuth{false, true},
				Domains: map[ipnet.Type][]domain.Domain{
					ipnet.IP4: {domain.FQDN("a.b.c")},
				},
			},
			ok:       false,
			expected: nil,
			prepareMockPP: func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().IsEnabledFor(pp.Info).Return(true),
					m.EXPECT().Infof(pp.EmojiEnvVars, "Checking settings . . ."),
					m.EXPECT().IncIndent().Return(m),
					m.EXPECT().Errorf(pp.EmojiImpossible, "CF_API_TOKEN is empty, but the updater should have stopped earlier"),
					m.EXPECT().Errorf(pp.EmojiImpossible, "Please report the bug at https://github.com/favonia/cloudflare-ddns/issues/new"), //nolint:lll
				)
			},
		},
		"auth-waf-unsupported": {
			input: &config.Config{ //nolint:exhaustruct
				Auth:     fakeAuth{true, false},
				WAFLists: []string{"list"},
			},
			ok:       false,
			expected: nil,
			prepareMockPP: func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().IsEnabledFor(pp.Info).Return(true),
					m.EXPECT().Infof(pp.EmojiEnvVars, "Checking settings . . ."),
					m.EXPECT().IncIndent().Return(m),
					m.EXPECT().Errorf(pp.EmojiUserError, "Please give CF_ACCOUNT_ID to specify the WAF lists to update"),
				)
			},
		},
		"once/update-on-start": {
			input: &config.Config{ //nolint:exhaustruct
				Auth: fakeAuth{true, true},
				Domains: map[ipnet.Type][]domain.Domain{
					ipnet.IP4: {domain.FQDN("a.b.c")},
				},
				UpdateOnStart: false,
			},
			ok:       false,
			expected: nil,
			prepareMockPP: func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().IsEnabledFor(pp.Info).Return(true),
					m.EXPECT().Infof(pp.EmojiEnvVars, "Checking settings . . ."),
					m.EXPECT().IncIndent().Return(m),
					m.EXPECT().Errorf(pp.EmojiUserError, "UPDATE_ON_START=false is incompatible with UPDATE_CRON=@once"),
				)
			},
		},
		"once/delete-on-stop": {
			input: &config.Config{ //nolint:exhaustruct
				Auth: fakeAuth{true, true},
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
					m.EXPECT().IsEnabledFor(pp.Info).Return(true),
					m.EXPECT().Infof(pp.EmojiEnvVars, "Checking settings . . ."),
					m.EXPECT().IncIndent().Return(m),
					m.EXPECT().Errorf(pp.EmojiUserError, "DELETE_ON_STOP=true will immediately delete all domains and WAF lists when UPDATE_CRON=@once"), //nolint:lll
				)
			},
		},
		"nilprovider": {
			input: &config.Config{ //nolint:exhaustruct
				Auth:          fakeAuth{true, true},
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
					m.EXPECT().IsEnabledFor(pp.Info).Return(true),
					m.EXPECT().Infof(pp.EmojiEnvVars, "Checking settings . . ."),
					m.EXPECT().IncIndent().Return(m),
					m.EXPECT().Errorf(pp.EmojiUserError,
						"Nothing to update because both IP4_PROVIDER and IP6_PROVIDER are %q",
						"none"),
				)
			},
		},
		"dns6empty": {
			input: &config.Config{ //nolint:exhaustruct
				Auth:          fakeAuth{true, true},
				UpdateOnStart: true,
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP4: provider.NewCloudflareTrace(),
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				Domains: map[ipnet.Type][]domain.Domain{
					ipnet.IP4: {domain.FQDN("a.b.c")},
				},
				ProxiedTemplate: "false",
			},
			ok: true,
			expected: &config.Config{ //nolint:exhaustruct
				Auth:          fakeAuth{true, true},
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
			},
			prepareMockPP: func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().IsEnabledFor(pp.Info).Return(true),
					m.EXPECT().Infof(pp.EmojiEnvVars, "Checking settings . . ."),
					m.EXPECT().IncIndent().Return(m),
					m.EXPECT().Warningf(pp.EmojiUserWarning,
						"IP%d_PROVIDER was changed to %q because no domains or WAF lists use %s",
						6, "none", "IPv6"),
				)
			},
		},
		"dns6empty-ip4none": {
			input: &config.Config{ //nolint:exhaustruct
				Auth:          fakeAuth{true, true},
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
					m.EXPECT().IsEnabledFor(pp.Info).Return(true),
					m.EXPECT().Infof(pp.EmojiEnvVars, "Checking settings . . ."),
					m.EXPECT().IncIndent().Return(m),
					m.EXPECT().Warningf(pp.EmojiUserWarning,
						"IP%d_PROVIDER was changed to %q because no domains or WAF lists use %s",
						6, "none", "IPv6"),
					m.EXPECT().Errorf(pp.EmojiUserError,
						"Nothing to update because both IP4_PROVIDER and IP6_PROVIDER are %q",
						"none"),
				)
			},
		},
		"ip4none": {
			input: &config.Config{ //nolint:exhaustruct
				Auth:          fakeAuth{true, true},
				UpdateOnStart: true,
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				Domains: map[ipnet.Type][]domain.Domain{
					ipnet.IP4: {domain.FQDN("a.b.c"), domain.FQDN("d.e.f")},
					ipnet.IP6: {domain.FQDN("a.b.c"), domain.FQDN("g.h.i")},
				},
				ProxiedTemplate: "false",
			},
			ok: true,
			expected: &config.Config{ //nolint:exhaustruct
				Auth:          fakeAuth{true, true},
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
			},
			prepareMockPP: func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().IsEnabledFor(pp.Info).Return(true),
					m.EXPECT().Infof(pp.EmojiEnvVars, "Checking settings . . ."),
					m.EXPECT().IncIndent().Return(m),
					m.EXPECT().Warningf(pp.EmojiUserWarning,
						"Domain %q is ignored because it is only for %s but %s is disabled",
						"d.e.f", "IPv4", "IPv4"),
				)
			},
		},
		"ignored/dns": {
			input: &config.Config{ //nolint:exhaustruct
				Auth:          fakeAuth{true, true},
				UpdateOnStart: true,
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				Domains:         map[ipnet.Type][]domain.Domain{},
				WAFLists:        []string{"list"},
				TTL:             10000,
				ProxiedTemplate: "true",
				RecordComment:   "hello",
			},
			ok: true,
			expected: &config.Config{ //nolint:exhaustruct
				Auth:          fakeAuth{true, true},
				UpdateOnStart: true,
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				Domains:         map[ipnet.Type][]domain.Domain{},
				WAFLists:        []string{"list"},
				TTL:             10000,
				ProxiedTemplate: "true",
				Proxied:         map[domain.Domain]bool{},
				RecordComment:   "hello",
			},
			prepareMockPP: func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().IsEnabledFor(pp.Info).Return(true),
					m.EXPECT().Infof(pp.EmojiEnvVars, "Checking settings . . ."),
					m.EXPECT().IncIndent().Return(m),
					m.EXPECT().Warningf(pp.EmojiUserWarning,
						"TTL=%v is ignored because no domains will be updated",
						api.TTL(10000)),
					m.EXPECT().Warningf(pp.EmojiUserWarning,
						"PROXIED=%s is ignored because no domains will be updated",
						"true"),
					m.EXPECT().Warningf(pp.EmojiUserWarning,
						"RECORD_COMMENT=%s is ignored because no domains will be updated",
						"hello"),
				)
			},
		},
		"ignored/waf": {
			input: &config.Config{ //nolint:exhaustruct
				Auth:          fakeAuth{true, true},
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
			},
			ok: true,
			expected: &config.Config{ //nolint:exhaustruct
				Auth:          fakeAuth{true, true},
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
			},
			prepareMockPP: func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().IsEnabledFor(pp.Info).Return(true),
					m.EXPECT().Infof(pp.EmojiEnvVars, "Checking settings . . ."),
					m.EXPECT().IncIndent().Return(m),
					m.EXPECT().Warningf(pp.EmojiUserWarning,
						"WAF_LIST_DESCRIPTION=%s is ignored because no WAF lists will be updated",
						"My list"),
				)
			},
		},
		"proxied": {
			input: &config.Config{ //nolint:exhaustruct
				Auth:          fakeAuth{true, true},
				UpdateOnStart: true,
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				Domains: map[ipnet.Type][]domain.Domain{
					ipnet.IP6: {domain.FQDN("a.b.c"), domain.FQDN("a.bb.c"), domain.FQDN("a.d.e.f")},
				},
				ProxiedTemplate: ` true && !is(a.bb.c) `,
			},
			ok: true,
			expected: &config.Config{ //nolint:exhaustruct
				Auth:          fakeAuth{true, true},
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
			},
			prepareMockPP: func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().IsEnabledFor(pp.Info).Return(true),
					m.EXPECT().Infof(pp.EmojiEnvVars, "Checking settings . . ."),
					m.EXPECT().IncIndent().Return(m),
				)
			},
		},
		"proxied/invalid/1": {
			input: &config.Config{ //nolint:exhaustruct
				Auth:          fakeAuth{true, true},
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
					m.EXPECT().IsEnabledFor(pp.Info).Return(true),
					m.EXPECT().Infof(pp.EmojiEnvVars, "Checking settings . . ."),
					m.EXPECT().IncIndent().Return(m),
					m.EXPECT().Errorf(pp.EmojiUserError, "%s (%q) is not a boolean expression: got unexpected token %q", keyProxied, `range`, `range`), //nolint:lll
				)
			},
		},
		"proxied/invalid/2": {
			input: &config.Config{ //nolint:exhaustruct
				Auth:          fakeAuth{true, true},
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
					m.EXPECT().IsEnabledFor(pp.Info).Return(true),
					m.EXPECT().Infof(pp.EmojiEnvVars, "Checking settings . . ."),
					m.EXPECT().IncIndent().Return(m),
					m.EXPECT().Errorf(pp.EmojiUserError, "%s (%q) is not a boolean expression: got unexpected token %q", keyProxied, `999`, `999`), //nolint:lll
				)
			},
		},
		"proxied/invalid/3": {
			input: &config.Config{ //nolint:exhaustruct
				Auth:          fakeAuth{true, true},
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
					m.EXPECT().IsEnabledFor(pp.Info).Return(true),
					m.EXPECT().Infof(pp.EmojiEnvVars, "Checking settings . . ."),
					m.EXPECT().IncIndent().Return(m),
					m.EXPECT().Errorf(pp.EmojiUserError, `%s (%q) is missing %q at the end`, keyProxied, `is(12345`, ")"),
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
