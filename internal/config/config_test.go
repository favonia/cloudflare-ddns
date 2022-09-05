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
	"github.com/favonia/cloudflare-ddns/internal/domain"
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
func TestReadAuth(t *testing.T) {
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
func TestReadAuthWithFile(t *testing.T) {
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

//nolint:paralleltest // environment vars are global
func TestReadDomainMap(t *testing.T) {
	for name, tc := range map[string]struct {
		domains       string
		ip4Domains    string
		ip6Domains    string
		expected      map[ipnet.Type][]domain.Domain
		ok            bool
		prepareMockPP func(*mocks.MockPP)
	}{
		"full": {
			"  a1, a2", "b1,  b2,b2", "c1,c2",
			map[ipnet.Type][]domain.Domain{
				ipnet.IP4: {domain.FQDN("a1"), domain.FQDN("a2"), domain.FQDN("b1"), domain.FQDN("b2")},
				ipnet.IP6: {domain.FQDN("a1"), domain.FQDN("a2"), domain.FQDN("c1"), domain.FQDN("c2")},
			},
			true,
			nil,
		},
		"duplicate": {
			"  a1, a1", "a1,  a1,a1", "*.a1,a1,*.a1,*.a1",
			map[ipnet.Type][]domain.Domain{
				ipnet.IP4: {domain.FQDN("a1")},
				ipnet.IP6: {domain.FQDN("a1"), domain.Wildcard("a1")},
			},
			true,
			nil,
		},
		"empty": {
			" ", "   ", "",
			map[ipnet.Type][]domain.Domain{
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

			var field map[ipnet.Type][]domain.Domain
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

//nolint:funlen
func TestParseTTL(t *testing.T) {
	t.Parallel()

	domain, _ := domain.New("example.io")
	for name, tc := range map[string]struct {
		val           string
		ttl           api.TTL
		ok            bool
		prepareMockPP func(*mocks.MockPP)
	}{
		"empty": {
			"", 0, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiUserError, "TTL of %s (%q) is not a number: %v", domain.Describe(), "", gomock.Any())
			},
		},
		"0": {
			"0   ", 0, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiUserError, "TTL of %s (%d) should be 1 (auto) or between 30 and 86400", domain.Describe(), 0) //nolint:lll
			},
		},
		"-1": {
			"   -1", 0, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiUserError, "TTL of %s (%d) should be 1 (auto) or between 30 and 86400", domain.Describe(), -1) //nolint:lll
			},
		},
		"1": {"   1   ", 1, true, nil},
		"20": {
			"   20   ", 0, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiUserError, "TTL of %s (%d) should be 1 (auto) or between 30 and 86400", domain.Describe(), 20) //nolint:lll
			},
		},
		"9999999": {
			"   9999999   ", 0, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiUserError, "TTL of %s (%d) should be 1 (auto) or between 30 and 86400", domain.Describe(), 9999999) //nolint:lll
			},
		},
		"words": {
			"   word   ", 0, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiUserError, "TTL of %s (%q) is not a number: %v", domain.Describe(), "word", gomock.Any())
			},
		},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}
			ttl, ok := config.ParseTTL(mockPP, domain, tc.val)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.ttl, ttl)
		})
	}
}

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
		mockPP.EXPECT().Infof(pp.EmojiConfig, "Policies:"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "IPv4 provider:             %s", "cloudflare.trace"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "IPv4 domains:              %s", "(none)"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "IPv6 provider:             %s", "cloudflare.trace"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "IPv6 domains:              %s", "(none)"),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "Scheduling:"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Timezone:                  %s", Some("UTC (UTC+00 now)", "Local (UTC+00 now)")), //nolint:lll
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Update frequency:          %v", cron.MustNew("@every 5m")),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Update on start?           %t", true),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Delete on stop?            %t", false),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Cache expiration:          %v", time.Hour*6),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "New DNS records:"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Proxied domains:           %s", "(none)"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Non-proxied domains:       %s", "(none)"),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "Timeouts:"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "IP detection:              %v", time.Second*5),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Record updating:           %v", time.Second*30),
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
		mockPP.EXPECT().Infof(pp.EmojiConfig, "Policies:"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "IPv4 provider:             %s", "cloudflare.trace"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "IPv4 domains:              %s", "test4.org, *.test4.org"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "IPv6 provider:             %s", "cloudflare.trace"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "IPv6 domains:              %s", "test6.org, *.test6.org"),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "Scheduling:"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Timezone:                  %s", Some("UTC (UTC+00 now)", "Local (UTC+00 now)")), //nolint:lll
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Update frequency:          %v", cron.MustNew("@every 5m")),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Update on start?           %t", true),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Delete on stop?            %t", false),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Cache expiration:          %v", time.Hour*6),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "New DNS records:"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "%-26s %s", "Domains with TTL=1 (auto):", "a, c"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "%-26s %s", "Domains with TTL=30000:", "b, d"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Proxied domains:           %s", "a, b"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Non-proxied domains:       %s", "c, d"),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "Timeouts:"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "IP detection:              %v", time.Second*5),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Record updating:           %v", time.Second*30),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "Monitors:"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "%-26s (URL redacted)", "Healthchecks.io:"),
	)

	c := config.Default()

	c.Domains[ipnet.IP4] = []domain.Domain{domain.FQDN("test4.org"), domain.Wildcard("test4.org")}
	c.Domains[ipnet.IP6] = []domain.Domain{domain.FQDN("test6.org"), domain.Wildcard("test6.org")}

	c.TTL = map[domain.Domain]api.TTL{}
	c.TTL[domain.FQDN("a")] = 1
	c.TTL[domain.FQDN("b")] = 30000
	c.TTL[domain.FQDN("c")] = 1
	c.TTL[domain.FQDN("d")] = 30000

	c.Proxied = map[domain.Domain]bool{}
	c.Proxied[domain.FQDN("a")] = true
	c.Proxied[domain.FQDN("b")] = true
	c.Proxied[domain.FQDN("c")] = false
	c.Proxied[domain.FQDN("d")] = false

	m, ok := monitor.NewHealthChecks(mockPP, "http://user:pass@host/path")
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
		mockPP.EXPECT().Infof(pp.EmojiConfig, "Policies:"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "IPv4 provider:             %s", "none"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "IPv6 provider:             %s", "none"),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "Scheduling:"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Timezone:                  %s", Some("UTC (UTC+00 now)", "Local (UTC+00 now)")), //nolint:lll
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Update frequency:          %v", cron.Schedule(nil)),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Update on start?           %t", false),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Delete on stop?            %t", false),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Cache expiration:          %v", time.Duration(0)),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "New DNS records:"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Proxied domains:           %s", "(none)"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Non-proxied domains:       %s", "(none)"),
		mockPP.EXPECT().Infof(pp.EmojiConfig, "Timeouts:"),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "IP detection:              %v", time.Duration(0)),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Record updating:           %v", time.Duration(0)),
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
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Use default %s=%s", "TTL", ""),
		innerMockPP.EXPECT().Infof(pp.EmojiBullet, "Use default %s=%s", "PROXIED", ""),
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

//nolint:funlen
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
				Domains: map[ipnet.Type][]domain.Domain{
					ipnet.IP4: {domain.FQDN("a.b.c")},
					ipnet.IP6: {},
				},
				TTLTemplate:     "1",
				ProxiedTemplate: "false",
			},
			ok: true,
			expected: &config.Config{ //nolint:exhaustruct
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP4: provider.NewCloudflareTrace(),
				},
				Domains: map[ipnet.Type][]domain.Domain{
					ipnet.IP4: {domain.FQDN("a.b.c")},
					ipnet.IP6: {},
				},
				TTLTemplate: "1",
				TTL: map[domain.Domain]api.TTL{
					domain.FQDN("a.b.c"): 1,
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
					ipnet.IP6: provider.NewCloudflareTrace(),
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
					m.EXPECT().Errorf(pp.EmojiUserError, "Both IPv4 and IPv6 are disabled"),
				)
			},
		},
		"ignored-ip4-domains": {
			input: &config.Config{ //nolint:exhaustruct
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				Domains: map[ipnet.Type][]domain.Domain{
					ipnet.IP4: {domain.FQDN("a.b.c"), domain.FQDN("d.e.f")},
					ipnet.IP6: {domain.FQDN("a.b.c"), domain.FQDN("g.h.i")},
				},
				TTLTemplate:     "1",
				ProxiedTemplate: "false",
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
				TTLTemplate: "1",
				TTL: map[domain.Domain]api.TTL{
					domain.FQDN("a.b.c"): 1,
					domain.FQDN("g.h.i"): 1,
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
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				Domains: map[ipnet.Type][]domain.Domain{
					ipnet.IP6: {domain.FQDN("a.b.c"), domain.FQDN("a.bb.c"), domain.FQDN("a.d.e.f")},
				},
				TTLTemplate:     `{{if suffix "b.c"}} 60 {{else if domain "d.e.f" "a.bb.c" }} 90 {{else}} 120 {{end}}`,
				ProxiedTemplate: ` {{not (domain "a.bb.c")}} `,
			},
			ok: true,
			expected: &config.Config{ //nolint:exhaustruct
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				Domains: map[ipnet.Type][]domain.Domain{
					ipnet.IP6: {domain.FQDN("a.b.c"), domain.FQDN("a.bb.c"), domain.FQDN("a.d.e.f")},
				},
				TTLTemplate: `{{if suffix "b.c"}} 60 {{else if domain "d.e.f" "a.bb.c" }} 90 {{else}} 120 {{end}}`,
				TTL: map[domain.Domain]api.TTL{
					domain.FQDN("a.b.c"):   60,
					domain.FQDN("a.bb.c"):  90,
					domain.FQDN("a.d.e.f"): 120,
				},
				ProxiedTemplate: ` {{not (domain "a.bb.c")}} `,
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
		"template/invalid/ttl": {
			input: &config.Config{ //nolint:exhaustruct
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				Domains: map[ipnet.Type][]domain.Domain{
					ipnet.IP6: {domain.FQDN("a.b.c"), domain.FQDN("a.bb.c"), domain.FQDN("a.d.e.f")},
				},
				TTLTemplate:     `{{if}}`,
				ProxiedTemplate: ` {{not (domain "a.b.c")}} `,
			},
			ok:       false,
			expected: nil,
			prepareMockPP: func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().IsEnabledFor(pp.Info).Return(true),
					m.EXPECT().Infof(pp.EmojiEnvVars, "Checking settings . . ."),
					m.EXPECT().IncIndent().Return(m),
					m.EXPECT().Errorf(pp.EmojiUserError, "%q is not a valid template: %v", "{{if}}", gomock.Any()),
				)
			},
		},
		"template/error/ttl": {
			input: &config.Config{ //nolint:exhaustruct
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				Domains: map[ipnet.Type][]domain.Domain{
					ipnet.IP6: {domain.FQDN("a.b.c")},
				},
				TTLTemplate:     `not a number`,
				ProxiedTemplate: `{{not (domain "a.b.c")}}`,
			},
			ok:       false,
			expected: nil,
			prepareMockPP: func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().IsEnabledFor(pp.Info).Return(true),
					m.EXPECT().Infof(pp.EmojiEnvVars, "Checking settings . . ."),
					m.EXPECT().IncIndent().Return(m),
					m.EXPECT().Errorf(pp.EmojiUserError, "TTL of %s (%q) is not a number: %v", "a.b.c", "not a number", gomock.Any()),
				)
			},
		},
		"template/error/ttl/out-of-range": {
			input: &config.Config{ //nolint:exhaustruct
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				Domains: map[ipnet.Type][]domain.Domain{
					ipnet.IP6: {domain.FQDN("a.b.c")},
				},
				TTLTemplate:     `{{if (domain "a.b.c")}} 2 {{end}}`,
				ProxiedTemplate: `{{not (domain "a.b.c")}}`,
			},
			ok:       false,
			expected: nil,
			prepareMockPP: func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().IsEnabledFor(pp.Info).Return(true),
					m.EXPECT().Infof(pp.EmojiEnvVars, "Checking settings . . ."),
					m.EXPECT().IncIndent().Return(m),
					m.EXPECT().Errorf(pp.EmojiUserError, "TTL of %s (%d) should be 1 (auto) or between 30 and 86400", "a.b.c", 2),
				)
			},
		},
		"template/error/proxied": {
			input: &config.Config{ //nolint:exhaustruct
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				Domains: map[ipnet.Type][]domain.Domain{
					ipnet.IP6: {domain.FQDN("a.b.c")},
				},
				TTLTemplate:     `1`,
				ProxiedTemplate: `{{12345}}`,
			},
			ok:       false,
			expected: nil,
			prepareMockPP: func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().IsEnabledFor(pp.Info).Return(true),
					m.EXPECT().Infof(pp.EmojiEnvVars, "Checking settings . . ."),
					m.EXPECT().IncIndent().Return(m),
					m.EXPECT().Errorf(pp.EmojiUserError, "Proxy setting of %s (%q) is not a boolean value: %v", "a.b.c", "12345", gomock.Any()), //nolint:lll
				)
			},
		},
		"template/error/proxied/ill-formed": {
			input: &config.Config{ //nolint:exhaustruct
				Provider: map[ipnet.Type]provider.Provider{
					ipnet.IP6: provider.NewCloudflareTrace(),
				},
				Domains: map[ipnet.Type][]domain.Domain{
					ipnet.IP6: {domain.FQDN("a.b.c")},
				},
				TTLTemplate:     `1`,
				ProxiedTemplate: `{{domain 12345}}`,
			},
			ok:       false,
			expected: nil,
			prepareMockPP: func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().IsEnabledFor(pp.Info).Return(true),
					m.EXPECT().Infof(pp.EmojiEnvVars, "Checking settings . . ."),
					m.EXPECT().IncIndent().Return(m),
					m.EXPECT().Errorf(pp.EmojiUserError, "Could not execute the template %q: %v", "{{domain 12345}}", gomock.Any()), //nolint:lll
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
			if tc.ok {
				require.Equal(t, tc.expected, cfg)
			} else {
				require.Equal(t, tc.input, cfg)
			}
		})
	}
}
