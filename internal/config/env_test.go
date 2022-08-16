package config_test

import (
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/config"
	"github.com/favonia/cloudflare-ddns/internal/cron"
	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/monitor"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider"
)

const keyPrefix = "TEST-11D39F6A9A97AFAFD87CCEB-"

func set(t *testing.T, key string, set bool, val string) {
	t.Helper()

	if set {
		t.Setenv(key, val)
	} else {
		t.Setenv(key, "")
		os.Unsetenv(key)
	}
}

func store(t *testing.T, key string, val string) { t.Helper(); set(t, key, true, val) }
func unset(t *testing.T, keys ...string) {
	t.Helper()
	for _, k := range keys {
		set(t, k, false, "")
	}
}

//nolint:paralleltest // environment vars are global
func TestGetenv(t *testing.T) {
	key := keyPrefix + "VAR"
	for name, tc := range map[string]struct {
		set      bool
		val      string
		expected string
	}{
		"nil":    {false, "", ""},
		"empty":  {true, "", ""},
		"simple": {true, "VAL", "VAL"},
		"space1": {true, "    VAL     ", "VAL"},
		"space2": {true, "     VAL    VAL2 ", "VAL    VAL2"},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			set(t, key, tc.set, tc.val)
			require.Equal(t, tc.expected, config.Getenv(key))
		})
	}
}

//nolint:paralleltest // environment vars are global
func TestReadQuiet(t *testing.T) {
	key := keyPrefix + "QUIET"
	for name, tc := range map[string]struct {
		set           bool
		val           string
		ok            bool
		prepareMockPP func(*mocks.MockPP)
	}{
		"nil":   {false, "", true, nil},
		"empty": {true, " ", true, nil},
		"true": {
			true, " true", true,
			func(m *mocks.MockPP) {
				m.EXPECT().SetLevel(pp.Notice)
			},
		},
		"false": {
			true, "    false ", true,
			func(m *mocks.MockPP) {
				m.EXPECT().SetLevel(pp.Info)
			},
		},
		"illform": {
			true, "weird", false,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiUserError, "Failed to parse %q: %v", "weird", gomock.Any())
			},
		},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)

			set(t, key, tc.set, tc.val)

			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}

			var wrappedPP pp.PP = mockPP

			ok := config.ReadQuiet(key, &wrappedPP)
			require.Equal(t, tc.ok, ok)
		})
	}
}

//nolint:funlen,paralleltest // environment vars are global
func TestReadBool(t *testing.T) {
	key := keyPrefix + "BOOL"
	for name, tc := range map[string]struct {
		set           bool
		val           string
		oldField      bool
		newField      bool
		ok            bool
		prepareMockPP func(*mocks.MockPP)
	}{
		"nil1": {
			false, "", true, true, true,
			func(m *mocks.MockPP) {
				m.EXPECT().Infof(pp.EmojiBullet, "Use default %s=%t", key, true)
			},
		},
		"nil2": {
			false, "", false, false, true,
			func(m *mocks.MockPP) {
				m.EXPECT().Infof(pp.EmojiBullet, "Use default %s=%t", key, false)
			},
		},
		"empty1": {
			true, " ", true, true, true,
			func(m *mocks.MockPP) {
				m.EXPECT().Infof(pp.EmojiBullet, "Use default %s=%t", key, true)
			},
		},
		"empty2": {
			true, " \t ", false, false, true,
			func(m *mocks.MockPP) {
				m.EXPECT().Infof(pp.EmojiBullet, "Use default %s=%t", key, false)
			},
		},
		"true1":  {true, "true ", true, true, true, nil},
		"true2":  {true, " \t true", false, true, true, nil},
		"false1": {true, "false ", true, false, true, nil},
		"false2": {true, " false", false, false, true, nil},
		"illform1": {
			true, "weird\t  ", false, false, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiUserError, "Failed to parse %q: %v", "weird", gomock.Any())
			},
		},
		"illform2": {
			true, " weird", true, true, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiUserError, "Failed to parse %q: %v", "weird", gomock.Any())
			},
		},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)

			set(t, key, tc.set, tc.val)

			field := tc.oldField
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}
			ok := config.ReadBool(mockPP, key, &field)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.newField, field)
		})
	}
}

//nolint:paralleltest // environment vars are global
func TestReadNonnegInt(t *testing.T) {
	key := keyPrefix + "INT"
	for name, tc := range map[string]struct {
		set           bool
		val           string
		oldField      int
		newField      int
		ok            bool
		prepareMockPP func(*mocks.MockPP)
	}{
		"nil": {
			false, "", 100, 100, true,
			func(m *mocks.MockPP) {
				m.EXPECT().Infof(pp.EmojiBullet, "Use default %s=%d", key, 100)
			},
		},
		"empty": {
			true, "", 100, 100, true,
			func(m *mocks.MockPP) {
				m.EXPECT().Infof(pp.EmojiBullet, "Use default %s=%d", key, 100)
			},
		},
		"zero": {true, "0   ", 100, 0, true, nil},
		"-1": {
			true, "   -1", 100, 100, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiUserError, "Failed to parse %q: %d is negative", "-1", gomock.Any())
			},
		},
		"1": {true, "   1   ", 100, 1, true, nil},
		"1.0": {
			true, "   1.0   ", 100, 100, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiUserError, "Failed to parse %q: %v", "1.0", gomock.Any())
			},
		},
		"words": {
			true, "   word   ", 100, 100, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiUserError, "Failed to parse %q: %v", "word", gomock.Any())
			},
		},
		"9999999": {true, "   9999999   ", 100, 9999999, true, nil},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)

			set(t, key, tc.set, tc.val)

			field := tc.oldField
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}
			ok := config.ReadNonnegInt(mockPP, key, &field)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.newField, field)
		})
	}
}

//nolint:paralleltest // environment vars are global
func TestReadDomains(t *testing.T) {
	key := keyPrefix + "DOMAINS"
	type ds = []domain.Domain
	type f = domain.FQDN
	type w = domain.Wildcard
	for name, tc := range map[string]struct {
		set           bool
		val           string
		oldField      ds
		newField      ds
		ok            bool
		prepareMockPP func(*mocks.MockPP)
	}{
		"nil":       {false, "", ds{f("test.org")}, ds{}, true, nil},
		"empty":     {true, "", ds{f("test.org")}, ds{}, true, nil},
		"star":      {true, "*", ds{}, ds{w("")}, true, nil},
		"wildcard1": {true, "*.a", ds{}, ds{w("a")}, true, nil},
		"wildcard2": {true, "*.a.b", ds{}, ds{w("a.b")}, true, nil},
		"test1":     {true, "書.org ,  Bücher.org  ", ds{f("random.org")}, ds{f("xn--rov.org"), f("xn--bcher-kva.org")}, true, nil},                      //nolint:lll
		"test2":     {true, "  \txn--rov.org    ,   xn--Bcher-kva.org  ", ds{f("random.org")}, ds{f("xn--rov.org"), f("xn--bcher-kva.org")}, true, nil}, //nolint:lll
		"illformed1": {
			true, "xn--:D.org",
			ds{f("random.org")},
			ds{f("xn--:d.org")},
			true,
			func(m *mocks.MockPP) {
				m.EXPECT().Warningf(pp.EmojiUserError, "Domain %q was added but it is ill-formed: %v", "xn--:d.org", gomock.Any()) //nolint:lll
			},
		},
		"illformed2": {
			true, "*.xn--:D.org",
			ds{f("random.org")},
			ds{w("xn--:d.org")},
			true,
			func(m *mocks.MockPP) {
				m.EXPECT().Warningf(pp.EmojiUserError, "Domain %q was added but it is ill-formed: %v", "*.xn--:d.org", gomock.Any()) //nolint:lll
			},
		},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)

			set(t, key, tc.set, tc.val)

			field := tc.oldField
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}
			ok := config.ReadDomains(mockPP, key, &field)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.newField, field)
		})
	}
}

//nolint:paralleltest,funlen // paralleltest should not be used because environment vars are global
func TestReadProvider(t *testing.T) {
	key := keyPrefix + "PROVIDER"
	keyDeprecated := keyPrefix + "DEPRECATED"

	var (
		none            provider.Provider
		cloudflareDOH   = provider.NewCloudflareDOH()
		cloudflareTrace = provider.NewCloudflareTrace()
		local           = provider.NewLocal()
		ipify           = provider.NewIpify()
	)

	for name, tc := range map[string]struct {
		set           bool
		val           string
		setDeprecated bool
		valDeprecated string
		oldField      provider.Provider
		newField      provider.Provider
		ok            bool
		prepareMockPP func(*mocks.MockPP)
	}{
		"nil": {
			false, "", false, "", none, none, true,
			func(m *mocks.MockPP) {
				m.EXPECT().Infof(pp.EmojiBullet, "Use default %s=%s", key, "none")
			},
		},
		"deprecated/empty": {
			false, "", true, "", local, local, true,
			func(m *mocks.MockPP) {
				m.EXPECT().Infof(pp.EmojiBullet, "Use default %s=%s", key, "local")
			},
		},
		"deprecated/cloudflare": {
			false, "", true, "    cloudflare\t   ", none, cloudflareTrace, true,
			func(m *mocks.MockPP) {
				m.EXPECT().Warningf(
					pp.EmojiUserWarning,
					`Parameter %s and provider "cloudflare" were deprecated; use %s=cloudflare.doh or %s=cloudflare.trace`, //nolint:lll
					keyDeprecated, key, key,
				)
			},
		},
		"deprecated/cloudflare.trace": {
			false, "", true, " cloudflare.trace", none, cloudflareTrace, true,
			func(m *mocks.MockPP) {
				m.EXPECT().Warningf(
					pp.EmojiUserWarning,
					`Parameter %s was deprecated; use %s=%s`,
					keyDeprecated,
					key,
					"cloudflare.trace",
				)
			},
		},
		"deprecated/cloudflare.doh": {
			false, "", true, "    \tcloudflare.doh   ", none, cloudflareDOH, true,
			func(m *mocks.MockPP) {
				m.EXPECT().Warningf(
					pp.EmojiUserWarning,
					`Parameter %s was deprecated; use %s=%s`,
					keyDeprecated,
					key,
					"cloudflare.doh",
				)
			},
		},
		"deprecated/unmanaged": {
			false, "", true, "   unmanaged   ", cloudflareTrace, none, true,
			func(m *mocks.MockPP) {
				m.EXPECT().Warningf(
					pp.EmojiUserWarning,
					`Parameter %s was deprecated; use %s=none`,
					keyDeprecated,
					key,
				)
			},
		},
		"deprecated/local": {
			false, "", true, "   local   ", cloudflareTrace, local, true,
			func(m *mocks.MockPP) {
				m.EXPECT().Warningf(
					pp.EmojiUserWarning,
					`Parameter %s was deprecated; use %s=%s`,
					keyDeprecated,
					key,
					"local",
				)
			},
		},
		"deprecated/ipify": {
			false, "", true, "     ipify  ", cloudflareTrace, ipify, true,
			func(m *mocks.MockPP) {
				m.EXPECT().Warningf(
					pp.EmojiUserWarning,
					`Parameter %s was deprecated; use %s=%s`,
					keyDeprecated,
					key,
					"ipify",
				)
			},
		},
		"deprecated/others": {
			false, "", true, "   something-else ", ipify, ipify, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiUserError, "Failed to parse %q: not a valid provider", "something-else")
			},
		},
		"conflicts": {
			true, "cloudflare.doh", true, "cloudflare.doh", ipify, ipify, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(
					pp.EmojiUserError,
					`Cannot have both %s and %s set`,
					key, keyDeprecated,
				)
			},
		},
		"empty": {
			false, "", false, "", local, local, true,
			func(m *mocks.MockPP) {
				m.EXPECT().Infof(pp.EmojiBullet, "Use default %s=%s", key, "local")
			},
		},
		"cloudflare": {
			true, "    cloudflare\t   ", false, "", none, none, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(
					pp.EmojiUserError,
					`Parameter %s does not accept "cloudflare"; use "cloudflare.doh" or "cloudflare.trace"`,
					key, key,
				)
			},
		},
		"cloudflare.trace": {true, " cloudflare.trace", false, "", none, cloudflareTrace, true, nil},
		"cloudflare.doh":   {true, "    \tcloudflare.doh   ", false, "", none, cloudflareDOH, true, nil},
		"none":             {true, "   none   ", false, "", cloudflareTrace, none, true, nil},
		"local":            {true, "   local   ", false, "", cloudflareTrace, local, true, nil},
		"ipify":            {true, "     ipify  ", false, "", cloudflareTrace, ipify, true, nil},
		"others": {
			true, "   something-else ", false, "", ipify, ipify, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiUserError, "Failed to parse %q: not a valid provider", "something-else")
			},
		},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)

			set(t, key, tc.set, tc.val)
			set(t, keyDeprecated, tc.setDeprecated, tc.valDeprecated)

			field := tc.oldField
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}
			ok := config.ReadProvider(mockPP, key, keyDeprecated, &field)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.newField, field)
		})
	}
}

//nolint:paralleltest // environment vars are global
func TestReadNonnegDuration(t *testing.T) {
	key := keyPrefix + "DURATION"

	for name, tc := range map[string]struct {
		set           bool
		val           string
		oldField      time.Duration
		newField      time.Duration
		ok            bool
		prepareMockPP func(*mocks.MockPP)
	}{
		"nil": {
			false, "", time.Second, time.Second, true,
			func(m *mocks.MockPP) {
				m.EXPECT().Infof(pp.EmojiBullet, "Use default %s=%v", key, time.Second)
			},
		},
		"empty": {
			true, "", 0, 0, true,
			func(m *mocks.MockPP) {
				m.EXPECT().Infof(pp.EmojiBullet, "Use default %s=%v", key, time.Duration(0))
			},
		},
		"100s": {true, "    100s\t   ", 0, time.Second * 100, true, nil},
		"1": {
			true, "  1  ", 123, 123, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiUserError, "Failed to parse %q: %v", "1", gomock.Any())
			},
		},
		"-1s": {
			true, "  -1s  ", 456, 456, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiUserError, "Failed to parse %q: %v is negative", "-1s", -time.Second)
			},
		},
		"0h": {true, "  0h  ", 123456, 0, true, nil},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)

			set(t, key, tc.set, tc.val)

			field := tc.oldField
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}
			ok := config.ReadNonnegDuration(mockPP, key, &field)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.newField, field)
		})
	}
}

//nolint:paralleltest // environment vars are global
func TestReadCron(t *testing.T) {
	key := keyPrefix + "CRON"

	for name, tc := range map[string]struct {
		set           bool
		val           string
		oldField      cron.Schedule
		newField      cron.Schedule
		ok            bool
		prepareMockPP func(*mocks.MockPP)
	}{
		"nil": {
			false, "", cron.MustNew("* * * * *"), cron.MustNew("* * * * *"), true,
			func(m *mocks.MockPP) {
				m.EXPECT().Infof(
					pp.EmojiBullet,
					"Use default %s=%v",
					key,
					cron.MustNew("* * * * *"),
				)
			},
		},
		"empty": {
			true, "", cron.MustNew("@every 3m"), cron.MustNew("@every 3m"), true,
			func(m *mocks.MockPP) {
				m.EXPECT().Infof(
					pp.EmojiBullet,
					"Use default %s=%v",
					key,
					cron.MustNew("@every 3m"),
				)
			},
		},
		"@": {true, " @daily  ", cron.MustNew("@yearly"), cron.MustNew("@daily"), true, nil},
		"illformed": {
			true, " @ddddd  ", cron.MustNew("*/4 * * * *"), cron.MustNew("*/4 * * * *"), false,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiUserError, "Failed to parse %q: %v", "@ddddd", gomock.Any())
			},
		},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)

			set(t, key, tc.set, tc.val)

			field := tc.oldField
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}
			ok := config.ReadCron(mockPP, key, &field)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.newField, field)
		})
	}
}

func urlMustParse(t *testing.T, u string) *url.URL {
	t.Helper()
	url, err := url.Parse(u)
	require.Nil(t, err)
	return url
}

//nolint:paralleltest,funlen // paralleltest should not be used because environment vars are global
func TestReadHealthChecksURL(t *testing.T) {
	key := keyPrefix + "HEALTHCHECKS"

	type mon = monitor.Monitor

	for name, tc := range map[string]struct {
		set           bool
		val           string
		oldField      []mon
		newField      []mon
		ok            bool
		prepareMockPP func(*mocks.MockPP)
	}{
		"unset": {
			false, "", []mon{}, []mon{}, true, nil,
		},
		"empty": {
			true, "", []mon{}, []mon{}, true, nil,
		},
		"example": {
			true, "https://hi.org/1234",
			[]mon{},
			[]mon{&monitor.HealthChecks{
				BaseURL:    urlMustParse(t, "https://hi.org/1234"),
				Timeout:    monitor.HealthChecksDefaultTimeout,
				MaxRetries: monitor.HealthChecksDefaultMaxRetries,
			}},
			true,
			nil,
		},
		"password": {
			true, "https://me:pass@hi.org/1234",
			[]mon{},
			[]mon{&monitor.HealthChecks{
				BaseURL:    urlMustParse(t, "https://me:pass@hi.org/1234"),
				Timeout:    monitor.HealthChecksDefaultTimeout,
				MaxRetries: monitor.HealthChecksDefaultMaxRetries,
			}},
			true,
			nil,
		},
		"fragment": {
			true, "https://hi.org/1234#fragment",
			[]mon{},
			[]mon{&monitor.HealthChecks{
				BaseURL:    urlMustParse(t, "https://hi.org/1234#fragment"),
				Timeout:    monitor.HealthChecksDefaultTimeout,
				MaxRetries: monitor.HealthChecksDefaultMaxRetries,
			}},
			true,
			nil,
		},
		"query": {
			true, "https://hi.org/1234?hello=123",
			[]mon{},
			[]mon{&monitor.HealthChecks{
				BaseURL:    urlMustParse(t, "https://hi.org/1234?hello=123"),
				Timeout:    monitor.HealthChecksDefaultTimeout,
				MaxRetries: monitor.HealthChecksDefaultMaxRetries,
			}},
			true,
			nil,
		},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)

			set(t, key, tc.set, tc.val)

			field := append([]mon{}, tc.oldField...)
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}
			ok := config.ReadHealthChecksURL(mockPP, key, &field)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.newField, field)
		})
	}
}
