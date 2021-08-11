package config_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/config"
	"github.com/favonia/cloudflare-ddns/internal/cron"
	"github.com/favonia/cloudflare-ddns/internal/detector"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

const keyPrefix = "TEST-11D39F6A9A97AFAFD87CCEB-"

func set(key string, val string) {
	if os.Getenv(key) != "" {
		panic(fmt.Sprintf("%s was already set", key))
	}

	os.Setenv(key, val)
}

func unset(key string) {
	os.Unsetenv(key)
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
			if tc.set {
				set(key, tc.val)
				defer unset(key)
			}
			require.Equal(t, tc.expected, config.Getenv(key))
		})
	}
}

//nolint:paralleltest // environment vars are global
func TestReadQuiet(t *testing.T) {
	key := keyPrefix + "QUIET"
	for name, tc := range map[string]struct {
		set       bool
		val       string
		oldField  pp.Level
		newField  pp.Level
		ok        bool
		ppRecords []pp.Record
	}{
		"nil1":   {false, "", pp.Verbose, pp.Verbose, true, nil},
		"nil2":   {false, "", pp.Quiet, pp.Quiet, true, nil},
		"empty1": {true, "  ", pp.Verbose, pp.Verbose, true, nil},
		"empty2": {true, " ", pp.Quiet, pp.Quiet, true, nil},
		"true1":  {true, "true   ", pp.Verbose, pp.Quiet, true, nil},
		"true2":  {true, " true", pp.Quiet, pp.Quiet, true, nil},
		"false1": {true, "    false ", pp.Verbose, pp.Verbose, true, nil},
		"false2": {true, " false    ", pp.Quiet, pp.Verbose, true, nil},
		"illform1": {
			true, "weird", pp.Verbose, pp.Verbose, false,
			[]pp.Record{
				pp.NewRecord(0, pp.Error, pp.EmojiUserError, `Failed to parse "weird": strconv.ParseBool: parsing "weird": invalid syntax`), //nolint:lll
			},
		},
		"illform2": {
			true, "weird", pp.Quiet, pp.Quiet, false,
			[]pp.Record{
				pp.NewRecord(0, pp.Error, pp.EmojiUserError, `Failed to parse "weird": strconv.ParseBool: parsing "weird": invalid syntax`), //nolint:lll
			},
		},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			if tc.set {
				set(key, tc.val)
				defer unset(key)
			}

			ppmock := pp.NewMock()
			ppmock.SetLevel(tc.oldField)
			ok := config.ReadQuiet(ppmock, key)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.newField, ppmock.Level)
			require.Equal(t, tc.ppRecords, ppmock.Records)
		})
	}
}

//nolint:funlen,paralleltest // environment vars are global
func TestReadBool(t *testing.T) {
	key := keyPrefix + "BOOL"
	for name, tc := range map[string]struct {
		set       bool
		val       string
		oldField  bool
		newField  bool
		ok        bool
		ppRecords []pp.Record
	}{
		"nil1": {
			false, "", true, true, true,
			[]pp.Record{
				pp.NewRecord(0, pp.Info, pp.EmojiBullet, `Use default TEST-11D39F6A9A97AFAFD87CCEB-BOOL=true`),
			},
		},
		"nil2": {
			false, "", false, false, true,
			[]pp.Record{
				pp.NewRecord(0, pp.Info, pp.EmojiBullet, `Use default TEST-11D39F6A9A97AFAFD87CCEB-BOOL=false`),
			},
		},
		"empty1": {
			true, " ", true, true, true,
			[]pp.Record{
				pp.NewRecord(0, pp.Info, pp.EmojiBullet, `Use default TEST-11D39F6A9A97AFAFD87CCEB-BOOL=true`),
			},
		},
		"empty2": {
			true, " \t ", false, false, true,
			[]pp.Record{
				pp.NewRecord(0, pp.Info, pp.EmojiBullet, `Use default TEST-11D39F6A9A97AFAFD87CCEB-BOOL=false`),
			},
		},
		"true1":  {true, "true ", true, true, true, nil},
		"true2":  {true, " \t true", false, true, true, nil},
		"false1": {true, "false ", true, false, true, nil},
		"false2": {true, " false", false, false, true, nil},
		"illform1": {
			true, "weird\t  ", false, false, false,
			[]pp.Record{
				pp.NewRecord(0, pp.Error, pp.EmojiUserError, `Failed to parse "weird": strconv.ParseBool: parsing "weird": invalid syntax`), //nolint:lll
			},
		},
		"illform2": {
			true, " weird", true, true, false,
			[]pp.Record{
				pp.NewRecord(0, pp.Error, pp.EmojiUserError, `Failed to parse "weird": strconv.ParseBool: parsing "weird": invalid syntax`), //nolint:lll
			},
		},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			if tc.set {
				set(key, tc.val)
				defer unset(key)
			}

			field := tc.oldField
			ppmock := pp.NewMock()
			ok := config.ReadBool(ppmock, key, &field)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.newField, field)
			require.Equal(t, tc.ppRecords, ppmock.Records)
		})
	}
}

//nolint:paralleltest // environment vars are global
func TestReadNonnegInt(t *testing.T) {
	key := keyPrefix + "INT"
	for name, tc := range map[string]struct {
		set       bool
		val       string
		oldField  int
		newField  int
		ok        bool
		ppRecords []pp.Record
	}{
		"nil": {
			false, "", 100, 100, true,
			[]pp.Record{
				pp.NewRecord(0, pp.Info, pp.EmojiBullet, `Use default TEST-11D39F6A9A97AFAFD87CCEB-INT=100`),
			},
		},
		"empty": {
			true, "", 100, 100, true,
			[]pp.Record{
				pp.NewRecord(0, pp.Info, pp.EmojiBullet, `Use default TEST-11D39F6A9A97AFAFD87CCEB-INT=100`),
			},
		},
		"zero": {true, "0   ", 100, 0, true, nil},
		"-1": {
			true, "   -1", 100, 100, false,
			[]pp.Record{
				pp.NewRecord(0, pp.Error, pp.EmojiUserError, `Failed to parse "-1": -1 is negative`),
			},
		},
		"1": {true, "   1   ", 100, 1, true, nil},
		"1.0": {
			true, "   1.0   ", 100, 100, false,
			[]pp.Record{
				pp.NewRecord(0, pp.Error, pp.EmojiUserError, `Failed to parse "1.0": strconv.Atoi: parsing "1.0": invalid syntax`), //nolint:lll
			},
		},
		"words": {
			true, "   words   ", 100, 100, false,
			[]pp.Record{
				pp.NewRecord(0, pp.Error, pp.EmojiUserError, `Failed to parse "words": strconv.Atoi: parsing "words": invalid syntax`), //nolint:lll
			},
		},
		"9999999": {true, "   9999999   ", 100, 9999999, true, nil},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			if tc.set {
				set(key, tc.val)
				defer unset(key)
			}

			field := tc.oldField
			ppmock := pp.NewMock()
			ok := config.ReadNonnegInt(ppmock, key, &field)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.newField, field)
			require.Equal(t, tc.ppRecords, ppmock.Records)
		})
	}
}

//nolint:paralleltest // environment vars are global
func TestReadDomains(t *testing.T) {
	key := keyPrefix + "DOMAINS"
	type ds = []api.FQDN
	for name, tc := range map[string]struct {
		set       bool
		val       string
		oldField  ds
		newField  ds
		ok        bool
		ppRecords []pp.Record
	}{
		"nil":   {false, "", ds{"test.org"}, ds{}, true, nil},
		"empty": {true, "", ds{"test.org"}, ds{}, true, nil},
		"test1": {true, "書.org ,  Bücher.org  ", ds{"random.org"}, ds{"xn--rov.org", "xn--bcher-kva.org"}, true, nil},
		"test2": {true, "  \txn--rov.org    ,   xn--Bcher-kva.org  ", ds{"random.org"}, ds{"xn--rov.org", "xn--bcher-kva.org"}, true, nil}, //nolint:lll
		"illformed": {
			true, "xn--:D.org",
			ds{"random.org"},
			ds{"xn--:d.org"},
			true,
			[]pp.Record{
				pp.NewRecord(0, pp.Error, pp.EmojiUserError, `Domain "xn--:d.org" was added but it is ill-formed: idna: disallowed rune U+003A`), //nolint:lll
			},
		},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			if tc.set {
				set(key, tc.val)
				defer unset(key)
			}

			field := tc.oldField
			ppmock := pp.NewMock()
			ok := config.ReadDomains(ppmock, key, &field)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.newField, field)
			require.Equal(t, tc.ppRecords, ppmock.Records)
		})
	}
}

//nolint:paralleltest // environment vars are global
func TestReadPolicy(t *testing.T) {
	key := keyPrefix + "POLICY"

	var (
		cloudflare = detector.NewCloudflare()
		local      = detector.NewLocal()
		unmanaged  = detector.NewUnmanaged()
		ipify      = detector.NewIpify()
	)

	for name, tc := range map[string]struct {
		set       bool
		val       string
		oldField  detector.Policy
		newField  detector.Policy
		ok        bool
		ppRecords []pp.Record
	}{
		"nil": {
			false, "", unmanaged, unmanaged, true,
			[]pp.Record{
				pp.NewRecord(0, pp.Info, pp.EmojiBullet, `Use default TEST-11D39F6A9A97AFAFD87CCEB-POLICY=unmanaged`),
			},
		},
		"empty": {
			true, "", local, local, true,
			[]pp.Record{
				pp.NewRecord(0, pp.Info, pp.EmojiBullet, `Use default TEST-11D39F6A9A97AFAFD87CCEB-POLICY=local`),
			},
		},
		"cloudflare": {true, "    cloudflare\t   ", unmanaged, cloudflare, true, nil},
		"unmanaged":  {true, "   unmanaged   ", cloudflare, unmanaged, true, nil},
		"local":      {true, "   local   ", cloudflare, local, true, nil},
		"ipify":      {true, "     ipify  ", cloudflare, ipify, true, nil},
		"others": {
			true, "   something-else ", ipify, ipify, false,
			[]pp.Record{
				pp.NewRecord(0, pp.Error, pp.EmojiUserError, `Failed to parse "something-else": not a valid policy`),
			},
		},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			if tc.set {
				set(key, tc.val)
				defer unset(key)
			}

			field := tc.oldField
			ppmock := pp.NewMock()
			ok := config.ReadPolicy(ppmock, key, &field)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.newField, field)
			require.Equal(t, tc.ppRecords, ppmock.Records)
		})
	}
}

//nolint:paralleltest // environment vars are global
func TestReadNonnegDuration(t *testing.T) {
	key := keyPrefix + "DURATION"

	for name, tc := range map[string]struct {
		set       bool
		val       string
		oldField  time.Duration
		newField  time.Duration
		ok        bool
		ppRecords []pp.Record
	}{
		"nil": {
			false, "", time.Second, time.Second, true,
			[]pp.Record{
				pp.NewRecord(0, pp.Info, pp.EmojiBullet, `Use default TEST-11D39F6A9A97AFAFD87CCEB-DURATION=1s`),
			},
		},
		"empty": {
			true, "", 0, 0, true,
			[]pp.Record{
				pp.NewRecord(0, pp.Info, pp.EmojiBullet, `Use default TEST-11D39F6A9A97AFAFD87CCEB-DURATION=0s`),
			},
		},
		"100s": {true, "    100s\t   ", 0, time.Second * 100, true, nil},
		"1": {
			true, "  1  ", 123, 123, false,
			[]pp.Record{
				pp.NewRecord(0, pp.Error, pp.EmojiUserError, `Failed to parse "1": time: missing unit in duration "1"`),
			},
		},
		"-1s": {
			true, "  -1s  ", 456, 456, false,
			[]pp.Record{
				pp.NewRecord(0, pp.Error, pp.EmojiUserError, `Failed to parse "-1s": -1s is negative`),
			},
		},
		"0h": {true, "  0h  ", 123456, 0, true, nil},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			if tc.set {
				set(key, tc.val)
				defer unset(key)
			}

			field := tc.oldField
			ppmock := pp.NewMock()
			ok := config.ReadNonnegDuration(ppmock, key, &field)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.newField, field)
			require.Equal(t, tc.ppRecords, ppmock.Records)
		})
	}
}

//nolint:paralleltest // environment vars are global
func TestReadCron(t *testing.T) {
	key := keyPrefix + "CRON"

	for name, tc := range map[string]struct {
		set       bool
		val       string
		oldField  cron.Schedule
		newField  cron.Schedule
		ok        bool
		ppRecords []pp.Record
	}{
		"nil": {
			false, "", cron.MustNew("* * * * *"), cron.MustNew("* * * * *"), true,
			[]pp.Record{
				pp.NewRecord(0, pp.Info, pp.EmojiBullet, `Use default TEST-11D39F6A9A97AFAFD87CCEB-CRON=* * * * *`),
			},
		},
		"empty": {
			true, "", cron.MustNew("@every 3m"), cron.MustNew("@every 3m"), true,
			[]pp.Record{
				pp.NewRecord(0, pp.Info, pp.EmojiBullet, `Use default TEST-11D39F6A9A97AFAFD87CCEB-CRON=@every 3m`),
			},
		},
		"@": {true, " @daily  ", cron.MustNew("@yearly"), cron.MustNew("@daily"), true, nil},
		"illformed": {
			true, " @ddddd  ", cron.MustNew("*/4 * * * *"), cron.MustNew("*/4 * * * *"), false,
			[]pp.Record{
				pp.NewRecord(0, pp.Error, pp.EmojiUserError, `Failed to parse "@ddddd": parsing "@ddddd": unrecognized descriptor: @ddddd`), //nolint:lll
			},
		},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			if tc.set {
				set(key, tc.val)
				defer unset(key)
			}

			field := tc.oldField
			ppmock := pp.NewMock()
			ok := config.ReadCron(ppmock, key, &field)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.newField, field)
			require.Equal(t, tc.ppRecords, ppmock.Records)
		})
	}
}
