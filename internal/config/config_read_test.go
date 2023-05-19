package config_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/config"
	"github.com/favonia/cloudflare-ddns/internal/cron"
	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider"
)

//nolint:paralleltest // environment variables are global
func TestReadEnvWithOnlyToken(t *testing.T) {
	for _, use1001 := range []bool{true, false} {
		t.Run(fmt.Sprintf("use1001=%t", use1001), func(t *testing.T) {
			mockCtrl := gomock.NewController(t)

			unset(t,
				"CF_API_TOKEN", "CF_API_TOKEN_FILE", "CF_ACCOUNT_ID",
				"IP4_PROVIDER", "IP6_PROVIDER",
				"DOMAINS", "IP4_DOMAINS", "IP6_DOMAINS",
				"UPDATE_CRON", "UPDATE_ON_START", "DELETE_ON_STOP", "CACHE_EXPIRATION", "TTL", "PROXIED", "DETECTION_TIMEOUT")

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
				innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Use default %s=%s", "UPDATE_CRON", "@disabled"),
				innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Use default %s=%t", "UPDATE_ON_START", false),
				innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Use default %s=%t", "DELETE_ON_STOP", false),
				innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Use default %s=%v", "CACHE_EXPIRATION", time.Duration(0)),
				innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Use default %s=%d", "TTL", api.TTL(0)),
				innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Use default %s=%s", "PROXIED", ""),
				innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Use default %s=%v", "DETECTION_TIMEOUT", time.Duration(0)),
				innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Use default %s=%v", "UPDATE_TIMEOUT", time.Duration(0)),
			)
			ok := cfg.ReadEnv(mockPP, use1001)
			require.True(t, ok)
		})
	}
}

//nolint:paralleltest // environment variables are global
func TestReadEnvEmpty(t *testing.T) {
	for _, use1001 := range []bool{true, false} {
		t.Run(fmt.Sprintf("use1001=%t", use1001), func(t *testing.T) {
			mockCtrl := gomock.NewController(t)

			unset(t,
				"CF_API_TOKEN", "CF_API_TOKEN_FILE", "CF_ACCOUNT_ID",
				"IP4_PROVIDER", "IP6_PROVIDER",
				"IP4_POLICY", "IP6_POLICY",
				"DOMAINS", "IP4_DOMAINS", "IP6_DOMAINS",
				"UPDATE_CRON", "UPDATE_ON_START", "DELETE_ON_STOP", "CACHE_EXPIRATION", "TTL", "PROXIED", "DETECTION_TIMEOUT")

			var cfg config.Config
			mockPP := mocks.NewMockPP(mockCtrl)
			innerMockPP := mocks.NewMockPP(mockCtrl)
			gomock.InOrder(
				mockPP.EXPECT().IsEnabledFor(pp.Info).Return(true),
				mockPP.EXPECT().Infof(pp.EmojiEnvVars, "Reading settings . . ."),
				mockPP.EXPECT().IncIndent().Return(innerMockPP),
				innerMockPP.EXPECT().Errorf(pp.EmojiUserError, "Needs either CF_API_TOKEN or CF_API_TOKEN_FILE"),
			)
			ok := cfg.ReadEnv(mockPP, use1001)
			require.False(t, ok)
		})
	}
}

//nolint:funlen
func TestNormalizeConfig(t *testing.T) {
	t.Parallel()

	keyProxied := "PROXIED"
	var empty config.Config

	for name, tc := range map[string]struct {
		input         *config.Config
		ok            bool
		expected      *config.Config
		prepareMockPP func(m *mocks.MockPP)
	}{
		"nil": {
			input:    &empty,
			ok:       false,
			expected: &empty,
			prepareMockPP: func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().IsEnabledFor(pp.Info).Return(true),
					m.EXPECT().Infof(pp.EmojiEnvVars, "Checking settings . . ."),
					m.EXPECT().IncIndent().Return(m),
					m.EXPECT().Errorf(pp.EmojiUserError, "No domains were specified in DOMAINS, IP4_DOMAINS, or IP6_DOMAINS"),
				)
			},
		},
		"empty": {
			input: &config.Config{ //nolint:exhaustruct
				Domains: map[ipnet.Type][]domain.Domain{
					ipnet.IP4: {},
					ipnet.IP6: {},
				},
			},
			ok:       false,
			expected: nil,
			prepareMockPP: func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().IsEnabledFor(pp.Info).Return(true),
					m.EXPECT().Infof(pp.EmojiEnvVars, "Checking settings . . ."),
					m.EXPECT().IncIndent().Return(m),
					m.EXPECT().Errorf(pp.EmojiUserError, "No domains were specified in DOMAINS, IP4_DOMAINS, or IP6_DOMAINS"),
				)
			},
		},
		"empty-ip6": {
			input: &config.Config{ //nolint:exhaustruct
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP4: provider.NewCloudflareTrace(true),
					ipnet.IP6: provider.NewCloudflareTrace(false),
				},
				Domains: map[ipnet.Type][]domain.Domain{
					ipnet.IP4: {domain.FQDN("a.b.c")},
					ipnet.IP6: {},
				},
				ProxiedTemplate: "false",
			},
			ok: true,
			expected: &config.Config{ //nolint:exhaustruct
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP4: provider.NewCloudflareTrace(true),
				},
				Domains: map[ipnet.Type][]domain.Domain{
					ipnet.IP4: {domain.FQDN("a.b.c")},
					ipnet.IP6: {},
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
						"IP%d_PROVIDER was changed to %q because no domains were set for %s",
						6, "none", "IPv6"),
				)
			},
		},
		"empty-ip6-none-ip4": {
			input: &config.Config{ //nolint:exhaustruct
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP6: provider.NewCloudflareTrace(true),
				},
				Domains: map[ipnet.Type][]domain.Domain{
					ipnet.IP4: {domain.FQDN("a.b.c")},
					ipnet.IP6: {},
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
						"IP%d_PROVIDER was changed to %q because no domains were set for %s",
						6, "none", "IPv6"),
					m.EXPECT().Errorf(pp.EmojiUserError,
						"Nothing to update because both IP4_PROVIDER and IP6_PROVIDER are %q",
						"none"),
				)
			},
		},
		"ignored-ip4-domains": {
			input: &config.Config{ //nolint:exhaustruct
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP6: provider.NewCloudflareTrace(true),
				},
				Domains: map[ipnet.Type][]domain.Domain{
					ipnet.IP4: {domain.FQDN("a.b.c"), domain.FQDN("d.e.f")},
					ipnet.IP6: {domain.FQDN("a.b.c"), domain.FQDN("g.h.i")},
				},
				ProxiedTemplate: "false",
			},
			ok: true,
			expected: &config.Config{ //nolint:exhaustruct
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP6: provider.NewCloudflareTrace(true),
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
		"template": {
			input: &config.Config{ //nolint:exhaustruct
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP6: provider.NewCloudflareTrace(false),
				},
				Domains: map[ipnet.Type][]domain.Domain{
					ipnet.IP6: {domain.FQDN("a.b.c"), domain.FQDN("a.bb.c"), domain.FQDN("a.d.e.f")},
				},
				ProxiedTemplate: ` true && !is(a.bb.c) `,
			},
			ok: true,
			expected: &config.Config{ //nolint:exhaustruct
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP6: provider.NewCloudflareTrace(false),
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
		"template/invalid/proxied": {
			input: &config.Config{ //nolint:exhaustruct
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP6: provider.NewCloudflareTrace(true),
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
		"template/error/proxied": {
			input: &config.Config{ //nolint:exhaustruct
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP6: provider.NewCloudflareTrace(false),
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
		"template/error/proxied/ill-formed": {
			input: &config.Config{ //nolint:exhaustruct
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP6: provider.NewCloudflareTrace(true),
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
		"delete-on-stop/without-cron": {
			input: &config.Config{ //nolint:exhaustruct
				DeleteOnStop: true,
			},
			ok:       false,
			expected: nil,
			prepareMockPP: func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().IsEnabledFor(pp.Info).Return(true),
					m.EXPECT().Infof(pp.EmojiEnvVars, "Checking settings . . ."),
					m.EXPECT().IncIndent().Return(m),
					m.EXPECT().Errorf(pp.EmojiUserError, "DELETE_ON_STOP=true will immediately delete all DNS records when UPDATE_CRON=@disabled"), //nolint:lll
				)
			},
		},
		"delete-on-stop/with-cron": {
			input: &config.Config{ //nolint:exhaustruct
				DeleteOnStop: true,
				UpdateCron:   cron.MustNew("@every 5m"),
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP6: provider.NewCloudflareTrace(false),
				},
				Domains: map[ipnet.Type][]domain.Domain{
					ipnet.IP6: {domain.FQDN("a.b.c")},
				},
				ProxiedTemplate: "false",
			},
			ok: true,
			expected: &config.Config{ //nolint:exhaustruct
				DeleteOnStop: true,
				UpdateCron:   cron.MustNew("@every 5m"),
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP6: provider.NewCloudflareTrace(false),
				},
				Domains: map[ipnet.Type][]domain.Domain{
					ipnet.IP6: {domain.FQDN("a.b.c")},
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
				)
			},
		},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			mockCtrl := gomock.NewController(t)

			cfg := tc.input
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}
			ok := cfg.NormalizeConfig(mockPP)
			require.Equal(t, tc.ok, ok)
			if tc.ok {
				require.Equal(t, tc.expected, cfg)
			} else {
				require.Equal(t, tc.input, cfg)
			}
		})
	}
}
