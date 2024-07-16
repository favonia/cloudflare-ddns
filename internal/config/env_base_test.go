package config_test

import (
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/config"
	"github.com/favonia/cloudflare-ddns/internal/cron"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
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
		t.Run(name, func(t *testing.T) {
			set(t, key, tc.set, tc.val)
			require.Equal(t, tc.expected, config.Getenv(key))
		})
	}
}

//nolint:paralleltest // environment vars are global
func TestGetenvs(t *testing.T) {
	key := keyPrefix + "VAR"
	for name, tc := range map[string]struct {
		set      bool
		val      string
		expected []string
	}{
		"nil":         {false, "", []string{}},
		"empty":       {true, "", []string{}},
		"only-spaces": {true, "\n   \n  \n \t", []string{}},
		"simple":      {true, "VAL", []string{"VAL"}},
		"space1":      {true, "    VAL1 \nVAL2    ", []string{"VAL1", "VAL2"}},
		"space2":      {true, "     VAL1 \n   VAL2 ", []string{"VAL1", "VAL2"}},
	} {
		t.Run(name, func(t *testing.T) {
			set(t, key, tc.set, tc.val)
			require.Equal(t, tc.expected, config.Getenvs(key))
		})
	}
}

//nolint:paralleltest // environment vars are global
func TestReadString(t *testing.T) {
	key := keyPrefix + "STRING"
	for name, tc := range map[string]struct {
		set           bool
		val           string
		oldField      string
		newField      string
		ok            bool
		prepareMockPP func(*mocks.MockPP)
	}{
		"unset": {
			false, "", "hi", "hi", true,
			func(m *mocks.MockPP) {
				m.EXPECT().Infof(pp.EmojiBullet, "Use default %s=%s", key, "hi")
			},
		},
		"empty1": {
			true, " ", "hello", "hello", true,
			func(m *mocks.MockPP) {
				m.EXPECT().Infof(pp.EmojiBullet, "Use default %s=%s", key, "hello")
			},
		},
		"empty2": {
			true, " \t ", "aloha", "aloha", true,
			func(m *mocks.MockPP) {
				m.EXPECT().Infof(pp.EmojiBullet, "Use default %s=%s", key, "aloha")
			},
		},
		"string": {true, "string ", "hey", "string", true, nil},
	} {
		t.Run(name, func(t *testing.T) {
			set(t, key, tc.set, tc.val)
			field := tc.oldField
			mockCtrl := gomock.NewController(t)
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}
			ok := config.ReadString(mockPP, key, &field)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.newField, field)
		})
	}
}

//nolint:paralleltest // environment vars are global
func TestReadEmoji(t *testing.T) {
	key := keyPrefix + "EMOJI"
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
				m.EXPECT().SetEmoji(true)
			},
		},
		"false": {
			true, "    false ", true,
			func(m *mocks.MockPP) {
				m.EXPECT().SetEmoji(false)
			},
		},
		"illform": {
			true, "weird", false,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiUserError, "%s (%q) is not a boolean: %v", key, "weird", gomock.Any())
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			set(t, key, tc.set, tc.val)
			mockCtrl := gomock.NewController(t)
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}

			var wrappedPP pp.PP = mockPP

			ok := config.ReadEmoji(key, &wrappedPP)
			require.Equal(t, tc.ok, ok)
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
				m.EXPECT().SetVerbosity(pp.Notice)
			},
		},
		"false": {
			true, "    false ", true,
			func(m *mocks.MockPP) {
				m.EXPECT().SetVerbosity(pp.Info)
			},
		},
		"illform": {
			true, "weird", false,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiUserError, "%s (%q) is not a boolean: %v", key, "weird", gomock.Any())
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			set(t, key, tc.set, tc.val)
			mockCtrl := gomock.NewController(t)
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
				m.EXPECT().Errorf(pp.EmojiUserError, "%s (%q) is not a boolean: %v", key, "weird", gomock.Any())
			},
		},
		"illform2": {
			true, " weird", true, true, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiUserError, "%s (%q) is not a boolean: %v", key, "weird", gomock.Any())
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			set(t, key, tc.set, tc.val)
			field := tc.oldField
			mockCtrl := gomock.NewController(t)
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
				m.EXPECT().Errorf(pp.EmojiUserError, "%s (%d) is negative", key, -1)
			},
		},
		"1": {true, "   1   ", 100, 1, true, nil},
		"1.0": {
			true, "   1.0   ", 100, 100, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiUserError, "%s (%q) is not a number: %v", key, "1.0", gomock.Any())
			},
		},
		"words": {
			true, "   word   ", 100, 100, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiUserError, "%s (%q) is not a number: %v", key, "word", gomock.Any())
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			set(t, key, tc.set, tc.val)
			field := tc.oldField
			mockCtrl := gomock.NewController(t)
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

//nolint:funlen,paralleltest // environment vars are global
func TestReadTTL(t *testing.T) {
	key := keyPrefix + "TTL"
	for name, tc := range map[string]struct {
		set           bool
		val           string
		oldField      api.TTL
		newField      api.TTL
		ok            bool
		prepareMockPP func(*mocks.MockPP)
	}{
		"empty": {
			true, "", api.TTLAuto, api.TTLAuto, true,
			func(m *mocks.MockPP) {
				m.EXPECT().Infof(pp.EmojiBullet, "Use default %s=%d", key, api.TTLAuto)
			},
		},
		"0": {
			true, "0   ", api.TTLAuto, api.TTLAuto, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiUserError, "%s (%d) should be 1 (auto) or between 30 and 86400", key, 0)
			},
		},
		"-1": {
			true, "   -1", api.TTLAuto, api.TTLAuto, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiUserError, "%s (%d) should be 1 (auto) or between 30 and 86400", key, -1)
			},
		},
		"1": {true, "   1   ", api.TTLAuto, api.TTLAuto, true, nil},
		"20": {
			true, "   20   ", api.TTLAuto, api.TTLAuto, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiUserError, "%s (%d) should be 1 (auto) or between 30 and 86400", key, 20)
			},
		},
		"9999999": {
			true, "   9999999   ", api.TTLAuto, api.TTLAuto, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiUserError, "%s (%d) should be 1 (auto) or between 30 and 86400", key, 9999999)
			},
		},
		"words": {
			true, "   word   ", api.TTLAuto, api.TTLAuto, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiUserError, "%s (%q) is not a number: %v", key, "word", gomock.Any())
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			set(t, key, tc.set, tc.val)
			field := tc.oldField
			mockCtrl := gomock.NewController(t)
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}
			ok := config.ReadTTL(mockPP, key, &field)
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
				m.EXPECT().Errorf(pp.EmojiUserError, "%s (%q) is not a time duration: %v", key, "1", gomock.Any())
			},
		},
		"-1s": {
			true, "  -1s  ", 456, 456, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiUserError, "%s (%v) is negative", key, -time.Second)
			},
		},
		"0h": {true, "  0h  ", 123456, 0, true, nil},
	} {
		t.Run(name, func(t *testing.T) {
			set(t, key, tc.set, tc.val)
			field := tc.oldField
			mockCtrl := gomock.NewController(t)
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

//nolint:paralleltest,funlen // environment vars are global
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
					"Use default %s=%s",
					key,
					"* * * * *",
				)
			},
		},
		"empty": {
			true, "", cron.MustNew("@every 3m"), cron.MustNew("@every 3m"), true,
			func(m *mocks.MockPP) {
				m.EXPECT().Infof(
					pp.EmojiBullet,
					"Use default %s=%s",
					key,
					"@every 3m",
				)
			},
		},
		"@daily": {true, " @daily  ", cron.MustNew("@yearly"), cron.MustNew("@daily"), true, nil},
		"@disabled": {
			true, " @disabled  ", cron.MustNew("@yearly"), nil, true,
			func(m *mocks.MockPP) {
				m.EXPECT().Warningf(pp.EmojiUserWarning, "%s=%s is deprecated; use %s=@once", key, "@disabled", gomock.Any())
			},
		},
		"@nevermore": {
			true, " @nevermore\t", cron.MustNew("@yearly"), nil, true,
			func(m *mocks.MockPP) {
				m.EXPECT().Warningf(pp.EmojiUserWarning, "%s=%s is deprecated; use %s=@once", key, "@nevermore", gomock.Any())
			},
		},
		"@once": {true, "\t\t@once", cron.MustNew("@yearly"), nil, true, nil},
		"illformed": {
			true, " @ddddd  ", cron.MustNew("*/4 * * * *"), cron.MustNew("*/4 * * * *"), false,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiUserError, "%s (%q) is not a cron expression: %v", key, "@ddddd", gomock.Any())
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			set(t, key, tc.set, tc.val)
			field := tc.oldField
			mockCtrl := gomock.NewController(t)
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
	require.NoError(t, err)
	return url
}
