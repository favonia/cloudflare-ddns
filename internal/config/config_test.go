package config_test

import (
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/config"
	"github.com/favonia/cloudflare-ddns/internal/cron"
	"github.com/favonia/cloudflare-ddns/internal/file"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/monitor"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider"
)

func TestDefaultConfigNotNil(t *testing.T) {
	t.Parallel()

	require.NotNil(t, config.Default())
}

//nolint:paralleltest // environment vars are global
func TestReadAuthToken(t *testing.T) {
	unset(t, "CF_API_TOKEN", "CF_API_TOKEN_FILE", "CF_ACCOUNT_ID")

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
	unset(t, "CF_API_TOKEN", "CF_API_TOKEN_FILE", "CF_ACCOUNT_ID")

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
		expected      map[ipnet.Type][]api.Domain
		ok            bool
		prepareMockPP func(*mocks.MockPP)
	}{
		"full": {
			"  a1, a2", "b1,  b2,b2", "c1,c2",
			map[ipnet.Type][]api.Domain{
				ipnet.IP4: {api.FQDN("a1"), api.FQDN("a2"), api.FQDN("b1"), api.FQDN("b2")},
				ipnet.IP6: {api.FQDN("a1"), api.FQDN("a2"), api.FQDN("c1"), api.FQDN("c2")},
			},
			true,
			nil,
		},
		"duplicate": {
			"  a1, a1", "a1,  a1,a1", "*.a1,a1,*.a1,*.a1",
			map[ipnet.Type][]api.Domain{
				ipnet.IP4: {api.FQDN("a1")},
				ipnet.IP6: {api.FQDN("a1"), api.Wildcard("a1")},
			},
			true,
			nil,
		},
		"empty": {
			" ", "   ", "",
			map[ipnet.Type][]api.Domain{
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

			field := map[ipnet.Type][]api.Domain{}
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}
			ok := config.ReadDomainMap(mockPP, &field)
			require.Equal(t, tc.ok, ok)
			require.ElementsMatch(t, tc.expected[ipnet.IP4], field[ipnet.IP4])
			require.ElementsMatch(t, tc.expected[ipnet.IP6], field[ipnet.IP6])
		})
	}
}

//nolint:funlen,paralleltest // environment vars are global
func TestReadProviderMap(t *testing.T) {
	var (
		none            provider.Provider
		cloudflareTrace = provider.NewCloudflareTrace()
		cloudflareDOH   = provider.NewCloudflareDOH()
		local           = provider.NewLocal()
		ipify           = provider.NewIpify()
	)

	for name, tc := range map[string]struct {
		ip4Provider   string
		ip6Provider   string
		expected      map[ipnet.Type]provider.Provider
		ok            bool
		prepareMockPP func(*mocks.MockPP)
	}{
		"full": {
			"cloudflare.trace", "ipify",
			map[ipnet.Type]provider.Provider{
				ipnet.IP4: cloudflareTrace,
				ipnet.IP6: ipify,
			},
			true,
			nil,
		},
		"4": {
			"local", "  ",
			map[ipnet.Type]provider.Provider{
				ipnet.IP4: local,
				ipnet.IP6: local,
			},
			true,
			func(m *mocks.MockPP) {
				m.EXPECT().Infof(pp.EmojiBullet, "Use default %s=%s", "IP6_PROVIDER", "local")
			},
		},
		"6": {
			"    ", "cloudflare.doh",
			map[ipnet.Type]provider.Provider{
				ipnet.IP4: none,
				ipnet.IP6: cloudflareDOH,
			},
			true,
			func(m *mocks.MockPP) {
				m.EXPECT().Infof(pp.EmojiBullet, "Use default %s=%s", "IP4_PROVIDER", "none")
			},
		},
		"empty": {
			" ", "   ",
			map[ipnet.Type]provider.Provider{
				ipnet.IP4: none,
				ipnet.IP6: local,
			},
			true,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Infof(pp.EmojiBullet, "Use default %s=%s", "IP4_PROVIDER", "none"),
					m.EXPECT().Infof(pp.EmojiBullet, "Use default %s=%s", "IP6_PROVIDER", "local"),
				)
			},
		},
		"illformed": {
			" flare", "   ",
			map[ipnet.Type]provider.Provider{
				ipnet.IP4: none,
				ipnet.IP6: local,
			},
			false,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiUserError, "Failed to parse %q: not a valid provider", "flare")
			},
		},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)

			store(t, "IP4_PROVIDER", tc.ip4Provider)
			store(t, "IP6_PROVIDER", tc.ip6Provider)

			field := map[ipnet.Type]provider.Provider{ipnet.IP4: none, ipnet.IP6: local}
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}
			ok := config.ReadProviderMap(mockPP, &field)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.expected, field)
		})
	}
}

type someMatcher struct {
	matchers []gomock.Matcher
}

func (sm someMatcher) Matches(x interface{}) bool {
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

func Some(xs ...interface{}) gomock.Matcher {
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
	gomock.InOrder( //nolint:dupl
		mockPP.EXPECT().IsEnabledFor(pp.Info).Return(true),
		mockPP.EXPECT().Infof(pp.EmojiEnvVars, "Current settings:"),
		mockPP.EXPECT().IncIndent().Return(mockPP),
		mockPP.EXPECT().IncIndent().Return(innerMockPP),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "Policies:"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "IPv4 provider:    %s", "cloudflare.trace"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "IPv4 domains:     %s", "(none)"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "IPv6 provider:    %s", "cloudflare.trace"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "IPv6 domains:     %s", "(none)"),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "Scheduling:"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Timezone:         %s", Some("UTC (UTC+00 now)", "Local (UTC+00 now)")),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Update frequency: %v", cron.MustNew("@every 5m")),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Update on start?  %t", true),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Delete on stop?   %t", false),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Cache expiration: %v", time.Hour*6),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "New DNS records:"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "TTL:              %s", "1 (automatic)"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Proxied:          %s", "(none)"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Non-proxied:      %s", "(none)"),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "Timeouts:"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "IP detection:     %v", time.Second*5),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Record updating:  %v", time.Second*30),
	)
	config.Default().Print(mockPP)
}

//nolint:paralleltest // changing the environment variable TZ
func TestPrintDomains(t *testing.T) {
	mockCtrl := gomock.NewController(t)

	store(t, "TZ", "UTC")

	mockPP := mocks.NewMockPP(mockCtrl)
	innerMockPP := mocks.NewMockPP(mockCtrl)
	gomock.InOrder( //nolint:dupl
		mockPP.EXPECT().IsEnabledFor(pp.Info).Return(true),
		mockPP.EXPECT().Infof(pp.EmojiEnvVars, "Current settings:"),
		mockPP.EXPECT().IncIndent().Return(mockPP),
		mockPP.EXPECT().IncIndent().Return(innerMockPP),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "Policies:"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "IPv4 provider:    %s", "cloudflare.trace"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "IPv4 domains:     %s", "test4.org, *.test4.org"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "IPv6 provider:    %s", "cloudflare.trace"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "IPv6 domains:     %s", "test6.org, *.test6.org"),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "Scheduling:"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Timezone:         %s", Some("UTC (UTC+00 now)", "Local (UTC+00 now)")),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Update frequency: %v", cron.MustNew("@every 5m")),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Update on start?  %t", true),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Delete on stop?   %t", false),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Cache expiration: %v", time.Hour*6),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "New DNS records:"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "TTL:              %s", "1 (automatic)"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Proxied:          %s", "(none)"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Non-proxied:      %s", "(none)"),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "Timeouts:"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "IP detection:     %v", time.Second*5),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Record updating:  %v", time.Second*30),
	)

	c := config.Default()

	c.Domains[ipnet.IP4] = []api.Domain{api.FQDN("test4.org"), api.Wildcard("test4.org")}
	c.Domains[ipnet.IP6] = []api.Domain{api.FQDN("test6.org"), api.Wildcard("test6.org")}

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
		mockPP.EXPECT().Infof(pp.EmojiConfig, "Policies:"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "IPv4 provider:    %s", "none"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "IPv6 provider:    %s", "none"),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "Scheduling:"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Timezone:         %s", Some("UTC (UTC+00 now)", "Local (UTC+00 now)")),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Update frequency: %v", cron.Schedule(nil)),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Update on start?  %t", false),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Delete on stop?   %t", false),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Cache expiration: %v", time.Duration(0)),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "New DNS records:"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "TTL:              %s", "0"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Proxied:          %s", "(none)"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Non-proxied:      %s", "(none)"),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "Timeouts:"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "IP detection:     %v", time.Duration(0)),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Record updating:  %v", time.Duration(0)),
	)
	var cfg config.Config
	cfg.Print(mockPP)
}

//nolint:paralleltest // changing the environment variable TZ
func TestPrintMonitors(t *testing.T) {
	mockCtrl := gomock.NewController(t)

	store(t, "TZ", "UTC")

	c := config.Default()

	mockPP := mocks.NewMockPP(mockCtrl)
	innerMockPP := mocks.NewMockPP(mockCtrl)
	gomock.InOrder(
		mockPP.EXPECT().IsEnabledFor(pp.Info).Return(true),
		mockPP.EXPECT().Infof(pp.EmojiEnvVars, "Current settings:"),
		mockPP.EXPECT().IncIndent().Return(mockPP),
		mockPP.EXPECT().IncIndent().Return(innerMockPP),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "Policies:"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "IPv4 provider:    %s", "cloudflare.trace"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "IPv4 domains:     %s", "(none)"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "IPv6 provider:    %s", "cloudflare.trace"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "IPv6 domains:     %s", "(none)"),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "Scheduling:"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Timezone:         %s", Some("UTC (UTC+00 now)", "Local (UTC+00 now)")),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Update frequency: %v", cron.MustNew("@every 5m")),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Update on start?  %t", true),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Delete on stop?   %t", false),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Cache expiration: %v", time.Hour*6),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "New DNS records:"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "TTL:              %s", "1 (automatic)"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Proxied:          %s", "(none)"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Non-proxied:      %s", "(none)"),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "Timeouts:"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "IP detection:     %v", time.Second*5),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Record updating:  %v", time.Second*30),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "Monitors:"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "%-17s (URL redacted)", "Healthchecks.io:"),
	)

	m, ok := monitor.NewHealthChecks(mockPP, "http://user:pass@host/path")
	require.True(t, ok)

	c.Monitors = []monitor.Monitor{m}
	c.Print(mockPP)
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

//nolint:paralleltest // environment variables are global
func TestReadEnvWithOnlyToken(t *testing.T) {
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
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Use default %s=%v", "UPDATE_CRON", cron.Schedule(nil)),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Use default %s=%t", "UPDATE_ON_START", false),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Use default %s=%t", "DELETE_ON_STOP", false),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Use default %s=%v", "CACHE_EXPIRATION", time.Duration(0)),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Use default %s=%d", "TTL", 0),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Use default %s=%t", "PROXIED", false),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Use default %s=%v", "DETECTION_TIMEOUT", time.Duration(0)),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Use default %s=%v", "UPDATE_TIMEOUT", time.Duration(0)),
	)
	ok := cfg.ReadEnv(mockPP)
	require.True(t, ok)
}

//nolint:paralleltest // environment variables are global
func TestReadEnvEmpty(t *testing.T) {
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
	ok := cfg.ReadEnv(mockPP)
	require.False(t, ok)
}

//nolint:funlen,maintidx
func TestNormalize(t *testing.T) {
	t.Parallel()

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
					m.EXPECT().Errorf(pp.EmojiUserError, "No domains were specified"),
				)
			},
		},
		"empty": {
			input: &config.Config{ //nolint:exhaustruct
				Domains: map[ipnet.Type][]api.Domain{
					ipnet.IP4: {},
					ipnet.IP6: {},
				},
				ProxiedByDomain: map[api.Domain]bool{},
			},
			ok: false,
			expected: &config.Config{ //nolint:exhaustruct
				Domains: map[ipnet.Type][]api.Domain{
					ipnet.IP4: {},
					ipnet.IP6: {},
				},
				ProxiedByDomain: map[api.Domain]bool{},
			},
			prepareMockPP: func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().IsEnabledFor(pp.Info).Return(true),
					m.EXPECT().Infof(pp.EmojiEnvVars, "Checking settings . . ."),
					m.EXPECT().IncIndent().Return(m),
					m.EXPECT().Errorf(pp.EmojiUserError, "No domains were specified"),
				)
			},
		},
		"empty-ip6": {
			input: &config.Config{ //nolint:exhaustruct
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP4: provider.NewCloudflareTrace(),
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				Domains: map[ipnet.Type][]api.Domain{
					ipnet.IP4: {api.FQDN("a.b.c")},
					ipnet.IP6: {},
				},
				ProxiedByDomain: map[api.Domain]bool{},
			},
			ok: true,
			expected: &config.Config{ //nolint:exhaustruct
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP4: provider.NewCloudflareTrace(),
					ipnet.IP6: nil,
				},
				Domains: map[ipnet.Type][]api.Domain{
					ipnet.IP4: {api.FQDN("a.b.c")},
					ipnet.IP6: {},
				},
				ProxiedByDomain: map[api.Domain]bool{
					api.FQDN("a.b.c"): false,
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
					ipnet.IP4: nil,
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				Domains: map[ipnet.Type][]api.Domain{
					ipnet.IP4: {api.FQDN("a.b.c")},
					ipnet.IP6: {},
				},
				ProxiedByDomain: map[api.Domain]bool{},
			},
			ok: false,
			expected: &config.Config{ //nolint:exhaustruct
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP4: nil,
					ipnet.IP6: nil,
				},
				Domains: map[ipnet.Type][]api.Domain{
					ipnet.IP4: {api.FQDN("a.b.c")},
					ipnet.IP6: {},
				},
				ProxiedByDomain: map[api.Domain]bool{},
			},
			prepareMockPP: func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().IsEnabledFor(pp.Info).Return(true),
					m.EXPECT().Infof(pp.EmojiEnvVars, "Checking settings . . ."),
					m.EXPECT().IncIndent().Return(m),
					m.EXPECT().Warningf(pp.EmojiUserWarning,
						"IP%d_PROVIDER was changed to %q because no domains were set for %s",
						6, "none", "IPv6"),
					m.EXPECT().Errorf(pp.EmojiUserError, "Both IPv4 and IPv6 are disabled"),
				)
			},
		},
		"ignored-ip4-domains": {
			input: &config.Config{ //nolint:exhaustruct
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP4: nil,
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				Domains: map[ipnet.Type][]api.Domain{
					ipnet.IP4: {api.FQDN("a.b.c"), api.FQDN("d.e.f")},
					ipnet.IP6: {api.FQDN("a.b.c"), api.FQDN("g.h.i")},
				},
				ProxiedByDomain: map[api.Domain]bool{},
			},
			ok: true,
			expected: &config.Config{ //nolint:exhaustruct
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP4: nil,
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				Domains: map[ipnet.Type][]api.Domain{
					ipnet.IP4: {api.FQDN("a.b.c"), api.FQDN("d.e.f")},
					ipnet.IP6: {api.FQDN("a.b.c"), api.FQDN("g.h.i")},
				},
				ProxiedByDomain: map[api.Domain]bool{
					api.FQDN("a.b.c"): false,
					api.FQDN("g.h.i"): false,
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
		"ignored-proxied": {
			input: &config.Config{ //nolint:exhaustruct
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP4: nil,
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				Domains: map[ipnet.Type][]api.Domain{
					ipnet.IP4: nil,
					ipnet.IP6: {api.FQDN("a.b.c")},
				},
				ProxiedByDomain: map[api.Domain]bool{
					api.FQDN("a.b.c"): true,
					api.FQDN("d.e.f"): true,
					api.FQDN("g.h.i"): false,
				},
			},
			ok: true,
			expected: &config.Config{ //nolint:exhaustruct
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP4: nil,
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				Domains: map[ipnet.Type][]api.Domain{
					ipnet.IP4: nil,
					ipnet.IP6: {api.FQDN("a.b.c")},
				},
				ProxiedByDomain: map[api.Domain]bool{
					api.FQDN("a.b.c"): true,
				},
			},
			prepareMockPP: func(m *mocks.MockPP) {
				call0 := m.EXPECT().IsEnabledFor(pp.Info).Return(true)
				call1 := m.EXPECT().Infof(pp.EmojiEnvVars, "Checking settings . . .").After(call0)
				call2 := m.EXPECT().IncIndent().Return(m).After(call1)
				m.EXPECT().Warningf(pp.EmojiUserWarning,
					"Domain %q was listed in %s, but it is ignored because it is not managed by the updater",
					"d.e.f", "PROXIED_DOMAINS").After(call2)
				m.EXPECT().Warningf(pp.EmojiUserWarning,
					"Domain %q was listed in %s, but it is ignored because it is not managed by the updater",
					"g.h.i", "NON_PROXIED_DOMAINS").After(call2)
			},
		},
		"good": {
			input: &config.Config{ //nolint:exhaustruct
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP4: nil,
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				Domains: map[ipnet.Type][]api.Domain{
					ipnet.IP4: nil,
					ipnet.IP6: {api.FQDN("a.b.c")},
				},
				ProxiedByDomain: map[api.Domain]bool{
					api.FQDN("a.b.c"): true,
				},
			},
			ok: true,
			expected: &config.Config{ //nolint:exhaustruct
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP4: nil,
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				Domains: map[ipnet.Type][]api.Domain{
					ipnet.IP4: nil,
					ipnet.IP6: {api.FQDN("a.b.c")},
				},
				ProxiedByDomain: map[api.Domain]bool{
					api.FQDN("a.b.c"): true,
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
			ok := cfg.NormalizeDomains(mockPP)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.expected, cfg)
		})
	}
}
