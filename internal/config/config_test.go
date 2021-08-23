package config_test

import (
	"testing"
	"testing/fstest"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/config"
	"github.com/favonia/cloudflare-ddns/internal/cron"
	"github.com/favonia/cloudflare-ddns/internal/detector"
	"github.com/favonia/cloudflare-ddns/internal/file"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func TestDefaultConfigNotNil(t *testing.T) {
	t.Parallel()

	require.NotNil(t, config.Default())
}

//nolint:paralleltest // environment vars are global
func TestReadAuthToken(t *testing.T) {
	unset(t, "CF_API_TOKEN")
	unset(t, "CF_API_TOKEN_FILE")
	unset(t, "CF_ACCOUNT_ID")

	for name, tc := range map[string]struct {
		token         string
		account       string
		ok            bool
		prepareMockPP func(*mocks.MockPP)
	}{
		"full":      {"123456789", "secret account", true, nil},
		"noaccount": {"123456789", "", true, nil},
		"notoken": {
			"", "account", false,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiUserError, "Needs either CF_API_TOKEN or CF_API_TOKEN_FILE")
			},
		},
		"copycat": {
			"YOUR-CLOUDFLARE-API-TOKEN", "", false,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiUserError, "You need to provide a real API token as CF_API_TOKEN")
			},
		},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)

			store(t, "CF_API_TOKEN", tc.token)
			store(t, "CF_ACCOUNT_ID", tc.account)

			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}

			var field api.Auth
			ok := config.ReadAuth(mockPP, &field)
			require.Equal(t, tc.ok, ok)
			if tc.ok {
				require.Equal(t, &api.CloudflareAuth{Token: tc.token, AccountID: tc.account, BaseURL: ""}, field)
			} else {
				require.Nil(t, field)
			}
		})
	}
}

func useMemFS(memfs fstest.MapFS) {
	file.FS = memfs
}

//nolint:funlen,paralleltest // environment vars and file system are global
func TestReadAuthTokenWithFile(t *testing.T) {
	unset(t, "CF_API_TOKEN")
	unset(t, "CF_API_TOKEN_FILE")
	unset(t, "CF_ACCOUNT_ID")

	for name, tc := range map[string]struct {
		token         string
		tokenFile     string
		account       string
		actualPath    string
		actualContent string
		expected      string
		ok            bool
		prepareMockPP func(*mocks.MockPP)
	}{
		"ok": {"", "test.txt", "secret account", "test.txt", "hello", "hello", true, nil},
		"both": {
			"123456789", "test.txt", "secret account", "test.txt", "hello", "", false,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiUserError, "Cannot have both CF_API_TOKEN and CF_API_TOKEN_FILE set")
			},
		},
		"wrong.path": {
			"", "wrong.txt", "secret account", "actual.txt", "hello", "", false,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiUserError, "Failed to read %q: %v", "wrong.txt", gomock.Any())
			},
		},
		"empty": {
			"", "test.txt", "secret account", "test.txt", "", "", false,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiUserError, "The token in the file specified by CF_API_TOKEN_FILE is empty")
			},
		},
		"invalid path": {
			"", "dir", "secret account", "dir/test.txt", "hello", "", false,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiUserError, "Failed to read %q: %v", "dir", gomock.Any())
			},
		},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)

			store(t, "CF_API_TOKEN", tc.token)
			store(t, "CF_API_TOKEN_FILE", tc.tokenFile)
			store(t, "CF_ACCOUNT_ID", tc.account)

			useMemFS(fstest.MapFS{
				tc.actualPath: &fstest.MapFile{
					Data:    []byte(tc.actualContent),
					Mode:    0o644,
					ModTime: time.Unix(1234, 5678),
					Sys:     nil,
				},
			})

			var field api.Auth
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}
			ok := config.ReadAuth(mockPP, &field)
			require.Equal(t, tc.ok, ok)
			if tc.expected != "" {
				require.Equal(t, &api.CloudflareAuth{Token: tc.expected, AccountID: tc.account, BaseURL: ""}, field)
			} else {
				require.Nil(t, field)
			}
		})
	}
}

//nolint:paralleltest // environment vars are global
func TestReadDomainMap(t *testing.T) {
	for name, tc := range map[string]struct {
		domains       string
		ip4Domains    string
		ip6Domains    string
		expected      map[ipnet.Type][]api.FQDN
		ok            bool
		prepareMockPP func(*mocks.MockPP)
	}{
		"full": {
			"  a1, a2", "b1,  b2,b2", "c1,c2",
			map[ipnet.Type][]api.FQDN{
				ipnet.IP4: {"a1", "a2", "b1", "b2"},
				ipnet.IP6: {"a1", "a2", "c1", "c2"},
			},
			true,
			nil,
		},
		"empty": {
			" ", "   ", "",
			map[ipnet.Type][]api.FQDN{
				ipnet.IP4: {},
				ipnet.IP6: {},
			},
			true,
			nil,
		},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)

			store(t, "DOMAINS", tc.domains)
			store(t, "IP4_DOMAINS", tc.ip4Domains)
			store(t, "IP6_DOMAINS", tc.ip6Domains)

			field := map[ipnet.Type][]api.FQDN{}
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}
			ok := config.ReadDomainMap(mockPP, field)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.expected, field)
		})
	}
}

//nolint:funlen,paralleltest // environment vars are global
func TestReadPolicyMap(t *testing.T) {
	var (
		unmanaged  detector.Policy
		cloudflare = detector.NewCloudflare()
		local      = detector.NewLocal()
		ipify      = detector.NewIpify()
	)

	for name, tc := range map[string]struct {
		ip4Policy     string
		ip6Policy     string
		expected      map[ipnet.Type]detector.Policy
		ok            bool
		prepareMockPP func(*mocks.MockPP)
	}{
		"full": {
			"cloudflare", "ipify",
			map[ipnet.Type]detector.Policy{
				ipnet.IP4: cloudflare,
				ipnet.IP6: ipify,
			},
			true,
			nil,
		},
		"4": {
			"local", "  ",
			map[ipnet.Type]detector.Policy{
				ipnet.IP4: local,
				ipnet.IP6: local,
			},
			true,
			func(m *mocks.MockPP) {
				m.EXPECT().Infof(pp.EmojiBullet, "Use default %s=%s", "IP6_POLICY", "local")
			},
		},
		"6": {
			"    ", "ipify",
			map[ipnet.Type]detector.Policy{
				ipnet.IP4: unmanaged,
				ipnet.IP6: ipify,
			},
			true,
			func(m *mocks.MockPP) {
				m.EXPECT().Infof(pp.EmojiBullet, "Use default %s=%s", "IP4_POLICY", "unmanaged")
			},
		},
		"empty": {
			" ", "   ",
			map[ipnet.Type]detector.Policy{
				ipnet.IP4: unmanaged,
				ipnet.IP6: local,
			},
			true,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Infof(pp.EmojiBullet, "Use default %s=%s", "IP4_POLICY", "unmanaged"),
					m.EXPECT().Infof(pp.EmojiBullet, "Use default %s=%s", "IP6_POLICY", "local"),
				)
			},
		},
		"illformed": {
			" flare", "   ",
			map[ipnet.Type]detector.Policy{
				ipnet.IP4: unmanaged,
				ipnet.IP6: local,
			},
			false,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiUserError, "Failed to parse %q: not a valid policy", "flare")
			},
		},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)

			store(t, "IP4_POLICY", tc.ip4Policy)
			store(t, "IP6_POLICY", tc.ip6Policy)

			field := map[ipnet.Type]detector.Policy{ipnet.IP4: unmanaged, ipnet.IP6: local}
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}
			ok := config.ReadPolicyMap(mockPP, field)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.expected, field)
		})
	}
}

func TestPrintDefault(t *testing.T) {
	t.Parallel()
	mockCtrl := gomock.NewController(t)

	store(t, "TZ", "UTC")

	mockPP := mocks.NewMockPP(mockCtrl)
	innerMockPP := mocks.NewMockPP(mockCtrl)
	gomock.InOrder(
		mockPP.EXPECT().Infof(pp.EmojiEnvVars, "Current settings:"),
		mockPP.EXPECT().IncIndent().Return(mockPP),
		mockPP.EXPECT().IncIndent().Return(innerMockPP),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "Policies:"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "IPv4 policy:      %s", "cloudflare"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "IPv4 domains:     %v", []api.FQDN(nil)),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "IPv6 policy:      %s", "cloudflare"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "IPv6 domains:     %v", []api.FQDN(nil)),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "Scheduling:"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Timezone:         %s", "UTC (UTC+00 now)"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Update frequency: %v", cron.MustNew("@every 5m")),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Update on start?  %t", true),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Delete on stop?   %t", false),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Cache expiration: %v", time.Hour*6),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "New DNS records:"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "TTL:              %s", "1 (automatic)"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Proxied:          %t", false),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "Timeouts"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "IP detection:     %v", time.Second*5),
	)
	config.Print(mockPP, config.Default())
}

func TestPrintEmpty(t *testing.T) {
	t.Parallel()
	mockCtrl := gomock.NewController(t)

	store(t, "TZ", "UTC")

	mockPP := mocks.NewMockPP(mockCtrl)
	innerMockPP := mocks.NewMockPP(mockCtrl)
	gomock.InOrder(
		mockPP.EXPECT().Infof(pp.EmojiEnvVars, "Current settings:"),
		mockPP.EXPECT().IncIndent().Return(mockPP),
		mockPP.EXPECT().IncIndent().Return(innerMockPP),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "Policies:"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "IPv4 policy:      %s", "unmanaged"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "IPv6 policy:      %s", "unmanaged"),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "Scheduling:"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Timezone:         %s", "UTC (UTC+00 now)"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Update frequency: %v", cron.Schedule(nil)),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Update on start?  %t", false),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Delete on stop?   %t", false),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Cache expiration: %v", time.Duration(0)),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "New DNS records:"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "TTL:              %s", "0"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Proxied:          %t", false),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "Timeouts"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "IP detection:     %v", time.Duration(0)),
	)
	config.Print(mockPP, &config.Config{})
}

func TestNormalize(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		input         *config.Config
		ok            bool
		expected      *config.Config
		prepareMockPP func(*mocks.MockPP)
	}{
		"nil": {
			input:    &config.Config{},
			ok:       false,
			expected: &config.Config{},
			prepareMockPP: func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiUserError, "No domains were specified")
			},
		},
		"empty": {
			input: &config.Config{
				Domains: map[ipnet.Type][]api.FQDN{
					ipnet.IP4: {},
					ipnet.IP6: {},
				},
			},
			ok: false,
			expected: &config.Config{
				Domains: map[ipnet.Type][]api.FQDN{
					ipnet.IP4: {},
					ipnet.IP6: {},
				},
			},
			prepareMockPP: func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiUserError, "No domains were specified")
			},
		},
		"empty-ip6": {
			input: &config.Config{
				Policy: map[ipnet.Type]detector.Policy{
					ipnet.IP4: detector.NewCloudflare(),
					ipnet.IP6: detector.NewCloudflare(),
				},
				Domains: map[ipnet.Type][]api.FQDN{
					ipnet.IP4: {api.FQDN("a.b.c")},
					ipnet.IP6: {},
				},
			},
			ok: true,
			expected: &config.Config{
				Policy: map[ipnet.Type]detector.Policy{
					ipnet.IP4: detector.NewCloudflare(),
					ipnet.IP6: nil,
				},
				Domains: map[ipnet.Type][]api.FQDN{
					ipnet.IP4: {api.FQDN("a.b.c")},
					ipnet.IP6: {},
				},
			},
			prepareMockPP: func(m *mocks.MockPP) {
				m.EXPECT().Warningf(pp.EmojiUserWarning,
					"IP%d_POLICY was changed to %q because no domains were set for %s",
					6, "unmanaged", "IPv6")
			},
		},
		"empty-ip6-unmanaged-ip4": {
			input: &config.Config{
				Policy: map[ipnet.Type]detector.Policy{
					ipnet.IP4: nil,
					ipnet.IP6: detector.NewCloudflare(),
				},
				Domains: map[ipnet.Type][]api.FQDN{
					ipnet.IP4: {api.FQDN("a.b.c")},
					ipnet.IP6: {},
				},
			},
			ok: false,
			expected: &config.Config{
				Policy: map[ipnet.Type]detector.Policy{
					ipnet.IP4: nil,
					ipnet.IP6: nil,
				},
				Domains: map[ipnet.Type][]api.FQDN{
					ipnet.IP4: {api.FQDN("a.b.c")},
					ipnet.IP6: {},
				},
			},
			prepareMockPP: func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Warningf(pp.EmojiUserWarning,
						"IP%d_POLICY was changed to %q because no domains were set for %s",
						6, "unmanaged", "IPv6"),
					m.EXPECT().Errorf(pp.EmojiUserError, "Both IPv4 and IPv6 are unmanaged"),
				)
			},
		},
		"ignored-ip4-domains": {
			input: &config.Config{
				Policy: map[ipnet.Type]detector.Policy{
					ipnet.IP4: nil,
					ipnet.IP6: detector.NewCloudflare(),
				},
				Domains: map[ipnet.Type][]api.FQDN{
					ipnet.IP4: {api.FQDN("a.b.c"), api.FQDN("d.e.f")},
					ipnet.IP6: {api.FQDN("a.b.c")},
				},
			},
			ok: true,
			expected: &config.Config{
				Policy: map[ipnet.Type]detector.Policy{
					ipnet.IP4: nil,
					ipnet.IP6: detector.NewCloudflare(),
				},
				Domains: map[ipnet.Type][]api.FQDN{
					ipnet.IP4: {api.FQDN("a.b.c"), api.FQDN("d.e.f")},
					ipnet.IP6: {api.FQDN("a.b.c")},
				},
			},
			prepareMockPP: func(m *mocks.MockPP) {
				m.EXPECT().Warningf(pp.EmojiUserWarning,
					"Domain %q is ignored because it is only for %s but %s is unmanaged",
					"d.e.f", "IPv4", "IPv4")
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
			ok := cfg.Normalize(mockPP)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.expected, cfg)
		})
	}
}
